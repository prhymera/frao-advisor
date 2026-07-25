// Package cost provides DeepSeek pricing tables and cost calculation.
// Pricing reflects the permanent 75% discount effective June 2026.
package cost

// ModelPrices holds per-model pricing in USD per 1M tokens.
type ModelPrices struct {
	InputPricePerM  float64 // Cache miss input
	OutputPricePerM float64 // Output
	CacheHitInput   float64 // Cache hit input (~98% discount)
}

// DefaultPrices is the canonical pricing table for all supported models.
var DefaultPrices = map[string]ModelPrices{
	"deepseek-v4-pro": {
		InputPricePerM:  0.435,
		OutputPricePerM: 0.87,
		CacheHitInput:   0.003625,
	},
	"deepseek-v4-flash": {
		InputPricePerM:  0.14,
		OutputPricePerM: 0.28,
		CacheHitInput:   0.0028,
	},
}

// CalculateCost computes the cost of an API call based on token usage.
// Returns (inputCost, outputCost, totalCost) in USD.
// If the model is unknown, uses deepseek-v4-pro pricing as fallback.
func CalculateCost(model string, promptTokens, completionTokens int, cacheHit bool) (inputCost, outputCost, totalCost float64) {
	prices, ok := DefaultPrices[model]
	if !ok {
		prices = DefaultPrices["deepseek-v4-pro"]
	}

	inputPrice := prices.InputPricePerM
	if cacheHit {
		inputPrice = prices.CacheHitInput
	}

	inputCost = float64(promptTokens) / 1_000_000 * inputPrice
	outputCost = float64(completionTokens) / 1_000_000 * prices.OutputPricePerM
	totalCost = inputCost + outputCost
	return
}

// CacheSavings calculates how much was saved by cache hits compared to
// paying cache-miss prices for the prompt.
func CacheSavings(model string, promptTokens, cacheHitTokens int) float64 {
	prices, ok := DefaultPrices[model]
	if !ok {
		prices = DefaultPrices["deepseek-v4-pro"]
	}
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
	switch model {
	case "deepseek-v4-pro":
		return "DeepSeek V4 Pro"
	case "deepseek-v4-flash":
		return "DeepSeek V4 Flash"
	default:
		return model
	}
}
