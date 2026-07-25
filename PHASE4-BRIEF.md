# Frao Advisor Dashboard — Phase 4: Charts & Data Visualization

> **Mission**: Audit, refine, and complete all chart visualizations in the Datastar dashboard.
> **State**: Phase 3 built the full dashboard skeleton (server, handlers, render, templates, CSS). Charts exist but may need polishing.
> **Branch**: `phase4-charts` (this worktree)
> **Container**: Fresh Claude session — zero context bleed.

---

## 1. Current State

The dashboard package at `dashboard/` contains:
```
dashboard/
├── server.go            HTTP server + routing + Start()
├── handlers.go          6 SSE handlers (overview, timeline, experts, deliberations, costs, metrics)
├── render.go            HTML fragment + Chart.js chart builders
├── static/style.css     Dark theme CSS
└── templates/layout.gohtml  Base layout with Datastar CDN + Chart.js
```

The `db/` package has all needed queries:
- `GetOverviewMetrics()` → overview KPIs + daily cost series
- `ExpertBreakdown()` → per-expert usage/cost/tokens
- `CostByModel()` → cost per model
- `CostByType()` → cost per type (consult/review/deliberation)
- `UsageHeatmap()` → 7×24 time-of-day heatmap
- `Timeline()` → paginated advice list
- `RecentActivity()` → last N entries

**Build status**: `go build` and `go vet` both pass cleanly.

## 2. What to Build

Audit each chart implementation in `render.go` and `handlers.go`. For each, verify:

### Chart Audit Checklist

| Chart | Location | Must Have |
|-------|----------|-----------|
| Cost sparkline | Overview handler | 30-day trend, responsive canvas, proper destroy/recreate |
| Expert usage bar | Experts handler | Per-expert count, sorted descending, color-coded |
| Expert cost pie | Experts handler | Doughnut with percentage labels |
| Cost-over-time line | Costs handler | Daily/weekly aggregation, smooth line |
| Cost-by-model pie | Costs handler | v4-pro vs v4-flash split |
| Cost-by-type bar | Costs handler | Consultations vs reviews vs deliberations |
| Budget gauge | Costs handler | Current spend vs projected, colored zones |
| Usage heatmap | Metrics handler | 7×24 grid, color intensity scale |
| Avg latency line | Metrics handler | Response time trend |

### For each chart, ensure:

1. **Responsive sizing** — `responsive: true`, `maintainAspectRatio: true`, canvas resizes with window
2. **Dark theme** — Chart.js defaults use `color: '#94a3b8'` (slate), grid lines `#1e293b`, tooltips dark
3. **Destroy/recreate** — On SSE patch, destroy old chart instance before creating new one (IIFE pattern)
4. **Empty data** — If no data, show a placeholder message instead of a blank canvas
5. **Error handling** — If data fetch fails, show error state
6. **Animation** — `animation.duration: 800`, easing `easeOutQuart` for polish
7. **Tooltips** — Proper labels, values formatted ($ for cost, % for CPU, number for count)

### Chart.js IIFE Pattern (must use this)

```javascript
(function(){
  var ctx = document.getElementById('chart-id');
  if (!ctx) return;
  var existing = Chart.getChart(ctx);
  if (existing) existing.destroy();
  new Chart(ctx, {
    type: '...',
    data: { ... },
    options: {
      responsive: true,
      maintainAspectRatio: true,
      plugins: {
        legend: { labels: { color: '#94a3b8' } },
        tooltip: { backgroundColor: '#1e293b', titleColor: '#f1f5f9', bodyColor: '#94a3b8' }
      },
      scales: {
        x: { ticks: { color: '#64748b' }, grid: { color: '#1e293b' } },
        y: { ticks: { color: '#64748b' }, grid: { color: '#1e293b' } }
      }
    }
  });
})();
```

## 3. Design Spec

- **Palette**: Same dark theme from Phase 3 (bg #0f172a, card #1e293b, text #94a3b8)
- **Chart colors**: 5-color categorical palette: `#06b6d4` (cyan), `#f59e0b` (amber), `#8b5cf6` (violet), `#10b981` (emerald), `#f43f5e` (rose)
- **Sequential (heatmap)**: Single-hue blue scale from `#0c4a6e` to `#38bdf8`
- **Gauge**: Red-Yellow-Green gradient: `#f43f5e` → `#f59e0b` → `#10b981`
- **Typography**: Chart labels in `'JetBrains Mono', monospace` for numbers, `system-ui` for category labels
- **Formatting**: Costs as `$0.00`, token counts with commas, percentages as `0.0%`

## 4. Step-by-Step

1. Read `dashboard/render.go` fully — understand current chart implementations
2. Read `dashboard/handlers.go` — understand data flow to charts
3. Fix each chart following the audit checklist
4. Add missing chart types (gauge, heatmap if not present)
5. Verify responsive behavior
6. Run: `go build ./... && go vet ./...`

## 5. Do NOT

- Modify `db/` package or add new queries (already complete)
- Modify MCP handlers in `main.go`
- Change the `server.go` routing
- Add npm packages or build tools
- Use `templ` — stick with stdlib `html/template`

## 6. Completion Protocol

```
git add -A
echo "=== PHASE_COMPLETE ==="
```

Then print a summary of what was fixed/changed. Do NOT commit.

---

## ⚠️ MANDATORY: Self-Review Using Advisor Tools

The frao-advisor MCP server is **globally registered**. Run `/advisor-on` at the start of your session to ensure the protocol is active.

### Three Checkpoints

1. **After planning** → `frao-expert-review` (architect): validate chart architecture
2. **After each file** → `frao-expert-review` (code-reviewer): verify correctness
3. **Before completion** → `frao-consult`: final second opinion

CRITICAL/HIGH findings block completion — fix them.
