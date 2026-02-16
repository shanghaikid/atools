package compressor

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// Config defines context compressor settings.
type Config struct {
	Enabled        bool   `yaml:"enabled"`
	ThresholdTokens int   `yaml:"threshold_tokens"`
	KeepRecent     int    `yaml:"keep_recent"`
	SummaryModel   string `yaml:"summary_model"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SummarizeFunc is called to summarize old messages via an LLM.
// It takes a prompt and returns the summary text.
type SummarizeFunc func(model string, messages []Message) (string, error)

// Compressor checks conversation length and compresses if over threshold.
type Compressor struct {
	cfg         Config
	summarizeFn SummarizeFunc
}

// New creates a Compressor from config. Returns nil if not enabled.
func New(cfg Config, fn SummarizeFunc) *Compressor {
	if !cfg.Enabled {
		return nil
	}
	if cfg.ThresholdTokens <= 0 {
		cfg.ThresholdTokens = 50000
	}
	if cfg.KeepRecent <= 0 {
		cfg.KeepRecent = 10
	}
	if cfg.SummaryModel == "" {
		cfg.SummaryModel = "gpt-4o-mini"
	}
	return &Compressor{cfg: cfg, summarizeFn: fn}
}

// Compress checks if the messages exceed the token threshold.
// If so, it splits into system + old + recent, summarizes old messages,
// and returns the compressed message array as raw JSON.
// Returns the original messages unchanged if no compression is needed.
func (c *Compressor) Compress(messages json.RawMessage) json.RawMessage {
	var msgs []Message
	if err := json.Unmarshal(messages, &msgs); err != nil {
		return messages
	}

	// Estimate total tokens
	total := 0
	for _, m := range msgs {
		total += estimateTokens(m.Content)
	}

	if total < c.cfg.ThresholdTokens {
		return messages
	}

	// Split: system messages + old + recent
	var systemMsgs []Message
	var conversationMsgs []Message

	for _, m := range msgs {
		if m.Role == "system" {
			systemMsgs = append(systemMsgs, m)
		} else {
			conversationMsgs = append(conversationMsgs, m)
		}
	}

	// Keep the most recent N conversation messages
	keepRecent := c.cfg.KeepRecent
	if keepRecent >= len(conversationMsgs) {
		// Not enough messages to compress
		return messages
	}

	oldMsgs := conversationMsgs[:len(conversationMsgs)-keepRecent]
	recentMsgs := conversationMsgs[len(conversationMsgs)-keepRecent:]

	// Summarize old messages
	summary, err := c.summarize(oldMsgs)
	if err != nil {
		log.Printf("COMPRESS: summarize error: %v", err)
		return messages
	}

	// Build compressed message array
	var result []Message
	result = append(result, systemMsgs...)
	result = append(result, Message{
		Role:    "system",
		Content: fmt.Sprintf("[Conversation summary: %d earlier messages]\n%s", len(oldMsgs), summary),
	})
	result = append(result, recentMsgs...)

	compressed, err := json.Marshal(result)
	if err != nil {
		return messages
	}

	log.Printf("COMPRESS: %d tokens → ~%d tokens (%d messages summarized, %d kept)",
		total, estimateTokens(string(compressed)), len(oldMsgs), keepRecent)

	return compressed
}

func (c *Compressor) summarize(msgs []Message) (string, error) {
	if c.summarizeFn == nil {
		return c.fallbackSummarize(msgs), nil
	}

	summaryPrompt := []Message{
		{Role: "system", Content: "Summarize the following conversation concisely. Focus on key decisions, facts, and context that would be needed to continue the conversation. Be brief."},
		{Role: "user", Content: formatMessagesForSummary(msgs)},
	}

	return c.summarizeFn(c.cfg.SummaryModel, summaryPrompt)
}

// fallbackSummarize creates a simple extractive summary without an LLM.
func (c *Compressor) fallbackSummarize(msgs []Message) string {
	var parts []string
	for _, m := range msgs {
		content := m.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		parts = append(parts, fmt.Sprintf("[%s]: %s", m.Role, content))
	}
	return strings.Join(parts, "\n")
}

func formatMessagesForSummary(msgs []Message) string {
	var parts []string
	for _, m := range msgs {
		parts = append(parts, fmt.Sprintf("%s: %s", m.Role, m.Content))
	}
	return strings.Join(parts, "\n\n")
}
