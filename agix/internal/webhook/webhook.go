package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"text/template"
	"time"

	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/store"
)

// Handler manages webhook execution.
type Handler struct {
	cfg      config.WebhookConfig
	proxyCfg *config.Config
	store    *store.Store
	client   *http.Client
}

// New creates a new webhook Handler.
func New(cfg config.WebhookConfig, proxyCfg *config.Config, st *store.Store) *Handler {
	return &Handler{
		cfg:      cfg,
		proxyCfg: proxyCfg,
		store:    st,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Definitions returns the configured webhook definitions.
func (h *Handler) Definitions() map[string]config.WebhookDefinition {
	return h.cfg.Definitions
}

// VerifySignature checks the HMAC-SHA256 signature for a webhook.
func VerifySignature(secret string, body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// Execute runs a webhook: render template, call LLM, update store, fire callback.
func (h *Handler) Execute(execID int64, name string, payload string) {
	start := time.Now()
	def, ok := h.cfg.Definitions[name]
	if !ok {
		h.store.UpdateWebhookExecution(execID, "failed", "", "webhook not found", 0, 0)
		return
	}

	// Render prompt template
	prompt, err := renderTemplate(def.PromptTemplate, payload)
	if err != nil {
		h.store.UpdateWebhookExecution(execID, "failed", "", fmt.Sprintf("template render: %s", err), time.Since(start).Milliseconds(), 0)
		return
	}

	// Update status to running
	h.store.UpdateWebhookExecution(execID, "running", "", "", 0, 0)

	// Send to LLM
	result, err := h.sendToLLM(def.Model, prompt)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		h.store.UpdateWebhookExecution(execID, "failed", "", fmt.Sprintf("llm call: %s", err), duration, 0)
		return
	}

	// Fire callback if configured
	callbackCode := 0
	if def.CallbackURL != "" {
		callbackCode, err = h.sendCallback(def.CallbackURL, name, result)
		if err != nil {
			log.Printf("WEBHOOK: callback failed for %s: %v", name, err)
			h.store.UpdateWebhookExecution(execID, "callback_failed", result, fmt.Sprintf("callback: %s", err), duration, callbackCode)
			return
		}
	}

	h.store.UpdateWebhookExecution(execID, "completed", result, "", duration, callbackCode)
	log.Printf("WEBHOOK: %s completed in %dms", name, duration)
}

// renderTemplate renders a Go text/template with the payload.
func renderTemplate(tmplStr, payload string) (string, error) {
	tmpl, err := template.New("webhook").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	data := map[string]string{"Payload": payload}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// sendToLLM sends a prompt directly to the upstream LLM provider.
func (h *Handler) sendToLLM(model, prompt string) (string, error) {
	// Build OpenAI-compatible request
	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// Send through localhost proxy so routing/budget/tracking all apply
	url := fmt.Sprintf("http://localhost:%d/v1/chat/completions", h.proxyCfg.Port)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Name", "webhook")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Extract content from OpenAI-compatible response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return result.Choices[0].Message.Content, nil
}

// sendCallback POSTs the result to the callback URL.
func (h *Handler) sendCallback(url, webhookName, result string) (int, error) {
	payload := map[string]string{
		"webhook": webhookName,
		"result":  result,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal callback: %w", err)
	}

	resp, err := h.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("post callback: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return resp.StatusCode, fmt.Errorf("callback returned %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}
