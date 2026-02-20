package responsepolicy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNew_Disabled(t *testing.T) {
	p, err := New(Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != nil {
		t.Fatal("expected nil policy when disabled")
	}
}

func TestNew_InvalidPattern(t *testing.T) {
	_, err := New(Config{
		Enabled: true,
		RedactPatterns: []RedactRuleConfig{
			{Name: "bad", Pattern: "[invalid"},
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid regex pattern")
	}
}

func openaiResponse(content string) []byte {
	resp := map[string]any{
		"choices": []map[string]any{
			{
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 20,
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

func anthropicResponse(content string) []byte {
	resp := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": content},
		},
		"stop_reason": "end_turn",
		"usage": map[string]any{
			"input_tokens":  10,
			"output_tokens": 20,
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestApply(t *testing.T) {
	tests := []struct {
		name           string
		cfg            Config
		body           []byte
		agentName      string
		wantContains   string
		wantNotContain string
		wantApplied    []string
		wantNoChange   bool
	}{
		{
			name: "redact API key in OpenAI response",
			cfg: Config{
				Enabled: true,
				RedactPatterns: []RedactRuleConfig{
					{Name: "api-keys", Pattern: `(?i)(sk-[a-zA-Z0-9]{20,})`, Replacement: "[REDACTED_KEY]"},
				},
			},
			body:           openaiResponse("Here is your key: sk-abcdefghijklmnopqrstuvwxyz"),
			agentName:      "test-agent",
			wantContains:   "[REDACTED_KEY]",
			wantNotContain: "sk-abcdefghijklmnopqrstuvwxyz",
			wantApplied:    []string{"redact:api-keys"},
		},
		{
			name: "redact email in Anthropic response",
			cfg: Config{
				Enabled: true,
				RedactPatterns: []RedactRuleConfig{
					{Name: "emails", Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, Replacement: "[REDACTED_EMAIL]"},
				},
			},
			body:           anthropicResponse("Contact us at admin@example.com for help."),
			agentName:      "",
			wantContains:   "[REDACTED_EMAIL]",
			wantNotContain: "admin@example.com",
			wantApplied:    []string{"redact:emails"},
		},
		{
			name: "max output chars truncation",
			cfg: Config{
				Enabled:        true,
				MaxOutputChars: 10,
			},
			body:         openaiResponse("This is a very long response that should be truncated"),
			agentName:    "test",
			wantContains: "[TRUNCATED]",
			wantApplied:  []string{"truncate"},
		},
		{
			name: "format validation warns on non-JSON",
			cfg: Config{
				Enabled:     true,
				ForceFormat: "json",
			},
			body:        openaiResponse("This is not JSON"),
			agentName:   "test",
			wantApplied: []string{"format_warning:not_json"},
		},
		{
			name: "format validation passes valid JSON",
			cfg: Config{
				Enabled:     true,
				ForceFormat: "json",
			},
			body:         openaiResponse(`{"key":"value"}`),
			agentName:    "test",
			wantNoChange: true,
		},
		{
			name: "per-agent redaction",
			cfg: Config{
				Enabled: true,
				Agents: map[string]AgentPolicy{
					"sensitive-agent": {
						RedactPatterns: []RedactRuleConfig{
							{Name: "ips", Pattern: `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`, Replacement: "[REDACTED_IP]"},
						},
					},
				},
			},
			body:           openaiResponse("Server at 192.168.1.100 is running."),
			agentName:      "sensitive-agent",
			wantContains:   "[REDACTED_IP]",
			wantNotContain: "192.168.1.100",
			wantApplied:    []string{"redact:ips"},
		},
		{
			name: "per-agent policy does not apply to other agents",
			cfg: Config{
				Enabled: true,
				Agents: map[string]AgentPolicy{
					"sensitive-agent": {
						RedactPatterns: []RedactRuleConfig{
							{Name: "ips", Pattern: `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`, Replacement: "[REDACTED_IP]"},
						},
					},
				},
			},
			body:         openaiResponse("Server at 192.168.1.100 is running."),
			agentName:    "other-agent",
			wantNoChange: true,
		},
		{
			name: "per-agent max output chars override",
			cfg: Config{
				Enabled:        true,
				MaxOutputChars: 1000,
				Agents: map[string]AgentPolicy{
					"limited-agent": {
						MaxOutputChars: 5,
					},
				},
			},
			body:         openaiResponse("This response is longer than 5 chars"),
			agentName:    "limited-agent",
			wantContains: "[TRUNCATED]",
			wantApplied:  []string{"truncate"},
		},
		{
			name: "no rules match — no change",
			cfg: Config{
				Enabled: true,
				RedactPatterns: []RedactRuleConfig{
					{Name: "ssn", Pattern: `\b\d{3}-\d{2}-\d{4}\b`, Replacement: "[REDACTED_SSN]"},
				},
			},
			body:         openaiResponse("Hello world, nothing sensitive here."),
			agentName:    "test",
			wantNoChange: true,
		},
		{
			name: "default replacement when not specified",
			cfg: Config{
				Enabled: true,
				RedactPatterns: []RedactRuleConfig{
					{Name: "keys", Pattern: `(?i)(sk-[a-zA-Z0-9]{20,})`},
				},
			},
			body:         openaiResponse("key: sk-abcdefghijklmnopqrstuvwxyz"),
			agentName:    "test",
			wantContains: "[REDACTED]",
			wantApplied:  []string{"redact:keys"},
		},
		{
			name: "combined global and per-agent rules",
			cfg: Config{
				Enabled: true,
				RedactPatterns: []RedactRuleConfig{
					{Name: "emails", Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, Replacement: "[EMAIL]"},
				},
				Agents: map[string]AgentPolicy{
					"strict": {
						RedactPatterns: []RedactRuleConfig{
							{Name: "ips", Pattern: `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`, Replacement: "[IP]"},
						},
					},
				},
			},
			body:           openaiResponse("Contact admin@test.com at 10.0.0.1"),
			agentName:      "strict",
			wantContains:   "[EMAIL]",
			wantNotContain: "admin@test.com",
			wantApplied:    []string{"redact:emails", "redact:ips"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.cfg)
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}
			if p == nil {
				t.Fatal("expected non-nil policy")
			}

			result, applied := p.Apply(tt.body, tt.agentName)

			if tt.wantNoChange {
				if len(applied) != 0 {
					t.Errorf("expected no applied rules, got %v", applied)
				}
				return
			}

			if tt.wantContains != "" && !strings.Contains(string(result), tt.wantContains) {
				t.Errorf("result should contain %q, got: %s", tt.wantContains, result)
			}

			if tt.wantNotContain != "" && strings.Contains(string(result), tt.wantNotContain) {
				t.Errorf("result should NOT contain %q, got: %s", tt.wantNotContain, result)
			}

			if len(tt.wantApplied) > 0 {
				if len(applied) != len(tt.wantApplied) {
					t.Errorf("expected %d applied rules, got %d: %v", len(tt.wantApplied), len(applied), applied)
				}
				for i, want := range tt.wantApplied {
					if i < len(applied) && applied[i] != want {
						t.Errorf("applied[%d] = %q, want %q", i, applied[i], want)
					}
				}
			}
		})
	}
}

func TestExtractContent_InvalidJSON(t *testing.T) {
	content := extractContent([]byte("not json"))
	if content != "" {
		t.Errorf("expected empty content for invalid JSON, got %q", content)
	}
}

func TestReplaceContent_InvalidJSON(t *testing.T) {
	body := []byte("not json")
	result := replaceContent(body, "new content")
	if string(result) != string(body) {
		t.Error("expected body unchanged for invalid JSON")
	}
}
