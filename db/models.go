package db

import "time"

// ─── Session ────────────────────────────────────────────────────────────

type Session struct {
	ID            string `json:"id"`
	SessionID     string `json:"session_id"`
	ClientName    string `json:"client_name"`
	ClientVersion string `json:"client_version"`
	Project       string `json:"project"`
	StartedAt     string `json:"started_at"`
	LastActive    string `json:"last_active"`
	CallCount     int    `json:"call_count"`
}

// ─── Consultation ───────────────────────────────────────────────────────

type Consultation struct {
	ID              string  `json:"id"`
	SessionID       string  `json:"session_id"`
	Question        string  `json:"question"`
	Response        string  `json:"response"`
	Model           string  `json:"model"`
	ReasoningEffort string  `json:"reasoning_effort"`
	PromptTokens    int     `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	InputCost       float64 `json:"input_cost"`
	OutputCost      float64 `json:"output_cost"`
	InputPriceUsed  float64 `json:"input_price_used"`
	OutputPriceUsed float64 `json:"output_price_used"`
	CacheHit        bool    `json:"cache_hit"`
	DurationMs      int64   `json:"duration_ms"`
	CreatedAt       string  `json:"created_at"`
}

// TotalCost returns the total cost of the consultation.
func (c *Consultation) TotalCost() float64 { return c.InputCost + c.OutputCost }

// TotalTokens returns the total tokens used.
func (c *Consultation) TotalTokens() int { return c.PromptTokens + c.CompletionTokens }

// ─── ExpertReview ───────────────────────────────────────────────────────

type ExpertReview struct {
	ID               string  `json:"id"`
	SessionID        string  `json:"session_id"`
	ExpertKey        string  `json:"expert_key"`
	Context          string  `json:"context"`
	Analysis         string  `json:"analysis"`
	Model            string  `json:"model"`
	ReasoningEffort  string  `json:"reasoning_effort"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	InputCost        float64 `json:"input_cost"`
	OutputCost       float64 `json:"output_cost"`
	InputPriceUsed   float64 `json:"input_price_used"`
	OutputPriceUsed  float64 `json:"output_price_used"`
	CacheHit         bool    `json:"cache_hit"`
	DurationMs       int64   `json:"duration_ms"`
	CreatedAt        string  `json:"created_at"`
}

func (r *ExpertReview) TotalCost() float64  { return r.InputCost + r.OutputCost }
func (r *ExpertReview) TotalTokens() int    { return r.PromptTokens + r.CompletionTokens }

// ─── Deliberation ───────────────────────────────────────────────────────

type Deliberation struct {
	ID                   string  `json:"id"`
	SessionID            string  `json:"session_id"`
	Context              string  `json:"context"`
	Synthesis            string  `json:"synthesis"`
	ExpertCount          int     `json:"expert_count"`
	ExpertKeys           string  `json:"expert_keys"`
	Model                string  `json:"model"`
	ReasoningEffort      string  `json:"reasoning_effort"`
	TotalPromptTokens    int     `json:"total_prompt_tokens"`
	TotalCompletionTokens int    `json:"total_completion_tokens"`
	TotalInputCost       float64 `json:"total_input_cost"`
	TotalOutputCost      float64 `json:"total_output_cost"`
	DurationMs           int64   `json:"duration_ms"`
	CreatedAt            string  `json:"created_at"`
}

func (d *Deliberation) TotalCost() float64 { return d.TotalInputCost + d.TotalOutputCost }
func (d *Deliberation) TotalTokens() int   { return d.TotalPromptTokens + d.TotalCompletionTokens }

// ─── DeliberationContribution ───────────────────────────────────────────

type DeliberationContribution struct {
	ID              string  `json:"id"`
	DeliberationID  string  `json:"deliberation_id"`
	ExpertKey       string  `json:"expert_key"`
	Analysis        string  `json:"analysis"`
	PromptTokens    int     `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	InputCost       float64 `json:"input_cost"`
	OutputCost      float64 `json:"output_cost"`
	DurationMs      int64   `json:"duration_ms"`
	SortOrder       int     `json:"sort_order"`
	CreatedAt       string  `json:"created_at"`
}

// ─── Dashboard Helpers ──────────────────────────────────────────────────

type DayCost struct {
	Date string  `json:"date"`
	Cost float64 `json:"cost"`
}

type ExpertStat struct {
	ExpertKey         string  `json:"expert_key"`
	Count             int     `json:"count"`
	TotalCost         float64 `json:"total_cost"`
	AvgTokens         float64 `json:"avg_tokens"`
	TotalPromptTokens int     `json:"total_prompt_tokens"`
	TotalCompTokens   int     `json:"total_completion_tokens"`
	AvgDurationMs     float64 `json:"avg_duration_ms"`
}

type ModelCost struct {
	Model string  `json:"model"`
	Cost  float64 `json:"cost"`
	Count int     `json:"count"`
}

type TypeCost struct {
	Type      string  `json:"type"`
	Cost      float64 `json:"cost"`
	Count     int     `json:"count"`
	AvgTokens float64 `json:"avg_tokens"`
}

type HeatmapCell struct {
	DayOfWeek  int `json:"day_of_week"`
	HourOfDay  int `json:"hour_of_day"`
	Count      int `json:"count"`
}

type OverviewMetrics struct {
	TotalAdvice      int       `json:"total_advice"`
	TotalCost        float64   `json:"total_cost"`
	ActiveSessions   int       `json:"active_sessions"`
	AvgCostPerCall   float64   `json:"avg_cost_per_call"`
	TotalTokens      int       `json:"total_tokens"`
	Consultations    int       `json:"consultations"`
	ExpertReviews    int       `json:"expert_reviews"`
	Deliberations    int       `json:"deliberations"`
	DailyCostSeries  []DayCost `json:"daily_cost_series"`
	CostToday        float64   `json:"cost_today"`
	CostYesterday    float64   `json:"cost_yesterday"`
	CostChangePct    float64   `json:"cost_change_pct"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
}

// Timeless time helpers for consistent ISO8601 formatting.
func nowISO() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }
func todayISO() string { return time.Now().UTC().Format("2006-01-02") }
