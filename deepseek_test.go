package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func advisorOKBody() string {
	return `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
}

// TestDoChatRetriesOn429 verifies a transient 429 is retried and succeeds.
func TestDoChatRetriesOn429(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			http.Error(w, `{"error":{"message":"rate limited"}}`, http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, advisorOKBody())
	}))
	defer srv.Close()

	c := NewDeepSeekClient("sk-test", srv.URL, "deepseek-v4-pro")
	res, err := c.ChatWithEffort("sys", "user", 0.1, "low")
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if res.Text != "ok" {
		t.Fatalf("unexpected text %q", res.Text)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("expected 2 calls (1 retry), got %d", calls)
	}
}

// TestDoChatNoRetryOn400 verifies permanent 4xx errors are NOT retried.
func TestDoChatNoRetryOn400(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		http.Error(w, `{"error":{"message":"bad request"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewDeepSeekClient("sk-test", srv.URL, "deepseek-v4-pro")
	_, err := c.ChatWithEffort("sys", "user", 0.1, "low")
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !strings.Contains(err.Error(), "API error 400") {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("expected no retry on 400, got %d calls", calls)
	}
}

// TestIsTransientError pins the transient/permanent classification, including
// that client timeouts are deliberately NOT retried.
func TestIsTransientError(t *testing.T) {
	cases := []struct {
		err    error
		status int
		want   bool
	}{
		{nil, 429, true},
		{nil, 500, true},
		{nil, 502, true},
		{nil, 400, false},
		{nil, 402, false},
		{nil, 200, false},
		{errors.New("read: connection reset by peer"), 0, true},
		{errors.New("read: connection refused"), 0, true},
		{errors.New("EOF"), 0, true},
		{errors.New("broken pipe"), 0, true},
		{errors.New("context deadline exceeded (Client.Timeout or context cancellation while reading body)"), 0, false},
		{errors.New("API error 400: bad"), 0, false},
	}
	for _, c := range cases {
		if got := isTransientError(c.err, c.status); got != c.want {
			t.Errorf("isTransientError(%v, %d) = %v, want %v", c.err, c.status, got, c.want)
		}
	}
}

func TestRetryBackoffPositive(t *testing.T) {
	for _, n := range []int{0, 1, 2, 5} {
		if retryBackoff(n) <= 0 {
			t.Fatalf("retryBackoff(%d) should be positive", n)
		}
	}
}

// TestDoChatRetriesTruncatedBody verifies that a 200 carrying an empty body —
// the signature of the upstream gateway cutting a long-running request — is
// retried rather than treated as a permanent failure. This was the advisor's
// single most common failure mode (34 of 38 recorded errors).
func TestDoChatRetriesTruncatedBody(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			// A clean 200 with no body: reads without error, parses as nothing.
			w.WriteHeader(http.StatusOK)
			return
		}
		fmt.Fprint(w, advisorOKBody())
	}))
	defer srv.Close()

	c := NewDeepSeekClient("sk-test", srv.URL, "deepseek-flash")
	res, err := c.ChatWithEffort("sys", "user", 0.1, "low")
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if res.Text != "ok" {
		t.Fatalf("unexpected text %q", res.Text)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("expected 2 calls (1 retry), got %d", calls)
	}
}

// TestDoChatRetriesTruncatedJSON covers the other shape of the same upstream
// fault: a body cut mid-JSON rather than emptied outright.
func TestDoChatRetriesTruncatedJSON(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"hal`)
			return
		}
		fmt.Fprint(w, advisorOKBody())
	}))
	defer srv.Close()

	c := NewDeepSeekClient("sk-test", srv.URL, "deepseek-flash")
	res, err := c.ChatWithEffort("sys", "user", 0.1, "low")
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if res.Text != "ok" {
		t.Fatalf("unexpected text %q", res.Text)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("expected 2 calls (1 retry), got %d", calls)
	}
}

// TestTruncatedBodyIsTransient pins that a cut 200 body is retryable while a
// client timeout is deliberately not.
func TestTruncatedBodyIsTransient(t *testing.T) {
	if !isTransientError(fmt.Errorf("%w (0 bytes)", errTruncatedBody), 200) {
		t.Error("truncated 200 body should be transient")
	}
	if isTransientError(errors.New("context deadline exceeded (Client.Timeout exceeded while awaiting headers)"), 0) {
		t.Error("client timeout must not be retried")
	}
}

// TestEffortConfigTiers pins the effort mapping against what the API actually
// accepts. Only low|high|max are real, so medium collapses onto high; every
// tier shares the one budget that can complete inside the upstream timeout.
func TestEffortConfigTiers(t *testing.T) {
	cases := []struct {
		effort     string
		wantEffort string
	}{
		{"low", "low"},
		{"minimal", "low"},
		{"medium", "high"},
		{"high", "high"},
		{"", "high"},
		{"xhigh", "max"},
		{"ultra", "max"},
	}
	for _, c := range cases {
		thinking, effort, maxTokens := effortConfig(c.effort)
		if thinking == nil || thinking.Type != "enabled" {
			t.Errorf("effortConfig(%q): thinking should be enabled", c.effort)
		}
		if effort != c.wantEffort {
			t.Errorf("effortConfig(%q) effort = %q, want %q", c.effort, effort, c.wantEffort)
		}
		if maxTokens != maxOutputTokens {
			t.Errorf("effortConfig(%q) maxTokens = %d, want %d", c.effort, maxTokens, maxOutputTokens)
		}
	}
}
