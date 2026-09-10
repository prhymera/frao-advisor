package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/prhymera/frao-advisor/db"
)

// TestCaptureErrorPersists verifies a failed tool call lands in advisor_errors
// end-to-end through Persistence (the path consult/expert-review failures take).
func TestCaptureErrorPersists(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	p := &Persistence{database: database}
	currentSessionID = p.EnsureSession("test-session")

	p.CaptureError("consult", "call", "low", "boom: context deadline exceeded", "some question")

	var n int
	if err := database.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM advisor_errors WHERE tool='consult' AND error LIKE '%boom%'`).Scan(&n); err != nil {
		t.Fatalf("query advisor_errors: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 advisor_errors row, got %d", n)
	}
}

// TestCaptureErrorNilSafe verifies a nil receiver is a no-op, not a panic.
func TestCaptureErrorNilSafe(t *testing.T) {
	var p *Persistence
	p.CaptureError("consult", "call", "low", "boom", "q") // must not panic
}

// TestCaptureRecordsRequestedEffort verifies the effort stored alongside a
// result is the effort actually requested. Both capture paths hardcoded
// "high", so every low/medium/xhigh call was mislabelled in the dashboard.
func TestCaptureRecordsRequestedEffort(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	p := &Persistence{database: database}
	session := p.EnsureSession("effort-session")

	result := &ChatResult{
		Text:             "answer",
		Model:            "deepseek-flash",
		PromptTokens:     10,
		CompletionTokens: 5,
		DurationMs:       123,
	}

	p.CaptureConsult(session, "q", result, "low")
	p.CaptureExpertReview(session, "code-reviewer", "ctx", result, "xhigh")

	var consultEffort string
	if err := database.QueryRowContext(context.Background(),
		`SELECT reasoning_effort FROM consultations LIMIT 1`).Scan(&consultEffort); err != nil {
		t.Fatalf("query consultations: %v", err)
	}
	if consultEffort != "low" {
		t.Errorf("consult reasoning_effort = %q, want %q", consultEffort, "low")
	}

	var reviewEffort string
	if err := database.QueryRowContext(context.Background(),
		`SELECT reasoning_effort FROM expert_reviews LIMIT 1`).Scan(&reviewEffort); err != nil {
		t.Fatalf("query expert_reviews: %v", err)
	}
	if reviewEffort != "xhigh" {
		t.Errorf("expert_review reasoning_effort = %q, want %q", reviewEffort, "xhigh")
	}
}
