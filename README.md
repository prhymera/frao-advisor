# Frao Advisor MCP

Expert review, multi-perspective analysis, and second opinions for Claude Code — powered by **DeepSeek V4 Pro** directly. No OpenRouter, no extra costs.

Built in **Go** from deliberation's proven architecture, porting the expert persona system, consensus patterns, and MCP interface to a zero-dependency binary that calls DeepSeek's native API.

## Architecture

```
frao-advisor/
├── main.go        # MCP server — JSON-RPC 2.0 over stdio, tool dispatch
├── deepseek.go    # DeepSeek API client — OpenAI-compatible /chat/completions
├── personas.go    # Expert personas — 7 domain specialists with system prompts
├── types.go       # Shared types — JSON-RPC, MCP, and API message shapes
├── README.md      # This file
```

The binary implements the **Model Context Protocol (MCP)** — it speaks JSON-RPC 2.0 over stdin/stdout, so Claude Code launches it as a subprocess and communicates line-by-line.

### Ported from deliberation

The core patterns come from [antonbabenko/deliberation](https://github.com/antonbabenko/deliberation) (120★, 62 releases):

| Deliberation Component | Frao Advisor Equivalent |
|------------------------|------------------------|
| Expert personas (7) | Same 7, adapted to Go |
| `openai-compatible.js` provider | `deepseek.go` — direct DeepSeek API |
| `orchestrate.js` askAll/askOne | `frao-consult` + `frao-expert-review` |
| `orchestrate.js` consensus | `frao-multi-perspective` (run N experts + synthesize) |
| `server/mcp/index.js` | `main.go` — MCP stdio server |
| `config.json` | Environment variables |

What we **don't** need from deliberation:
- OpenRouter (we call DeepSeek directly)
- Codex CLI / Antigravity CLI / Grok (single-provider focus)
- Multi-model consensus (same model, different personas)
- Session persistence (stateless by design)

## Tools

| Tool | Description |
|------|-------------|
| `frao-expert-list` | List all 7 expert personas |
| `frao-consult` | Second opinion from deepseek-v4-pro on any question |
| `frao-expert-review` | Review code/architecture through a specific expert lens |
| `frao-multi-perspective` | Analyze from N expert angles, then synthesize |

### Expert Personas

| Key | Role | Best For |
|-----|------|----------|
| `architect` | Solutions Architect | System design, coupling, scalability |
| `code-reviewer` | Senior Code Reviewer | Correctness, bugs, maintainability |
| `security-analyst` | Security Engineer | Vulnerabilities, threat modeling |
| `debugger` | Debugging Specialist | Root-cause analysis, crash investigations |
| `tech-lead` | Technical Lead | Consistency, risk, delivery decisions |
| `scope-analyst` | Scope Analyst | Requirements, ambiguities, edge cases |
| `researcher` | Technical Researcher | Libraries, APIs, best practices |

## Usage

### Prerequisites

- A **DeepSeek API key** with access to `deepseek-v4-pro`
- Go 1.26+ (to build), or use the prebuilt binary

### Build

```bash
cd tools/frao-advisor
go build -o frao-advisor .
```

### Run (standalone test)

```bash
DEEPSEEK_API_KEY="sk-..." ./frao-advisor
```

It listens on stdin for JSON-RPC messages and writes responses to stdout.

### Claude Code Integration

Add to your MCP server config (`~/.claude.json` or `.mcp.json`):

```json
{
  "mcpServers": {
    "frao-advisor": {
      "command": "/path/to/frao-advisor",
      "args": [],
      "env": {
        "DEEPSEEK_API_KEY": "sk-...",
        "ADVISOR_MODEL": "deepseek-v4-pro"
      }
    }
  }
}
```

Or use the CLI:

```bash
claude mcp add frao-advisor -- /path/to/frao-advisor
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DEEPSEEK_API_KEY` | — | DeepSeek API key (required) |
| `ANTHROPIC_AUTH_TOKEN` | — | Fallback if `DEEPSEEK_API_KEY` is unset |
| `ADVISOR_MODEL` | `deepseek-v4-pro` | Model to use for all advisor calls |
| `ADVISOR_API_BASE` | `https://api.deepseek.com/v1` | API base URL |

## Examples

```
frao-consult: "Review this nginx config for security issues"
frao-expert-review: expert=architect "Should I split the gateway into two services?"
frao-multi-perspective: experts=[architect,security-analyst,tech-lead] "Review the new deployment topology"
```

## Deliberation Fork

The deliberation fork lives at [github.com/prhymera/frao-deliberation](https://github.com/prhymera/frao-deliberation) and tracks upstream. It is kept for reference; the Go-based frao-advisor is the active implementation.

## License

MIT — see [LICENSE](LICENSE) (same as deliberation).
