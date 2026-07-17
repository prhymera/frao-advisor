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
func (c *DeepSeekClient) Chat(system, user string, temperature float64, maxTokens int) (string, error) {
	messages := []ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
	return c.chat(messages, temperature, maxTokens)
}

// ChatWithMessages sends a chat completion with pre-built messages.
func (c *DeepSeekClient) ChatWithMessages(messages []ChatMessage, temperature float64, maxTokens int) (string, error) {
	return c.chat(messages, temperature, maxTokens)
}

func (c *DeepSeekClient) chat(messages []ChatMessage, temperature float64, maxTokens int) (string, error) {
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
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(raw[:min(len(raw), 500)]))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(raw, &chatResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}
