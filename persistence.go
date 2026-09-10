package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"github.com/prhymera/frao-advisor/cost"
	"github.com/prhymera/frao-advisor/db"
	"github.com/prhymera/frao-advisor/publish"
)

// Background context for database operations.
var ctx = context.Background()

// Persistence bridges MCP tool handlers to the database.
// If the database is unavailable, all methods are safe no-ops.
type Persistence struct {
	database     *db.DB
	publisher    *publish.Publisher
	sessionLabel string
}

// usedPrices returns the effective per-model rates for the moment of the call,
// so the price columns recorded next to a cost match the cost itself
// (alias-resolved and peak-adjusted).
func usedPrices(model string) cost.ModelPrices {
	return cost.PricesFor(model, time.Now())
}

// CaptureConsult persists a frao-consult result.
func (p *Persistence) CaptureConsult(sessionID string, question string, result *ChatResult, effort string) {
	if p == nil || p.database == nil {
		return
	}
	inputCost, outputCost, _ := cost.CalculateCost(result.Model, result.PromptTokens, result.CompletionTokens, false)

	id := uuidV4()
	err := p.database.InsertConsultation(ctx, db.InsertConsultationParams{
		ID:               id,
		SessionID:        sessionID,
		Question:         question,
		Response:         result.Text,
		Model:            result.Model,
		ReasoningEffort:  effort,
		PromptTokens:     result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
		InputCost:        inputCost,
		OutputCost:       outputCost,
		InputPriceUsed:   usedPrices(result.Model).InputPricePerM,
		OutputPriceUsed:  usedPrices(result.Model).OutputPricePerM,
		CacheHit:         false,
		DurationMs:       result.DurationMs,
	})
	if err != nil {
		log.Printf("persist consultation: %v", err)
	}

	p.publisher.Publish(db.Event{
		ID:               id,
		Type:             "consultation",
		SessionID:        sessionID,
		SessionLabel:     p.sessionLabel,
		Question:         question,
		Response:         result.Text,
		Model:            result.Model,
		ReasoningEffort:  effort,
		PromptTokens:     result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
		InputCost:        inputCost,
		OutputCost:       outputCost,
		InputPriceUsed:   usedPrices(result.Model).InputPricePerM,
		OutputPriceUsed:  usedPrices(result.Model).OutputPricePerM,
		CacheHit:         false,
		DurationMs:       result.DurationMs,
	})
}

// CaptureExpertReview persists a frao-expert-review result.
func (p *Persistence) CaptureExpertReview(sessionID, expertKey, context string, result *ChatResult, effort string) {
	if p == nil || p.database == nil {
		return
	}
	inputCost, outputCost, _ := cost.CalculateCost(result.Model, result.PromptTokens, result.CompletionTokens, false)

	id := uuidV4()
	err := p.database.InsertExpertReview(ctx, db.InsertExpertReviewParams{
		ID:               id,
		SessionID:        sessionID,
		ExpertKey:        expertKey,
		Context:          context,
		Analysis:         result.Text,
		Model:            result.Model,
		ReasoningEffort:  effort,
		PromptTokens:     result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
		InputCost:        inputCost,
		OutputCost:       outputCost,
		InputPriceUsed:   usedPrices(result.Model).InputPricePerM,
		OutputPriceUsed:  usedPrices(result.Model).OutputPricePerM,
		CacheHit:         false,
		DurationMs:       result.DurationMs,
	})
	if err != nil {
		log.Printf("persist expert review: %v", err)
	}

	p.publisher.Publish(db.Event{
		ID:               id,
		Type:             "expert_review",
		SessionID:        sessionID,
		SessionLabel:     p.sessionLabel,
		ExpertKey:        expertKey,
		Context:          context,
		Analysis:         result.Text,
		Model:            result.Model,
		ReasoningEffort:  effort,
		PromptTokens:     result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
		InputCost:        inputCost,
		OutputCost:       outputCost,
		InputPriceUsed:   usedPrices(result.Model).InputPricePerM,
		OutputPriceUsed:  usedPrices(result.Model).OutputPricePerM,
		CacheHit:         false,
		DurationMs:       result.DurationMs,
	})
}

// CaptureDeliberation persists a frao-multi-perspective result with contributions.
type ContributionResult struct {
	ExpertKey string
	Result    *ChatResult
}

func (p *Persistence) CaptureDeliberation(sessionID, context, synthesis string, expertKeys []string,
	reasoningEffort string, contributions []ContributionResult, synthesisResult *ChatResult) {
	if p == nil || p.database == nil {
		return
	}

	deliberationID := uuidV4()
	var totalPrompt, totalCompletion int
	var totalInputCost, totalOutputCost float64
	var totalDuration int64

	// Pass 1: sum contribution + synthesis usage and build contribution events.
	// Contribution IDs are fixed here so the local insert and the published
	// event reference the same records (idempotent ingest).
	contributionEvents := make([]db.Contribution, 0, len(contributions))
	for i, c := range contributions {
		iCost, oCost, _ := cost.CalculateCost(c.Result.Model, c.Result.PromptTokens, c.Result.CompletionTokens, false)
		totalPrompt += c.Result.PromptTokens
		totalCompletion += c.Result.CompletionTokens
		totalInputCost += iCost
		totalOutputCost += oCost
		totalDuration += c.Result.DurationMs

		contributionEvents = append(contributionEvents, db.Contribution{
			ID:               uuidV4(),
			ExpertKey:        c.ExpertKey,
			Analysis:         c.Result.Text,
			PromptTokens:     c.Result.PromptTokens,
			CompletionTokens: c.Result.CompletionTokens,
			InputCost:        iCost,
			OutputCost:       oCost,
			DurationMs:       c.Result.DurationMs,
			SortOrder:        i,
		})
	}

	// Add synthesis cost
	if synthesisResult != nil {
		synCost, synOutCost, _ := cost.CalculateCost(synthesisResult.Model, synthesisResult.PromptTokens, synthesisResult.CompletionTokens, false)
		totalInputCost += synCost
		totalOutputCost += synOutCost
		totalDuration += synthesisResult.DurationMs
	}

	expertKeysJSON := `["` + expertKeys[0] + `"]` // simplified — full JSON in production
	for i := 1; i < len(expertKeys); i++ {
		expertKeysJSON = expertKeysJSON[:len(expertKeysJSON)-1] + `, "` + expertKeys[i] + `"]`
	}

	// Deliberation row FIRST so contribution FK inserts resolve.
	err := p.database.InsertDeliberation(ctx, db.InsertDeliberationParams{
		ID:                    deliberationID,
		SessionID:             sessionID,
		Context:               context,
		Synthesis:             synthesis,
		ExpertCount:           len(expertKeys),
		ExpertKeys:            expertKeysJSON,
		Model:                 synthesisResult.Model,
		ReasoningEffort:       reasoningEffort,
		TotalPromptTokens:     totalPrompt,
		TotalCompletionTokens: totalCompletion,
		TotalInputCost:        totalInputCost,
		TotalOutputCost:       totalOutputCost,
		DurationMs:            totalDuration,
	})
	if err != nil {
		log.Printf("persist deliberation: %v", err)
	}

	// Pass 2: persist contributions (parent row now exists).
	for i, ev := range contributionEvents {
		err := p.database.InsertDeliberationContribution(ctx, db.InsertContributionParams{
			ID:               ev.ID,
			DeliberationID:   deliberationID,
			ExpertKey:        ev.ExpertKey,
			Analysis:         ev.Analysis,
			PromptTokens:     ev.PromptTokens,
			CompletionTokens: ev.CompletionTokens,
			InputCost:        ev.InputCost,
			OutputCost:       ev.OutputCost,
			DurationMs:       ev.DurationMs,
			SortOrder:        i,
		})
		if err != nil {
			log.Printf("persist deliberation contribution: %v", err)
		}
	}

	p.publisher.Publish(db.Event{
		ID:                    deliberationID,
		Type:                  "deliberation",
		SessionID:             sessionID,
		SessionLabel:          p.sessionLabel,
		Context:               context,
		Synthesis:             synthesis,
		ExpertCount:           len(expertKeys),
		ExpertKeys:            expertKeysJSON,
		Model:                 synthesisResult.Model,
		ReasoningEffort:       reasoningEffort,
		TotalPromptTokens:     totalPrompt,
		TotalCompletionTokens: totalCompletion,
		TotalInputCost:        totalInputCost,
		TotalOutputCost:       totalOutputCost,
		DurationMs:            totalDuration,
		Contributions:         contributionEvents,
	})
}

// CaptureError persists a failed tool call so missed calls are countable.
// Consult and expert-review failures previously returned to the caller with no
// durable record; multi-perspective only leaked partial failures into
// contribution text. This is the audit trail.
func (p *Persistence) CaptureError(tool, stage, effort, errMsg, contextSnip string) {
	if p == nil || p.database == nil {
		return
	}
	if len(contextSnip) > 200 {
		contextSnip = contextSnip[:200]
	}
	err := p.database.InsertError(ctx, db.InsertErrorParams{
		ID:          uuidV4(),
		SessionID:   currentSessionID,
		Tool:        tool,
		Stage:       stage,
		Effort:      effort,
		Error:       errMsg,
		ContextSnip: contextSnip,
	})
	if err != nil {
		log.Printf("persist error: %v", err)
	}
}

// EnsureSession looks up or creates a session record.
func (p *Persistence) EnsureSession(sessionID string) string {
	if p == nil || p.database == nil {
		return sessionID
	}
	id, err := p.database.EnsureSession(ctx, sessionID)
	if err != nil {
		log.Printf("ensure session: %v", err)
		return sessionID
	}
	return id
}

// uuidV4 generates a version 4 UUID using crypto/rand.
func uuidV4() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
