package router

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNew_NilWhenDisabled(t *testing.T) {
	if r := New(Config{Enabled: false}); r != nil {
		t.Error("expected nil when disabled")
	}
}

func TestNew_NilWhenEmpty(t *testing.T) {
	if r := New(Config{Enabled: true}); r != nil {
		t.Error("expected nil when no tiers/model_map")
	}
}

func TestRoute_NoMapping(t *testing.T) {
	r := New(Config{
		Enabled:  true,
		Tiers:    map[string]TierConfig{"simple": {MaxMessages: 3}},
		ModelMap: map[string]map[string]string{"gpt-4o": {"simple": "gpt-4o-mini"}},
	})
	msgs, _ := json.Marshal([]map[string]string{{"role": "user", "content": "hi"}})
	model, tier := r.Route("claude-opus-4-6", msgs) // no mapping for this model
	if model != "claude-opus-4-6" || tier != "" {
		t.Errorf("expected no routing, got model=%q tier=%q", model, tier)
	}
}

func TestRoute_SimpleRequest(t *testing.T) {
	r := New(Config{
		Enabled: true,
		Tiers: map[string]TierConfig{
			"simple": {MaxMessageTokens: 500, MaxMessages: 3, KeywordsAbsent: []string{"analyze", "refactor"}},
		},
		ModelMap: map[string]map[string]string{
			"gpt-4o": {"simple": "gpt-4o-mini"},
		},
	})

	msgs, _ := json.Marshal([]map[string]string{
		{"role": "user", "content": "What is 2+2?"},
	})

	model, tier := r.Route("gpt-4o", msgs)
	if model != "gpt-4o-mini" {
		t.Errorf("expected gpt-4o-mini, got %q", model)
	}
	if tier != "simple" {
		t.Errorf("expected tier simple, got %q", tier)
	}
}

func TestRoute_ComplexRequest_TooManyMessages(t *testing.T) {
	r := New(Config{
		Enabled: true,
		Tiers: map[string]TierConfig{
			"simple": {MaxMessages: 2},
		},
		ModelMap: map[string]map[string]string{
			"gpt-4o": {"simple": "gpt-4o-mini"},
		},
	})

	msgs, _ := json.Marshal([]map[string]string{
		{"role": "user", "content": "hi"},
		{"role": "assistant", "content": "hello"},
		{"role": "user", "content": "how are you"},
	})

	model, tier := r.Route("gpt-4o", msgs)
	if model != "gpt-4o" || tier != "" {
		t.Errorf("expected no routing for complex request, got model=%q tier=%q", model, tier)
	}
}

func TestRoute_ComplexRequest_KeywordPresent(t *testing.T) {
	r := New(Config{
		Enabled: true,
		Tiers: map[string]TierConfig{
			"simple": {MaxMessages: 10, KeywordsAbsent: []string{"refactor"}},
		},
		ModelMap: map[string]map[string]string{
			"gpt-4o": {"simple": "gpt-4o-mini"},
		},
	})

	msgs, _ := json.Marshal([]map[string]string{
		{"role": "user", "content": "Please refactor this function"},
	})

	model, _ := r.Route("gpt-4o", msgs)
	if model != "gpt-4o" {
		t.Errorf("expected no routing when keyword present, got %q", model)
	}
}

func TestRoute_SystemMessagesNotCounted(t *testing.T) {
	r := New(Config{
		Enabled: true,
		Tiers: map[string]TierConfig{
			"simple": {MaxMessages: 1},
		},
		ModelMap: map[string]map[string]string{
			"gpt-4o": {"simple": "gpt-4o-mini"},
		},
	})

	msgs, _ := json.Marshal([]map[string]string{
		{"role": "system", "content": "You are helpful"},
		{"role": "user", "content": "hi"},
	})

	model, tier := r.Route("gpt-4o", msgs)
	if model != "gpt-4o-mini" || tier != "simple" {
		t.Errorf("system message should not count, got model=%q tier=%q", model, tier)
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 1},         // 1 word * 1.3 = 1
		{"hello world", 2},   // 2 words * 1.3 = 2
		{strings.Repeat("word ", 100), 130}, // 100 words * 1.3 = 130
	}

	for _, tt := range tests {
		got := estimateTokens(tt.input)
		if got != tt.want {
			t.Errorf("estimateTokens(%q) = %d, want %d", tt.input[:min(20, len(tt.input))], got, tt.want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
