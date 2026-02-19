package promptinject

import (
	"encoding/json"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		cfg    Config
		wantNil bool
	}{
		{
			name:    "empty config returns nil",
			cfg:     Config{},
			wantNil: true,
		},
		{
			name:    "global only",
			cfg:     Config{Global: "hello"},
			wantNil: false,
		},
		{
			name:    "agents only",
			cfg:     Config{Agents: map[string]string{"a": "b"}},
			wantNil: false,
		},
		{
			name:    "default position is prepend",
			cfg:     Config{Global: "x"},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inj := New(tt.cfg)
			if (inj == nil) != tt.wantNil {
				t.Errorf("New() nil = %v, want nil = %v", inj == nil, tt.wantNil)
			}
		})
	}
}

func TestInject(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		body      string
		agentName string
		wantSys   string // expected system message content
		wantCount int    // expected number of messages
	}{
		{
			name:      "no system message → injects new one",
			cfg:       Config{Global: "Be helpful."},
			body:      `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`,
			agentName: "",
			wantSys:   "Be helpful.",
			wantCount: 2,
		},
		{
			name:      "existing system message + prepend",
			cfg:       Config{Global: "Be safe.", Position: "prepend"},
			body:      `{"model":"gpt-4o","messages":[{"role":"system","content":"Original."},{"role":"user","content":"hello"}]}`,
			agentName: "",
			wantSys:   "Be safe.\nOriginal.",
			wantCount: 2,
		},
		{
			name:      "existing system message + append",
			cfg:       Config{Global: "Be safe.", Position: "append"},
			body:      `{"model":"gpt-4o","messages":[{"role":"system","content":"Original."},{"role":"user","content":"hello"}]}`,
			agentName: "",
			wantSys:   "Original.\nBe safe.",
			wantCount: 2,
		},
		{
			name:      "global only, no agent match",
			cfg:       Config{Global: "Global rule.", Agents: map[string]string{"other": "Other prompt."}},
			body:      `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`,
			agentName: "unknown-agent",
			wantSys:   "Global rule.",
			wantCount: 2,
		},
		{
			name:      "agent only, no global",
			cfg:       Config{Agents: map[string]string{"coder": "You are a coder."}},
			body:      `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`,
			agentName: "coder",
			wantSys:   "You are a coder.",
			wantCount: 2,
		},
		{
			name:      "global + agent combined",
			cfg:       Config{Global: "Be safe.", Agents: map[string]string{"coder": "Code carefully."}},
			body:      `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`,
			agentName: "coder",
			wantSys:   "Be safe.\nCode carefully.",
			wantCount: 2,
		},
		{
			name:      "empty agent name → global only",
			cfg:       Config{Global: "Global.", Agents: map[string]string{"coder": "Coder."}},
			body:      `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`,
			agentName: "",
			wantSys:   "Global.",
			wantCount: 2,
		},
		{
			name:      "no matching agent config → global only",
			cfg:       Config{Global: "Global.", Agents: map[string]string{"coder": "Coder."}},
			body:      `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`,
			agentName: "reviewer",
			wantSys:   "Global.",
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inj := New(tt.cfg)
			if inj == nil {
				t.Fatal("New() returned nil for valid config")
			}

			result := inj.Inject([]byte(tt.body), tt.agentName)

			var parsed struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.Unmarshal(result, &parsed); err != nil {
				t.Fatalf("failed to parse result: %v", err)
			}

			if len(parsed.Messages) != tt.wantCount {
				t.Errorf("message count = %d, want %d", len(parsed.Messages), tt.wantCount)
			}

			if parsed.Messages[0].Role != "system" {
				t.Errorf("first message role = %q, want \"system\"", parsed.Messages[0].Role)
			}

			if parsed.Messages[0].Content != tt.wantSys {
				t.Errorf("system content = %q, want %q", parsed.Messages[0].Content, tt.wantSys)
			}
		})
	}
}

func TestInject_NoOp(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		body      string
		agentName string
	}{
		{
			name:      "no matching config → body unchanged",
			cfg:       Config{Agents: map[string]string{"coder": "Coder."}},
			body:      `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`,
			agentName: "reviewer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inj := New(tt.cfg)
			if inj == nil {
				t.Fatal("New() returned nil")
			}

			result := inj.Inject([]byte(tt.body), tt.agentName)

			// Normalize both for comparison
			var orig, res any
			json.Unmarshal([]byte(tt.body), &orig)
			json.Unmarshal(result, &res)

			origJSON, _ := json.Marshal(orig)
			resJSON, _ := json.Marshal(res)

			if string(origJSON) != string(resJSON) {
				t.Errorf("body was modified when it should not have been:\ngot:  %s\nwant: %s", resJSON, origJSON)
			}
		})
	}
}

func TestInject_InvalidJSON(t *testing.T) {
	inj := New(Config{Global: "test"})
	result := inj.Inject([]byte("not json"), "agent")
	if string(result) != "not json" {
		t.Error("invalid JSON should return body unchanged")
	}
}
