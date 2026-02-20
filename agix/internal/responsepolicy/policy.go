package responsepolicy

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// Config holds response policy settings from YAML config.
type Config struct {
	Enabled        bool                   `yaml:"enabled"`
	RedactPatterns []RedactRuleConfig     `yaml:"redact_patterns"`
	MaxOutputChars int                    `yaml:"max_output_chars"`
	ForceFormat    string                 `yaml:"force_format"`
	Agents         map[string]AgentPolicy `yaml:"agents"`
}

// RedactRuleConfig defines a redaction rule in config.
type RedactRuleConfig struct {
	Name        string `yaml:"name"`
	Pattern     string `yaml:"pattern"`
	Replacement string `yaml:"replacement"`
}

// AgentPolicy defines per-agent response policy overrides.
type AgentPolicy struct {
	RedactPatterns []RedactRuleConfig `yaml:"redact_patterns"`
	MaxOutputChars int                `yaml:"max_output_chars"`
	ForceFormat    string             `yaml:"force_format"`
}

// redactRule is a compiled redaction rule.
type redactRule struct {
	name        string
	re          *regexp.Regexp
	replacement string
}

// Policy applies post-processing rules to LLM responses.
type Policy struct {
	rules          []redactRule
	maxOutputChars int
	forceFormat    string
	agents         map[string]*agentPolicy
}

type agentPolicy struct {
	rules          []redactRule
	maxOutputChars int
	forceFormat    string
}

// New creates a new Policy from config. Returns nil if disabled.
func New(cfg Config) (*Policy, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	rules, err := compileRules(cfg.RedactPatterns)
	if err != nil {
		return nil, fmt.Errorf("compile global redact patterns: %w", err)
	}

	agents := make(map[string]*agentPolicy, len(cfg.Agents))
	for name, ap := range cfg.Agents {
		agentRules, err := compileRules(ap.RedactPatterns)
		if err != nil {
			return nil, fmt.Errorf("compile redact patterns for agent %q: %w", name, err)
		}
		agents[name] = &agentPolicy{
			rules:          agentRules,
			maxOutputChars: ap.MaxOutputChars,
			forceFormat:    ap.ForceFormat,
		}
	}

	return &Policy{
		rules:          rules,
		maxOutputChars: cfg.MaxOutputChars,
		forceFormat:    cfg.ForceFormat,
		agents:         agents,
	}, nil
}

func compileRules(configs []RedactRuleConfig) ([]redactRule, error) {
	var rules []redactRule
	for _, rc := range configs {
		re, err := regexp.Compile(rc.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q in rule %q: %w", rc.Pattern, rc.Name, err)
		}
		replacement := rc.Replacement
		if replacement == "" {
			replacement = "[REDACTED]"
		}
		rules = append(rules, redactRule{
			name:        rc.Name,
			re:          re,
			replacement: replacement,
		})
	}
	return rules, nil
}

// Apply applies all response policies to the response body.
// Returns the modified body and a list of applied rule names (for headers/logging).
func (p *Policy) Apply(respBody []byte, agentName string) ([]byte, []string) {
	content := extractContent(respBody)
	if content == "" {
		return respBody, nil
	}

	var applied []string

	// Determine effective rules: global + per-agent
	effectiveRules := p.rules
	effectiveMaxChars := p.maxOutputChars
	effectiveFormat := p.forceFormat

	if agentName != "" {
		if ap, ok := p.agents[agentName]; ok {
			effectiveRules = append(effectiveRules, ap.rules...)
			if ap.maxOutputChars > 0 {
				effectiveMaxChars = ap.maxOutputChars
			}
			if ap.forceFormat != "" {
				effectiveFormat = ap.forceFormat
			}
		}
	}

	// Apply redaction rules
	for _, rule := range effectiveRules {
		if rule.re.MatchString(content) {
			content = rule.re.ReplaceAllString(content, rule.replacement)
			applied = append(applied, "redact:"+rule.name)
		}
	}

	// Apply max-length truncation
	if effectiveMaxChars > 0 && len(content) > effectiveMaxChars {
		content = content[:effectiveMaxChars] + "\n[TRUNCATED]"
		applied = append(applied, "truncate")
	}

	// Apply format validation
	if effectiveFormat == "json" {
		if !json.Valid([]byte(content)) {
			applied = append(applied, "format_warning:not_json")
			log.Printf("RESPONSE_POLICY: content is not valid JSON for agent %q", agentName)
		}
	}

	if len(applied) == 0 {
		return respBody, nil
	}

	// Replace the content field in the response body
	result := replaceContent(respBody, content)
	log.Printf("RESPONSE_POLICY: applied %s for agent %q", strings.Join(applied, ", "), agentName)
	return result, applied
}

// extractContent extracts the text content from an LLM response body.
// Supports both OpenAI and Anthropic response formats.
func extractContent(body []byte) string {
	// Try OpenAI format: choices[0].message.content
	var openai struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &openai); err == nil && len(openai.Choices) > 0 {
		return openai.Choices[0].Message.Content
	}

	// Try Anthropic format: content[0].text
	var anthropic struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &anthropic); err == nil && len(anthropic.Content) > 0 {
		for _, block := range anthropic.Content {
			if block.Type == "text" {
				return block.Text
			}
		}
	}

	return ""
}

// replaceContent replaces the text content in an LLM response body.
func replaceContent(body []byte, newContent string) []byte {
	// Try OpenAI format
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}

	if choicesRaw, ok := raw["choices"]; ok {
		var choices []map[string]json.RawMessage
		if err := json.Unmarshal(choicesRaw, &choices); err == nil && len(choices) > 0 {
			var msg map[string]json.RawMessage
			if err := json.Unmarshal(choices[0]["message"], &msg); err == nil {
				contentJSON, _ := json.Marshal(newContent)
				msg["content"] = contentJSON
				msgJSON, _ := json.Marshal(msg)
				choices[0]["message"] = msgJSON
				choicesJSON, _ := json.Marshal(choices)
				raw["choices"] = choicesJSON
				out, _ := json.Marshal(raw)
				return out
			}
		}
	}

	// Try Anthropic format
	if contentRaw, ok := raw["content"]; ok {
		var blocks []map[string]json.RawMessage
		if err := json.Unmarshal(contentRaw, &blocks); err == nil {
			for i, block := range blocks {
				var blockType string
				if err := json.Unmarshal(block["type"], &blockType); err == nil && blockType == "text" {
					contentJSON, _ := json.Marshal(newContent)
					blocks[i]["text"] = contentJSON
					blocksJSON, _ := json.Marshal(blocks)
					raw["content"] = blocksJSON
					out, _ := json.Marshal(raw)
					return out
				}
			}
		}
	}

	return body
}
