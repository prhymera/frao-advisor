package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DeepSeek API client — calls the OpenAI-compatible /chat/completions endpoint
// directly using the configured API key and model. No external SDK needed.

type DeepSeekClient struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

func NewDeepSeekClient(apiKey, baseURL, model string) *DeepSeekClient {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	if model == "" {
		model = "deepseek-flash"
	}
	// ADVISOR_TIMEOUT_SECONDS bounds each DeepSeek round-trip. Successful
	// reviews have been observed at 299s, so the default sits deliberately
	// ABOVE the ~300s point at which DeepSeek's gateway cuts long responses:
	// a cut then surfaces as a truncated 200 body (retryable) rather than as
	// our own client timeout, which is not retried.
	timeoutSec := 330
	if v := getEnv("ADVISOR_TIMEOUT_SECONDS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSec = n
		}
	}
	return &DeepSeekClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		http: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

// Chat sends a chat completion request and returns the response text.
// Maintains backward compatibility — delegates to ChatWithUsage internally.
func (c *DeepSeekClient) Chat(system, user string, temperature float64, maxTokens int) (string, error) {
	result, err := c.ChatWithUsage(system, user, temperature, maxTokens)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// ChatWithMessages sends a chat completion with pre-built messages.
func (c *DeepSeekClient) ChatWithMessages(messages []ChatMessage, temperature float64, maxTokens int) (string, error) {
	result, err := c.ChatWithMessagesWithUsage(messages, temperature, maxTokens)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// ChatWithUsage returns a ChatResult with token usage information.
func (c *DeepSeekClient) ChatWithUsage(system, user string, temperature float64, maxTokens int) (*ChatResult, error) {
	messages := []ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
	return c.chatWithUsage(messages, temperature, maxTokens)
}

// ChatWithMessagesWithUsage returns a ChatResult with pre-built messages.
func (c *DeepSeekClient) ChatWithMessagesWithUsage(messages []ChatMessage, temperature float64, maxTokens int) (*ChatResult, error) {
	return c.chatWithUsage(messages, temperature, maxTokens)
}

// chatWithUsage preserves the legacy no-thinking call path.
func (c *DeepSeekClient) chatWithUsage(messages []ChatMessage, temperature float64, maxTokens int) (*ChatResult, error) {
	return c.doChat(messages, temperature, maxTokens, nil, "")
}

// ChatWithEffort sends a chat completion at the requested reasoning effort.
// DeepSeek V4 reasons by default, and its chain-of-thought counts against
// max_tokens — when it exhausts the budget the API returns empty content with
// finish_reason="length". We size the budget by effort and, if content comes
// back empty, retry once with thinking disabled so callers never get a silent
// blank answer.
func (c *DeepSeekClient) ChatWithEffort(system, user string, temperature float64, effort string) (*ChatResult, error) {
	messages := []ChatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}}
	thinking, reasoningEffort, maxTokens := effortConfig(effort)
	result, err := c.doChat(messages, temperature, maxTokens, thinking, reasoningEffort)
	if err != nil {
		return nil, err
	}
	if result.Text == "" {
		log.Printf("empty model output (finish_reason=%s) — retrying without thinking", result.FinishReason)
		fallback, ferr := c.doChat(messages, temperature, 16384, &ThinkingConfig{Type: "disabled"}, "")
		if ferr == nil && fallback.Text != "" {
			fallback.FellBack = true
			return fallback, nil
		}
		if ferr != nil {
			return nil, fmt.Errorf("empty model output (finish_reason=%s); thinking-disabled retry failed: %w", result.FinishReason, ferr)
		}
		return nil, fmt.Errorf("empty model output (finish_reason=%s); thinking-disabled retry also returned empty content", result.FinishReason)
	}
	return result, nil
}

// maxOutputTokens caps the output budget for every tier.
//
// Measured throughput on this workload is ~55 output tokens/second, so a
// 32K-token budget needs ~600s to generate — far past the point where
// DeepSeek's gateway cuts the request (no advisor call has ever succeeded
// beyond 298s). A budget above ~16K therefore cannot be honoured: the call is
// cut mid-answer, and reasoning tokens count against the same budget. 16384 is
// the largest budget that can realistically complete.
const maxOutputTokens = 16384

// effortConfig maps the tool's reasoning_effort arg to DeepSeek's thinking
// controls and an output budget.
//
// The API accepts only low|high|max. medium, xhigh, minimal and ultra are
// compatibility aliases that collapse onto those three (medium->high,
// xhigh->high, minimal->low, ultra->max), so there is no genuine middle tier
// to select — the tiers below differ only where the API actually allows it.
func effortConfig(effort string) (*ThinkingConfig, string, int) {
	switch strings.ToLower(effort) {
	case "low", "minimal":
		return &ThinkingConfig{Type: "enabled"}, "low", maxOutputTokens
	case "xhigh", "ultra", "max":
		return &ThinkingConfig{Type: "enabled"}, "max", maxOutputTokens
	default: // "medium", "high" or unset — the API's default reasoning depth
		return &ThinkingConfig{Type: "enabled"}, "high", maxOutputTokens
	}
}

// maxRetries bounds how many times a single DeepSeek call is re-attempted
// after a transient upstream failure. Client timeouts are NOT retried — a
// second long wait rarely helps and would double latency — but quick failures
// (rate-limit 429, 5xx, dropped connections) usually succeed on retry.
const maxRetries = 2

// errTruncatedBody marks a 200 response whose body was empty or not valid
// JSON. DeepSeek's gateway cuts long-running requests near the client timeout
// and returns a short or empty body instead of an error status; this sentinel
// wraps that case so it can be classified as retryable.
var errTruncatedBody = errors.New("empty or truncated response body")

// isTransientError reports whether a failed attempt is worth retrying.
//
// The client timeout is deliberately NOT retried: a second multi-minute wait
// rarely helps and would double latency. A truncated 200 body IS retried — the
// request reached the model and the gateway cut the answer, so a fresh attempt
// is the only recovery path.
func isTransientError(err error, status int) bool {
	if status == 429 || (status >= 500 && status < 600) {
		return true
	}
	if errors.Is(err, errTruncatedBody) {
		return true
	}
	if err == nil {
		return false
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "broken pipe"),
		strings.Contains(msg, "EOF"),
		strings.Contains(msg, "no such host"),
		strings.Contains(msg, "TLS handshake timeout"):
		return true
	}
	return false
}

// retryBackoff returns the pause before retry attempt n (0-indexed). Short and
// bounded so concurrent sessions don't pile onto a throttled API.
func retryBackoff(attempt int) time.Duration {
	switch attempt {
	case 0:
		return 2 * time.Second
	case 1:
		return 5 * time.Second
	default:
		return 12 * time.Second
	}
}

// doChat is the raw chat-completion round-trip. thinking/reasoningEffort are
// passed through to the API (nil/"" for the legacy no-thinking path).
func (c *DeepSeekClient) doChat(messages []ChatMessage, temperature float64, maxTokens int, thinking *ThinkingConfig, reasoningEffort string) (*ChatResult, error) {
	start := time.Now()

	if temperature == 0 {
		temperature = 0.2
	}
	if maxTokens == 0 {
		maxTokens = 8192
	}

	body := ChatRequest{
		Model:           c.model,
		Messages:        messages,
		Temperature:     temperature,
		MaxTokens:       maxTokens,
		Thinking:        thinking,
		ReasoningEffort: reasoningEffort,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Attempt loop with bounded retry on transient upstream failures. The
	// serialization lock is off by default (concurrency), so retrying is what
	// absorbs the occasional throttle/dropped-connection instead of a shared
	// lock serializing all sessions.
	//
	// A 200 whose body is empty or not valid JSON is also treated as transient:
	// that is the signature of the upstream gateway cutting a long-running
	// request, and it is the advisor's single most common failure. Without a
	// retry the whole call is lost after already waiting out the timeout.
	var raw []byte
	status := 0
	var callErr error
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		raw, status, callErr = c.roundTrip(req)
		if callErr == nil && status == 200 && json.Valid(bytes.TrimSpace(raw)) {
			break
		}
		if callErr == nil && status == 200 {
			callErr = fmt.Errorf("%w (%d bytes)", errTruncatedBody, len(raw))
		}
		if attempt >= maxRetries || !isTransientError(callErr, status) {
			break
		}
		log.Printf("transient upstream failure (status=%d, err=%v) — retrying %d/%d", status, callErr, attempt+1, maxRetries)
		time.Sleep(retryBackoff(attempt))
	}
	if callErr != nil {
		return nil, callErr
	}
	if status != 200 {
		return nil, fmt.Errorf("API error %d: %s", status, string(raw[:min(len(raw), 500)]))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(raw, &chatResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response choices")
	}

	result := &ChatResult{
		Text:         chatResp.Choices[0].Message.Content,
		Model:        c.model,
		FinishReason: chatResp.Choices[0].FinishReason,
		DurationMs:   time.Since(start).Milliseconds(),
		Temperature:  temperature,
		MaxTokens:    maxTokens,
	}

	if chatResp.Usage != nil {
		result.PromptTokens = chatResp.Usage.PromptTokens
		result.CompletionTokens = chatResp.Usage.CompletionTokens
		result.TotalTokens = chatResp.Usage.TotalTokens
		result.PromptCacheHitTokens = chatResp.Usage.PromptCacheHitTokens
		result.PromptCacheMissTokens = chatResp.Usage.PromptCacheMissTokens
	}

	return result, nil
}

// roundTrip performs one HTTP request (optionally under the serialization
// lock, when ADVISOR_SERIALIZE=1) and returns the raw body plus status code.
func (c *DeepSeekClient) roundTrip(req *http.Request) ([]byte, int, error) {
	var resp *http.Response
	err := withDeepSeekLock(func() error {
		var callErr error
		resp, callErr = c.http.Do(req)
		return callErr
	})
	if err != nil {
		return nil, 0, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return raw, resp.StatusCode, nil
}
