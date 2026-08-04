package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/prhymera/frao-advisor/db"
	"github.com/starfederation/datastar-go/datastar"
)

// Handlers holds shared state for all dashboard HTTP handlers.
type Handlers struct {
	DB     *db.DB
	layout *template.Template
}

// LayoutPage serves the full HTML shell (not SSE).
func (h *Handlers) LayoutPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.layout.Execute(w, nil)
}

// ─── Overview ─────────────────────────────────────────────────

func (h *Handlers) Overview(w http.ResponseWriter, r *http.Request) {
	if !isDatastarReq(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	sse := datastar.NewSSE(w, r)
	ctx := r.Context()

	metrics, err := h.DB.GetOverviewMetrics(ctx)
	if err != nil {
		sse.PatchElements(`<div id="content"><div class="empty-state"><div class="empty-icon">&#9888;</div><h2>Unable to load data</h2><p class="empty-desc">` + err.Error() + `</p><a href="#" data-on-click="$$get('/dashboard/overview')" class="retry-link">Retry</a></div></div>`)
		return
	}

	if metrics.TotalAdvice == 0 {
		sse.PatchElements(`<div id="content"><div class="empty-state"><div class="empty-icon">&#9632;</div><h2>No data yet</h2><p class="empty-desc">No advice recorded yet. Data appears as you use the advisor tools.</p></div></div>`)
		return
	}

	sse.PatchElements(`<div id="content">` + renderOverviewContent(metrics) + `</div>`)
}

// ─── Timeline ─────────────────────────────────────────────────

func (h *Handlers) Timeline(w http.ResponseWriter, r *http.Request) {
	if !isDatastarReq(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	sse := datastar.NewSSE(w, r)
	ctx := r.Context()

	// Read filters — prefer URL query params (vanilla JS client),
	// fall back to Datastar signals for compatibility.
	typeFilter := r.URL.Query().Get("typeFilter")
	expertFilter := r.URL.Query().Get("expertFilter")

	var signals struct {
		TypeFilter   string `json:"typeFilter"`
		ExpertFilter string `json:"expertFilter"`
	}
	_ = datastar.ReadSignals(r, &signals)
	if typeFilter == "" {
		typeFilter = signals.TypeFilter
	}
	if expertFilter == "" {
		expertFilter = signals.ExpertFilter
	}

	// Read pagination from URL query params
	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	// Expert filter is handled by SQL, type filter is in-memory
	var entries []db.TimelineEntry
	var total int
	var err error

	if typeFilter != "" && typeFilter != "all" {
		// Fetch generously and filter in-memory
		entries, _, err = h.DB.Timeline(ctx, "", expertFilter, 10000, 0)
		if err != nil {
			sse.PatchElements(errorContent("Unable to load timeline", err.Error()))
			return
		}
		entries = filterByType(entries, typeFilter)
		total = len(entries)
		if offset >= total {
			offset = 0
		}
		end := offset + limit
		if end > total {
			end = total
		}
		entries = entries[offset:end]
	} else {
		entries, total, err = h.DB.Timeline(ctx, "", expertFilter, limit, offset)
		if err != nil {
			sse.PatchElements(errorContent("Unable to load timeline", err.Error()))
			return
		}
	}

	sse.PatchElements(`<div id="content">` + renderTimelineContent(entries, total, limit, offset, typeFilter, expertFilter) + `</div>`)
}

// ─── Experts ─────────────────────────────────────────────────

func (h *Handlers) Experts(w http.ResponseWriter, r *http.Request) {
	if !isDatastarReq(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	sse := datastar.NewSSE(w, r)
	ctx := r.Context()

	stats, err := h.DB.ExpertBreakdown(ctx)
	if err != nil {
		sse.PatchElements(errorContent("Unable to load expert data", err.Error()))
		return
	}

	sse.PatchElements(`<div id="content">` + renderExpertsContent(stats) + `</div>`)
}

// ─── Deliberations ─────────────────────────────────────────────

func (h *Handlers) Deliberations(w http.ResponseWriter, r *http.Request) {
	if !isDatastarReq(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	sse := datastar.NewSSE(w, r)
	ctx := r.Context()

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	delibs, _, err := h.DB.DeliberationsList(ctx, limit, offset)
	if err != nil {
		sse.PatchElements(errorContent("Unable to load deliberations", err.Error()))
		return
	}

	sse.PatchElements(`<div id="content">` + renderDeliberationsContent(delibs, len(delibs)) + `</div>`)
}

// ─── Costs ────────────────────────────────────────────────────

func (h *Handlers) Costs(w http.ResponseWriter, r *http.Request) {
	if !isDatastarReq(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	sse := datastar.NewSSE(w, r)
	ctx := r.Context()

	modelCosts, err := h.DB.CostByModel(ctx)
	if err != nil {
		sse.PatchElements(errorContent("Unable to load cost data", err.Error()))
		return
	}

	typeCosts, err := h.DB.CostByType(ctx)
	if err != nil {
		sse.PatchElements(errorContent("Unable to load cost data", err.Error()))
		return
	}

	overview, _ := h.DB.GetOverviewMetrics(ctx)
	if overview == nil {
		overview = &db.OverviewMetrics{}
	}

	sse.PatchElements(`<div id="content">` + renderCostsContent(modelCosts, typeCosts, overview) + `</div>`)
}

// ─── Metrics ─────────────────────────────────────────────────

func (h *Handlers) Metrics(w http.ResponseWriter, r *http.Request) {
	if !isDatastarReq(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	sse := datastar.NewSSE(w, r)
	ctx := r.Context()

	heatmap, err := h.DB.UsageHeatmap(ctx)
	if err != nil {
		sse.PatchElements(errorContent("Unable to load metrics", err.Error()))
		return
	}

	latency, err := h.DB.DailyLatency(ctx, 30)
	if err != nil {
		sse.PatchElements(errorContent("Unable to load metrics", err.Error()))
		return
	}

	overview, _ := h.DB.GetOverviewMetrics(ctx)
	if overview == nil {
		overview = &db.OverviewMetrics{}
	}

	sse.PatchElements(`<div id="content">` + renderMetricsContent(heatmap, overview, latency) + `</div>`)
}

// ─── Error Content Helper ─────────────────────────────────────

// ─── Detail ────────────────────────────────────────────────────

func (h *Handlers) Detail(w http.ResponseWriter, r *http.Request) {
	if !isDatastarReq(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	sse := datastar.NewSSE(w, r)
	ctx := r.Context()

	typeStr := r.URL.Query().Get("type")
	id := r.URL.Query().Get("id")

	if typeStr == "" || id == "" {
		sse.PatchElements(errorContent("Missing parameters", "type and id are required"))
		return
	}

	var html string
	var err error
	switch typeStr {
	case "consultation":
		html, err = h.renderConsultationDetail(ctx, id)
	case "expert_review":
		html, err = h.renderExpertReviewDetail(ctx, id)
	case "deliberation":
		html, err = h.renderDeliberationDetail(ctx, id)
	default:
		sse.PatchElements(errorContent("Unknown type", typeStr))
		return
	}

	if err != nil {
		sse.PatchElements(errorContent("Unable to load detail", err.Error()))
		return
	}

	sse.PatchElements(`<div id="content">` + html + `</div>`)
}

// ─── Detail Rendering Helpers ────────────────────────────────

func (h *Handlers) renderConsultationDetail(ctx context.Context, id string) (string, error) {
	var c db.Consultation
	err := h.DB.QueryRowContext(ctx,
		"SELECT id, session_id, question, response, model, prompt_tokens, completion_tokens, input_cost, output_cost, duration_ms, created_at FROM consultations WHERE id = ?", id,
	).Scan(&c.ID, &c.SessionID, &c.Question, &c.Response, &c.Model,
		&c.PromptTokens, &c.CompletionTokens, &c.InputCost, &c.OutputCost, &c.DurationMs, &c.CreatedAt)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(`<div class="detail-view">`)
	b.WriteString(`<a href="#" class="btn" style="margin-bottom:16px;display:inline-block" data-on-click="$$get('/dashboard/timeline')">&larr; Back to Timeline</a>`)
	b.WriteString(`<div class="card"><div class="card-header">Consultation <span class="badge-consultation" style="padding:2px 8px;border-radius:4px;font-size:10px;background:rgba(6,182,212,0.15);color:#06b6d4">` + c.Model + `</span></div>`)
	b.WriteString(`<table><tbody>`)
	fmt.Fprintf(&b, `<tr><td style="width:100px;font-weight:600">Question</td><td>%s</td></tr>`, html.EscapeString(c.Question))
	fmt.Fprintf(&b, `<tr><td style="font-weight:600;vertical-align:top">Response</td><td style="white-space:pre-wrap">%s</td></tr>`, html.EscapeString(c.Response))
	tok := formatNumber(c.PromptTokens+c.CompletionTokens) + " (" + formatNumber(c.PromptTokens) + " prompt + " + formatNumber(c.CompletionTokens) + " completion)"
	fmt.Fprintf(&b, `<tr><td style="font-weight:600">Tokens</td><td>%s</td></tr>`, tok)
	fmt.Fprintf(&b, `<tr><td style="font-weight:600">Cost</td><td>%s</td></tr>`, formatCost(c.InputCost+c.OutputCost))
	fmt.Fprintf(&b, `<tr><td style="font-weight:600">Duration</td><td>%s</td></tr>`, formatDuration(c.DurationMs))
	b.WriteString(`</tbody></table></div></div>`)
	return b.String(), nil
}

func (h *Handlers) renderExpertReviewDetail(ctx context.Context, id string) (string, error) {
	var r db.ExpertReview
	err := h.DB.QueryRowContext(ctx,
		"SELECT id, session_id, expert_key, context, analysis, model, prompt_tokens, completion_tokens, input_cost, output_cost, duration_ms, created_at FROM expert_reviews WHERE id = ?", id,
	).Scan(&r.ID, &r.SessionID, &r.ExpertKey, &r.Context, &r.Analysis, &r.Model,
		&r.PromptTokens, &r.CompletionTokens, &r.InputCost, &r.OutputCost, &r.DurationMs, &r.CreatedAt)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(`<div class="detail-view">`)
	b.WriteString(`<a href="#" class="btn" style="margin-bottom:16px;display:inline-block" data-on-click="$$get('/dashboard/timeline')">&larr; Back to Timeline</a>`)
	b.WriteString(`<div class="card"><div class="card-header">Expert Review: ` + html.EscapeString(r.ExpertKey) + `</div>`)
	b.WriteString(`<table><tbody>`)
	fmt.Fprintf(&b, `<tr><td style="width:100px;font-weight:600">Context</td><td>%s</td></tr>`, html.EscapeString(truncate(r.Context, 500)))
	fmt.Fprintf(&b, `<tr><td style="font-weight:600;vertical-align:top">Analysis</td><td style="white-space:pre-wrap">%s</td></tr>`, html.EscapeString(r.Analysis))
	fmt.Fprintf(&b, `<tr><td style="font-weight:600">Model</td><td>%s</td></tr>`, html.EscapeString(r.Model))
	fmt.Fprintf(&b, `<tr><td style="font-weight:600">Cost</td><td>%s</td></tr>`, formatCost(r.InputCost+r.OutputCost))
	b.WriteString(`</tbody></table></div></div>`)
	return b.String(), nil
}

func (h *Handlers) renderDeliberationDetail(ctx context.Context, id string) (string, error) {
	var d db.Deliberation
	err := h.DB.QueryRowContext(ctx,
		"SELECT id, session_id, context, synthesis, expert_count, expert_keys, model, total_prompt_tokens, total_completion_tokens, total_input_cost, total_output_cost, duration_ms, created_at FROM deliberations WHERE id = ?", id,
	).Scan(&d.ID, &d.SessionID, &d.Context, &d.Synthesis, &d.ExpertCount, &d.ExpertKeys, &d.Model,
		&d.TotalPromptTokens, &d.TotalCompletionTokens, &d.TotalInputCost, &d.TotalOutputCost, &d.DurationMs, &d.CreatedAt)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(`<div class="detail-view">`)
	b.WriteString(`<a href="#" class="btn" style="margin-bottom:16px;display:inline-block" data-on-click="$$get('/dashboard/deliberations')">&larr; Back to Deliberations</a>`)
	b.WriteString(`<div class="card"><div class="card-header">Deliberation (` + fmt.Sprintf("%d", d.ExpertCount) + ` experts)</div>`)
	b.WriteString(`<table><tbody>`)
	fmt.Fprintf(&b, `<tr><td style="width:100px;font-weight:600">Context</td><td>%s</td></tr>`, html.EscapeString(truncate(d.Context, 500)))
	fmt.Fprintf(&b, `<tr><td style="font-weight:600;vertical-align:top">Synthesis</td><td style="white-space:pre-wrap">%s</td></tr>`, html.EscapeString(d.Synthesis))
	fmt.Fprintf(&b, `<tr><td style="font-weight:600">Cost</td><td>%s</td></tr>`, formatCost(d.TotalInputCost+d.TotalOutputCost))
	b.WriteString(`</tbody></table></div></div>`)
	return b.String(), nil
}

func errorContent(title, detail string) string {
	return `<div id="content"><div class="empty-state"><div class="empty-icon">&#9888;</div><h2>` + title + `</h2><p class="empty-desc">` + detail + `</p><a href="#" onclick="location.reload()" class="retry-link">Reload</a></div></div>`
}

// ─── Ingest API ─────────────────────────────────────────────────

// Health returns 200 when the dashboard and its database are up.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	if h.DB != nil {
		var one int
		if err := h.DB.QueryRowContext(r.Context(), `SELECT 1`).Scan(&one); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// IngestEvent accepts an advice record published by an MCP process and
// persists it. Idempotent by record ID (the inserts use INSERT OR IGNORE),
// so retries and double-writes are harmless.
func (h *Handlers) IngestEvent(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	defer r.Body.Close()

	var ev db.Event
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := ev.Validate(); err != nil {
		http.Error(w, "invalid event: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.DB.IngestEvent(r.Context(), ev); err != nil {
		http.Error(w, "ingest failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
