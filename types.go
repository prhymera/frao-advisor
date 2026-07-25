package main

// JSON-RPC 2.0 request
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JSON-RPC 2.0 response
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCP Initialize
type InitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    any            `json:"capabilities"`
	ClientInfo      InitializeInfo `json:"clientInfo"`
}

type InitializeInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    ToolCapability `json:"capabilities"`
	ServerInfo      InitializeInfo `json:"serverInfo"`
}

type ToolCapability struct {
	Tools *struct{} `json:"tools,omitempty"`
}

// MCP Tool definitions
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertySchema `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

type PropertySchema struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
	Items       *struct {
		Type string `json:"type"`
		Enum []string `json:"enum,omitempty"`
	} `json:"items,omitempty"`
}

// Tool call request
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	Meta      *ToolMeta      `json:"_meta,omitempty"`
}

type ToolMeta struct {
	ProgressToken any `json:"progressToken,omitempty"`
}

type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	MimeType string `json:"mimeType,omitempty"`
}

// ─── DeepSeek API types ────────────────────────────────────────────────

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// Usage holds token counts returned by the DeepSeek API.
type Usage struct {
	PromptTokens       int `json:"prompt_tokens"`
	CompletionTokens   int `json:"completion_tokens"`
	TotalTokens        int `json:"total_tokens"`
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens,omitempty"`
}

type ChatResponse struct {
	ID      string `json:"id,omitempty"`
	Model   string `json:"model,omitempty"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *Usage `json:"usage,omitempty"`
}

// ChatResult holds the full DeepSeek API response plus usage metadata.
type ChatResult struct {
	Text               string
	PromptTokens       int
	CompletionTokens   int
	TotalTokens        int
	PromptCacheHitTokens  int
	PromptCacheMissTokens int
	Model              string
	DurationMs         int64
	Temperature        float64
	MaxTokens          int
}

// Expert persona
type Expert struct {
	Name        string
	SystemPrompt string
}

// Tool argument helpers
type ConsultArgs struct {
	Question        string `json:"question"`
	ReasoningEffort string `json:"reasoning_effort"`
}

type ExpertReviewArgs struct {
	Expert          string `json:"expert"`
	Context         string `json:"context"`
	ReasoningEffort string `json:"reasoning_effort"`
}

type MultiPerspectiveArgs struct {
	Context         string   `json:"context"`
	Experts         []string `json:"experts"`
	ReasoningEffort string   `json:"reasoning_effort"`
}
