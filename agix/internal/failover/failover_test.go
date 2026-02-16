package failover

import "testing"

func TestNew_NilOnEmpty(t *testing.T) {
	if f := New(Config{}); f != nil {
		t.Error("expected nil for empty config")
	}
	if f := New(Config{Chains: map[string][]string{}}); f != nil {
		t.Error("expected nil for empty chains")
	}
}

func TestNew_DefaultMaxRetries(t *testing.T) {
	f := New(Config{
		Chains: map[string][]string{"gpt-4o": {"claude-sonnet-4-20250514"}},
	})
	if f.MaxRetries() != 1 {
		t.Errorf("MaxRetries() = %d, want 1", f.MaxRetries())
	}
}

func TestFallbackModels(t *testing.T) {
	f := New(Config{
		MaxRetries: 2,
		Chains: map[string][]string{
			"gpt-4o":          {"claude-sonnet-4-20250514", "deepseek-chat"},
			"claude-opus-4-6": {"gpt-5"},
		},
	})

	tests := []struct {
		model string
		want  int
	}{
		{"gpt-4o", 2},
		{"claude-opus-4-6", 1},
		{"no-chain", 0},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			chain := f.FallbackModels(tt.model)
			if len(chain) != tt.want {
				t.Errorf("FallbackModels(%q) len = %d, want %d", tt.model, len(chain), tt.want)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{200, false},
		{400, false},
		{401, false},
		{429, false},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
		{599, true},
	}

	for _, tt := range tests {
		if got := IsRetryable(tt.code); got != tt.want {
			t.Errorf("IsRetryable(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}
