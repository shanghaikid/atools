package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestVerifySignature(t *testing.T) {
	tests := []struct {
		name      string
		secret    string
		body      []byte
		signature string
		want      bool
	}{
		{
			name:   "valid signature",
			secret: "test-secret",
			body:   []byte(`{"event":"deploy"}`),
			signature: func() string {
				mac := hmac.New(sha256.New, []byte("test-secret"))
				mac.Write([]byte(`{"event":"deploy"}`))
				return hex.EncodeToString(mac.Sum(nil))
			}(),
			want: true,
		},
		{
			name:      "invalid signature",
			secret:    "test-secret",
			body:      []byte(`{"event":"deploy"}`),
			signature: "deadbeef",
			want:      false,
		},
		{
			name:      "empty signature",
			secret:    "test-secret",
			body:      []byte(`{"event":"deploy"}`),
			signature: "",
			want:      false,
		},
		{
			name:   "empty body",
			secret: "test-secret",
			body:   []byte{},
			signature: func() string {
				mac := hmac.New(sha256.New, []byte("test-secret"))
				return hex.EncodeToString(mac.Sum(nil))
			}(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VerifySignature(tt.secret, tt.body, tt.signature)
			if got != tt.want {
				t.Errorf("VerifySignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    string
		payload string
		want    string
		wantErr bool
	}{
		{
			name:    "simple template",
			tmpl:    "Analyze: {{.Payload}}",
			payload: "deploy event",
			want:    "Analyze: deploy event",
		},
		{
			name:    "multiline template",
			tmpl:    "Event:\n{{.Payload}}\nEnd.",
			payload: "test payload",
			want:    "Event:\ntest payload\nEnd.",
		},
		{
			name:    "invalid template",
			tmpl:    "{{.Invalid",
			payload: "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderTemplate(tt.tmpl, tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("renderTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExecuteIntegration(t *testing.T) {
	// Create a mock LLM server
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "Analysis complete: looks good",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer llmServer.Close()

	// Create a mock callback server
	var callbackReceived bool
	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callbackReceived = true
		w.WriteHeader(http.StatusOK)
	}))
	defer callbackServer.Close()

	st := newTestStore(t)

	// We can't easily test Execute end-to-end without a running proxy,
	// but we can test the store integration
	execID, err := st.InsertWebhookExecution("test-hook", "pending", `{"event":"deploy"}`)
	if err != nil {
		t.Fatalf("InsertWebhookExecution() error: %v", err)
	}

	if execID <= 0 {
		t.Fatalf("InsertWebhookExecution() returned invalid ID: %d", execID)
	}

	// Update to completed
	err = st.UpdateWebhookExecution(execID, "completed", "Analysis result", "", 1500, 200)
	if err != nil {
		t.Fatalf("UpdateWebhookExecution() error: %v", err)
	}

	// Query back
	execs, err := st.QueryWebhookExecutions(10, "test-hook")
	if err != nil {
		t.Fatalf("QueryWebhookExecutions() error: %v", err)
	}

	if len(execs) != 1 {
		t.Fatalf("got %d executions, want 1", len(execs))
	}

	exec := execs[0]
	if exec.WebhookName != "test-hook" {
		t.Errorf("webhook_name = %q, want %q", exec.WebhookName, "test-hook")
	}
	if exec.Status != "completed" {
		t.Errorf("status = %q, want %q", exec.Status, "completed")
	}
	if exec.Result != "Analysis result" {
		t.Errorf("result = %q, want %q", exec.Result, "Analysis result")
	}
	if exec.DurationMS != 1500 {
		t.Errorf("duration_ms = %d, want 1500", exec.DurationMS)
	}
	if exec.CallbackCode != 200 {
		t.Errorf("callback_code = %d, want 200", exec.CallbackCode)
	}
	_ = callbackReceived
}

func TestQueryWebhookExecutionsFiltering(t *testing.T) {
	st := newTestStore(t)

	// Insert executions for different webhooks
	for _, name := range []string{"hook-a", "hook-b", "hook-a"} {
		_, err := st.InsertWebhookExecution(name, "completed", "payload")
		if err != nil {
			t.Fatalf("InsertWebhookExecution() error: %v", err)
		}
	}

	// Query all
	all, err := st.QueryWebhookExecutions(10, "")
	if err != nil {
		t.Fatalf("QueryWebhookExecutions() error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("got %d executions, want 3", len(all))
	}

	// Query filtered
	filtered, err := st.QueryWebhookExecutions(10, "hook-a")
	if err != nil {
		t.Fatalf("QueryWebhookExecutions() error: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("got %d filtered executions, want 2", len(filtered))
	}
}

func TestQueryWebhookExecutionsLimit(t *testing.T) {
	st := newTestStore(t)

	for i := 0; i < 5; i++ {
		_, err := st.InsertWebhookExecution("hook", "completed", fmt.Sprintf("payload-%d", i))
		if err != nil {
			t.Fatalf("InsertWebhookExecution() error: %v", err)
		}
	}

	limited, err := st.QueryWebhookExecutions(3, "")
	if err != nil {
		t.Fatalf("QueryWebhookExecutions() error: %v", err)
	}
	if len(limited) != 3 {
		t.Errorf("got %d executions with limit=3, want 3", len(limited))
	}
}

func TestNewHandler(t *testing.T) {
	st := newTestStore(t)
	cfg := config.WebhookConfig{
		Enabled: true,
		Definitions: map[string]config.WebhookDefinition{
			"test": {
				Secret:         "secret",
				Model:          "gpt-4o-mini",
				PromptTemplate: "Analyze: {{.Payload}}",
			},
		},
	}
	proxyCfg := &config.Config{Port: 8080}

	h := New(cfg, proxyCfg, st)
	if h == nil {
		t.Fatal("New() returned nil")
	}

	defs := h.Definitions()
	if len(defs) != 1 {
		t.Errorf("got %d definitions, want 1", len(defs))
	}
	if _, ok := defs["test"]; !ok {
		t.Error("missing 'test' definition")
	}
}

func TestSendCallback(t *testing.T) {
	var receivedBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	st := newTestStore(t)
	h := New(config.WebhookConfig{}, &config.Config{Port: 8080}, st)

	code, err := h.sendCallback(server.URL, "test-hook", "result text")
	if err != nil {
		t.Fatalf("sendCallback() error: %v", err)
	}
	if code != 200 {
		t.Errorf("callback code = %d, want 200", code)
	}
	if receivedBody["webhook"] != "test-hook" {
		t.Errorf("webhook = %q, want %q", receivedBody["webhook"], "test-hook")
	}
	if receivedBody["result"] != "result text" {
		t.Errorf("result = %q, want %q", receivedBody["result"], "result text")
	}
}

func TestSendCallbackError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	st := newTestStore(t)
	h := New(config.WebhookConfig{}, &config.Config{Port: 8080}, st)

	code, err := h.sendCallback(server.URL, "test-hook", "result")
	if err == nil {
		t.Error("expected error for 500 callback response")
	}
	if code != 500 {
		t.Errorf("callback code = %d, want 500", code)
	}
}

// Suppress unused import warnings
var _ = time.Now
