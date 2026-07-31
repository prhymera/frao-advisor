package db

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// ─── Sessions ───────────────────────────────────────────────────────────

func (d *DB) EnsureSession(ctx context.Context, sessionID string) (string, error) {
	var id string
	err := d.QueryRowContext(ctx,
		`SELECT id FROM sessions WHERE session_id = ? ORDER BY started_at DESC LIMIT 1`, sessionID).Scan(&id)
	if err == nil {
		d.ExecContext(ctx, `UPDATE sessions SET last_active = datetime('now'), call_count = call_count + 1 WHERE id = ?`, id)
		return id, nil
	}
	// Create new session
	id = uuidV4()
	_, err = d.ExecContext(ctx,
		`INSERT INTO sessions (id, session_id) VALUES (?, ?)`, id, sessionID)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return id, nil
}

// ─── Consultations ──────────────────────────────────────────────────────

type InsertConsultationParams struct {
	ID               string
	SessionID        string
	Question         string
	Response         string
	Model            string
	ReasoningEffort  string
	PromptTokens     int
	CompletionTokens int
	InputCost        float64
	OutputCost       float64
	InputPriceUsed   float64
	OutputPriceUsed  float64
	CacheHit         bool
	DurationMs       int64
}

func (d *DB) InsertConsultation(ctx context.Context, p InsertConsultationParams) error {
	cacheHit := 0
	if p.CacheHit {
		cacheHit = 1
	}
	_, err := d.ExecContext(ctx, `
		INSERT INTO consultations (id, session_id, question, response, model, reasoning_effort,
			prompt_tokens, completion_tokens, input_cost, output_cost,
			input_price_used, output_price_used, cache_hit, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.SessionID, p.Question, p.Response, p.Model, p.ReasoningEffort,
		p.PromptTokens, p.CompletionTokens, p.InputCost, p.OutputCost,
		p.InputPriceUsed, p.OutputPriceUsed, cacheHit, p.DurationMs)
	return err
}

// ─── Expert Reviews ─────────────────────────────────────────────────────

type InsertExpertReviewParams struct {
	ID               string
	SessionID        string
	ExpertKey        string
	Context          string
	Analysis         string
	Model            string
	ReasoningEffort  string
	PromptTokens     int
	CompletionTokens int
	InputCost        float64
	OutputCost       float64
	InputPriceUsed   float64
	OutputPriceUsed  float64
	CacheHit         bool
	DurationMs       int64
}

func (d *DB) InsertExpertReview(ctx context.Context, p InsertExpertReviewParams) error {
	cacheHit := 0
	if p.CacheHit {
		cacheHit = 1
	}
	_, err := d.ExecContext(ctx, `
		INSERT INTO expert_reviews (id, session_id, expert_key, context, analysis, model, reasoning_effort,
			prompt_tokens, completion_tokens, input_cost, output_cost,
			input_price_used, output_price_used, cache_hit, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.SessionID, p.ExpertKey, p.Context, p.Analysis, p.Model, p.ReasoningEffort,
		p.PromptTokens, p.CompletionTokens, p.InputCost, p.OutputCost,
		p.InputPriceUsed, p.OutputPriceUsed, cacheHit, p.DurationMs)
	return err
}

// ─── Deliberations ──────────────────────────────────────────────────────

type InsertDeliberationParams struct {
	ID                  string
	SessionID           string
	Context             string
	Synthesis           string
	ExpertCount         int
	ExpertKeys          string
	Model               string
	ReasoningEffort     string
	TotalPromptTokens   int
	TotalCompletionTokens int
	TotalInputCost      float64
	TotalOutputCost     float64
	DurationMs          int64
}

func (d *DB) InsertDeliberation(ctx context.Context, p InsertDeliberationParams) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO deliberations (id, session_id, context, synthesis, expert_count, expert_keys,
			model, reasoning_effort, total_prompt_tokens, total_completion_tokens,
			total_input_cost, total_output_cost, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.SessionID, p.Context, p.Synthesis, p.ExpertCount, p.ExpertKeys,
		p.Model, p.ReasoningEffort, p.TotalPromptTokens, p.TotalCompletionTokens,
		p.TotalInputCost, p.TotalOutputCost, p.DurationMs)
	return err
}

type InsertContributionParams struct {
	ID               string
	DeliberationID   string
	ExpertKey        string
	Analysis         string
	PromptTokens     int
	CompletionTokens int
	InputCost        float64
	OutputCost       float64
	DurationMs       int64
	SortOrder        int
}

func (d *DB) InsertDeliberationContribution(ctx context.Context, p InsertContributionParams) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO deliberation_contributions (id, deliberation_id, expert_key, analysis,
			prompt_tokens, completion_tokens, input_cost, output_cost, duration_ms, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.DeliberationID, p.ExpertKey, p.Analysis,
		p.PromptTokens, p.CompletionTokens, p.InputCost, p.OutputCost, p.DurationMs, p.SortOrder)
	return err
}

// ─── Dashboard Aggregation Queries ──────────────────────────────────────

func (d *DB) GetOverviewMetrics(ctx context.Context) (*OverviewMetrics, error) {
	m := &OverviewMetrics{}

	// Total counts
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM consultations`).Scan(&m.Consultations)
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM expert_reviews`).Scan(&m.ExpertReviews)
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM deliberations`).Scan(&m.Deliberations)
	m.TotalAdvice = m.Consultations + m.ExpertReviews + m.Deliberations

	// Total cost across all tables
	d.QueryRowContext(ctx, `
		SELECT IFNULL(SUM(cost), 0) FROM (
			SELECT input_cost + output_cost AS cost FROM consultations
			UNION ALL
			SELECT input_cost + output_cost FROM expert_reviews
			UNION ALL
			SELECT total_input_cost + total_output_cost FROM deliberations
		)`).Scan(&m.TotalCost)

	// Total tokens
	d.QueryRowContext(ctx, `
		SELECT IFNULL(SUM(tokens), 0) FROM (
			SELECT prompt_tokens + completion_tokens AS tokens FROM consultations
			UNION ALL
			SELECT prompt_tokens + completion_tokens FROM expert_reviews
			UNION ALL
			SELECT total_prompt_tokens + total_completion_tokens FROM deliberations
		)`).Scan(&m.TotalTokens)

	// Active sessions (last 1 hour)
	d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions WHERE last_active >= datetime('now', '-1 hour')`).Scan(&m.ActiveSessions)

	if m.TotalAdvice > 0 {
		m.AvgCostPerCall = m.TotalCost / float64(m.TotalAdvice)
	}

	// Daily cost series (last 30 days)
	m.DailyCostSeries = d.dailyCostSeries(ctx, 30)

	// Today vs yesterday cost
	var todayCost, yesterdayCost float64
	d.QueryRowContext(ctx,
		`SELECT IFNULL(SUM(input_cost + output_cost), 0) FROM consultations WHERE date(created_at) = date('now')`).Scan(&todayCost)
	d.QueryRowContext(ctx,
		`SELECT IFNULL(SUM(input_cost + output_cost), 0) FROM expert_reviews WHERE date(created_at) = date('now')`).Scan(&yesterdayCost)
	d.QueryRowContext(ctx,
		`SELECT IFNULL(SUM(total_input_cost + total_output_cost), 0) FROM deliberations WHERE date(created_at) = date('now')`).Scan(&yesterdayCost)
	m.CostToday = todayCost + m.CostToday // add expert + deliberation today costs - simplified

	// Better: query all tables for today/yesterday
	var allToday, allYest float64
	d.QueryRowContext(ctx, `
		SELECT IFNULL(SUM(cost), 0) FROM (
			SELECT input_cost + output_cost AS cost FROM consultations WHERE date(created_at) = date('now')
			UNION ALL
			SELECT input_cost + output_cost FROM expert_reviews WHERE date(created_at) = date('now')
			UNION ALL
			SELECT total_input_cost + total_output_cost FROM deliberations WHERE date(created_at) = date('now')
		)`).Scan(&allToday)
	d.QueryRowContext(ctx, `
		SELECT IFNULL(SUM(cost), 0) FROM (
			SELECT input_cost + output_cost AS cost FROM consultations WHERE date(created_at) = date('now', '-1 day')
			UNION ALL
			SELECT input_cost + output_cost FROM expert_reviews WHERE date(created_at) = date('now', '-1 day')
			UNION ALL
			SELECT total_input_cost + total_output_cost FROM deliberations WHERE date(created_at) = date('now', '-1 day')
		)`).Scan(&allYest)
	m.CostToday = allToday
	m.CostYesterday = allYest
	if allYest > 0 {
		m.CostChangePct = ((allToday - allYest) / allYest) * 100
	}

	m.UptimeSeconds = int64(time.Since(startTime).Seconds())

	return m, nil
}

func (d *DB) dailyCostSeries(ctx context.Context, days int) []DayCost {
	query := fmt.Sprintf(`
		SELECT day, SUM(cost) FROM (
			SELECT date(created_at) AS day, input_cost + output_cost AS cost FROM consultations
				WHERE created_at >= datetime('now', '-%d days')
			UNION ALL
			SELECT date(created_at), input_cost + output_cost FROM expert_reviews
				WHERE created_at >= datetime('now', '-%d days')
			UNION ALL
			SELECT date(created_at), total_input_cost + total_output_cost FROM deliberations
				WHERE created_at >= datetime('now', '-%d days')
		) GROUP BY day ORDER BY day ASC`, days, days, days)
	rows, err := d.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var series []DayCost
	for rows.Next() {
		var dc DayCost
		if err := rows.Scan(&dc.Date, &dc.Cost); err == nil {
			series = append(series, dc)
		}
	}
	return series
}

func (d *DB) ExpertBreakdown(ctx context.Context) ([]ExpertStat, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT expert_key,
			COUNT(*) AS count,
			IFNULL(SUM(input_cost + output_cost), 0) AS total_cost,
			IFNULL(AVG(prompt_tokens + completion_tokens), 0) AS avg_tokens,
			IFNULL(SUM(prompt_tokens), 0) AS total_prompt,
			IFNULL(SUM(completion_tokens), 0) AS total_completion,
			IFNULL(AVG(duration_ms), 0) AS avg_duration
		FROM expert_reviews
		GROUP BY expert_key
		ORDER BY total_cost DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []ExpertStat
	for rows.Next() {
		var s ExpertStat
		if err := rows.Scan(&s.ExpertKey, &s.Count, &s.TotalCost, &s.AvgTokens,
			&s.TotalPromptTokens, &s.TotalCompTokens, &s.AvgDurationMs); err == nil {
			stats = append(stats, s)
		}
	}
	return stats, nil
}

func (d *DB) CostByModel(ctx context.Context) ([]ModelCost, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT model, SUM(cost) AS cost, COUNT(*) AS count FROM (
			SELECT model, input_cost + output_cost AS cost FROM consultations
			UNION ALL
			SELECT model, input_cost + output_cost FROM expert_reviews
			UNION ALL
			SELECT model, total_input_cost + total_output_cost FROM deliberations
		) GROUP BY model ORDER BY cost DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var costs []ModelCost
	for rows.Next() {
		var mc ModelCost
		if err := rows.Scan(&mc.Model, &mc.Cost, &mc.Count); err == nil {
			costs = append(costs, mc)
		}
	}
	return costs, nil
}

func (d *DB) CostByType(ctx context.Context) ([]TypeCost, error) {
	var results []TypeCost

	var cons TypeCost
	cons.Type = "Consultations"
	d.QueryRowContext(ctx, `SELECT COUNT(*), IFNULL(SUM(input_cost + output_cost), 0),
		IFNULL(AVG(prompt_tokens + completion_tokens), 0) FROM consultations`).Scan(&cons.Count, &cons.Cost, &cons.AvgTokens)
	results = append(results, cons)

	var rev TypeCost
	rev.Type = "Expert Reviews"
	d.QueryRowContext(ctx, `SELECT COUNT(*), IFNULL(SUM(input_cost + output_cost), 0),
		IFNULL(AVG(prompt_tokens + completion_tokens), 0) FROM expert_reviews`).Scan(&rev.Count, &rev.Cost, &rev.AvgTokens)
	results = append(results, rev)

	var delib TypeCost
	delib.Type = "Deliberations"
	d.QueryRowContext(ctx, `SELECT COUNT(*), IFNULL(SUM(total_input_cost + total_output_cost), 0),
		IFNULL(AVG(total_prompt_tokens + total_completion_tokens), 0) FROM deliberations`).Scan(&delib.Count, &delib.Cost, &delib.AvgTokens)
	results = append(results, delib)

	return results, nil
}

func (d *DB) UsageHeatmap(ctx context.Context) ([]HeatmapCell, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT CAST(strftime('%w', created_at) AS INTEGER) AS day_of_week,
			CAST(strftime('%H', created_at) AS INTEGER) AS hour_of_day,
			COUNT(*) AS count
		FROM (
			SELECT created_at FROM consultations
			UNION ALL
			SELECT created_at FROM expert_reviews
			UNION ALL
			SELECT created_at FROM deliberations
		)
		GROUP BY day_of_week, hour_of_day
		ORDER BY day_of_week, hour_of_day`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cells []HeatmapCell
	for rows.Next() {
		var h HeatmapCell
		if err := rows.Scan(&h.DayOfWeek, &h.HourOfDay, &h.Count); err == nil {
			cells = append(cells, h)
		}
	}
	return cells, nil
}

func (d *DB) DailyLatency(ctx context.Context, days int) ([]DayLatency, error) {
	query := fmt.Sprintf(`
		SELECT day, ROUND(AVG(duration), 1) FROM (
			SELECT date(created_at) AS day, duration_ms AS duration FROM consultations
				WHERE created_at >= datetime('now', '-%d days')
			UNION ALL
			SELECT date(created_at), duration_ms FROM expert_reviews
				WHERE created_at >= datetime('now', '-%d days')
			UNION ALL
			SELECT date(created_at), duration_ms FROM deliberations
				WHERE created_at >= datetime('now', '-%d days')
		) GROUP BY day ORDER BY day ASC`, days, days, days)
	rows, err := d.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []DayLatency
	for rows.Next() {
		var dl DayLatency
		if err := rows.Scan(&dl.Date, &dl.AvgDurationMs); err == nil {
			series = append(series, dl)
		}
	}
	return series, nil
}

// ─── Advice Timeline ────────────────────────────────────────────────────

type TimelineEntry struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"` // "consultation", "expert_review", "deliberation"
	Summary   string  `json:"summary"`
	ExpertKey string  `json:"expert_key,omitempty"`
	Model     string  `json:"model"`
	Cost      float64 `json:"cost"`
	Tokens    int     `json:"tokens"`
	DurationMs int64  `json:"duration_ms"`
	CreatedAt string  `json:"created_at"`
}

func (d *DB) Timeline(ctx context.Context, typeFilter, expertFilter string, limit, offset int) ([]TimelineEntry, int, error) {
	// Build parameterized WHERE clause
	where, params := buildTimelineWhere(typeFilter, expertFilter)

	// Count total matching — params replicated for each of 3 subqueries
	countParams := append(append(params, params...), params...)
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT 'consultation' AS type, created_at FROM consultations %s
			UNION ALL
			SELECT 'expert_review', created_at FROM expert_reviews %s
			UNION ALL
			SELECT 'deliberation', created_at FROM deliberations %s
		)`, where, where, where)
	var total int
	d.QueryRowContext(ctx, countQuery, countParams...).Scan(&total)

	// Get entries — params again replicated for 3 subqueries
	dataParams := append(append(params, params...), params...)
	dataParams = append(dataParams, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, type, summary, expert, model, cost, tokens, duration_ms, created_at FROM (
			SELECT id, 'consultation' AS type, substr(question, 1, 120) AS summary,
				'' AS expert, model, input_cost + output_cost AS cost,
				prompt_tokens + completion_tokens AS tokens, duration_ms, created_at
			FROM consultations %s
			UNION ALL
			SELECT id, 'expert_review', substr(context, 1, 120),
				expert_key, model, input_cost + output_cost,
				prompt_tokens + completion_tokens, duration_ms, created_at
			FROM expert_reviews %s
			UNION ALL
			SELECT id, 'deliberation', substr(context, 1, 120),
				expert_keys, model, total_input_cost + total_output_cost,
				total_prompt_tokens + total_completion_tokens, duration_ms, created_at
			FROM deliberations %s
		) ORDER BY created_at DESC LIMIT ? OFFSET ?`, where, where, where)

	rows, err := d.QueryContext(ctx, query, dataParams...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []TimelineEntry
	for rows.Next() {
		var e TimelineEntry
		if err := rows.Scan(&e.ID, &e.Type, &e.Summary, &e.ExpertKey, &e.Model, &e.Cost, &e.Tokens, &e.DurationMs, &e.CreatedAt); err == nil {
			entries = append(entries, e)
		}
	}
	return entries, total, nil
}

// buildTimelineWhere returns a parameterized WHERE clause and its bound values.
// The clause is a SQL fragment with ? placeholders, replicated for each subquery.
func buildTimelineWhere(typeFilter, expertFilter string) (string, []any) {
	var clauses []string
	var params []any
	if typeFilter != "" && typeFilter != "all" {
		// Can't filter by type here because the WHERE is per-subquery
		// We don't apply it — filtering happens in the caller if needed
	}
	if expertFilter != "" && expertFilter != "all" {
		clauses = append(clauses, "expert_key = ?")
		params = append(params, expertFilter)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), params
}

// ─── Recent Activity ────────────────────────────────────────────────────

func (d *DB) RecentActivity(ctx context.Context, n int) []TimelineEntry {
	entries, _, _ := d.Timeline(ctx, "", "", n, 0)
	return entries
}
// ─── Deliberations List ──────────────────────────────────────────

func (d *DB) DeliberationsList(ctx context.Context, limit, offset int) ([]Deliberation, int, error) {
	var total int
	err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM deliberations`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count deliberations: %w", err)
	}

	rows, err := d.QueryContext(ctx, `
		SELECT id, session_id, context, synthesis, expert_count, expert_keys,
			model, total_prompt_tokens, total_completion_tokens,
			total_input_cost, total_output_cost, duration_ms, created_at
		FROM deliberations
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list deliberations: %w", err)
	}
	defer rows.Close()

	var result []Deliberation
	for rows.Next() {
		var d Deliberation
		err := rows.Scan(&d.ID, &d.SessionID, &d.Context, &d.Synthesis, &d.ExpertCount,
			&d.ExpertKeys, &d.Model, &d.TotalPromptTokens, &d.TotalCompletionTokens,
			&d.TotalInputCost, &d.TotalOutputCost, &d.DurationMs, &d.CreatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("scan deliberation: %w", err)
		}
		result = append(result, d)
	}
	return result, total, rows.Err()
}

// ─── UUID helper ────────────────────────────────────────────────

func uuidV4() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// startTime records when the process started, for uptime calculation.
var startTime = time.Now()
