// Package cost provides DeepSeek pricing tables and cost calculation.
//
// Prices are the off-peak (base) rates effective 2026-09-10, the DeepSeek
// V4.1-Flash release. Peak hours bill at 2x off-peak.
package cost

import (
	"log"
	"time"
)

// ModelPrices holds per-model pricing in USD per 1M tokens, off-peak.
type ModelPrices struct {
	InputPricePerM  float64 // Cache miss input
	OutputPricePerM float64 // Output
	CacheHitInput   float64 // Cache hit input
}

// DefaultPrices is the canonical off-peak pricing table for all supported
// models. Peak rates are 2x these values (see isPeak) and are applied by
// PricesFor rather than stored here, so the table stays a single source of
// truth for the base rate.
var DefaultPrices = map[string]ModelPrices{
	// Canonical model name since the 2026-09-10 V4.1-Flash release.
	"deepseek-flash": {
		InputPricePerM:  0.15,
		OutputPricePerM: 0.60,
		CacheHitInput:   0.003,
	},
	// Retiring 2026-09-14; requests are thereafter routed to V4.1-Flash and
	// billed at Flash prices.
	"deepseek-v4-pro": {
		InputPricePerM:  0.66,
		OutputPricePerM: 1.98,
		CacheHitInput:   0.022,
	},
}

// modelAliases maps retired or legacy request names onto the model that
// actually serves them. DeepSeek routes these to V4.1-Flash and bills at the
// Flash price, so they must not fall through to the unknown-model fallback.
var modelAliases = map[string]string{
	"deepseek-v4-flash":            "deepseek-flash",
	"deepseek-v4-flash-vision-exp": "deepseek-flash",
	"deepseek-chat":                "deepseek-flash",
	"deepseek-reasoner":            "deepseek-flash",
}

// fallbackModel prices any model we do not recognise. It is deliberately the
// most expensive entry so that an unlisted model over-reports rather than
// silently under-reports spend.
const fallbackModel = "deepseek-v4-pro"

// isPeak reports whether t falls in DeepSeek's peak billing window: 01:00-04:00
// and 06:00-10:00 UTC, Monday-Friday. Off-peak is half of peak.
func isPeak(t time.Time) bool {
	u := t.UTC()
	if d := u.Weekday(); d == time.Saturday || d == time.Sunday {
		return false
	}
	h := u.Hour()
	return (h >= 1 && h < 4) || (h >= 6 && h < 10)
}

// ResolveModel normalises a request name to the model that serves it.
func ResolveModel(model string) string {
	if canonical, ok := modelAliases[model]; ok {
		return canonical
	}
	return model
}

// PricesFor returns the effective prices for a model at the given time,
// applying alias resolution and the peak multiplier. Unknown models fall back
// to fallbackModel and are logged, so a new model name never silently
// mis-bills: it shows up in the advisor log instead.
func PricesFor(model string, at time.Time) ModelPrices {
	name := ResolveModel(model)
	prices, ok := DefaultPrices[name]
	if !ok {
		log.Printf("cost: unknown model %q — pricing as %s", model, fallbackModel)
		prices = DefaultPrices[fallbackModel]
	}
	if isPeak(at) {
		prices.InputPricePerM *= 2
		prices.OutputPricePerM *= 2
		prices.CacheHitInput *= 2
	}
	return prices
}

// CalculateCostAt computes the cost of an API call at a given time.
// Returns (inputCost, outputCost, totalCost) in USD.
func CalculateCostAt(model string, promptTokens, completionTokens int, cacheHit bool, at time.Time) (inputCost, outputCost, totalCost float64) {
	prices := PricesFor(model, at)

	inputPrice := prices.InputPricePerM
	if cacheHit {
		inputPrice = prices.CacheHitInput
	}

	inputCost = float64(promptTokens) / 1_000_000 * inputPrice
	outputCost = float64(completionTokens) / 1_000_000 * prices.OutputPricePerM
	totalCost = inputCost + outputCost
	return
}

// CalculateCost computes the cost of an API call at the current time.
func CalculateCost(model string, promptTokens, completionTokens int, cacheHit bool) (inputCost, outputCost, totalCost float64) {
	return CalculateCostAt(model, promptTokens, completionTokens, cacheHit, time.Now())
}

// CacheSavings calculates how much was saved by cache hits compared to
// paying cache-miss prices for the prompt.
func CacheSavings(model string, promptTokens, cacheHitTokens int) float64 {
	prices := PricesFor(model, time.Now())
	if cacheHitTokens > promptTokens {
		cacheHitTokens = promptTokens
	}
	missCost := float64(promptTokens) / 1_000_000 * prices.InputPricePerM
	actualCost := (float64(promptTokens-cacheHitTokens) / 1_000_000 * prices.InputPricePerM) +
		(float64(cacheHitTokens) / 1_000_000 * prices.CacheHitInput)
	return missCost - actualCost
}

// ModelLabel returns a human-readable label for a model ID.
func ModelLabel(model string) string {
	switch ResolveModel(model) {
	case "deepseek-flash":
		return "DeepSeek V4.1 Flash"
	case "deepseek-v4-pro":
		return "DeepSeek V4 Pro"
	default:
		return model
	}
}
