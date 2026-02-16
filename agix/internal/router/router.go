package router

import (
	"encoding/json"
	"strings"
)

// TierConfig defines criteria for a routing tier.
type TierConfig struct {
	MaxMessageTokens int      `yaml:"max_message_tokens"`
	MaxMessages      int      `yaml:"max_messages"`
	KeywordsAbsent   []string `yaml:"keywords_absent"`
}

// Config holds smart routing configuration.
type Config struct {
	Enabled  bool                          `yaml:"enabled"`
	Tiers    map[string]TierConfig         `yaml:"tiers"`
	ModelMap map[string]map[string]string   `yaml:"model_map"`
}

// Router selects cheaper models for simple requests.
type Router struct {
	tiers    map[string]TierConfig
	modelMap map[string]map[string]string
}

// New creates a Router from config. Returns nil if not enabled or empty.
func New(cfg Config) *Router {
	if !cfg.Enabled || len(cfg.Tiers) == 0 || len(cfg.ModelMap) == 0 {
		return nil
	}
	return &Router{
		tiers:    cfg.Tiers,
		modelMap: cfg.ModelMap,
	}
}

// Route returns the routed model for the given request.
// Returns the original model if no routing applies.
// Also returns the tier name matched (empty if none).
func (r *Router) Route(model string, messages json.RawMessage) (routedModel, tier string) {
	mapping, ok := r.modelMap[model]
	if !ok {
		return model, ""
	}

	classified := r.classify(messages)
	if classified == "" {
		return model, ""
	}

	if target, ok := mapping[classified]; ok {
		return target, classified
	}
	return model, ""
}

// classify determines the tier for a set of messages.
func (r *Router) classify(messages json.RawMessage) string {
	var msgs []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(messages, &msgs); err != nil {
		return ""
	}

	for tierName, tier := range r.tiers {
		if r.matchesTier(tier, msgs) {
			return tierName
		}
	}
	return ""
}

func (r *Router) matchesTier(tier TierConfig, msgs []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}) bool {
	// Count non-system messages
	userMsgs := 0
	for _, m := range msgs {
		if m.Role != "system" {
			userMsgs++
		}
	}

	// Check message count
	if tier.MaxMessages > 0 && userMsgs > tier.MaxMessages {
		return false
	}

	// Check token estimate for all messages
	totalTokens := 0
	allContent := ""
	for _, m := range msgs {
		totalTokens += estimateTokens(m.Content)
		allContent += " " + strings.ToLower(m.Content)
	}

	if tier.MaxMessageTokens > 0 && totalTokens > tier.MaxMessageTokens {
		return false
	}

	// Check keyword absence
	for _, kw := range tier.KeywordsAbsent {
		if strings.Contains(allContent, strings.ToLower(kw)) {
			return false
		}
	}

	return true
}

// estimateTokens estimates the token count for a string using word count * 1.3.
func estimateTokens(s string) int {
	words := len(strings.Fields(s))
	return int(float64(words) * 1.3)
}
