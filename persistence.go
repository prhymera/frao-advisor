package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"

	"github.com/prhymera/frao-advisor/cost"
	"github.com/prhymera/frao-advisor/db"
)

// Background context for database operations.
var ctx = context.Background()

// Persistence bridges MCP tool handlers to the database.
// If the database is unavailable, all methods are safe no-ops.
type Persistence struct {
	database *db.DB
}

// CaptureConsult persists a frao-consult result.
func (p *Persistence) CaptureConsult(sessionID string, question string, result *ChatResult) {
	if p == nil || p.database == nil {
		return
	}
	inputCost, outputCost, _ := cost.CalculateCost(result.Model, result.PromptTokens, result.CompletionTokens, false)

	err := p.database.InsertConsultation(ctx, db.InsertConsultationParams{
		ID:               uuidV4(),
		SessionID:        sessionID,
		Question:         question,
		Response:         result.Text,
		Model:            result.Model,
		ReasoningEffort:  "high", // TODO: pass actual effort from args
		PromptTokens:     result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
		InputCost:        inputCost,
		OutputCost:       outputCost,
		InputPriceUsed:   cost.DefaultPrices[result.Model].InputPricePerM,
		OutputPriceUsed:  cost.DefaultPrices[result.Model].OutputPricePerM,
		CacheHit:         false,
		DurationMs:       result.DurationMs,
	})
	if err != nil {
		log.Printf("persist consultation: %v", err)
	}
}

// CaptureExpertReview persists a frao-expert-review result.
func (p *Persistence) CaptureExpertReview(sessionID, expertKey, context string, result *ChatResult) {
	if p == nil || p.database == nil {
		return
	}
	inputCost, outputCost, _ := cost.CalculateCost(result.Model, result.PromptTokens, result.CompletionTokens, false)

	err := p.database.InsertExpertReview(ctx, db.InsertExpertReviewParams{
		ID:               uuidV4(),
		SessionID:        sessionID,
		ExpertKey:        expertKey,
		Context:          context,
		Analysis:         result.Text,
		Model:            result.Model,
		ReasoningEffort:  "high",
		PromptTokens:     result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
		InputCost:        inputCost,
		OutputCost:       outputCost,
		InputPriceUsed:   cost.DefaultPrices[result.Model].InputPricePerM,
		OutputPriceUsed:  cost.DefaultPrices[result.Model].OutputPricePerM,
		CacheHit:         false,
		DurationMs:       result.DurationMs,
	})
	if err != nil {
		log.Printf("persist expert review: %v", err)
	}
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

	// Persist each contribution
	for i, c := range contributions {
		iCost, oCost, _ := cost.CalculateCost(c.Result.Model, c.Result.PromptTokens, c.Result.CompletionTokens, false)
		totalPrompt += c.Result.PromptTokens
		totalCompletion += c.Result.CompletionTokens
		totalInputCost += iCost
		totalOutputCost += oCost
		totalDuration += c.Result.DurationMs

		err := p.database.InsertDeliberationContribution(ctx, db.InsertContributionParams{
			ID:               uuidV4(),
			DeliberationID:   deliberationID,
			ExpertKey:        c.ExpertKey,
			Analysis:         c.Result.Text,
			PromptTokens:     c.Result.PromptTokens,
			CompletionTokens: c.Result.CompletionTokens,
			InputCost:        iCost,
			OutputCost:       oCost,
			DurationMs:       c.Result.DurationMs,
			SortOrder:        i,
		})
		if err != nil {
			log.Printf("persist deliberation contribution: %v", err)
		}
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

	err := p.database.InsertDeliberation(ctx, db.InsertDeliberationParams{
		ID:                  deliberationID,
		SessionID:           sessionID,
		Context:             context,
		Synthesis:           synthesis,
		ExpertCount:         len(expertKeys),
		ExpertKeys:          expertKeysJSON,
		Model:               synthesisResult.Model,
		ReasoningEffort:     reasoningEffort,
		TotalPromptTokens:   totalPrompt,
		TotalCompletionTokens: totalCompletion,
		TotalInputCost:      totalInputCost,
		TotalOutputCost:     totalOutputCost,
		DurationMs:          totalDuration,
	})
	if err != nil {
		log.Printf("persist deliberation: %v", err)
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

