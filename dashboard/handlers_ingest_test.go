package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prhymera/frao-advisor/db"
)

func newTestHandlers(t *testing.T) *Handlers {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return &Handlers{DB: d}
}

func postEvent(t *testing.T, h *Handlers, ev db.Event) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(ev)
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.IngestEvent(rr, req)
	return rr
}

func TestHealth(t *testing.T) {
	h := newTestHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	h.Health(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %q, want ok status", rr.Body.String())
	}
}

func TestIngestHTTPValidTypes(t *testing.T) {
	h := newTestHandlers(t)
	events := []db.Event{
		{ID: "c-1", Type: "consultation", SessionID: "s-1", Question: "q", Response: "r"},
		{ID: "r-1", Type: "expert_review", SessionID: "s-1", ExpertKey: "architect", Context: "ctx", Analysis: "a"},
		{ID: "d-1", Type: "deliberation", SessionID: "s-1", Context: "ctx", Synthesis: "syn", ExpertCount: 1, ExpertKeys: `["architect"]`},
	}
	for _, ev := range events {
		rr := postEvent(t, h, ev)
		if rr.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200 (body %s)", ev.Type, rr.Code, rr.Body.String())
		}
	}
}

func TestIngestHTTPInvalidType(t *testing.T) {
	h := newTestHandlers(t)
	rr := postEvent(t, h, db.Event{ID: "x", Type: "bogus", SessionID: "s-1"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestIngestHTTPMissingField(t *testing.T) {
	h := newTestHandlers(t)
	// consultation missing response
	rr := postEvent(t, h, db.Event{ID: "c-1", Type: "consultation", SessionID: "s-1", Question: "q"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestIngestHTTPBadJSON(t *testing.T) {
	h := newTestHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader([]byte("{not json")))
	rr := httptest.NewRecorder()
	h.IngestEvent(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestIngestHTTPOversizedBody(t *testing.T) {
	h := newTestHandlers(t)
	// Body larger than the 1 MiB ingest limit.
	big := `{"id":"c-1","type":"consultation","session_id":"s-1","question":"` + strings.Repeat("a", 2<<20) + `","response":"r"}`
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader([]byte(big)))
	rr := httptest.NewRecorder()
	h.IngestEvent(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestIngestHTTPIdempotent(t *testing.T) {
	h := newTestHandlers(t)
	ev := db.Event{ID: "c-1", Type: "consultation", SessionID: "s-1", Question: "q", Response: "r"}
	for i := 0; i < 2; i++ {
		rr := postEvent(t, h, ev)
		if rr.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d", i+1, rr.Code)
		}
	}
	var n int
	if err := h.DB.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM consultations").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("consultations = %d, want 1 (idempotent)", n)
	}
}
