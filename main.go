// Frao Advisor MCP — Go implementation.
//
// Provides expert review, multi-perspective analysis, and second opinions
// using DeepSeek V4 Pro directly. No OpenRouter, no extra costs.
//
// Architecture is ported from deliberation (github.com/antonbabenko/deliberation):
//   - Expert personas with focused system prompts
//   - Multi-perspective parallel analysis with synthesis
//   - Direct DeepSeek API integration via OpenAI-compatible endpoint
//   - MCP protocol over stdio (JSON-RPC 2.0)
//
// Usage:
//   DEEPSEEK_API_KEY=sk-... go run .  # or build first
//
// Environment variables:
//   DEEPSEEK_API_KEY   — required: DeepSeek API key
//   ADVISOR_MODEL      — model name (default: deepseek-v4-pro)
//   ADVISOR_API_BASE   — API base URL (default: https://api.deepseek.com/v1)

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	log.SetPrefix("[frao-advisor] ")
	log.SetFlags(log.Ltime | log.Lmsgprefix)

	// Route subcommands before entering MCP mode
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			projectDir := "."
			global := false
			for i := 2; i < len(os.Args); i++ {
				switch os.Args[i] {
				case "--global":
					global = true
				default:
					projectDir = os.Args[i]
				}
			}
			binPath, _ := os.Executable()
			runSetup(projectDir, binPath, global)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	// Read configuration from environment
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_AUTH_TOKEN")
	}
	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY or ANTHROPIC_AUTH_TOKEN must be set")
	}

	model := getEnv("ADVISOR_MODEL", "deepseek-v4-pro")
	baseURL := getEnv("ADVISOR_API_BASE", "https://api.deepseek.com/v1")

	client := NewDeepSeekClient(apiKey, baseURL, model)
	log.Printf("starting — model: %s, api: %s", model, baseURL)

	// MCP server: read JSON-RPC requests from stdin, write responses to stdout
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 4*1024*1024) // 4MB max line

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("parse error: %v", err)
			writeError(nil, -32700, "Parse error", nil)
			continue
		}

		// Handle notifications (no ID) — silently accept
		if req.ID == nil {
			continue
		}

		// Route method
		switch req.Method {
		case "initialize":
			handleInitialize(req)
		case "tools/list":
			handleToolsList(req)
		case "tools/call":
			handleToolCall(req, client)
		default:
			writeError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method), nil)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// writeResponse sends a JSON-RPC response to stdout.
func writeResponse(id any, result any) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	writeJSON(resp)
}

// writeError sends a JSON-RPC error response to stdout.
func writeError(id any, code int, message string, data any) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	writeJSON(resp)
}

func writeJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}
	fmt.Println(string(data))
}

// ─── Initialize ────────────────────────────────────────────────────────

func handleInitialize(req Request) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ToolCapability{
			Tools: &struct{}{},
		},
		ServerInfo: InitializeInfo{
			Name:    "frao-advisor",
			Version: "1.0.0",
		},
	}
	writeResponse(req.ID, result)
}

// ─── Tools List ────────────────────────────────────────────────────────

var tools = []Tool{
	{
		Name:        "frao-expert-list",
		Description: "List all available expert personas with their focus areas",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
			Required:   []string{},
		},
	},
	{
		Name: "frao-consult",
		Description: "Send any question or code to deepseek-v4-pro for a second opinion. " +
			"Use when you need an independent review of a design decision, bug analysis, or technical question.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"question": {
					Type:        "string",
					Description: "The question, code, or context you want a second opinion on",
				},
				"reasoning_effort": {
					Type:        "string",
					Description: "How deeply to reason. high/xhigh yields better quality but takes longer",
					Enum:        []string{"low", "medium", "high", "xhigh"},
					Default:     "high",
				},
			},
			Required: []string{"question"},
		},
	},
	{
		Name: "frao-expert-review",
		Description: "Review code, architecture, or a plan through the lens of a specific expert persona. " +
			"Choose the persona that matches the concern: architects for system design, code-reviewers for correctness, security-analysts for vulnerabilities.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"expert": {
					Type:        "string",
					Description: "Expert persona for the review",
					Enum:        expertKeys,
				},
				"context": {
					Type:        "string",
					Description: "The code diff, architecture, config, or plan to review",
				},
				"reasoning_effort": {
					Type:        "string",
					Description: "How deeply to reason",
					Enum:        []string{"low", "medium", "high", "xhigh"},
					Default:     "high",
				},
			},
			Required: []string{"expert", "context"},
		},
	},
	{
		Name: "frao-multi-perspective",
		Description: "Analyze a complex problem from multiple expert perspectives in parallel, then synthesize a unified recommendation. " +
			"Use for high-stakes decisions: deployment strategies, major refactors, security audits, architecture changes.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"context": {
					Type:        "string",
					Description: "The problem, code, or design to analyze from multiple angles",
				},
				"experts": {
					Type:        "array",
					Description: "Which experts to consult (default: architect, code-reviewer, security-analyst)",
					Items: &struct {
						Type string   `json:"type"`
						Enum []string `json:"enum,omitempty"`
					}{
						Type: "string",
						Enum: expertKeys,
					},
				},
				"reasoning_effort": {
					Type:        "string",
					Description: "How deeply to reason",
					Enum:        []string{"low", "medium", "high", "xhigh"},
					Default:     "high",
				},
			},
			Required: []string{"context"},
		},
	},
}

func handleToolsList(req Request) {
	result := map[string]any{
		"tools": tools,
	}
	writeResponse(req.ID, result)
}

func printUsage() {
	fmt.Println(`Frao Advisor MCP — expert reviews and second opinions using DeepSeek V4 Pro.

Usage:
  frao-advisor                  Run as MCP server (stdin/stdout JSON-RPC)
  frao-advisor setup [dir]           Install slash commands in project .claude/commands
  frao-advisor setup --global       Install slash commands in ~/.claude/commands (recommended)
  frao-advisor help             Show this help

Setup:
  Run from your project root to install /advisor-on and /advisor-off commands:
    frao-advisor setup .

Environment:
  DEEPSEEK_API_KEY    DeepSeek API key (required)
  ADVISOR_MODEL       Model name (default: deepseek-v4-pro)
  ADVISOR_API_BASE    API base URL (default: https://api.deepseek.com/v1)`)
}

// ─── Tool Call ─────────────────────────────────────────────────────────

func handleToolCall(req Request, client *DeepSeekClient) {
	params, ok := req.Params.(map[string]any)
	if !ok {
		var p ToolCallParams
		data, _ := json.Marshal(req.Params)
		if err := json.Unmarshal(data, &p); err != nil {
			writeError(req.ID, -32602, "Invalid params", err.Error())
			return
		}
		params = map[string]any{"name": p.Name, "arguments": p.Arguments}
	}

	// Extract optional progress token from _meta for MCP progress notifications
	var progressToken any
	if meta, ok := params["_meta"].(map[string]any); ok {
		progressToken = meta["progressToken"]
	}

	name, _ := params["name"].(string)
	args, _ := params["arguments"].(map[string]any)

	var result ToolCallResult

	switch name {
	case "frao-expert-list":
		result = handleExpertList()
	case "frao-consult":
		result = handleConsult(args, client, progressToken)
	case "frao-expert-review":
		result = handleExpertReview(args, client, progressToken)
	case "frao-multi-perspective":
		result = handleMultiPerspective(args, client, progressToken)
	default:
		writeError(req.ID, -32602, "Unknown tool: "+name, nil)
		return
	}

	writeResponse(req.ID, result)
}

// sendProgress sends an MCP notifications/progress message to the client.
func sendProgress(progressToken any, progress, total float64, msg string) {
	if progressToken == nil {
		return
	}
	n := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/progress",
		"params": map[string]any{
			"progressToken": progressToken,
			"progress":      progress,
			"total":         total,
			"message":       msg,
		},
	}
	writeJSON(n)
}

// ─── Tool: frao-expert-list ───────────────────────────────────────────

func handleExpertList() ToolCallResult {
	var items []string
	for _, k := range expertKeys {
		e := experts[k]
		items = append(items, fmt.Sprintf("  • %s — %s", k, e.Name))
	}
	return textResult(strings.Join(items, "\n"))
}

// ─── Tool: frao-consult ───────────────────────────────────────────────

func handleConsult(args map[string]any, client *DeepSeekClient, progressToken any) ToolCallResult {
	question, _ := args["question"].(string)
	if question == "" {
		return errorResult("'question' is required")
	}

	effort := getArg(args, "reasoning_effort", "high")
	log.Printf("consult — effort=%s", effort)
	sendProgress(progressToken, 0.1, 1, "Consulting deepseek-v4-pro...")

	system := fmt.Sprintf(advisorSystemPreamble, effort)
	answer, err := client.Chat(system, question, 0.1, 8192)
	if err != nil {
		log.Printf("consult error: %v", err)
		return errorResult(fmt.Sprintf("Consult failed: %v", err))
	}

	sendProgress(progressToken, 1, 1, "Consult complete")
	log.Printf("consult — response %d bytes", len(answer))
	return textResult(answer)
}

// ─── Tool: frao-expert-review ─────────────────────────────────────────

func handleExpertReview(args map[string]any, client *DeepSeekClient, progressToken any) ToolCallResult {
	expertKey, _ := args["expert"].(string)
	context, _ := args["context"].(string)

	if expertKey == "" {
		return errorResult("'expert' is required")
	}
	if context == "" {
		return errorResult("'context' is required")
	}

	expert, ok := experts[expertKey]
	if !ok {
		return errorResult(fmt.Sprintf("Unknown expert '%s'. Available: %s", expertKey, strings.Join(expertKeys, ", ")))
	}

	effort := getArg(args, "reasoning_effort", "high")
	log.Printf("expert-review — %s, effort=%s", expertKey, effort)
	sendProgress(progressToken, 0.1, 1, fmt.Sprintf("Consulting %s...", expert.Name))

	system := fmt.Sprintf("%s\n\nReasoning effort: %s\nProvide structured analysis with severity ratings and concrete recommendations.", expert.SystemPrompt, effort)
	answer, err := client.Chat(system, context, 0.15, 8192)
	if err != nil {
		log.Printf("expert-review %s error: %v", expertKey, err)
		return errorResult(fmt.Sprintf("Review failed: %v", err))
	}

	sendProgress(progressToken, 1, 1, fmt.Sprintf("%s review complete", expert.Name))
	log.Printf("expert-review %s — response %d bytes", expertKey, len(answer))
	return textResult(answer)
}

// ─── Tool: frao-multi-perspective ─────────────────────────────────────

func handleMultiPerspective(args map[string]any, client *DeepSeekClient, progressToken any) ToolCallResult {
	context, _ := args["context"].(string)
	if context == "" {
		return errorResult("'context' is required")
	}

	// Determine which experts to use
	expertsRaw, _ := args["experts"].([]any)
	var selectedExperts []string
	if len(expertsRaw) > 0 {
		for _, e := range expertsRaw {
			if s, ok := e.(string); ok {
				if _, exists := experts[s]; exists {
					selectedExperts = append(selectedExperts, s)
				}
			}
		}
	}
	if len(selectedExperts) == 0 {
		selectedExperts = []string{"architect", "code-reviewer", "security-analyst"}
	}

	effort := getArg(args, "reasoning_effort", "high")
	total := float64(len(selectedExperts) + 1) // experts + synthesis
	log.Printf("multi-perspective — %d experts: %s, effort=%s", len(selectedExperts), strings.Join(selectedExperts, ", "), effort)

	// Step 1: Run each expert (sequentially to respect API rate limits)
	type perspective struct {
		Key      string
		Name     string
		Analysis string
	}

	var perspectives []perspective
	for i, key := range selectedExperts {
		expert := experts[key]
		log.Printf("multi-perspective — consulting %s (%d/%d)...", expert.Name, i+1, len(selectedExperts))
		sendProgress(progressToken, float64(i)/total, total, fmt.Sprintf("Consulting %s...", expert.Name))

		system := fmt.Sprintf("%s\n\nReasoning effort: %s\nYou are one of %d experts reviewing this. Focus solely on your domain. Do not defer to or repeat other perspectives.",
			expert.SystemPrompt, effort, len(selectedExperts))

		analysis, err := client.Chat(system, context, 0.15, 8192)
		if err != nil {
			log.Printf("multi-perspective %s error: %v", key, err)
			analysis = fmt.Sprintf("[Error consulting %s: %v]", expert.Name, err)
		}
		log.Printf("multi-perspective — %s: got %d bytes", key, len(analysis))
		perspectives = append(perspectives, perspective{Key: key, Name: expert.Name, Analysis: analysis})
	}

	// Step 2: Build synthesis input
	log.Print("multi-perspective — synthesizing...")
	sendProgress(progressToken, float64(len(selectedExperts))/total, total, "Synthesizing expert perspectives...")

	var parts []string
	for _, p := range perspectives {
		parts = append(parts, fmt.Sprintf("=== %s (%s) ===\n%s", p.Name, p.Key, p.Analysis))
	}

	synthesisInput := fmt.Sprintf("Synthesize the following expert perspectives into a unified recommendation.\n\nIdentify:\n  1. Areas of agreement\n  2. Areas of disagreement\n  3. Critical findings\n  4. Final recommendation\n\n%s", strings.Join(parts, "\n\n"))

	synthesis, err := client.Chat(synthesisSystemPrompt, synthesisInput, 0.2, 4096)
	if err != nil {
		log.Printf("synthesis error: %v", err)
		synthesis = fmt.Sprintf("[Synthesis failed: %v]", err)
	}

	sendProgress(progressToken, total, total, "Multi-perspective analysis complete")
	log.Print("multi-perspective — complete")

	// Build output
	var out strings.Builder
	for _, p := range perspectives {
		fmt.Fprintf(&out, "═══════════════════════════════════════\n")
		fmt.Fprintf(&out, "  🧠 %s (%s)\n", p.Name, p.Key)
		fmt.Fprintf(&out, "═══════════════════════════════════════\n")
		out.WriteString(p.Analysis)
		out.WriteString("\n\n")
	}
	fmt.Fprintf(&out, "═══════════════════════════════════════\n")
	fmt.Fprintf(&out, "  📋 SYNTHESIS\n")
	fmt.Fprintf(&out, "═══════════════════════════════════════\n")
	out.WriteString(synthesis)

	return textResult(out.String())
}

// ─── Helpers ───────────────────────────────────────────────────────────

func textResult(text string) ToolCallResult {
	return ToolCallResult{
		Content: []ContentBlock{{
			Type:     "text",
			Text:     text,
			MimeType: "text/markdown",
		}},
	}
}

func errorResult(msg string) ToolCallResult {
	return ToolCallResult{
		IsError: true,
		Content: []ContentBlock{{Type: "text", Text: msg}},
	}
}

func getArg(args map[string]any, key, fallback string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
