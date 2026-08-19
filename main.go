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
//   DEEPSEEK_API_KEY            — required: DeepSeek API key
//   ADVISOR_MODEL               — model name (default: deepseek-v4-pro)
//   ADVISOR_API_BASE            — API base URL (default: https://api.deepseek.com/v1)
//   ADVISOR_DASHBOARD_PORT      — dashboard port (default: 9753)
//   ADVISOR_DASHBOARD_URL       — dashboard endpoint for MCP publish (default: http://10.64.0.5:9753)
//   ADVISOR_SESSION_LABEL       — per-process session label (default: <workdir>-<pid>)
//   ADVISOR_DASHBOARD_DISABLE   — set to "1" to disable the embedded dashboard
//   ADVISOR_TIMEOUT_SECONDS     — per-DeepSeek-call timeout (default: 300)
//   ADVISOR_DEFAULT_EFFORT      — default reasoning_effort when unset (default: medium)
//   ADVISOR_SERIALIZE           — "0" disables cross-process call serialization

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prhymera/frao-advisor/dashboard"
	"github.com/prhymera/frao-advisor/db"
	"github.com/prhymera/frao-advisor/publish"
)

// Package-level state for handlers
var (
	persister        *Persistence
	currentSessionID string
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
		case "dashboard":
			runDashboard()
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

	// Initialize persistence
	dbPath := getEnv("ADVISOR_DB_PATH", filepath.Join(workDir(), "advisor.db"))
	database, err := db.Open(dbPath)
	if err != nil {
		log.Printf("WARNING: database unavailable (%v) — persistence disabled", err)
	} else {
		log.Printf("persistence active — %s", dbPath)
	}

	// Best-effort publishing to the standalone dashboard. Publishing never
	// blocks or fails the MCP tool path: if the dashboard is down, events are
	// logged and dropped. Set ADVISOR_DASHBOARD_URL="" to disable.
	dashURL := dashboardURL()
	publisher := publish.New(dashURL)
	if publisher == nil {
		log.Print("dashboard publishing disabled")
	} else {
		defer publisher.Flush()
		log.Printf("publishing advice to dashboard: %s", dashURL)
	}

	// Session label: human-readable, overridable, so the dashboard can
	// attribute usage across the different Claude sessions.
	sessionLabel := getEnv("ADVISOR_SESSION_LABEL", "")
	if sessionLabel == "" {
		sessionLabel = filepath.Base(workDir()) + "-" + strconv.Itoa(os.Getpid())
	}

	if database != nil {
		persister = &Persistence{database: database, publisher: publisher, sessionLabel: sessionLabel}
		defer database.Close()
	}
	currentSessionID = persister.EnsureSession(sessionLabel)

	// Embedded dashboard: off by default in MCP mode. ADVISOR_DASHBOARD_EMBED=1
	// restores the old single-process behavior for dev; ADVISOR_DASHBOARD_DISABLE=1
	// still forces it off. Run the standalone dashboard via `frao-advisor dashboard`.
	if database != nil && getEnv("ADVISOR_DASHBOARD_DISABLE", "") != "1" &&
		(getEnv("ADVISOR_MCP_DISABLE", "") == "1" || getEnv("ADVISOR_DASHBOARD_EMBED", "") == "1") {
		port := getEnv("ADVISOR_DASHBOARD_PORT", "9753")
		dashSrv := dashboard.Start(database, port)
		defer dashSrv.Close()
	}

	// ADVISOR_MCP_DISABLE=1 runs dashboard-only (no MCP stdin loop)
	if getEnv("ADVISOR_MCP_DISABLE", "") == "1" {
		log.Print("MCP disabled by ADVISOR_MCP_DISABLE — dashboard only")
		select {}
	}

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

func workDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// runDashboard starts the standalone dashboard service. It does not require a
// DeepSeek API key and blocks forever serving the UI + ingest API.
func runDashboard() {
	dbPath := getEnv("ADVISOR_DB_PATH", filepath.Join(workDir(), "advisor.db"))
	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("dashboard: database unavailable: %v", err)
	}
	defer database.Close()
	log.Printf("dashboard persistence — %s", dbPath)

	port := getEnv("ADVISOR_DASHBOARD_PORT", "9753")
	dashSrv := dashboard.Start(database, port)
	log.Printf("dashboard serving on http://%s", dashSrv.Addr)
	select {}
}

// dashboardURL returns the dashboard endpoint MCP processes publish to.
// An explicitly-empty ADVISOR_DASHBOARD_URL disables publishing; when unset,
// the default host 10.64.0.5 is used.
func dashboardURL() string {
	if v, ok := os.LookupEnv("ADVISOR_DASHBOARD_URL"); ok {
		return v
	}
	return "http://10.64.0.5:9753"
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
					Default:     defaultEffort(),
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
					Default:     defaultEffort(),
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
					Default:     defaultEffort(),
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
  frao-advisor dashboard            Run the standalone dashboard service
  frao-advisor setup [dir]           Install slash commands in project .claude/commands
  frao-advisor setup --global       Install slash commands in ~/.claude/commands (recommended)
  frao-advisor help             Show this help

Dashboard:
  The dashboard is a separate process. MCP processes publish advice records
  to it over HTTP (best-effort; never blocks the tool call).
    frao-advisor dashboard      # serves http://10.64.0.5:9753

Setup:
  Run from your project root to install /advisor-on and /advisor-off commands:
    frao-advisor setup .

Environment:
  DEEPSEEK_API_KEY              DeepSeek API key (required for MCP mode)
  ADVISOR_MODEL                 Model name (default: deepseek-v4-pro)
  ADVISOR_API_BASE              API base URL (default: https://api.deepseek.com/v1)
  ADVISOR_DB_PATH               SQLite database path (default: ./advisor.db)
  ADVISOR_DASHBOARD_PORT        Dashboard HTTP port (default: 9753)
  ADVISOR_DASHBOARD_HOST        Dashboard bind host (default: 10.64.0.5)
  ADVISOR_DASHBOARD_URL         Dashboard endpoint for MCP publish (default: http://10.64.0.5:9753; empty disables)
  ADVISOR_DASHBOARD_EMBED       Set to "1" to embed the dashboard in MCP mode (dev only)
  ADVISOR_DASHBOARD_DISABLE     Set to "1" to disable the dashboard (compat)
  ADVISOR_SESSION_LABEL         Human-readable session label (default: <workdir>-<pid>)
  ADVISOR_TIMEOUT_SECONDS      Per-DeepSeek-call timeout (default: 300)
  ADVISOR_DEFAULT_EFFORT       Default reasoning_effort when unset (default: medium)
  ADVISOR_SERIALIZE            "0" disables cross-process call serialization`)
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

func handleConsult(args map[string]any, client *DeepSeekClient, progressToken any) ToolCallResult {
	question, _ := args["question"].(string)
	if question == "" {
		return errorResult("'question' is required")
	}

	effort := getArg(args, "reasoning_effort", defaultEffort())
	log.Printf("consult — effort=%s", effort)
	sendProgress(progressToken, 0.1, 1, "Consulting deepseek-v4-pro...")

	system := fmt.Sprintf(advisorSystemPreamble, effort)
	result, err := client.ChatWithEffort(system, question, 0.1, effort)
	if err != nil {
		log.Printf("consult error: %v", err)
		return errorResult(fmt.Sprintf("Consult failed: %v", err))
	}

	persister.CaptureConsult(currentSessionID, question, result)

	sendProgress(progressToken, 1, 1, "Consult complete")
	log.Printf("consult — %d tokens, $%.6f, %dms", result.TotalTokens,
		float64(result.PromptTokens+result.CompletionTokens)/1_000_000*0.435, result.DurationMs)
	return textResult(result.Text)
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

	effort := getArg(args, "reasoning_effort", defaultEffort())
	log.Printf("expert-review — %s, effort=%s", expertKey, effort)
	sendProgress(progressToken, 0.1, 1, fmt.Sprintf("Consulting %s...", expert.Name))

	system := fmt.Sprintf("%s\n\nReasoning effort: %s\nProvide structured analysis with severity ratings and concrete recommendations.", expert.SystemPrompt, effort)
	result, err := client.ChatWithEffort(system, context, 0.15, effort)
	if err != nil {
		log.Printf("expert-review %s error: %v", expertKey, err)
		return errorResult(fmt.Sprintf("Review failed: %v", err))
	}

	persister.CaptureExpertReview(currentSessionID, expertKey, context, result)

	sendProgress(progressToken, 1, 1, fmt.Sprintf("%s review complete", expert.Name))
	log.Printf("expert-review %s — %d tokens, %dms", expertKey, result.TotalTokens, result.DurationMs)
	return textResult(result.Text)
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

	effort := getArg(args, "reasoning_effort", defaultEffort())
	total := float64(len(selectedExperts) + 1) // experts + synthesis
	log.Printf("multi-perspective — %d experts: %s, effort=%s", len(selectedExperts), strings.Join(selectedExperts, ", "), effort)

	// Step 1: Run each expert (sequentially to respect API rate limits)
	type contribution struct {
		Key    string
		Result *ChatResult
	}

	var contributions []contribution
	for i, key := range selectedExperts {
		expert := experts[key]
		log.Printf("multi-perspective — consulting %s (%d/%d)...", expert.Name, i+1, len(selectedExperts))
		sendProgress(progressToken, float64(i)/total, total, fmt.Sprintf("Consulting %s...", expert.Name))

		system := fmt.Sprintf("%s\n\nReasoning effort: %s\nYou are one of %d experts reviewing this. Focus solely on your domain. Do not defer to or repeat other perspectives.",
			expert.SystemPrompt, effort, len(selectedExperts))

		result, err := client.ChatWithEffort(system, context, 0.15, effort)
		if err != nil {
			log.Printf("multi-perspective %s error: %v", key, err)
			result = &ChatResult{Text: fmt.Sprintf("[Error consulting %s: %v]", expert.Name, err), Model: client.model}
		}
		log.Printf("multi-perspective — %s: got %d bytes", key, len(result.Text))
		contributions = append(contributions, contribution{Key: key, Result: result})
	}

	// Step 2: Build synthesis input
	log.Print("multi-perspective — synthesizing...")
	sendProgress(progressToken, float64(len(selectedExperts))/total, total, "Synthesizing expert perspectives...")

	var parts []string
	for _, c := range contributions {
		parts = append(parts, fmt.Sprintf("=== %s (%s) ===\n%s", experts[c.Key].Name, c.Key, c.Result.Text))
	}

	synthesisInput := fmt.Sprintf("Synthesize the following expert perspectives into a unified recommendation.\n\nIdentify:\n  1. Areas of agreement\n  2. Areas of disagreement\n  3. Critical findings\n  4. Final recommendation\n\n%s", strings.Join(parts, "\n\n"))

	synthesisResult, err := client.ChatWithEffort(synthesisSystemPrompt, synthesisInput, 0.2, effort)
	if err != nil {
		log.Printf("synthesis error: %v", err)
		synthesisResult = &ChatResult{Text: fmt.Sprintf("[Synthesis failed: %v]", err), Model: client.model}
	}
	synthesis := synthesisResult.Text

	sendProgress(progressToken, total, total, "Multi-perspective analysis complete")
	log.Print("multi-perspective — complete")

	// Build output
	var out strings.Builder
	for _, c := range contributions {
		fmt.Fprintf(&out, "═══════════════════════════════════════\n")
		fmt.Fprintf(&out, "  🧠 %s\n", experts[c.Key].Name)
		fmt.Fprintf(&out, "═══════════════════════════════════════\n")
		out.WriteString(c.Result.Text)
		out.WriteString("\n\n")
	}
	fmt.Fprintf(&out, "═══════════════════════════════════════\n")
	fmt.Fprintf(&out, "  📋 SYNTHESIS\n")
	fmt.Fprintf(&out, "═══════════════════════════════════════\n")
	out.WriteString(synthesis)

	// Persist the deliberation
	if persister != nil {
		var cons []ContributionResult
		for _, c := range contributions {
			cons = append(cons, ContributionResult{ExpertKey: c.Key, Result: c.Result})
		}
		persister.CaptureDeliberation(currentSessionID, context, synthesis, selectedExperts, effort, cons, synthesisResult)
	}

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
func defaultEffort() string {
	// Default reasoning effort for calls that don't specify one. "medium" is
	// a genuine middle tier (see effortConfig) that stays well under the
	// client timeout while keeping enough reasoning budget for real analysis.
	// ADVISOR_DEFAULT_EFFORT overrides it.
	if v := os.Getenv("ADVISOR_DEFAULT_EFFORT"); v != "" {
		return v
	}
	return "medium"
}
