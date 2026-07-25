package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	return &DeepSeekClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		http: &http.Client{
			Timeout: 180 * time.Second, // 3 min for deep thinking
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

// chatWithUsage is the internal implementation that calls the DeepSeek API
// and returns both content and usage metadata.
func (c *DeepSeekClient) chatWithUsage(messages []ChatMessage, temperature float64, maxTokens int) (*ChatResult, error) {
	start := time.Now()

	if temperature == 0 {
		temperature = 0.2
	}
	if maxTokens == 0 {
		maxTokens = 8192
	}

	body := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
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

	resp, err := c.http.Do(req)
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
		Text:        chatResp.Choices[0].Message.Content,
		Model:       c.model,
		DurationMs:  time.Since(start).Milliseconds(),
		Temperature: temperature,
		MaxTokens:   maxTokens,
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
