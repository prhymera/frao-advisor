package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func okChatBody() string {
	return `{"choices":[{"message":{"content":"finding"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
}

// TestMultiPerspectiveRunsExpertsConcurrently pins the fan-out. Experts are
// independent reviews of the same context, so running them one after another
// multiplied the worst case by the number of experts — three experts each
// running to the client timeout is ~15 minutes of wall clock, and one stalled
// expert blocked the other two.
func TestMultiPerspectiveRunsExpertsConcurrently(t *testing.T) {
	var mu sync.Mutex
	inFlight, maxInFlight, calls := 0, 0, 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inFlight++
		calls++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()

		time.Sleep(150 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()

		fmt.Fprint(w, okChatBody())
	}))
	defer srv.Close()

	client := NewDeepSeekClient("sk-test", srv.URL, "deepseek-flash")
	args := map[string]any{
		"context": "review this",
		"experts": []any{"architect", "code-reviewer", "security-analyst"},
	}

	res := handleMultiPerspective(args, client, nil)
	if res.IsError {
		t.Fatalf("unexpected tool error: %v", res.Content)
	}

	mu.Lock()
	gotMax, gotCalls := maxInFlight, calls
	mu.Unlock()

	if gotMax < 3 {
		t.Errorf("experts ran with max concurrency %d, want 3 (fan-out is sequential again?)", gotMax)
	}
	if gotCalls != 4 { // 3 experts + 1 synthesis
		t.Errorf("expected 4 DeepSeek calls (3 experts + synthesis), got %d", gotCalls)
	}

	// Concurrent completion must not scramble the reported order.
	text := res.Content[0].Text
	a := strings.Index(text, experts["architect"].Name)
	b := strings.Index(text, experts["code-reviewer"].Name)
	c := strings.Index(text, experts["security-analyst"].Name)
	if a < 0 || b < 0 || c < 0 {
		t.Fatalf("a contribution is missing from the output: architect=%d code-reviewer=%d security-analyst=%d", a, b, c)
	}
	if !(a < b && b < c) {
		t.Errorf("contributions out of requested order: architect=%d code-reviewer=%d security-analyst=%d", a, b, c)
	}
}

// TestMultiPerspectiveSkipsSynthesisOnAccountError verifies that an
// account-level rejection (billing/auth) short-circuits synthesis. Those fail
// identically for every call until the account is fixed, so spending another
// multi-minute call on synthesis buys nothing.
func TestMultiPerspectiveSkipsSynthesisOnAccountError(t *testing.T) {
	var mu sync.Mutex
	calls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		http.Error(w, `{"error":{"message":"Insufficient Balance"}}`, http.StatusPaymentRequired)
	}))
	defer srv.Close()

	client := NewDeepSeekClient("sk-test", srv.URL, "deepseek-flash")
	args := map[string]any{
		"context": "review this",
		"experts": []any{"architect", "code-reviewer"},
	}

	res := handleMultiPerspective(args, client, nil)

	mu.Lock()
	gotCalls := calls
	mu.Unlock()

	// 402 is permanent, so no retries: exactly one call per expert, no synthesis.
	if gotCalls != 2 {
		t.Errorf("expected 2 calls (experts only, synthesis skipped), got %d", gotCalls)
	}
	if !strings.Contains(res.Content[0].Text, "Synthesis skipped") {
		t.Errorf("expected a synthesis-skipped notice, got %q", res.Content[0].Text)
	}
}

// TestIsAccountError pins the account-level/permanent classification.
func TestIsAccountError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("API error 402: {\"error\":{\"message\":\"Insufficient Balance\"}}"), true},
		{fmt.Errorf("API error 401: unauthorized"), true},
		{fmt.Errorf("API error 400: bad request"), false},
		{fmt.Errorf("API error 500: server error"), false},
		{fmt.Errorf("empty or truncated response body (0 bytes)"), false},
	}
	for _, c := range cases {
		if got := isAccountError(c.err); got != c.want {
			t.Errorf("isAccountError(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
