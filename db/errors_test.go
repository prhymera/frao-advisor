package db

import (
	"context"
	"testing"
)

// TestAdvisorErrorsTableExists verifies the schema migration creates the
// advisor_errors audit table.
func TestAdvisorErrorsTableExists(t *testing.T) {
	d := openTestDB(t)
	var n int
	if err := d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='advisor_errors'`).Scan(&n); err != nil {
		t.Fatalf("query schema: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected advisor_errors table, got %d", n)
	}
}

// TestInsertErrorRoundTrip verifies a failed-call record persists and reads back.
func TestInsertErrorRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := openTestDB(t)

	sess, err := d.EnsureSession(ctx, "test-session")
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	err = d.InsertError(ctx, InsertErrorParams{
		ID:          "err-1",
		SessionID:   sess,
		Tool:        "consult",
		Stage:       "call",
		Effort:      "low",
		Error:       "context deadline exceeded (Client.Timeout or context cancellation while reading body)",
		ContextSnip: "some question",
	})
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}
	if got := countRows(t, d, "advisor_errors"); got != 1 {
		t.Fatalf("expected 1 advisor_errors row, got %d", got)
	}

	var tool, stage, effort, errMsg, snip string
	if err := d.QueryRowContext(ctx,
		`SELECT tool, stage, effort, error, context_snip FROM advisor_errors WHERE id='err-1'`,
	).Scan(&tool, &stage, &effort, &errMsg, &snip); err != nil {
		t.Fatalf("read error row: %v", err)
	}
	if tool != "consult" || stage != "call" || effort != "low" || errMsg == "" || snip != "some question" {
		t.Fatalf("unexpected row: tool=%q stage=%q effort=%q err=%q snip=%q", tool, stage, effort, errMsg, snip)
	}
}
