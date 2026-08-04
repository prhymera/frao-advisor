package publish

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prhymera/frao-advisor/db"
)

func TestNewEmptyURLDisabled(t *testing.T) {
	if p := New(""); p != nil {
		t.Fatalf("New(\"\") should return nil, got %v", p)
	}
}

func TestNilPublisherNoop(t *testing.T) {
	var p *Publisher
	p.Publish(db.Event{ID: "x"}) // must not panic
	p.Flush()                    // must not panic
}

func TestPublishSuccess(t *testing.T) {
	var got atomic.Int32
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Add(1)
		if r.URL.Path != "/api/events" {
			t.Errorf("path = %q, want /api/events", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := New(srv.URL)
	p.Publish(db.Event{
		ID: "evt-1", Type: "consultation", SessionID: "sess-1", SessionLabel: "lab",
		Question: "q", Response: "r", Model: "deepseek-v4-pro",
		PromptTokens: 10, CompletionTokens: 5, InputCost: 0.1, OutputCost: 0.2, DurationMs: 99,
	})
	p.Flush()

	if got.Load() != 1 {
		t.Fatalf("requests = %d, want 1", got.Load())
	}
	var ev db.Event
	if err := json.Unmarshal(gotBody, &ev); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if ev.ID != "evt-1" || ev.Type != "consultation" || ev.SessionLabel != "lab" {
		t.Errorf("round-trip mismatch: %+v", ev)
	}
	if ev.PromptTokens != 10 || ev.DurationMs != 99 {
		t.Errorf("usage fields mismatch: %+v", ev)
	}
}

func TestPublishDeliberationContributions(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := New(srv.URL)
	p.Publish(db.Event{
		ID: "delib-1", Type: "deliberation", SessionID: "s-1",
		Context: "ctx", Synthesis: "syn", ExpertCount: 2, ExpertKeys: `["a","b"]`,
		Contributions: []db.Contribution{
			{ID: "c-1", ExpertKey: "architect", Analysis: "a", SortOrder: 0},
			{ID: "c-2", ExpertKey: "reviewer", Analysis: "b", SortOrder: 1},
		},
	})
	p.Flush()

	var ev db.Event
	if err := json.Unmarshal(gotBody, &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ev.Contributions) != 2 {
		t.Fatalf("contributions = %d, want 2", len(ev.Contributions))
	}
	if ev.Contributions[1].ExpertKey != "reviewer" {
		t.Errorf("contribution[1] expert = %q, want reviewer", ev.Contributions[1].ExpertKey)
	}
}

func TestPublishRetriesOn500(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	old := retryDelay
	retryDelay = time.Millisecond
	defer func() { retryDelay = old }()

	p := New(srv.URL)
	p.Publish(db.Event{ID: "evt-1", Type: "consultation", SessionID: "s", Question: "q", Response: "r"})
	p.Flush()

	if count.Load() != 2 {
		t.Fatalf("requests = %d, want 2 (initial + retry)", count.Load())
	}
}

func TestPublishDownServerNoBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	url := srv.URL
	srv.Close() // dashboard now down

	old := retryDelay
	retryDelay = time.Millisecond
	defer func() { retryDelay = old }()

	p := New(url)
	done := make(chan struct{})
	go func() {
		p.Publish(db.Event{ID: "evt-1", Type: "consultation", SessionID: "s", Question: "q", Response: "r"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked when dashboard is down")
	}
	p.Flush() // should complete without panic
}
