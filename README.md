# Frao Advisor MCP

Expert review, multi-perspective analysis, and second opinions for Claude Code — powered by **DeepSeek V4 Pro** directly. No OpenRouter, no extra costs.

Built in **Go** from deliberation's proven architecture, porting the expert persona system, consensus patterns, and MCP interface to a zero-dependency binary that calls DeepSeek's native API.

## Architecture

```
frao-advisor/
├── main.go        # MCP server — JSON-RPC 2.0 over stdio, tool dispatch + `dashboard` subcommand
├── persistence.go # Local SQLite writes + best-effort publishing to the dashboard
├── deepseek.go    # DeepSeek API client — OpenAI-compatible /chat/completions
├── personas.go    # Expert personas — 7 domain specialists with system prompts
├── types.go       # Shared types — JSON-RPC, MCP, and API message shapes
├── dashboard/     # Standalone dashboard service — HTTP UI, ingest API, views
├── db/            # SQLite persistence + ingest (Event, IngestEvent, aggregation queries)
├── publish/       # MCP-side best-effort HTTP publisher (fire-and-forget)
└── README.md      # This file
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

### Dashboard (standalone service)

The dashboard is a **separate process** that aggregates advice records and
cost across all Claude sessions. Each MCP process keeps writing its own local
SQLite file **and** publishes every advice record to the dashboard over HTTP.

```bash
# Start the dashboard (binds 10.64.0.5:9753; override with ADVISOR_DASHBOARD_HOST)
make run-dashboard          # or: ./frao-advisor dashboard
```

MCP processes publish to it automatically via `ADVISOR_DASHBOARD_URL`
(default `http://10.64.0.5:9753`).

**Publishing is best-effort and never blocks the MCP tool call.** If the
dashboard is down or unreachable, the MCP logs the failure and proceeds — its
main task is returning advice. Set `ADVISOR_DASHBOARD_URL=""` to disable
publishing entirely.

Dashboard API:
- `POST /api/events` — accept an advice record (consultation / expert_review / deliberation). Idempotent by record ID.
- `GET  /api/health` — liveness check.

To preserve existing history, point the dashboard at the existing DB:
`ADVISOR_DB_PATH=/path/to/advisor.db ./frao-advisor dashboard`

#### Docker (single instance)

The dashboard can run as the one containerized instance:

```bash
make docker-up-dashboard     # build the image + start the single dashboard
make docker-ps-dashboard     # confirm it is healthy
make docker-logs-dashboard   # tail the logs
make docker-seed-dashboard   # copy your existing advisor.db into ./data once (preserves history)
make docker-down-dashboard   # stop it
```

- Binds `10.64.0.5:9753` on the host; the SQLite DB lives in `./data/advisor.db`
  (bind-mounted) so it survives container recreates.
- `container_name: frao-advisor-dashboard` + `restart: unless-stopped` guarantee
  exactly one dashboard instance at any time.
- The container runs non-root; `APP_UID: 1337` in the compose matches this host's
  user UID so the process can write the mounted DB. Adjust it on another host.
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
| `DEEPSEEK_API_KEY` | — | DeepSeek API key (required for MCP mode) |
| `ANTHROPIC_AUTH_TOKEN` | — | Fallback if `DEEPSEEK_API_KEY` is unset |
| `ADVISOR_MODEL` | `deepseek-v4-pro` | Model to use for all advisor calls |
| `ADVISOR_API_BASE` | `https://api.deepseek.com/v1` | API base URL |
| `ADVISOR_DB_PATH` | `./advisor.db` | SQLite path; for the dashboard, point at the existing DB to preserve history |
| `ADVISOR_DASHBOARD_URL` | `http://10.64.0.5:9753` | Dashboard endpoint MCP publishes to; `""` disables |
| `ADVISOR_DASHBOARD_HOST` | `10.64.0.5` | Dashboard bind host |
| `ADVISOR_DASHBOARD_PORT` | `9753` | Dashboard HTTP port |
| `ADVISOR_DASHBOARD_EMBED` | unset | `1` re-embeds the dashboard in MCP mode (dev only) |
| `ADVISOR_SESSION_LABEL` | `<workdir>-<pid>` | Human-readable label attributing usage to a session |

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
