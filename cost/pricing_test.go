package cost

import (
	"math"
	"testing"
	"time"
)

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// TestOffPeakBasePrices pins the canonical off-peak rates. These were stale
// before the 2026-09-10 V4.1-Flash release and under-reported spend.
func TestOffPeakBasePrices(t *testing.T) {
	offPeak := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) // Thursday midday

	got := PricesFor("deepseek-flash", offPeak)
	if !approxEqual(got.InputPricePerM, 0.15) ||
		!approxEqual(got.OutputPricePerM, 0.60) ||
		!approxEqual(got.CacheHitInput, 0.003) {
		t.Fatalf("deepseek-flash off-peak = %+v, want {0.15 0.60 0.003}", got)
	}

	pro := PricesFor("deepseek-v4-pro", offPeak)
	if !approxEqual(pro.InputPricePerM, 0.66) || !approxEqual(pro.OutputPricePerM, 1.98) {
		t.Fatalf("deepseek-v4-pro off-peak = %+v, want {0.66 1.98 0.022}", pro)
	}
}

// TestPeakDoubles rates: peak is exactly 2x off-peak, inside the documented
// 01:00-04:00 and 06:00-10:00 UTC windows on weekdays only.
func TestPeakDoubles(t *testing.T) {
	base := PricesFor("deepseek-flash", time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC))

	cases := []struct {
		name  string
		at    time.Time
		peaks bool
	}{
		{"early window start", time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC), true},
		{"early window inside", time.Date(2026, 9, 10, 2, 0, 0, 0, time.UTC), true},
		{"gap between windows", time.Date(2026, 9, 10, 4, 30, 0, 0, time.UTC), false},
		{"early window end (exclusive)", time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC), false},
		{"late window inside", time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC), true},
		{"late window end (exclusive)", time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC), false},
		{"weekend peak hour is off-peak", time.Date(2026, 9, 12, 2, 0, 0, 0, time.UTC), false},
	}

	for _, c := range cases {
		got := PricesFor("deepseek-flash", c.at)
		wantInput := base.InputPricePerM
		if c.peaks {
			wantInput = base.InputPricePerM * 2
		}
		if !approxEqual(got.InputPricePerM, wantInput) {
			t.Errorf("%s: input = %v, want %v", c.name, got.InputPricePerM, wantInput)
		}
	}
}

// TestAliasResolvesToFlash verifies legacy request names are billed as Flash.
// DeepSeek routes them to V4.1-Flash, so pricing them as anything else (or
// letting them hit the unknown-model fallback) would mis-report spend.
func TestAliasResolvesToFlash(t *testing.T) {
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	want := PricesFor("deepseek-flash", at)

	for _, alias := range []string{
		"deepseek-v4-flash",
		"deepseek-v4-flash-vision-exp",
		"deepseek-chat",
		"deepseek-reasoner",
	} {
		got := PricesFor(alias, at)
		if got != want {
			t.Errorf("PricesFor(%q) = %+v, want flash prices %+v", alias, got, want)
		}
		if label := ModelLabel(alias); label != "DeepSeek V4.1 Flash" {
			t.Errorf("ModelLabel(%q) = %q, want %q", alias, label, "DeepSeek V4.1 Flash")
		}
	}
}

// TestUnknownModelFallsBackConservatively verifies an unrecognised model is
// billed at the most expensive rate rather than silently under-reported.
func TestUnknownModelFallsBackConservatively(t *testing.T) {
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	got := PricesFor("some-future-model", at)
	want := DefaultPrices[fallbackModel]
	if got != want {
		t.Fatalf("unknown model = %+v, want fallback %+v", got, want)
	}
}

// TestCalculateCostAt pins the arithmetic end to end.
func TestCalculateCostAt(t *testing.T) {
	offPeak := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	in, out, total := CalculateCostAt("deepseek-flash", 1_000_000, 1_000_000, false, offPeak)
	if !approxEqual(in, 0.15) || !approxEqual(out, 0.60) || !approxEqual(total, 0.75) {
		t.Fatalf("off-peak cost = (%v, %v, %v), want (0.15, 0.6, 0.75)", in, out, total)
	}

	peak := time.Date(2026, 9, 10, 2, 0, 0, 0, time.UTC)
	in, out, total = CalculateCostAt("deepseek-flash", 1_000_000, 1_000_000, false, peak)
	if !approxEqual(in, 0.30) || !approxEqual(out, 1.20) || !approxEqual(total, 1.50) {
		t.Fatalf("peak cost = (%v, %v, %v), want (0.3, 1.2, 1.5)", in, out, total)
	}

	// A cache hit bills input at the cache-hit rate, not the cache-miss rate.
	in, _, _ = CalculateCostAt("deepseek-flash", 1_000_000, 0, true, offPeak)
	if !approxEqual(in, 0.003) {
		t.Fatalf("cache-hit input cost = %v, want 0.003", in)
	}
}

// TestModelLabel covers the known models and the raw passthrough.
func TestModelLabel(t *testing.T) {
	cases := map[string]string{
		"deepseek-flash":  "DeepSeek V4.1 Flash",
		"deepseek-v4-pro": "DeepSeek V4 Pro",
		"mystery-model":   "mystery-model",
	}
	for model, want := range cases {
		if got := ModelLabel(model); got != want {
			t.Errorf("ModelLabel(%q) = %q, want %q", model, got, want)
		}
	}
}
