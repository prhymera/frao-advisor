# Frao Advisor Dashboard — Phase 5: Polish & Mobile Responsiveness

> **Mission**: Make the dashboard flawless on mobile, accessible, and production-polished.
> **State**: Phases 1-4 complete — DB, cost engine, persistence, dashboard server, 6 views, charts.
> **Branch**: `phase5-polish` (this worktree)
> **Container**: Fresh Claude session — zero context bleed.

---

## 1. Current State

```
dashboard/
├── server.go            HTTP server + routing + Start()
├── handlers.go          6 SSE handlers with latency data
├── render.go            HTML + Chart.js builders, 8-color palette
├── static/style.css     Dark theme CSS
└── templates/layout.gohtml  Layout + FraoDashboard JS framework
```

Build: `go build` and `go vet` pass cleanly.

## 2. What to Build

### 2.1 Mobile Responsiveness
The dashboard must work flawlessly on **mobile** (375px width) and **desktop** (1920px).

- **Navigation**: Sidebar (desktop) → bottom tab bar (mobile) at breakpoint 768px
- **KPI cards**: 4-column grid → 2-column → 1-column as viewport shrinks
- **Charts**: Canvas charts must resize with the container (`responsive: true` already set, verify)
- **Tables**: Horizontal scroll on small screens, or collapse to cards
- **Touch targets**: Nav items minimum 44×44px, adequate spacing
- **Font sizes**: No less than 14px on mobile, headings scale appropriately
- **Margins/padding**: Consistent spacing that doesn't overflow on small screens

### 2.2 States Verification
Audit every view for these four states:

| State | What to Show |
|-------|-------------|
| **Loading** | `"Loading dashboard..."` while SSE fetches first view |
| **Empty** | Per-view message (e.g., "No advice recorded yet") |
| **Error** | Warning icon + error detail + reload/retry link |
| **Data** | Full rendering with charts, tables, and metrics |

### 2.3 Accessibility
- Keyboard navigation: all interactive elements reachable via Tab, visible focus rings
- ARIA labels on navigation items, chart canvases, buttons
- Color contrast: all text meets WCAG AA (4.5:1 ratio)
- `prefers-reduced-motion`: disable chart animations when set
- Screen reader friendly headings (h1 → h6 hierarchy)

### 2.4 Data Export
Add a CSV export button to the Metrics view:
- Button labeled "Export CSV" in the metrics toolbar
- Fetches `GET /export/csv` (add this route to server.go)
- CSV includes: date, type, expert, model, tokens, cost, duration
- Triggers browser download

### 2.5 Polish
- Consistent card shadows and border-radius across all views
- Hover states on interactive elements (nav items, buttons, table rows)
- Smooth transitions on navigation changes
- Loading shimmer/placeholder animation
- Favicon (inline SVG data URI, no external file)
- Page title: "Frao Advisor Dashboard"

## 3. Design Spec

- Same dark theme from Phases 3-4
- KPI cards: subtle `box-shadow: 0 1px 3px rgba(0,0,0,0.3)`, `border-radius: 8px`
- Bottom nav (mobile): `position: fixed`, `bottom: 0`, `z-index: 100`, bg `#1e293b`
- Focus ring: `outline: 2px solid #06b6d4`, `outline-offset: 2px`
- Export button: Secondary style, download icon, positioned top-right of metrics pane

## 4. Step-by-Step

1. Review `static/style.css` — add mobile responsive rules
2. Verify all 4 states (loading, empty, error, data) for each view
3. Add keyboard navigation + focus styles
4. Add `prefers-reduced-motion` support
5. Add CSV export route + button
6. Polish cards, hover states, transitions
7. Test at 375px, 768px, 1920px widths
8. Run: `go build ./... && go vet ./...`

## 5. Do NOT

- Add npm packages or external CSS frameworks
- Modify DB schema or queries
- Change chart implementation (Phase 4 is done)
- Use JavaScript build tools

---

## 6. MANDATORY: Final Step

When all work is complete and advisor checkpoints passed, run:

```bash
git add -A && echo '=== PHASE_COMPLETE ==='
```

---

## ⚠️ MANDATORY: Self-Review Using Advisor Tools

Run `/advisor-on` at session start. Three checkpoints:
1. **After planning** → `frao-expert-review` (architect): validate polish approach
2. **After CSS/responsive work** → `frao-expert-review` (code-reviewer): verify correctness
3. **Before completion** → `frao-consult`: final second opinion

CRITICAL/HIGH findings block completion.
