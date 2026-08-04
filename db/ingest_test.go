package db

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func countRows(t *testing.T, d *DB, table string) int {
	t.Helper()
	var n int
	if err := d.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestUpsertSessionByID(t *testing.T) {
	ctx := context.Background()
	d := openTestDB(t)

	// Create
	if err := d.UpsertSessionByID(ctx, "sess-1", "label-a"); err != nil {
		t.Fatalf("upsert create: %v", err)
	}
	var label string
	if err := d.QueryRowContext(ctx, "SELECT session_id FROM sessions WHERE id='sess-1'").Scan(&label); err != nil {
		t.Fatalf("select session: %v", err)
	}
	if label != "label-a" {
		t.Errorf("session_id = %q, want label-a", label)
	}

	// Update label + bump call_count
	if err := d.UpsertSessionByID(ctx, "sess-1", "label-b"); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	if err := d.QueryRowContext(ctx, "SELECT session_id FROM sessions WHERE id='sess-1'").Scan(&label); err != nil {
		t.Fatalf("select session 2: %v", err)
	}
	if label != "label-b" {
		t.Errorf("session_id = %q, want label-b", label)
	}
	var cc int
	if err := d.QueryRowContext(ctx, "SELECT call_count FROM sessions WHERE id='sess-1'").Scan(&cc); err != nil {
		t.Fatalf("select call_count: %v", err)
	}
	if cc != 2 {
		t.Errorf("call_count = %d, want 2", cc)
	}
}

func TestIngestConsultation(t *testing.T) {
	ctx := context.Background()
	d := openTestDB(t)
	ev := Event{ID: "c-1", Type: "consultation", SessionID: "s-1", SessionLabel: "lab",
		Question: "q", Response: "r", Model: "deepseek-v4-pro", PromptTokens: 10, DurationMs: 5}
	if err := d.IngestEvent(ctx, ev); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if n := countRows(t, d, "consultations"); n != 1 {
		t.Errorf("consultations = %d, want 1", n)
	}
	var label string
	if err := d.QueryRowContext(ctx, "SELECT session_id FROM sessions WHERE id='s-1'").Scan(&label); err != nil {
		t.Fatalf("select session: %v", err)
	}
	if label != "lab" {
		t.Errorf("session label = %q, want lab", label)
	}
}

func TestIngestExpertReview(t *testing.T) {
	ctx := context.Background()
	d := openTestDB(t)
	ev := Event{ID: "r-1", Type: "expert_review", SessionID: "s-1", ExpertKey: "architect",
		Context: "ctx", Analysis: "ana"}
	if err := d.IngestEvent(ctx, ev); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if n := countRows(t, d, "expert_reviews"); n != 1 {
		t.Errorf("expert_reviews = %d, want 1", n)
	}
}

func TestIngestDeliberationWithContributions(t *testing.T) {
	ctx := context.Background()
	d := openTestDB(t)
	ev := Event{ID: "delib-1", Type: "deliberation", SessionID: "s-1",
		Context: "ctx", Synthesis: "syn", ExpertCount: 2, ExpertKeys: `["architect","reviewer"]`,
		Contributions: []Contribution{
			{ID: "con-1", ExpertKey: "architect", Analysis: "a"},
			{ID: "con-2", ExpertKey: "reviewer", Analysis: "b"},
		}}
	if err := d.IngestEvent(ctx, ev); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if n := countRows(t, d, "deliberations"); n != 1 {
		t.Errorf("deliberations = %d, want 1", n)
	}
	// FK-order fix: contributions must land because the parent row exists first.
	if n := countRows(t, d, "deliberation_contributions"); n != 2 {
		t.Errorf("deliberation_contributions = %d, want 2 (FK order bug)", n)
	}
}

func TestIngestIdempotent(t *testing.T) {
	ctx := context.Background()
	d := openTestDB(t)
	ev := Event{ID: "c-1", Type: "consultation", SessionID: "s-1", Question: "q", Response: "r"}
	if err := d.IngestEvent(ctx, ev); err != nil {
		t.Fatalf("ingest 1: %v", err)
	}
	if err := d.IngestEvent(ctx, ev); err != nil {
		t.Fatalf("ingest 2: %v", err)
	}
	if n := countRows(t, d, "consultations"); n != 1 {
		t.Errorf("consultations = %d, want 1 (idempotent)", n)
	}
}

func TestIngestUnknownType(t *testing.T) {
	ctx := context.Background()
	d := openTestDB(t)
	ev := Event{ID: "x", Type: "bogus", SessionID: "s-1"}
	if err := d.IngestEvent(ctx, ev); err == nil {
		t.Error("expected error for unknown type")
	}
}
