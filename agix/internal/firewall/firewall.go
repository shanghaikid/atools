package firewall

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// Action defines what happens when a rule matches.
type Action string

const (
	ActionBlock Action = "block"
	ActionWarn  Action = "warn"
	ActionLog   Action = "log"
)

// RuleConfig defines a firewall rule from configuration.
type RuleConfig struct {
	Name     string `yaml:"name"`
	Category string `yaml:"category"`
	Pattern  string `yaml:"pattern"`
	Action   Action `yaml:"action"`
}

// Rule is a compiled firewall rule.
type Rule struct {
	Name     string
	Category string
	Pattern  *regexp.Regexp
	Action   Action
}

// Result is returned when scanning messages.
type Result struct {
	Blocked  bool
	Warnings []string
	Message  string // block reason
}

// Firewall scans messages against compiled rules.
type Firewall struct {
	rules []Rule
}

// Config holds firewall configuration.
type Config struct {
	Enabled bool         `yaml:"enabled"`
	Rules   []RuleConfig `yaml:"rules"`
}

// DefaultRules returns built-in rules for common patterns.
func DefaultRules() []RuleConfig {
	return []RuleConfig{
		{Name: "injection_ignore", Category: "injection", Pattern: `(?i)ignore\s+(all\s+)?(?:previous|prior|above)\s+instructions`, Action: ActionBlock},
		{Name: "injection_pretend", Category: "injection", Pattern: `(?i)pretend\s+you\s+are`, Action: ActionWarn},
		{Name: "pii_ssn", Category: "pii", Pattern: `\b\d{3}-\d{2}-\d{4}\b`, Action: ActionWarn},
		{Name: "pii_credit_card", Category: "pii", Pattern: `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`, Action: ActionWarn},
	}
}

// New creates a Firewall from config. Returns nil if not enabled.
func New(cfg Config) (*Firewall, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Merge default rules with user rules (user rules take precedence by name)
	ruleMap := make(map[string]RuleConfig)
	for _, r := range DefaultRules() {
		ruleMap[r.Name] = r
	}
	for _, r := range cfg.Rules {
		ruleMap[r.Name] = r
	}

	var rules []Rule
	for _, rc := range ruleMap {
		compiled, err := regexp.Compile(rc.Pattern)
		if err != nil {
			return nil, fmt.Errorf("compile rule %q pattern: %w", rc.Name, err)
		}
		rules = append(rules, Rule{
			Name:     rc.Name,
			Category: rc.Category,
			Pattern:  compiled,
			Action:   rc.Action,
		})
	}

	return &Firewall{rules: rules}, nil
}

// Scan checks all user messages against firewall rules.
func (f *Firewall) Scan(messages json.RawMessage) Result {
	var msgs []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(messages, &msgs); err != nil {
		return Result{}
	}

	// Only scan user messages
	var userContent []string
	for _, m := range msgs {
		if m.Role == "user" {
			userContent = append(userContent, m.Content)
		}
	}
	text := strings.Join(userContent, "\n")

	var result Result
	for _, rule := range f.rules {
		if rule.Pattern.MatchString(text) {
			switch rule.Action {
			case ActionBlock:
				result.Blocked = true
				result.Message = fmt.Sprintf("blocked by firewall rule %q (%s)", rule.Name, rule.Category)
				log.Printf("FIREWALL: BLOCK - rule %q matched (%s)", rule.Name, rule.Category)
				return result
			case ActionWarn:
				warning := fmt.Sprintf("firewall rule %q matched (%s)", rule.Name, rule.Category)
				result.Warnings = append(result.Warnings, warning)
				log.Printf("FIREWALL: WARN - rule %q matched (%s)", rule.Name, rule.Category)
			case ActionLog:
				log.Printf("FIREWALL: LOG - rule %q matched (%s)", rule.Name, rule.Category)
			}
		}
	}

	return result
}
