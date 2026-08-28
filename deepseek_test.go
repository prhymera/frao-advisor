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
