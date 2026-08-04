package db

import (
	"context"
	"errors"
	"fmt"
)

// Event is the wire-format payload published by MCP processes to the dashboard
// and consumed by the dashboard's ingest endpoint. It carries every field the
// dashboard needs to write into the consultations / expert_reviews /
// deliberations (+ contributions) tables, idempotently keyed by record ID.
type Event struct {
	ID           string `json:"id"`
	Type         string `json:"type"` // "consultation" | "expert_review" | "deliberation"
	SessionID    string `json:"session_id"`
	SessionLabel string `json:"session_label,omitempty"`

	// consultation
	Question string `json:"question,omitempty"`
	Response string `json:"response,omitempty"`

	// expert_review
	ExpertKey string `json:"expert_key,omitempty"`
	Context   string `json:"context,omitempty"`
	Analysis  string `json:"analysis,omitempty"`

	// deliberation
	Synthesis   string `json:"synthesis,omitempty"`
	ExpertCount int    `json:"expert_count,omitempty"`
	ExpertKeys  string `json:"expert_keys,omitempty"` // JSON array string

	// single-call usage (consultation / expert_review)
	Model            string  `json:"model,omitempty"`
	ReasoningEffort  string  `json:"reasoning_effort,omitempty"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	InputCost        float64 `json:"input_cost,omitempty"`
	OutputCost       float64 `json:"output_cost,omitempty"`
	InputPriceUsed   float64 `json:"input_price_used,omitempty"`
	OutputPriceUsed  float64 `json:"output_price_used,omitempty"`
	CacheHit         bool    `json:"cache_hit,omitempty"`
	DurationMs       int64   `json:"duration_ms,omitempty"`

	// deliberation totals
	TotalPromptTokens     int     `json:"total_prompt_tokens,omitempty"`
	TotalCompletionTokens int     `json:"total_completion_tokens,omitempty"`
	TotalInputCost        float64 `json:"total_input_cost,omitempty"`
	TotalOutputCost       float64 `json:"total_output_cost,omitempty"`

	Contributions []Contribution `json:"contributions,omitempty"`
}

// Contribution is one expert's input to a deliberation.
type Contribution struct {
	ID               string  `json:"id"`
	ExpertKey        string  `json:"expert_key"`
	Analysis         string  `json:"analysis"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	InputCost        float64 `json:"input_cost"`
	OutputCost       float64 `json:"output_cost"`
	DurationMs       int64   `json:"duration_ms"`
	SortOrder        int     `json:"sort_order"`
}

// Validate checks that every NOT NULL column is present. This matters because
// the inserts use INSERT OR IGNORE, which swallows constraint violations —
// without full validation a malformed payload would return 200 and silently
// drop the row.
func (e Event) Validate() error {
	if e.ID == "" {
		return errors.New("id is required")
	}
	if e.SessionID == "" {
		return errors.New("session_id is required")
	}
	switch e.Type {
	case "consultation":
		if e.Question == "" {
			return errors.New("question is required")
		}
		if e.Response == "" {
			return errors.New("response is required")
		}
	case "expert_review":
		if e.ExpertKey == "" {
			return errors.New("expert_key is required")
		}
		if e.Context == "" {
			return errors.New("context is required")
		}
		if e.Analysis == "" {
			return errors.New("analysis is required")
		}
	case "deliberation":
		if e.Context == "" {
			return errors.New("context is required")
		}
		if e.Synthesis == "" {
			return errors.New("synthesis is required")
		}
		if e.ExpertCount <= 0 {
			return errors.New("expert_count is required")
		}
		if e.ExpertKeys == "" {
			return errors.New("expert_keys is required")
		}
	default:
		return fmt.Errorf("unknown type %q", e.Type)
	}
	return nil
}

// UpsertSessionByID registers (or refreshes) a session keyed by the MCP's
// internal session UUID — the value used as the FK on every record — so the
// dashboard can attribute events to a human-readable label.
func (d *DB) UpsertSessionByID(ctx context.Context, id, label string) error {
	if label == "" {
		label = "unknown"
	}
	_, err := d.ExecContext(ctx, `
		INSERT INTO sessions (id, session_id, call_count)
		VALUES (?, ?, 1)
		ON CONFLICT(id) DO UPDATE SET
			session_id  = excluded.session_id,
			last_active = datetime('now'),
			call_count  = call_count + 1`, id, label)
	return err
}

// IngestEvent persists a published event. Deliberations insert the
// deliberations row BEFORE their contributions (foreign key order).
func (d *DB) IngestEvent(ctx context.Context, ev Event) error {
	if err := d.UpsertSessionByID(ctx, ev.SessionID, ev.SessionLabel); err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}
	switch ev.Type {
	case "consultation":
		return d.InsertConsultation(ctx, InsertConsultationParams{
			ID: ev.ID, SessionID: ev.SessionID, Question: ev.Question, Response: ev.Response,
			Model: ev.Model, ReasoningEffort: ev.ReasoningEffort,
			PromptTokens: ev.PromptTokens, CompletionTokens: ev.CompletionTokens,
			InputCost: ev.InputCost, OutputCost: ev.OutputCost,
			InputPriceUsed: ev.InputPriceUsed, OutputPriceUsed: ev.OutputPriceUsed,
			CacheHit: ev.CacheHit, DurationMs: ev.DurationMs,
		})
	case "expert_review":
		return d.InsertExpertReview(ctx, InsertExpertReviewParams{
			ID: ev.ID, SessionID: ev.SessionID, ExpertKey: ev.ExpertKey,
			Context: ev.Context, Analysis: ev.Analysis,
			Model: ev.Model, ReasoningEffort: ev.ReasoningEffort,
			PromptTokens: ev.PromptTokens, CompletionTokens: ev.CompletionTokens,
			InputCost: ev.InputCost, OutputCost: ev.OutputCost,
			InputPriceUsed: ev.InputPriceUsed, OutputPriceUsed: ev.OutputPriceUsed,
			CacheHit: ev.CacheHit, DurationMs: ev.DurationMs,
		})
	case "deliberation":
		if err := d.InsertDeliberation(ctx, InsertDeliberationParams{
			ID: ev.ID, SessionID: ev.SessionID, Context: ev.Context, Synthesis: ev.Synthesis,
			ExpertCount: ev.ExpertCount, ExpertKeys: ev.ExpertKeys,
			Model: ev.Model, ReasoningEffort: ev.ReasoningEffort,
			TotalPromptTokens: ev.TotalPromptTokens, TotalCompletionTokens: ev.TotalCompletionTokens,
			TotalInputCost: ev.TotalInputCost, TotalOutputCost: ev.TotalOutputCost,
			DurationMs: ev.DurationMs,
		}); err != nil {
			return err
		}
		for _, c := range ev.Contributions {
			if err := d.InsertDeliberationContribution(ctx, InsertContributionParams{
				ID: c.ID, DeliberationID: ev.ID, ExpertKey: c.ExpertKey, Analysis: c.Analysis,
				PromptTokens: c.PromptTokens, CompletionTokens: c.CompletionTokens,
				InputCost: c.InputCost, OutputCost: c.OutputCost,
				DurationMs: c.DurationMs, SortOrder: c.SortOrder,
			}); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown type %q", ev.Type)
	}
}
