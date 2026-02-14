package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/mcp"
	"github.com/agent-platform/agix/internal/store"
	"github.com/agent-platform/agix/internal/toolmgr"
)

func newTestProxy(t *testing.T) (*Proxy, *store.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := &config.Config{
		Port: 8080,
		Keys: map[string]string{
			"openai":    "sk-test-key",
			"anthropic": "sk-ant-test-key",
			"deepseek":  "sk-ds-test-key",
		},
		Budgets: map[string]config.Budget{
			"budget-agent": {
				DailyLimitUSD:   10.00,
				MonthlyLimitUSD: 100.00,
				AlertAtPercent:  80.0,
			},
		},
	}

	p := New(cfg, st, nil)
	return p, st
}

func TestHealthEndpoint(t *testing.T) {
	p, _ := newTestProxy(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse health response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("health status = %q, want %q", resp["status"], "ok")
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestModelsEndpoint(t *testing.T) {
	p, _ := newTestProxy(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("models status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse models response: %v", err)
	}

	if resp.Object != "list" {
		t.Errorf("models object = %q, want %q", resp.Object, "list")
	}

	if len(resp.Data) == 0 {
		t.Error("models data is empty")
	}

	// Verify each model has required fields
	for _, m := range resp.Data {
		if m.ID == "" {
			t.Error("model ID is empty")
		}
		if m.Object != "model" {
			t.Errorf("model object = %q, want %q", m.Object, "model")
		}
		if m.OwnedBy == "" {
			t.Error("model owned_by is empty")
		}
	}
}

func TestChatCompletionsMethodNotAllowed(t *testing.T) {
	p, _ := newTestProxy(t)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/v1/chat/completions", nil)
			w := httptest.NewRecorder()

			p.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s status = %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestChatCompletionsEmptyBody(t *testing.T) {
	p, _ := newTestProxy(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(""))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("empty body status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChatCompletionsMalformedJSON(t *testing.T) {
	p, _ := newTestProxy(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{invalid json"))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("malformed JSON status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChatCompletionsMissingModel(t *testing.T) {
	p, _ := newTestProxy(t)

	body := `{"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("missing model status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChatCompletionsUnsupportedProvider(t *testing.T) {
	p, _ := newTestProxy(t)

	body := `{"model":"llama-3-70b","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	// Should fail at buildUpstreamRequest with unsupported provider
	if w.Code != http.StatusBadGateway {
		t.Errorf("unsupported provider status = %d, want %d", w.Code, http.StatusBadGateway)
	}
}

func TestChatCompletionsMissingAPIKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := &config.Config{
		Port:    8080,
		Keys:    map[string]string{}, // No keys configured
		Budgets: map[string]config.Budget{},
	}

	p := New(cfg, st, nil)

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("missing API key status = %d, want %d", w.Code, http.StatusBadGateway)
	}
}

func TestExtractUsageOpenAI(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantInput   int
		wantOutput  int
	}{
		{
			name: "standard response",
			body: `{"usage":{"prompt_tokens":100,"completion_tokens":50}}`,
			wantInput:  100,
			wantOutput: 50,
		},
		{
			name:        "empty body",
			body:        `{}`,
			wantInput:   0,
			wantOutput:  0,
		},
		{
			name:        "malformed JSON",
			body:        `{invalid`,
			wantInput:   0,
			wantOutput:  0,
		},
		{
			name: "zero tokens",
			body: `{"usage":{"prompt_tokens":0,"completion_tokens":0}}`,
			wantInput:  0,
			wantOutput: 0,
		},
		{
			name: "large token counts",
			body: `{"usage":{"prompt_tokens":100000,"completion_tokens":50000}}`,
			wantInput:  100000,
			wantOutput: 50000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, output := extractUsage("openai", []byte(tt.body))
			if input != tt.wantInput {
				t.Errorf("input = %d, want %d", input, tt.wantInput)
			}
			if output != tt.wantOutput {
				t.Errorf("output = %d, want %d", output, tt.wantOutput)
			}
		})
	}
}

func TestExtractUsageAnthropic(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantInput   int
		wantOutput  int
	}{
		{
			name: "standard response",
			body: `{"usage":{"input_tokens":200,"output_tokens":100}}`,
			wantInput:  200,
			wantOutput: 100,
		},
		{
			name:        "empty body",
			body:        `{}`,
			wantInput:   0,
			wantOutput:  0,
		},
		{
			name:        "malformed JSON",
			body:        `not json`,
			wantInput:   0,
			wantOutput:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, output := extractUsage("anthropic", []byte(tt.body))
			if input != tt.wantInput {
				t.Errorf("input = %d, want %d", input, tt.wantInput)
			}
			if output != tt.wantOutput {
				t.Errorf("output = %d, want %d", output, tt.wantOutput)
			}
		})
	}
}

func TestExtractUsageDeepSeek(t *testing.T) {
	// DeepSeek uses same format as OpenAI
	input, output := extractUsage("deepseek", []byte(`{"usage":{"prompt_tokens":300,"completion_tokens":150}}`))
	if input != 300 {
		t.Errorf("input = %d, want 300", input)
	}
	if output != 150 {
		t.Errorf("output = %d, want 150", output)
	}
}

func TestExtractUsageUnknownProvider(t *testing.T) {
	input, output := extractUsage("unknown", []byte(`{"usage":{"prompt_tokens":100}}`))
	if input != 0 || output != 0 {
		t.Errorf("unknown provider: input=%d, output=%d, want 0, 0", input, output)
	}
}

func TestExtractStreamUsageOpenAI(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		wantInput   int
		wantOutput  int
	}{
		{
			name: "final chunk with usage",
			data: `{"usage":{"prompt_tokens":150,"completion_tokens":75}}`,
			wantInput:  150,
			wantOutput: 75,
		},
		{
			name:        "chunk without usage",
			data:        `{"choices":[{"delta":{"content":"hello"}}]}`,
			wantInput:   0,
			wantOutput:  0,
		},
		{
			name:        "malformed JSON",
			data:        `{bad`,
			wantInput:   0,
			wantOutput:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, output := extractStreamUsage("openai", []byte(tt.data))
			if input != tt.wantInput {
				t.Errorf("input = %d, want %d", input, tt.wantInput)
			}
			if output != tt.wantOutput {
				t.Errorf("output = %d, want %d", output, tt.wantOutput)
			}
		})
	}
}

func TestExtractStreamUsageAnthropic(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		wantInput   int
		wantOutput  int
	}{
		{
			name: "message_start with usage",
			data: `{"type":"message_start","message":{"usage":{"input_tokens":200,"output_tokens":0}}}`,
			wantInput:  200,
			wantOutput: 0,
		},
		{
			name: "message_delta with usage",
			data: `{"type":"message_delta","usage":{"input_tokens":0,"output_tokens":150}}`,
			wantInput:  0,
			wantOutput: 150,
		},
		{
			name:        "content_block_delta no usage",
			data:        `{"type":"content_block_delta","delta":{"text":"hello"}}`,
			wantInput:   0,
			wantOutput:  0,
		},
		{
			name:        "malformed JSON",
			data:        `invalid`,
			wantInput:   0,
			wantOutput:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, output := extractStreamUsage("anthropic", []byte(tt.data))
			if input != tt.wantInput {
				t.Errorf("input = %d, want %d", input, tt.wantInput)
			}
			if output != tt.wantOutput {
				t.Errorf("output = %d, want %d", output, tt.wantOutput)
			}
		})
	}
}

func TestConvertToAnthropicFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, result map[string]any)
	}{
		{
			name:  "basic conversion",
			input: `{"model":"claude-opus-4-6","messages":[{"role":"user","content":"hello"}],"max_tokens":100}`,
			check: func(t *testing.T, result map[string]any) {
				if result["model"] != "claude-opus-4-6" {
					t.Errorf("model = %v, want claude-opus-4-6", result["model"])
				}
				msgs := result["messages"].([]any)
				if len(msgs) != 1 {
					t.Errorf("messages len = %d, want 1", len(msgs))
				}
				if result["max_tokens"].(float64) != 100 {
					t.Errorf("max_tokens = %v, want 100", result["max_tokens"])
				}
			},
		},
		{
			name:  "system message extracted",
			input: `{"model":"claude-opus-4-6","messages":[{"role":"system","content":"You are helpful"},{"role":"user","content":"hello"}]}`,
			check: func(t *testing.T, result map[string]any) {
				if result["system"] != "You are helpful" {
					t.Errorf("system = %v, want 'You are helpful'", result["system"])
				}
				msgs := result["messages"].([]any)
				if len(msgs) != 1 {
					t.Errorf("messages should have 1 entry (no system), got %d", len(msgs))
				}
				msg := msgs[0].(map[string]any)
				if msg["role"] != "user" {
					t.Errorf("first message role = %v, want 'user'", msg["role"])
				}
			},
		},
		{
			name:  "default max_tokens when 0",
			input: `{"model":"claude-opus-4-6","messages":[{"role":"user","content":"hello"}]}`,
			check: func(t *testing.T, result map[string]any) {
				if result["max_tokens"].(float64) != 4096 {
					t.Errorf("max_tokens = %v, want 4096 (default)", result["max_tokens"])
				}
			},
		},
		{
			name:  "streaming flag preserved",
			input: `{"model":"claude-opus-4-6","messages":[{"role":"user","content":"hello"}],"stream":true}`,
			check: func(t *testing.T, result map[string]any) {
				if result["stream"] != true {
					t.Errorf("stream = %v, want true", result["stream"])
				}
			},
		},
		{
			name:  "temperature preserved",
			input: `{"model":"claude-opus-4-6","messages":[{"role":"user","content":"hello"}],"temperature":0.7}`,
			check: func(t *testing.T, result map[string]any) {
				if result["temperature"].(float64) != 0.7 {
					t.Errorf("temperature = %v, want 0.7", result["temperature"])
				}
			},
		},
		{
			name:    "malformed JSON",
			input:   `{bad json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToAnthropicFormat([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result map[string]any
			if err := json.Unmarshal(got, &result); err != nil {
				t.Fatalf("failed to parse result: %v", err)
			}

			tt.check(t, result)
		})
	}
}

func TestBuildUpstreamRequest(t *testing.T) {
	p, _ := newTestProxy(t)

	tests := []struct {
		name        string
		provider    string
		model       string
		body        string
		wantURL     string
		wantErr     bool
		checkHeaders func(t *testing.T, headers map[string]string)
	}{
		{
			name:     "openai request",
			provider: "openai",
			model:    "gpt-4o",
			body:     `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`,
			wantURL:  "https://api.openai.com/v1/chat/completions",
			checkHeaders: func(t *testing.T, headers map[string]string) {
				if !strings.HasPrefix(headers["Authorization"], "Bearer ") {
					t.Error("missing Bearer token in Authorization header")
				}
				if headers["Content-Type"] != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", headers["Content-Type"])
				}
			},
		},
		{
			name:     "anthropic request",
			provider: "anthropic",
			model:    "claude-opus-4-6",
			body:     `{"model":"claude-opus-4-6","messages":[{"role":"user","content":"hello"}]}`,
			wantURL:  "https://api.anthropic.com/v1/messages",
			checkHeaders: func(t *testing.T, headers map[string]string) {
				if headers["x-api-key"] == "" {
					t.Error("missing x-api-key header")
				}
				if headers["anthropic-version"] != "2023-06-01" {
					t.Errorf("anthropic-version = %q, want 2023-06-01", headers["anthropic-version"])
				}
			},
		},
		{
			name:     "deepseek request",
			provider: "deepseek",
			model:    "deepseek-chat",
			body:     `{"model":"deepseek-chat","messages":[{"role":"user","content":"hello"}]}`,
			wantURL:  "https://api.deepseek.com/chat/completions",
			checkHeaders: func(t *testing.T, headers map[string]string) {
				if !strings.HasPrefix(headers["Authorization"], "Bearer ") {
					t.Error("missing Bearer token in Authorization header")
				}
				if headers["Content-Type"] != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", headers["Content-Type"])
				}
			},
		},
		{
			name:     "unsupported provider",
			provider: "unknown",
			model:    "some-model",
			body:     `{}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, headers, _, err := p.buildUpstreamRequest(tt.provider, tt.model, []byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if url != tt.wantURL {
				t.Errorf("url = %q, want %q", url, tt.wantURL)
			}
			if tt.checkHeaders != nil {
				tt.checkHeaders(t, headers)
			}
		})
	}
}

func TestCheckBudgetNoBudgetConfigured(t *testing.T) {
	p, _ := newTestProxy(t)

	// Agent without budget should pass
	err := p.checkBudget("no-budget-agent")
	if err != nil {
		t.Errorf("checkBudget() for unconfigured agent returned error: %v", err)
	}
}

func TestCheckBudgetUnderLimit(t *testing.T) {
	p, st := newTestProxy(t)

	// Insert a small cost record for budget-agent
	now := time.Now().UTC()
	if err := st.Insert(&store.Record{
		Timestamp: now, AgentName: "budget-agent", Model: "gpt-4o", Provider: "openai",
		InputTokens: 100, OutputTokens: 50, CostUSD: 1.00, DurationMS: 100, StatusCode: 200,
	}); err != nil {
		t.Fatalf("Insert() error: %v", err)
	}

	err := p.checkBudget("budget-agent")
	if err != nil {
		t.Errorf("checkBudget() under limit returned error: %v", err)
	}
}

func TestForceNonStreaming(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool // expected stream value
	}{
		{
			name:  "stream true becomes false",
			input: `{"model":"gpt-4o","stream":true}`,
			want:  false,
		},
		{
			name:  "already false stays false",
			input: `{"model":"gpt-4o","stream":false}`,
			want:  false,
		},
		{
			name:  "no stream field gets false",
			input: `{"model":"gpt-4o"}`,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := forceNonStreaming([]byte(tt.input))
			var parsed map[string]any
			if err := json.Unmarshal(got, &parsed); err != nil {
				t.Fatalf("failed to parse result: %v", err)
			}
			if parsed["stream"] != false {
				t.Errorf("stream = %v, want %v", parsed["stream"], tt.want)
			}
		})
	}
}

func TestInjectToolsOpenAI(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[]}`)
	tools := []toolmgr.ToolEntry{
		{Tool: mcp.Tool{Name: "read_file", Description: "Read a file", InputSchema: map[string]any{"type": "object"}}, Server: "fs"},
	}

	result := injectTools(body, tools, "openai")

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	toolsArr, ok := parsed["tools"].([]any)
	if !ok || len(toolsArr) != 1 {
		t.Fatalf("expected 1 tool, got %v", parsed["tools"])
	}

	tool := toolsArr[0].(map[string]any)
	if tool["type"] != "function" {
		t.Errorf("tool type = %v, want function", tool["type"])
	}
	fn := tool["function"].(map[string]any)
	if fn["name"] != "read_file" {
		t.Errorf("function name = %v, want read_file", fn["name"])
	}
}

func TestInjectToolsAnthropic(t *testing.T) {
	body := []byte(`{"model":"claude-opus-4-6","messages":[]}`)
	tools := []toolmgr.ToolEntry{
		{Tool: mcp.Tool{Name: "read_file", Description: "Read a file"}, Server: "fs"},
	}

	result := injectTools(body, tools, "anthropic")

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	toolsArr, ok := parsed["tools"].([]any)
	if !ok || len(toolsArr) != 1 {
		t.Fatalf("expected 1 tool, got %v", parsed["tools"])
	}

	tool := toolsArr[0].(map[string]any)
	if tool["name"] != "read_file" {
		t.Errorf("tool name = %v, want read_file", tool["name"])
	}
	// Anthropic should have input_schema even if not set
	if _, ok := tool["input_schema"]; !ok {
		t.Error("expected input_schema for Anthropic tool")
	}
}

func TestExtractToolCallsOpenAI(t *testing.T) {
	body := []byte(`{
		"choices": [{
			"finish_reason": "tool_calls",
			"message": {
				"role": "assistant",
				"tool_calls": [{
					"id": "call_123",
					"type": "function",
					"function": {
						"name": "read_file",
						"arguments": "{\"path\":\"/tmp/test.txt\"}"
					}
				}]
			}
		}]
	}`)

	calls := extractToolCalls("openai", body)
	if len(calls) != 1 {
		t.Fatalf("extractToolCalls() = %d calls, want 1", len(calls))
	}
	if calls[0].ID != "call_123" {
		t.Errorf("call ID = %q, want call_123", calls[0].ID)
	}
	if calls[0].Name != "read_file" {
		t.Errorf("call name = %q, want read_file", calls[0].Name)
	}
	if calls[0].Arguments["path"] != "/tmp/test.txt" {
		t.Errorf("call args path = %v, want /tmp/test.txt", calls[0].Arguments["path"])
	}
}

func TestExtractToolCallsOpenAINoToolCalls(t *testing.T) {
	body := []byte(`{
		"choices": [{
			"finish_reason": "stop",
			"message": {"role": "assistant", "content": "Hello!"}
		}]
	}`)

	calls := extractToolCalls("openai", body)
	if len(calls) != 0 {
		t.Errorf("extractToolCalls() = %d calls, want 0", len(calls))
	}
}

func TestExtractToolCallsAnthropic(t *testing.T) {
	body := []byte(`{
		"stop_reason": "tool_use",
		"content": [
			{"type": "text", "text": "Let me read that file."},
			{"type": "tool_use", "id": "toolu_123", "name": "read_file", "input": {"path": "/tmp/test.txt"}}
		]
	}`)

	calls := extractToolCalls("anthropic", body)
	if len(calls) != 1 {
		t.Fatalf("extractToolCalls() = %d calls, want 1", len(calls))
	}
	if calls[0].ID != "toolu_123" {
		t.Errorf("call ID = %q, want toolu_123", calls[0].ID)
	}
	if calls[0].Name != "read_file" {
		t.Errorf("call name = %q, want read_file", calls[0].Name)
	}
}

func TestExtractToolCallsAnthropicNoToolUse(t *testing.T) {
	body := []byte(`{
		"stop_reason": "end_turn",
		"content": [{"type": "text", "text": "Here is your answer."}]
	}`)

	calls := extractToolCalls("anthropic", body)
	if len(calls) != 0 {
		t.Errorf("extractToolCalls() = %d calls, want 0", len(calls))
	}
}

func TestStripToolCallsOpenAI(t *testing.T) {
	body := []byte(`{
		"choices": [{
			"finish_reason": "stop",
			"message": {
				"role": "assistant",
				"content": "The file contains: hello world",
				"tool_calls": [{"id":"call_1","type":"function","function":{"name":"read_file"}}]
			}
		}]
	}`)

	result := stripToolCalls("openai", body)

	var parsed map[string]any
	json.Unmarshal(result, &parsed)

	choices := parsed["choices"].([]any)
	choice := choices[0].(map[string]any)
	msg := choice["message"].(map[string]any)

	if _, ok := msg["tool_calls"]; ok {
		t.Error("tool_calls should be stripped from response")
	}
	if choice["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %v, want stop", choice["finish_reason"])
	}
}

func TestStripToolCallsAnthropic(t *testing.T) {
	body := []byte(`{
		"stop_reason": "end_turn",
		"content": [
			{"type": "text", "text": "Here is the answer."},
			{"type": "tool_use", "id": "toolu_1", "name": "read_file"}
		]
	}`)

	result := stripToolCalls("anthropic", body)

	var parsed map[string]any
	json.Unmarshal(result, &parsed)

	if parsed["stop_reason"] != "end_turn" {
		t.Errorf("stop_reason = %v, want end_turn", parsed["stop_reason"])
	}

	content := parsed["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content len = %d, want 1 (tool_use stripped)", len(content))
	}
	block := content[0].(map[string]any)
	if block["type"] != "text" {
		t.Errorf("remaining block type = %v, want text", block["type"])
	}
}

func TestAppendToolResultsOpenAI(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"read the file"}]}`)
	respBody := []byte(`{
		"choices": [{
			"message": {
				"role": "assistant",
				"tool_calls": [{"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{}"}}]
			}
		}]
	}`)
	calls := []toolCall{{ID: "call_1", Name: "read_file"}}
	results := []string{"file content here"}

	result := appendToolResults(body, "openai", respBody, calls, results)

	var parsed map[string]any
	json.Unmarshal(result, &parsed)

	var messages []map[string]any
	msgData, _ := json.Marshal(parsed["messages"])
	json.Unmarshal(msgData, &messages)

	// Should have: user, assistant, tool
	if len(messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(messages))
	}
	if messages[0]["role"] != "user" {
		t.Errorf("messages[0].role = %v, want user", messages[0]["role"])
	}
	if messages[1]["role"] != "assistant" {
		t.Errorf("messages[1].role = %v, want assistant", messages[1]["role"])
	}
	if messages[2]["role"] != "tool" {
		t.Errorf("messages[2].role = %v, want tool", messages[2]["role"])
	}
	if messages[2]["tool_call_id"] != "call_1" {
		t.Errorf("messages[2].tool_call_id = %v, want call_1", messages[2]["tool_call_id"])
	}
	if messages[2]["content"] != "file content here" {
		t.Errorf("messages[2].content = %v, want 'file content here'", messages[2]["content"])
	}
}

func TestAppendToolResultsAnthropic(t *testing.T) {
	body := []byte(`{"model":"claude-opus-4-6","messages":[{"role":"user","content":"read the file"}]}`)
	respBody := []byte(`{
		"content": [
			{"type": "text", "text": "I'll read that."},
			{"type": "tool_use", "id": "toolu_1", "name": "read_file", "input": {}}
		]
	}`)
	calls := []toolCall{{ID: "toolu_1", Name: "read_file"}}
	results := []string{"file content here"}

	result := appendToolResults(body, "anthropic", respBody, calls, results)

	var parsed map[string]any
	json.Unmarshal(result, &parsed)

	var messages []map[string]any
	msgData, _ := json.Marshal(parsed["messages"])
	json.Unmarshal(msgData, &messages)

	// Should have: user, assistant, user (with tool_result)
	if len(messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(messages))
	}
	if messages[1]["role"] != "assistant" {
		t.Errorf("messages[1].role = %v, want assistant", messages[1]["role"])
	}
	if messages[2]["role"] != "user" {
		t.Errorf("messages[2].role = %v, want user", messages[2]["role"])
	}
}
