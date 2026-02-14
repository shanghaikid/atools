package pricing

import (
	"math"
	"testing"
)

func TestLookup(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		wantNil     bool
		wantInput   float64
		wantOutput  float64
		wantProvider string
	}{
		{
			name:        "exact match gpt-4o",
			model:       "gpt-4o",
			wantNil:     false,
			wantInput:   2.50,
			wantOutput:  10.00,
			wantProvider: "openai",
		},
		{
			name:        "exact match claude-opus-4-6",
			model:       "claude-opus-4-6",
			wantNil:     false,
			wantInput:   5.00,
			wantOutput:  25.00,
			wantProvider: "anthropic",
		},
		{
			name:        "exact match gpt-5",
			model:       "gpt-5",
			wantNil:     false,
			wantInput:   1.25,
			wantOutput:  10.00,
			wantProvider: "openai",
		},
		{
			name:        "exact match gpt-4.1",
			model:       "gpt-4.1",
			wantNil:     false,
			wantInput:   2.00,
			wantOutput:  8.00,
			wantProvider: "openai",
		},
		{
			name:        "case insensitive",
			model:       "GPT-4o",
			wantNil:     false,
			wantInput:   2.50,
			wantOutput:  10.00,
			wantProvider: "openai",
		},
		{
			name:        "prefix match versioned model",
			model:       "gpt-4o-2024-08-06",
			wantNil:     false,
			wantInput:   2.50,
			wantOutput:  10.00,
			wantProvider: "openai",
		},
		{
			name:        "exact match deepseek-chat",
			model:       "deepseek-chat",
			wantNil:     false,
			wantInput:   0.27,
			wantOutput:  1.10,
			wantProvider: "deepseek",
		},
		{
			name:        "exact match deepseek-reasoner",
			model:       "deepseek-reasoner",
			wantNil:     false,
			wantInput:   0.55,
			wantOutput:  2.19,
			wantProvider: "deepseek",
		},
		{
			name:    "unknown model returns nil",
			model:   "llama-3-70b",
			wantNil: true,
		},
		{
			name:    "empty model returns nil",
			model:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Lookup(tt.model)
			if tt.wantNil {
				if got != nil {
					t.Errorf("Lookup(%q) = %+v, want nil", tt.model, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("Lookup(%q) = nil, want non-nil", tt.model)
			}
			if got.InputPer1M != tt.wantInput {
				t.Errorf("Lookup(%q).InputPer1M = %f, want %f", tt.model, got.InputPer1M, tt.wantInput)
			}
			if got.OutputPer1M != tt.wantOutput {
				t.Errorf("Lookup(%q).OutputPer1M = %f, want %f", tt.model, got.OutputPer1M, tt.wantOutput)
			}
			if got.Provider != tt.wantProvider {
				t.Errorf("Lookup(%q).Provider = %q, want %q", tt.model, got.Provider, tt.wantProvider)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		inputTokens  int
		outputTokens int
		wantCost     float64
	}{
		{
			name:         "gpt-4o standard usage",
			model:        "gpt-4o",
			inputTokens:  1000,
			outputTokens: 500,
			wantCost:     (1000.0/1_000_000)*2.50 + (500.0/1_000_000)*10.00,
		},
		{
			name:         "claude-opus-4-6 standard usage",
			model:        "claude-opus-4-6",
			inputTokens:  10000,
			outputTokens: 2000,
			wantCost:     (10000.0/1_000_000)*5.00 + (2000.0/1_000_000)*25.00,
		},
		{
			name:         "zero tokens",
			model:        "gpt-4o",
			inputTokens:  0,
			outputTokens: 0,
			wantCost:     0,
		},
		{
			name:         "unknown model returns zero",
			model:        "llama-3-70b",
			inputTokens:  1000,
			outputTokens: 500,
			wantCost:     0,
		},
		{
			name:         "1M input tokens gpt-4.1",
			model:        "gpt-4.1",
			inputTokens:  1_000_000,
			outputTokens: 0,
			wantCost:     2.00,
		},
		{
			name:         "1M output tokens gpt-4.1",
			model:        "gpt-4.1",
			inputTokens:  0,
			outputTokens: 1_000_000,
			wantCost:     8.00,
		},
		{
			name:         "only input tokens",
			model:        "gpt-4o-mini",
			inputTokens:  500000,
			outputTokens: 0,
			wantCost:     (500000.0 / 1_000_000) * 0.15,
		},
		{
			name:         "only output tokens",
			model:        "gpt-4o-mini",
			inputTokens:  0,
			outputTokens: 500000,
			wantCost:     (500000.0 / 1_000_000) * 0.60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCost(tt.model, tt.inputTokens, tt.outputTokens)
			if math.Abs(got-tt.wantCost) > 1e-9 {
				t.Errorf("CalculateCost(%q, %d, %d) = %f, want %f", tt.model, tt.inputTokens, tt.outputTokens, got, tt.wantCost)
			}
		})
	}
}

func TestProviderForModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  string
	}{
		{name: "openai gpt-4o", model: "gpt-4o", want: "openai"},
		{name: "openai gpt-5", model: "gpt-5", want: "openai"},
		{name: "openai gpt-4.1-mini", model: "gpt-4.1-mini", want: "openai"},
		{name: "openai o1", model: "o1", want: "openai"},
		{name: "openai o3", model: "o3", want: "openai"},
		{name: "openai o3-mini", model: "o3-mini", want: "openai"},
		{name: "openai o4-mini", model: "o4-mini", want: "openai"},
		{name: "anthropic claude-opus-4-6", model: "claude-opus-4-6", want: "anthropic"},
		{name: "anthropic claude-sonnet-4-5-20250929", model: "claude-sonnet-4-5-20250929", want: "anthropic"},
		{name: "anthropic claude-haiku-4-5-20251001", model: "claude-haiku-4-5-20251001", want: "anthropic"},
		{name: "deepseek deepseek-chat", model: "deepseek-chat", want: "deepseek"},
		{name: "deepseek deepseek-reasoner", model: "deepseek-reasoner", want: "deepseek"},
		{name: "unknown model", model: "llama-3-70b", want: "unknown"},
		{name: "empty model", model: "", want: "unknown"},
		{name: "case insensitive gpt", model: "GPT-4o", want: "openai"},
		{name: "case insensitive claude", model: "Claude-opus-4-6", want: "anthropic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProviderForModel(tt.model)
			if got != tt.want {
				t.Errorf("ProviderForModel(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}

func TestListModels(t *testing.T) {
	got := ListModels()
	if len(got) == 0 {
		t.Fatal("ListModels() returned empty list")
	}

	// Verify all known models are in the list
	knownModels := []string{"gpt-5", "gpt-4o", "gpt-4.1", "o3", "o4-mini", "claude-opus-4-6", "claude-sonnet-4-5-20250929", "claude-haiku-4-5-20251001", "deepseek-chat", "deepseek-reasoner"}
	modelSet := make(map[string]bool)
	for _, m := range got {
		modelSet[m] = true
	}

	for _, m := range knownModels {
		if !modelSet[m] {
			t.Errorf("ListModels() missing expected model %q", m)
		}
	}
}
