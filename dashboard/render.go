package dashboard

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"strings"
	"time"

	"github.com/prhymera/frao-advisor/db"
)

// ══════════════════════════════════════════════════════════════
// FORMATTING HELPERS
// ══════════════════════════════════════════════════════════════

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b = append(b, ',')
		}
		b = append(b, byte(c))
	}
	return string(b)
}

func formatCost(c float64) string {
	if c < 0.01 {
		return fmt.Sprintf("$%.4f", c)
	}
	if c < 100 {
		return fmt.Sprintf("$%.2f", c)
	}
	return fmt.Sprintf("$%.2f", c)
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%.1fm", float64(ms)/60000)
}

func dayLabel(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.Format("Jan 2")
}

func dailyValues(series []db.DayCost) []float64 {
	v := make([]float64, len(series))
	for i, d := range series {
		v[i] = d.Cost
	}
	return v
}

func dailyLabels(series []db.DayCost) []string {
	l := make([]string, len(series))
	for i, d := range series {
		l[i] = dayLabel(d.Date)
	}
	return l
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// shortModel returns a concise label for known models.
func shortModel(m string) string {
	switch m {
	case "deepseek-v4-pro":
		return "DS V4 Pro"
	case "deepseek-v4-flash":
		return "DS Flash"
	default:
		return m
	}
}

// badgeLabel returns a short display label for timeline entry types.
func badgeLabel(t string) string {
	switch t {
	case "consultation":
		return "Consult"
	case "expert_review":
		return "Review"
	case "deliberation":
		return "MP"
	default:
		return t
	}
}

// friendlyTime converts an ISO timestamp to a relative human-readable form.
func friendlyTime(iso string) string {
	t, err := time.Parse("2006-01-02T15:04:05Z", iso)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", iso)
		if err != nil {
			return iso
		}
	}
	now := time.Now().UTC()
	diff := now.Sub(t)
	if diff < 1*time.Minute {
		return "just now"
	}
	if diff < 1*time.Hour {
		m := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", m)
	}
	if diff < 24*time.Hour {
		return t.Format("15:04")
	}
	if diff < 7*24*time.Hour {
		return t.Format("Mon 15:04")
	}
	return t.Format("Jan 2")
}

// ─── Chart Script Helper ───

// chartScript returns a self-executing function that waits for FraoDashboard
// to be available and then calls fn. This is used in SSE-patched fragments
// to initialize Chart.js charts after the DOM is updated.
func chartScript(fn string) string {
	return `<script>(function(){var c=function(){if(window.FraoDashboard){` + fn + `}else{setTimeout(c,10)}};c()})();</script>`
}

// ══════════════════════════════════════════════════════════════
// KPI CARD
// ══════════════════════════════════════════════════════════════

func kpiCard(id, label, value, subtext, accent string) string {
	var b strings.Builder
	b.WriteString(`<div class="kpi-card" id="` + id + `" style="border-left-color:` + accent + `">`)
	b.WriteString(`<div class="kpi-label">` + html.EscapeString(label) + `</div>`)
	b.WriteString(`<div class="kpi-value">` + value + `</div>`)
	if subtext != "" {
		b.WriteString(`<div class="kpi-subtext">` + subtext + `</div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func statItem(label, value string) string {
	return `<div class="stat-item"><span class="stat-label">` + html.EscapeString(label) + `</span><span class="stat-value">` + value + `</span></div>`
}

func metricCard(label, value string) string {
	return `<div class="metric-card"><div class="metric-value">` + value + `</div><div class="metric-label">` + html.EscapeString(label) + `</div></div>`
}

// ─── Cost Change Indicator ───

func costChangeHTML(today, yesterday, pct float64) string {
	if today <= 0 && yesterday <= 0 {
		return ""
	}
	if yesterday <= 0 {
		return `<span class="cost-change">Today: ` + formatCost(today) + `</span>`
	}
	arrow := "↑"
	color := "#ef4444"
	if pct < 0 {
		arrow = "↓"
		color = "#10b981"
	}
	abs := math.Abs(pct)
	return fmt.Sprintf(`<span class="cost-change">Today: %s <span style="color:%s">%s %.1f%%</span></span>`,
		formatCost(today), color, arrow, abs)
}

// ══════════════════════════════════════════════════════════════
// OVERVIEW
// ══════════════════════════════════════════════════════════════

func renderOverviewContent(m *db.OverviewMetrics) string {
	var b strings.Builder

	// KPI grid
	b.WriteString(`<div class="kpi-grid">`)
	b.WriteString(kpiCard("kpi-total", "Total Advice", formatNumber(m.TotalAdvice), "", "#06b6d4"))
	b.WriteString(kpiCard("kpi-cost", "Total Cost", formatCost(m.TotalCost),
		fmt.Sprintf("$%.4f avg/call", m.AvgCostPerCall), "#f59e0b"))
	b.WriteString(kpiCard("kpi-sessions", "Active Sessions", formatNumber(m.ActiveSessions),
		"last hour", "#10b981"))
	b.WriteString(kpiCard("kpi-tokens", "Total Tokens", formatNumber(m.TotalTokens), "", "#8b5cf6"))
	b.WriteString(`</div>`)

	// Cost sparkline card
	b.WriteString(`<div class="chart-row"><div class="card">`)
	b.WriteString(`<div class="card-header"><span>Cost Trend (30 days)</span>`)
	b.WriteString(costChangeHTML(m.CostToday, m.CostYesterday, m.CostChangePct))
	b.WriteString(`</div>`)
	b.WriteString(`<div class="chart-container"><canvas id="cost-sparkline" height="120"></canvas></div>`)
	b.WriteString(`</div></div>`)

	// Sparkline chart script
	vals := dailyValues(m.DailyCostSeries)
	valJSON, _ := json.Marshal(vals)
	b.WriteString(chartScript(`FraoDashboard.sparkline('cost-sparkline',` + string(valJSON) + `);`))

	// Quick stats row
	b.WriteString(`<div class="quick-stats">`)
	b.WriteString(statItem("Consultations", formatNumber(m.Consultations)))
	b.WriteString(statItem("Expert Reviews", formatNumber(m.ExpertReviews)))
	b.WriteString(statItem("Deliberations", formatNumber(m.Deliberations)))
	b.WriteString(`</div>`)

	return b.String()
}

// ══════════════════════════════════════════════════════════════
// TIMELINE
// ══════════════════════════════════════════════════════════════

func renderFilterBar(typeFilter, expertFilter string) string {
	var b strings.Builder
	b.WriteString(`<div class="filter-bar">`)
	b.WriteString(`<select data-bind="typeFilter" data-on-change="$$set('offset',0);$$get('/dashboard/timeline')">`)
	type opts struct {
		value, label string
	}
	for _, o := range []opts{
		{"all", "All Types"},
		{"consultation", "Consultations"},
		{"expert_review", "Expert Reviews"},
		{"deliberation", "Deliberations"},
	} {
		sel := ""
		if typeFilter == o.value {
			sel = " selected"
		}
		b.WriteString(`<option value="` + o.value + `"` + sel + `>` + o.label + `</option>`)
	}
	b.WriteString(`</select>`)
	b.WriteString(`</div>`)
	return b.String()
}

func renderTimelineContent(entries []db.TimelineEntry, total, limit, offset int, typeFilter, expertFilter string) string {
	var b strings.Builder

	if len(entries) == 0 {
		b.WriteString(renderFilterBar(typeFilter, expertFilter))
		b.WriteString(`<div class="empty-state" style="padding:40px 20px"><p class="empty-desc">No entries match your filters.</p></div>`)
		return b.String()
	}

	b.WriteString(renderFilterBar(typeFilter, expertFilter))
	b.WriteString(`<div class="timeline-list">`)

	for _, e := range entries {
		b.WriteString(`<div class="timeline-entry">`)
		b.WriteString(`<span class="timeline-badge badge-` + e.Type + `">` + badgeLabel(e.Type) + `</span>`)
		b.WriteString(`<div class="timeline-body">`)
		b.WriteString(`<div class="timeline-summary">` + html.EscapeString(truncate(e.Summary, 120)) + `</div>`)
		b.WriteString(`<div class="timeline-meta">`)
		b.WriteString(`<span>` + shortModel(e.Model) + `</span>`)
		b.WriteString(`<span>Cost: ` + formatCost(e.Cost) + `</span>`)
		b.WriteString(`<span>` + formatNumber(e.Tokens) + ` tok</span>`)
		if e.DurationMs > 0 {
			b.WriteString(`<span>` + formatDuration(e.DurationMs) + `</span>`)
		}
		if e.ExpertKey != "" {
			b.WriteString(`<span>` + html.EscapeString(e.ExpertKey) + `</span>`)
		}
		b.WriteString(`</div>`)
		b.WriteString(`</div>`)
		b.WriteString(`<span class="timeline-time">` + friendlyTime(e.CreatedAt) + `</span>`)
		b.WriteString(`</div>`)
	}

	b.WriteString(`</div>`)

	// Pagination
	if total > limit {
		currentPage := (offset / limit) + 1
		totalPages := (total + limit - 1) / limit
		if totalPages == 0 {
			totalPages = 1
		}

		b.WriteString(`<div class="pagination">`)
		if currentPage > 1 {
			prev := offset - limit
			if prev < 0 {
				prev = 0
			}
			b.WriteString(`<a href="#" data-on-click="$$get('/dashboard/timeline?offset=` +
				fmt.Sprintf("%d", prev) + `')">← Prev</a>`)
		} else {
			b.WriteString(`<button disabled>← Prev</button>`)
		}
		b.WriteString(`<span>Page ` + fmt.Sprintf("%d/%d", currentPage, totalPages) + `</span>`)
		if offset+limit < total {
			next := offset + limit
			b.WriteString(`<a href="#" data-on-click="$$get('/dashboard/timeline?offset=` +
				fmt.Sprintf("%d", next) + `')">Next →</a>`)
		} else {
			b.WriteString(`<button disabled>Next →</button>`)
		}
		b.WriteString(`</div>`)
	}

	return b.String()
}

// filterByType filters a timeline entry slice in-memory.
func filterByType(entries []db.TimelineEntry, typ string) []db.TimelineEntry {
	if typ == "" || typ == "all" {
		return entries
	}
	var out []db.TimelineEntry
	for _, e := range entries {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

// ══════════════════════════════════════════════════════════════
// EXPERTS
// ══════════════════════════════════════════════════════════════

func renderExpertsContent(stats []db.ExpertStat) string {
	var b strings.Builder

	if len(stats) == 0 {
		return emptyState("No expert reviews recorded yet.")
	}

	// Extract data for charts
	var keys, costVals, countVals []string
	for _, s := range stats {
		keys = append(keys, s.ExpertKey)
		costVals = append(costVals, fmt.Sprintf("%.6f", s.TotalCost))
		countVals = append(countVals, fmt.Sprintf("%d", s.Count))
	}
	keysJSON, _ := json.Marshal(keys)
	costJSON, _ := json.Marshal(costVals)
	countJSON, _ := json.Marshal(countVals)

	// Charts row
	b.WriteString(`<div class="chart-grid-2">`)
	b.WriteString(`<div class="card"><div class="card-header">Cost by Expert</div>`)
	b.WriteString(`<div class="chart-container"><canvas id="expert-bar" height="200"></canvas></div></div>`)
	b.WriteString(`<div class="card"><div class="card-header">Count by Expert</div>`)
	b.WriteString(`<div class="chart-container"><canvas id="expert-pie" height="200"></canvas></div></div>`)
	b.WriteString(`</div>`)
	b.WriteString(chartScript(`FraoDashboard.bar('expert-bar',` + string(keysJSON) + `,` + string(costJSON) + `,'Cost ($)');FraoDashboard.pie('expert-pie',` + string(keysJSON) + `,` + string(countJSON) + `);`))

	// Table
	b.WriteString(`<div class="card"><div class="card-header">Expert Breakdown</div>`)
	b.WriteString(`<div class="table-container"><table>`)
	b.WriteString(`<thead><tr><th>Expert</th><th>Count</th><th>Total Cost</th><th>Avg Tokens</th><th>Avg Duration</th></tr></thead><tbody>`)
	for _, s := range stats {
		b.WriteString(`<tr>`)
		b.WriteString(`<td>` + html.EscapeString(s.ExpertKey) + `</td>`)
		b.WriteString(`<td>` + formatNumber(s.Count) + `</td>`)
		b.WriteString(`<td>` + formatCost(s.TotalCost) + `</td>`)
		b.WriteString(`<td>` + formatNumber(int(s.AvgTokens)) + `</td>`)
		b.WriteString(`<td>` + formatDuration(int64(s.AvgDurationMs)) + `</td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	b.WriteString(`</div>`)

	return b.String()
}

// ══════════════════════════════════════════════════════════════
// DELIBERATIONS
// ══════════════════════════════════════════════════════════════

func renderDeliberationsContent(delibs []db.Deliberation, total int) string {
	var b strings.Builder

	if len(delibs) == 0 {
		return emptyState("No deliberations recorded yet.")
	}

	for _, d := range delibs {
		sigName := "delib_" + d.ID

		b.WriteString(`<div class="delib-item" data-signals="` + sigName + `:false">`)

		// Clickable header toggles the signal
		b.WriteString(`<div class="delib-header" data-on-click="$$toggle('` + sigName + `')">`)
		b.WriteString(`<span class="delib-title">` + html.EscapeString(truncate(d.Context, 100)) + `</span>`)
		b.WriteString(`<span class="delib-meta">`)
		b.WriteString(`<span>` + formatNumber(d.ExpertCount) + ` experts</span>`)
		b.WriteString(`<span>` + formatCost(d.TotalCost()) + `</span>`)
		b.WriteString(`<span>` + formatDuration(d.DurationMs) + `</span>`)
		b.WriteString(`<span>` + friendlyTime(d.CreatedAt) + `</span>`)
		b.WriteString(`</span>`)
		b.WriteString(`<span class="delib-toggle" data-class-open="` + sigName + `">&#9654;</span>`)
		b.WriteString(`</div>`)

		// Collapsible body
		b.WriteString(`<div class="delib-body" data-class-open="` + sigName + `">`)
		b.WriteString(`<div class="delib-synthesis">` + html.EscapeString(truncate(d.Synthesis, 400)) + `</div>`)
		b.WriteString(`<div class="delib-experts">`)
		for _, k := range parseExpertKeys(d.ExpertKeys) {
			b.WriteString(`<span class="delib-expert-tag">` + html.EscapeString(k) + `</span>`)
		}
		b.WriteString(`</div></div></div>`)
	}

	return b.String()
}

// parseExpertKeys parses a JSON array string like `["architect","reviewer"]`.
func parseExpertKeys(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "[]")
	if raw == "" {
		return nil
	}
	var keys []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"`)
		if part != "" {
			keys = append(keys, part)
		}
	}
	return keys
}

// ══════════════════════════════════════════════════════════════
// COSTS
// ══════════════════════════════════════════════════════════════

func renderCostsContent(modelCosts []db.ModelCost, typeCosts []db.TypeCost, overview *db.OverviewMetrics) string {
	var b strings.Builder

	if len(modelCosts) == 0 && len(typeCosts) == 0 {
		return emptyState("No cost data recorded yet.")
	}

	// Row 1: Cost by Day + Cost by Model
	b.WriteString(`<div class="chart-grid-2">`)
	b.WriteString(`<div class="card"><div class="card-header">Cost by Day</div>`)
	b.WriteString(`<div class="chart-container"><canvas id="cost-day" height="200"></canvas></div></div>`)
	b.WriteString(`<div class="card"><div class="card-header">Cost by Model</div>`)
	b.WriteString(`<div class="chart-container"><canvas id="cost-model" height="200"></canvas></div></div>`)
	b.WriteString(`</div>`)

	// Row 2: Cost by Type + Budget Gauge
	b.WriteString(`<div class="chart-grid-2">`)
	b.WriteString(`<div class="card"><div class="card-header">Cost by Type</div>`)
	b.WriteString(`<div class="chart-container"><canvas id="cost-type" height="200"></canvas></div></div>`)
	b.WriteString(`<div class="card"><div class="card-header">Budget Usage</div>`)
	b.WriteString(`<div class="chart-container"><canvas id="cost-gauge" height="200"></canvas></div></div>`)
	b.WriteString(`</div>`)

	// Chart data: cost by day
	dayLabels := dailyLabels(overview.DailyCostSeries)
	dayVals := dailyValues(overview.DailyCostSeries)
	dayLabJSON, _ := json.Marshal(dayLabels)
	dayValJSON, _ := json.Marshal(dayVals)

	// Chart data: cost by model
	var modelLabels, modelVals []string
	for _, m := range modelCosts {
		modelLabels = append(modelLabels, m.Model)
		modelVals = append(modelVals, fmt.Sprintf("%.6f", m.Cost))
	}
	mlJSON, _ := json.Marshal(modelLabels)
	mvJSON, _ := json.Marshal(modelVals)

	// Chart data: cost by type
	var typeLabels, typeVals []string
	for _, t := range typeCosts {
		typeLabels = append(typeLabels, t.Type)
		typeVals = append(typeVals, fmt.Sprintf("%.6f", t.Cost))
	}
	tlJSON, _ := json.Marshal(typeLabels)
	tvJSON, _ := json.Marshal(typeVals)

	// Budget gauge: use 2× total cost as max, min $1
	budgetMax := overview.TotalCost * 2
	if budgetMax < 1.0 {
		budgetMax = 1.0
	}

	b.WriteString(chartScript(`FraoDashboard.line('cost-day',` + string(dayLabJSON) + `,` + string(dayValJSON) + `,'Cost');` +
		`FraoDashboard.pie('cost-model',` + string(mlJSON) + `,` + string(mvJSON) + `);` +
		`FraoDashboard.bar('cost-type',` + string(tlJSON) + `,` + string(tvJSON) + `,'Cost');` +
		`FraoDashboard.gauge('cost-gauge',` + fmt.Sprintf("%.6f", overview.TotalCost) + `,`+fmt.Sprintf("%.6f", budgetMax)+`,'Total Cost');`))

	return b.String()
}

// ══════════════════════════════════════════════════════════════
// METRICS & HEATMAP
// ══════════════════════════════════════════════════════════════

func renderMetricsContent(cells []db.HeatmapCell, m *db.OverviewMetrics) string {
	var b strings.Builder

	if m.TotalAdvice == 0 {
		return emptyState("No usage data recorded yet.")
	}

	// Build heatmap grid
	grid := buildHeatmapGrid(cells)
	maxVal := 0
	for _, row := range grid {
		for _, v := range row {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	// Heatmap card
	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	hourLabels := []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23"}

	b.WriteString(`<div class="card">`)
	b.WriteString(`<div class="card-header">Activity by Day & Hour (UTC)</div>`)
	b.WriteString(`<div class="heatmap-wrap"><table class="heatmap-table">`)
	b.WriteString(`<thead><tr><th style="width:40px"></th>`)
	for _, h := range hourLabels {
		b.WriteString(`<th>` + h + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)

	for dayIdx, row := range grid {
		b.WriteString(`<tr><td class="heatmap-label">` + dayNames[dayIdx] + `</td>`)
		for hourIdx, count := range row {
			intensity := float64(count) / float64(maxVal)
			alpha := 0.05 + intensity*0.85
			if count == 0 {
				alpha = 0.02
			}
			title := fmt.Sprintf("%s %02d:00 — %d calls", dayNames[dayIdx], hourIdx, count)
			b.WriteString(`<td title="` + html.EscapeString(title) + `">`)
			b.WriteString(`<span class="heatmap-cell" style="background:rgba(6,182,212,` + fmt.Sprintf("%.2f", alpha) + `)">`)
			if count > 0 {
				b.WriteString(fmt.Sprintf("%d", count))
			}
			b.WriteString(`</span></td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></div></div>`)

	// Metrics cards
	peakDayName, peakHourStr := findPeakMetrics(grid, dayNames)
	b.WriteString(`<div class="metrics-grid">`)
	b.WriteString(metricCard("Total Sessions", formatNumber(m.TotalAdvice)))
	b.WriteString(metricCard("Total Cost", formatCost(m.TotalCost)))
	b.WriteString(metricCard("Peak Day", peakDayName))
	b.WriteString(metricCard("Peak Hour", peakHourStr))
	b.WriteString(`</div>`)

	return b.String()
}

func buildHeatmapGrid(cells []db.HeatmapCell) [][]int {
	grid := make([][]int, 7)
	for i := range grid {
		grid[i] = make([]int, 24)
	}
	for _, c := range cells {
		if c.DayOfWeek >= 0 && c.DayOfWeek < 7 && c.HourOfDay >= 0 && c.HourOfDay < 24 {
			grid[c.DayOfWeek][c.HourOfDay] = c.Count
		}
	}
	return grid
}

func findPeakMetrics(grid [][]int, dayNames []string) (string, string) {
	maxDay := 0
	maxDayVal := 0
	peakHourTotal := 0
	peakHourIdx := 0

	for day := 0; day < 7; day++ {
		for hour := 0; hour < 24; hour++ {
			c := grid[day][hour]
			if c > maxDayVal {
				maxDayVal = c
				maxDay = day
			}
		}
	}

	for hour := 0; hour < 24; hour++ {
		total := 0
		for day := 0; day < 7; day++ {
			total += grid[day][hour]
		}
		if total > peakHourTotal {
			peakHourTotal = total
			peakHourIdx = hour
		}
	}

	return dayNames[maxDay], fmt.Sprintf("%02d:00", peakHourIdx)
}

// ══════════════════════════════════════════════════════════════
// EMPTY / ERROR STATE
// ══════════════════════════════════════════════════════════════

func emptyState(message string) string {
	return `<div class="empty-state"><div class="empty-icon">&#9632;</div><h2>No data yet</h2><p class="empty-desc">` + html.EscapeString(message) + `</p></div>`
}
