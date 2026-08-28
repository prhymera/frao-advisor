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
