package main

import (
	"bytes"
	"encoding/json"
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
		model = "deepseek-v4-pro"
	}
	// ADVISOR_TIMEOUT_SECONDS bounds each DeepSeek round-trip. High-effort
	// thinking responses regularly exceed 3 minutes, so the default is 300s
	// (up from 180s) — long enough for deep reviews, short enough to fail
	// loudly instead of hanging forever.
	timeoutSec := 300
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

// effortConfig maps the tool's reasoning_effort arg to DeepSeek V4's thinking
// controls and an output budget generous enough that reasoning can't starve
// the final answer.
func effortConfig(effort string) (*ThinkingConfig, string, int) {
	switch strings.ToLower(effort) {
	case "low":
		return &ThinkingConfig{Type: "enabled"}, "low", 16384
	case "medium":
		// Genuine middle tier: medium reasoning effort with a 16K budget —
		// fast enough to stay well under the client timeout, deep enough for
		// real analysis. This is the default effort.
		return &ThinkingConfig{Type: "enabled"}, "medium", 16384
	case "xhigh":
		return &ThinkingConfig{Type: "enabled"}, "max", 65536
	default: // "high" or unset
		return &ThinkingConfig{Type: "enabled"}, "high", 32768
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

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	var resp *http.Response
	err = withDeepSeekLock(func() error {
		var callErr error
		resp, callErr = c.http.Do(req)
		return callErr
	})
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(raw[:min(len(raw), 500)]))
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
