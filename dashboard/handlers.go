package dashboard

import (
	"html/template"
	"net/http"
	"strconv"

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

	// Read signals for filters
	var signals struct {
		TypeFilter   string `json:"typeFilter"`
		ExpertFilter string `json:"expertFilter"`
	}
	_ = datastar.ReadSignals(r, &signals)

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

	if signals.TypeFilter != "" && signals.TypeFilter != "all" {
		// Fetch generously and filter in-memory
		entries, _, err = h.DB.Timeline(ctx, "", signals.ExpertFilter, 10000, 0)
		if err != nil {
			sse.PatchElements(errorContent("Unable to load timeline", err.Error()))
			return
		}
		entries = filterByType(entries, signals.TypeFilter)
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
		entries, total, err = h.DB.Timeline(ctx, "", signals.ExpertFilter, limit, offset)
		if err != nil {
			sse.PatchElements(errorContent("Unable to load timeline", err.Error()))
			return
		}
	}

	sse.PatchElements(`<div id="content">` + renderTimelineContent(entries, total, limit, offset, signals.TypeFilter, signals.ExpertFilter) + `</div>`)
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

func errorContent(title, detail string) string {
	return `<div id="content"><div class="empty-state"><div class="empty-icon">&#9888;</div><h2>` + title + `</h2><p class="empty-desc">` + detail + `</p><a href="#" onclick="location.reload()" class="retry-link">Reload</a></div></div>`
}
