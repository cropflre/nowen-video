package service

import (
	"math"
	"testing"
)

func TestEstimate_UsesListPriceForModelsOutsideCatalog(t *testing.T) {
	s := &AICostService{}
	// gpt-5.5 和 claude-sonnet-4-6 不在 catalog：以前按 provider 平均价（openai $2.5/$10、
	// anthropic $3/$15 per 1M）估算；官方标价为 $5/$30 与 $3/$15。
	cases := []struct {
		provider, model string
		in, out         float64 // USD per 1K tokens
	}{
		{"openai", "gpt-5.5", 0.005, 0.030},
		{"openai", "gpt-5-mini", 0.00025, 0.002},
		{"anthropic", "claude-opus-4-6", 0.005, 0.025},
	}
	for _, c := range cases {
		est, err := s.Estimate(c.provider, c.model, 1000, 1000)
		if err != nil {
			t.Fatalf("%s: %v", c.model, err)
		}
		if est.Fallback || math.Abs(est.InputPrice-c.in) > 1e-12 || math.Abs(est.OutputPrice-c.out) > 1e-12 {
			t.Errorf("%s: got in=%v out=%v fallback=%v, want %v/%v", c.model, est.InputPrice, est.OutputPrice, est.Fallback, c.in, c.out)
		}
	}
	// 不认识的型号仍走 provider 平均价。
	if est, _ := s.Estimate("openai", "my-finetune", 1000, 1000); !est.Fallback {
		t.Errorf("unknown model should fall back, got %+v", est)
	}
}
