package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/failover"
	"github.com/agent-platform/agix/internal/pricing"
	"github.com/agent-platform/agix/internal/ratelimit"
	"github.com/agent-platform/agix/internal/store"
	"github.com/agent-platform/agix/internal/toolmgr"
)

// Proxy is an HTTP reverse proxy that tracks API usage and costs.
type Proxy struct {
	cfg         *config.Config
	store       *store.Store
	toolMgr     *toolmgr.Manager
	rateLimiter *ratelimit.Limiter
	failover    *failover.Failover
	client      *http.Client
	mux         *http.ServeMux
}

// Option configures a Proxy.
type Option func(*Proxy)

// WithToolManager sets the MCP tool manager.
func WithToolManager(m *toolmgr.Manager) Option {
	return func(p *Proxy) { p.toolMgr = m }
}

// WithRateLimiter sets the per-agent rate limiter.
func WithRateLimiter(l *ratelimit.Limiter) Option {
	return func(p *Proxy) { p.rateLimiter = l }
}

// WithFailover sets the multi-provider failover handler.
func WithFailover(f *failover.Failover) Option {
	return func(p *Proxy) { p.failover = f }
}

// New creates a new Proxy with the given options.
func New(cfg *config.Config, st *store.Store, opts ...Option) *Proxy {
	p := &Proxy{
		cfg:   cfg,
		store: st,
		client: &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		},
		mux: http.NewServeMux(),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.mux.HandleFunc("/v1/chat/completions", p.handleChatCompletions)
	p.mux.HandleFunc("/v1/models", p.handleModels)
	p.mux.HandleFunc("/health", p.handleHealth)
	return p
}

// ServeHTTP implements http.Handler.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mux.ServeHTTP(w, r)
}

func (p *Proxy) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok"}`)
}

func (p *Proxy) handleModels(w http.ResponseWriter, r *http.Request) {
	models := pricing.ListModels()
	type modelEntry struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	}
	type response struct {
		Object string       `json:"object"`
		Data   []modelEntry `json:"data"`
	}
	resp := response{Object: "list"}
	for _, m := range models {
		resp.Data = append(resp.Data, modelEntry{
			ID:      m,
			Object:  "model",
			OwnedBy: pricing.ProviderForModel(m),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// chatRequest is the OpenAI-compatible request body.
type chatRequest struct {
	Model    string          `json:"model"`
	Messages json.RawMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	// Pass through all other fields
}

func (p *Proxy) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":"invalid JSON in request body"}`, http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		http.Error(w, `{"error":"model field is required"}`, http.StatusBadRequest)
		return
	}

	// Determine provider and upstream URL
	provider := pricing.ProviderForModel(req.Model)
	agentName := r.Header.Get("X-Agent-Name")

	// Check rate limit before budget
	if p.rateLimiter != nil && agentName != "" {
		result := p.rateLimiter.Allow(agentName)
		if !result.Allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(result.RetryAfter.Seconds())))
			http.Error(w, fmt.Sprintf(`{"error":"rate limited: %s"}`, result.Err.Error()), http.StatusTooManyRequests)
			return
		}
	}

	// Check budget before proxying
	if agentName != "" {
		if err := p.checkBudget(agentName); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"budget exceeded: %s"}`, err.Error()), http.StatusTooManyRequests)
			return
		}
	}

	// Check if we have tools for this agent
	var agentTools []toolmgr.ToolEntry
	if p.toolMgr != nil {
		agentTools = p.toolMgr.ToolsForAgent(agentName)
	}

	if len(agentTools) > 0 {
		// Tool-enhanced path: inject tools, force non-streaming, run tool loop
		p.handleToolEnhancedRequest(w, r, body, req.Model, provider, agentName, agentTools)
		return
	}

	start := time.Now()
	resp, actualModel, actualProvider, failoverFrom, err := p.doUpstreamRequest(r, body, req.Model, provider)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"upstream request failed: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	duration := time.Since(start)

	if req.Stream {
		p.handleStreamingResponse(w, resp, actualModel, actualProvider, agentName, start, duration, failoverFrom)
	} else {
		p.handleNonStreamingResponse(w, resp, actualModel, actualProvider, agentName, start, duration, failoverFrom)
	}
}

// doUpstreamRequest sends the request to the upstream provider, with failover on 5xx.
// Returns the response, actual model/provider used, and failover_from (empty if no failover).
func (p *Proxy) doUpstreamRequest(r *http.Request, body []byte, model, provider string) (*http.Response, string, string, string, error) {
	resp, err := p.sendToProvider(r, body, model, provider)
	if err != nil {
		return nil, model, provider, "", err
	}

	// Check if we should failover
	if p.failover == nil || !failover.IsRetryable(resp.StatusCode) {
		return resp, model, provider, "", nil
	}

	chain := p.failover.FallbackModels(model)
	if len(chain) == 0 {
		return resp, model, provider, "", nil
	}

	originalModel := model
	maxRetries := p.failover.MaxRetries()
	if maxRetries > len(chain) {
		maxRetries = len(chain)
	}

	for i := 0; i < maxRetries; i++ {
		resp.Body.Close()
		fallbackModel := chain[i]
		fallbackProvider := failover.ResolveProvider(fallbackModel)

		// Re-encode body with new model
		fallbackBody := replaceModel(body, fallbackModel)

		log.Printf("FAILOVER: %s (%s) → %s (%s) [attempt %d/%d]",
			model, provider, fallbackModel, fallbackProvider, i+1, maxRetries)

		resp, err = p.sendToProvider(r, fallbackBody, fallbackModel, fallbackProvider)
		if err != nil {
			continue
		}

		if !failover.IsRetryable(resp.StatusCode) {
			return resp, fallbackModel, fallbackProvider, originalModel, nil
		}
		model = fallbackModel
		provider = fallbackProvider
	}

	// All retries exhausted, return last response
	return resp, model, provider, originalModel, err
}

func (p *Proxy) sendToProvider(r *http.Request, body []byte, model, provider string) (*http.Response, error) {
	upstreamURL, upstreamHeaders, upstreamBody, err := p.buildUpstreamRequest(provider, model, body)
	if err != nil {
		return nil, err
	}

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}
	for k, v := range upstreamHeaders {
		upstreamReq.Header.Set(k, v)
	}

	return p.client.Do(upstreamReq)
}

// replaceModel replaces the model field in the request body.
func replaceModel(body []byte, newModel string) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	modelJSON, _ := json.Marshal(newModel)
	raw["model"] = modelJSON
	out, err := json.Marshal(raw)
	if err != nil {
		return body
	}
	return out
}

func (p *Proxy) buildUpstreamRequest(provider, model string, originalBody []byte) (string, map[string]string, []byte, error) {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	switch provider {
	case "openai":
		apiKey, ok := p.cfg.Keys["openai"]
		if !ok || apiKey == "" {
			return "", nil, nil, fmt.Errorf("OpenAI API key not configured")
		}
		headers["Authorization"] = "Bearer " + apiKey
		return "https://api.openai.com/v1/chat/completions", headers, originalBody, nil

	case "anthropic":
		apiKey, ok := p.cfg.Keys["anthropic"]
		if !ok || apiKey == "" {
			return "", nil, nil, fmt.Errorf("Anthropic API key not configured")
		}
		// Convert OpenAI format to Anthropic format
		anthBody, err := convertToAnthropicFormat(originalBody)
		if err != nil {
			return "", nil, nil, fmt.Errorf("convert to Anthropic format: %w", err)
		}
		headers["x-api-key"] = apiKey
		headers["anthropic-version"] = "2023-06-01"
		return "https://api.anthropic.com/v1/messages", headers, anthBody, nil

	case "deepseek":
		apiKey, ok := p.cfg.Keys["deepseek"]
		if !ok || apiKey == "" {
			return "", nil, nil, fmt.Errorf("DeepSeek API key not configured")
		}
		headers["Authorization"] = "Bearer " + apiKey
		return "https://api.deepseek.com/chat/completions", headers, originalBody, nil

	default:
		return "", nil, nil, fmt.Errorf("unsupported provider for model %q", model)
	}
}

// convertToAnthropicFormat converts an OpenAI-format request to Anthropic format.
func convertToAnthropicFormat(body []byte) ([]byte, error) {
	var openaiReq struct {
		Model       string `json:"model"`
		Messages    []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Stream      bool    `json:"stream"`
		MaxTokens   int     `json:"max_tokens,omitempty"`
		Temperature float64 `json:"temperature,omitempty"`
	}

	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return nil, err
	}

	// Separate system message from user/assistant messages
	var system string
	var messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	for _, msg := range openaiReq.Messages {
		if msg.Role == "system" {
			system = msg.Content
		} else {
			messages = append(messages, msg)
		}
	}

	maxTokens := openaiReq.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	anthReq := map[string]any{
		"model":      openaiReq.Model,
		"messages":   messages,
		"max_tokens": maxTokens,
	}
	if system != "" {
		anthReq["system"] = system
	}
	if openaiReq.Stream {
		anthReq["stream"] = true
	}
	if openaiReq.Temperature > 0 {
		anthReq["temperature"] = openaiReq.Temperature
	}

	return json.Marshal(anthReq)
}

func (p *Proxy) handleNonStreamingResponse(w http.ResponseWriter, resp *http.Response, model, provider, agentName string, start time.Time, duration time.Duration, failoverFrom ...string) {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, `{"error":"failed to read upstream response"}`, http.StatusBadGateway)
		return
	}

	// Extract usage from response
	inputTokens, outputTokens := extractUsage(provider, respBody)
	cost := pricing.CalculateCost(model, inputTokens, outputTokens)

	// Record to store
	var foFrom string
	if len(failoverFrom) > 0 {
		foFrom = failoverFrom[0]
	}
	record := &store.Record{
		Timestamp:    start,
		AgentName:    agentName,
		Model:        model,
		Provider:     provider,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      cost,
		DurationMS:   duration.Milliseconds(),
		StatusCode:   resp.StatusCode,
		FailoverFrom: foFrom,
	}
	p.store.InsertAsync(record)

	// Forward response to client
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	// Add cost tracking headers
	w.Header().Set("X-Cost-USD", fmt.Sprintf("%.6f", cost))
	w.Header().Set("X-Input-Tokens", fmt.Sprintf("%d", inputTokens))
	w.Header().Set("X-Output-Tokens", fmt.Sprintf("%d", outputTokens))
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func (p *Proxy) handleStreamingResponse(w http.ResponseWriter, resp *http.Response, model, provider, agentName string, start time.Time, duration time.Duration, failoverFrom ...string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	// Forward headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	var totalInput, totalOutput int
	scanner := bufio.NewScanner(resp.Body)
	// Increase buffer for large SSE events
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Forward line to client
		fmt.Fprintf(w, "%s\n", line)
		flusher.Flush()

		// Parse SSE data lines for usage
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				continue
			}
			input, output := extractStreamUsage(provider, []byte(data))
			if input > 0 {
				totalInput = input
			}
			if output > 0 {
				totalOutput = output
			}
		}
	}

	elapsed := time.Since(start)
	cost := pricing.CalculateCost(model, totalInput, totalOutput)

	// Record to store
	var foFrom string
	if len(failoverFrom) > 0 {
		foFrom = failoverFrom[0]
	}
	record := &store.Record{
		Timestamp:    start,
		AgentName:    agentName,
		Model:        model,
		Provider:     provider,
		InputTokens:  totalInput,
		OutputTokens: totalOutput,
		CostUSD:      cost,
		DurationMS:   elapsed.Milliseconds(),
		StatusCode:   resp.StatusCode,
		FailoverFrom: foFrom,
	}
	p.store.InsertAsync(record)
}

// extractUsage extracts token usage from a non-streaming response.
func extractUsage(provider string, body []byte) (inputTokens, outputTokens int) {
	switch provider {
	case "openai", "deepseek":
		var resp struct {
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(body, &resp); err == nil {
			return resp.Usage.PromptTokens, resp.Usage.CompletionTokens
		}
	case "anthropic":
		var resp struct {
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(body, &resp); err == nil {
			return resp.Usage.InputTokens, resp.Usage.OutputTokens
		}
	}
	return 0, 0
}

// extractStreamUsage extracts token usage from a single SSE data chunk.
func extractStreamUsage(provider string, data []byte) (inputTokens, outputTokens int) {
	switch provider {
	case "openai", "deepseek":
		var chunk struct {
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(data, &chunk); err == nil && chunk.Usage != nil {
			return chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens
		}
	case "anthropic":
		var chunk struct {
			Type  string `json:"type"`
			Usage *struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			Message *struct {
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(data, &chunk); err == nil {
			if chunk.Usage != nil {
				return chunk.Usage.InputTokens, chunk.Usage.OutputTokens
			}
			if chunk.Message != nil {
				return chunk.Message.Usage.InputTokens, chunk.Message.Usage.OutputTokens
			}
		}
	}
	return 0, 0
}

// handleToolEnhancedRequest runs the tool execution loop: inject tools → send to LLM → execute tool calls → repeat.
func (p *Proxy) handleToolEnhancedRequest(w http.ResponseWriter, r *http.Request, body []byte, model, provider, agentName string, tools []toolmgr.ToolEntry) {
	start := time.Now()

	// Force stream=false for tool-enhanced requests (agent is unaware of tools)
	body = forceNonStreaming(body)

	// Inject tool definitions into the request body
	body = injectTools(body, tools, provider)

	maxIter := p.cfg.Tools.MaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}

	var totalInput, totalOutput int

	for i := 0; i < maxIter; i++ {
		// Build upstream request
		upstreamURL, upstreamHeaders, upstreamBody, err := p.buildUpstreamRequestRaw(provider, model, body)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
			return
		}

		upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(upstreamBody))
		if err != nil {
			http.Error(w, `{"error":"failed to create upstream request"}`, http.StatusInternalServerError)
			return
		}
		for k, v := range upstreamHeaders {
			upstreamReq.Header.Set(k, v)
		}

		resp, err := p.client.Do(upstreamReq)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"upstream request failed: %s"}`, err.Error()), http.StatusBadGateway)
			return
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			http.Error(w, `{"error":"failed to read upstream response"}`, http.StatusBadGateway)
			return
		}

		// Accumulate tokens
		input, output := extractUsage(provider, respBody)
		totalInput += input
		totalOutput += output

		// Check if there are tool calls
		toolCalls := extractToolCalls(provider, respBody)
		if len(toolCalls) == 0 {
			// No tool calls — return final response to the agent
			// Strip tool-related fields from the response so agent is unaware
			finalBody := stripToolCalls(provider, respBody)
			cost := pricing.CalculateCost(model, totalInput, totalOutput)
			duration := time.Since(start)

			record := &store.Record{
				Timestamp:    start,
				AgentName:    agentName,
				Model:        model,
				Provider:     provider,
				InputTokens:  totalInput,
				OutputTokens: totalOutput,
				CostUSD:      cost,
				DurationMS:   duration.Milliseconds(),
				StatusCode:   resp.StatusCode,
			}
			p.store.InsertAsync(record)

			for k, vv := range resp.Header {
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}
			w.Header().Set("X-Cost-USD", fmt.Sprintf("%.6f", cost))
			w.Header().Set("X-Input-Tokens", fmt.Sprintf("%d", totalInput))
			w.Header().Set("X-Output-Tokens", fmt.Sprintf("%d", totalOutput))
			w.WriteHeader(resp.StatusCode)
			w.Write(finalBody)
			return
		}

		// Execute tool calls via MCP
		results := p.executeMCPTools(toolCalls)

		// Append assistant message + tool results to the conversation
		body = appendToolResults(body, provider, respBody, toolCalls, results)
	}

	// Exceeded max iterations
	http.Error(w, fmt.Sprintf(`{"error":"tool execution exceeded max iterations (%d)"}`, maxIter), http.StatusInternalServerError)
}

// toolCall represents a tool call extracted from an LLM response.
type toolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// forceNonStreaming sets stream=false in the request body.
func forceNonStreaming(body []byte) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	raw["stream"] = json.RawMessage(`false`)
	out, err := json.Marshal(raw)
	if err != nil {
		return body
	}
	return out
}

// injectTools adds tool definitions to the request body.
func injectTools(body []byte, tools []toolmgr.ToolEntry, provider string) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}

	if provider == "anthropic" {
		// Anthropic format: tools array with name, description, input_schema
		var anthTools []map[string]any
		for _, t := range tools {
			tool := map[string]any{
				"name": t.Name,
			}
			if t.Description != "" {
				tool["description"] = t.Description
			}
			if t.InputSchema != nil {
				tool["input_schema"] = t.InputSchema
			} else {
				tool["input_schema"] = map[string]any{"type": "object"}
			}
			anthTools = append(anthTools, tool)
		}
		data, _ := json.Marshal(anthTools)
		raw["tools"] = data
	} else {
		// OpenAI format: tools array with type=function and function object
		var oaiTools []map[string]any
		for _, t := range tools {
			fn := map[string]any{
				"name": t.Name,
			}
			if t.Description != "" {
				fn["description"] = t.Description
			}
			if t.InputSchema != nil {
				fn["parameters"] = t.InputSchema
			}
			oaiTools = append(oaiTools, map[string]any{
				"type":     "function",
				"function": fn,
			})
		}
		data, _ := json.Marshal(oaiTools)
		raw["tools"] = data
	}

	out, err := json.Marshal(raw)
	if err != nil {
		return body
	}
	return out
}

// extractToolCalls extracts tool calls from an LLM response.
func extractToolCalls(provider string, respBody []byte) []toolCall {
	switch provider {
	case "openai", "deepseek":
		return extractOpenAIToolCalls(respBody)
	case "anthropic":
		return extractAnthropicToolCalls(respBody)
	}
	return nil
}

func extractOpenAIToolCalls(body []byte) []toolCall {
	var resp struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"` // JSON string
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &resp); err != nil || len(resp.Choices) == 0 {
		return nil
	}

	choice := resp.Choices[0]
	if choice.FinishReason != "tool_calls" {
		return nil
	}

	var calls []toolCall
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		json.Unmarshal([]byte(tc.Function.Arguments), &args)
		calls = append(calls, toolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}
	return calls
}

func extractAnthropicToolCalls(body []byte) []toolCall {
	var resp struct {
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type  string         `json:"type"`
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	if resp.StopReason != "tool_use" {
		return nil
	}

	var calls []toolCall
	for _, block := range resp.Content {
		if block.Type == "tool_use" {
			calls = append(calls, toolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}
	return calls
}

// executeMCPTools executes tool calls via the tool manager concurrently.
// Different MCP servers are called in parallel; same-server calls are naturally
// serialized by the per-client mutex in the MCP client.
func (p *Proxy) executeMCPTools(calls []toolCall) []string {
	results := make([]string, len(calls))
	var wg sync.WaitGroup
	wg.Add(len(calls))
	for i, tc := range calls {
		go func(i int, tc toolCall) {
			defer wg.Done()
			text, err := p.toolMgr.CallTool(tc.Name, tc.Arguments)
			if err != nil {
				results[i] = fmt.Sprintf("Error executing tool %s: %s", tc.Name, err.Error())
			} else {
				results[i] = text
			}
		}(i, tc)
	}
	wg.Wait()
	return results
}

// appendToolResults appends the assistant response and tool results to the conversation.
func appendToolResults(body []byte, provider string, respBody []byte, calls []toolCall, results []string) []byte {
	switch provider {
	case "openai", "deepseek":
		return appendOpenAIToolResults(body, respBody, calls, results)
	case "anthropic":
		return appendAnthropicToolResults(body, respBody, calls, results)
	}
	return body
}

func appendOpenAIToolResults(body []byte, respBody []byte, calls []toolCall, results []string) []byte {
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}

	// Parse existing messages
	var messages []json.RawMessage
	if err := json.Unmarshal(req["messages"], &messages); err != nil {
		return body
	}

	// Extract the assistant message from the response and append it
	var resp struct {
		Choices []struct {
			Message json.RawMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil || len(resp.Choices) == 0 {
		return body
	}
	messages = append(messages, resp.Choices[0].Message)

	// Append tool result messages
	for i, tc := range calls {
		toolMsg := map[string]any{
			"role":         "tool",
			"tool_call_id": tc.ID,
			"content":      results[i],
		}
		data, _ := json.Marshal(toolMsg)
		messages = append(messages, data)
	}

	msgData, _ := json.Marshal(messages)
	req["messages"] = msgData

	out, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return out
}

func appendAnthropicToolResults(body []byte, respBody []byte, calls []toolCall, results []string) []byte {
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}

	var messages []json.RawMessage
	if err := json.Unmarshal(req["messages"], &messages); err != nil {
		return body
	}

	// Extract the assistant content from the response
	var resp struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return body
	}

	// Append assistant message
	assistantMsg := map[string]any{
		"role":    "assistant",
		"content": resp.Content,
	}
	data, _ := json.Marshal(assistantMsg)
	messages = append(messages, data)

	// Append user message with tool_result blocks
	var toolResults []map[string]any
	for i, tc := range calls {
		toolResults = append(toolResults, map[string]any{
			"type":       "tool_result",
			"tool_use_id": tc.ID,
			"content":    results[i],
		})
	}
	userMsg := map[string]any{
		"role":    "user",
		"content": toolResults,
	}
	data, _ = json.Marshal(userMsg)
	messages = append(messages, data)

	msgData, _ := json.Marshal(messages)
	req["messages"] = msgData

	out, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return out
}

// stripToolCalls removes tool-related fields from the final response so the agent is unaware.
func stripToolCalls(provider string, respBody []byte) []byte {
	switch provider {
	case "openai", "deepseek":
		return stripOpenAIToolCalls(respBody)
	case "anthropic":
		return stripAnthropicToolCalls(respBody)
	}
	return respBody
}

func stripOpenAIToolCalls(body []byte) []byte {
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(body, &resp); err != nil {
		return body
	}

	var choices []map[string]json.RawMessage
	if err := json.Unmarshal(resp["choices"], &choices); err != nil || len(choices) == 0 {
		return body
	}

	// Update finish_reason to "stop"
	choices[0]["finish_reason"] = json.RawMessage(`"stop"`)

	// Remove tool_calls from the message
	var message map[string]json.RawMessage
	if err := json.Unmarshal(choices[0]["message"], &message); err == nil {
		delete(message, "tool_calls")
		msgData, _ := json.Marshal(message)
		choices[0]["message"] = msgData
	}

	choicesData, _ := json.Marshal(choices)
	resp["choices"] = choicesData

	out, _ := json.Marshal(resp)
	return out
}

func stripAnthropicToolCalls(body []byte) []byte {
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(body, &resp); err != nil {
		return body
	}

	// Update stop_reason to "end_turn"
	resp["stop_reason"] = json.RawMessage(`"end_turn"`)

	// Filter out tool_use blocks from content
	var content []map[string]json.RawMessage
	if err := json.Unmarshal(resp["content"], &content); err == nil {
		var filtered []map[string]json.RawMessage
		for _, block := range content {
			var blockType string
			json.Unmarshal(block["type"], &blockType)
			if blockType != "tool_use" {
				filtered = append(filtered, block)
			}
		}
		if filtered == nil {
			filtered = []map[string]json.RawMessage{}
		}
		contentData, _ := json.Marshal(filtered)
		resp["content"] = contentData
	}

	out, _ := json.Marshal(resp)
	return out
}

// buildUpstreamRequestRaw builds the upstream request without format conversion.
// Used for tool loop iterations where the body is already in the correct provider format.
func (p *Proxy) buildUpstreamRequestRaw(provider, model string, body []byte) (string, map[string]string, []byte, error) {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	switch provider {
	case "openai":
		apiKey, ok := p.cfg.Keys["openai"]
		if !ok || apiKey == "" {
			return "", nil, nil, fmt.Errorf("OpenAI API key not configured")
		}
		headers["Authorization"] = "Bearer " + apiKey
		return "https://api.openai.com/v1/chat/completions", headers, body, nil

	case "anthropic":
		apiKey, ok := p.cfg.Keys["anthropic"]
		if !ok || apiKey == "" {
			return "", nil, nil, fmt.Errorf("Anthropic API key not configured")
		}
		headers["x-api-key"] = apiKey
		headers["anthropic-version"] = "2023-06-01"
		return "https://api.anthropic.com/v1/messages", headers, body, nil

	case "deepseek":
		apiKey, ok := p.cfg.Keys["deepseek"]
		if !ok || apiKey == "" {
			return "", nil, nil, fmt.Errorf("DeepSeek API key not configured")
		}
		headers["Authorization"] = "Bearer " + apiKey
		return "https://api.deepseek.com/chat/completions", headers, body, nil

	default:
		return "", nil, nil, fmt.Errorf("unsupported provider for model %q", model)
	}
}

func (p *Proxy) checkBudget(agentName string) error {
	budget, ok := p.cfg.Budgets[agentName]
	if !ok {
		return nil // No budget configured
	}

	now := time.Now().UTC()

	if budget.DailyLimitUSD > 0 {
		dailySpend, err := p.store.QueryAgentDailySpend(agentName, now)
		if err != nil {
			log.Printf("WARN: failed to check daily budget: %v", err)
			return nil // Allow on error
		}
		if dailySpend >= budget.DailyLimitUSD {
			return fmt.Errorf("daily limit of $%.2f reached (spent $%.2f)", budget.DailyLimitUSD, dailySpend)
		}
	}

	if budget.MonthlyLimitUSD > 0 {
		monthlySpend, err := p.store.QueryAgentMonthlySpend(agentName, now.Year(), now.Month())
		if err != nil {
			log.Printf("WARN: failed to check monthly budget: %v", err)
			return nil
		}
		if monthlySpend >= budget.MonthlyLimitUSD {
			return fmt.Errorf("monthly limit of $%.2f reached (spent $%.2f)", budget.MonthlyLimitUSD, monthlySpend)
		}
	}

	return nil
}
