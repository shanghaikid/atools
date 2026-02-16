package qualitygate

import (
	"encoding/json"
	"strings"
)

// ActionType defines what to do when a quality issue is detected.
type ActionType string

const (
	ActionRetry  ActionType = "retry"
	ActionWarn   ActionType = "warn"
	ActionReject ActionType = "reject"
)

// Config defines quality gate settings.
type Config struct {
	Enabled      bool       `yaml:"enabled"`
	MaxRetries   int        `yaml:"max_retries"`
	OnEmpty      ActionType `yaml:"on_empty"`
	OnTruncated  ActionType `yaml:"on_truncated"`
	OnRefusal    ActionType `yaml:"on_refusal"`
}

// Issue describes a detected quality problem.
type Issue struct {
	Type    string     // "empty", "truncated", "refusal"
	Action  ActionType
	Message string
}

// Gate checks non-streaming LLM responses for quality issues.
type Gate struct {
	cfg Config
}

// New creates a Gate from config. Returns nil if not enabled.
func New(cfg Config) *Gate {
	if !cfg.Enabled {
		return nil
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 2
	}
	if cfg.OnEmpty == "" {
		cfg.OnEmpty = ActionRetry
	}
	if cfg.OnTruncated == "" {
		cfg.OnTruncated = ActionWarn
	}
	if cfg.OnRefusal == "" {
		cfg.OnRefusal = ActionWarn
	}
	return &Gate{cfg: cfg}
}

// MaxRetries returns the configured max retry count.
func (g *Gate) MaxRetries() int {
	return g.cfg.MaxRetries
}

// Check inspects an OpenAI-compatible response body and returns any quality issue found.
// Returns nil if the response passes all checks.
func (g *Gate) Check(respBody []byte) *Issue {
	var resp struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil
	}

	if len(resp.Choices) == 0 {
		return &Issue{
			Type:    "empty",
			Action:  g.cfg.OnEmpty,
			Message: "response has no choices",
		}
	}

	choice := resp.Choices[0]
	content := strings.TrimSpace(choice.Message.Content)

	// Check empty
	if content == "" {
		return &Issue{
			Type:    "empty",
			Action:  g.cfg.OnEmpty,
			Message: "response content is empty",
		}
	}

	// Check truncated
	if choice.FinishReason == "length" {
		return &Issue{
			Type:    "truncated",
			Action:  g.cfg.OnTruncated,
			Message: "response truncated (finish_reason: length)",
		}
	}

	// Check refusal
	if isRefusal(content) {
		return &Issue{
			Type:    "refusal",
			Action:  g.cfg.OnRefusal,
			Message: "response appears to be a refusal",
		}
	}

	return nil
}

// isRefusal detects common LLM refusal patterns.
func isRefusal(content string) bool {
	lower := strings.ToLower(content)
	refusalPhrases := []string{
		"i cannot",
		"i can't",
		"i'm unable to",
		"i am unable to",
		"i'm not able to",
		"i am not able to",
		"as an ai",
		"as a language model",
	}
	for _, phrase := range refusalPhrases {
		if strings.HasPrefix(lower, phrase) {
			return true
		}
	}
	return false
}
