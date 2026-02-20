package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	Port       int                        `yaml:"port"`
	Keys       map[string]string          `yaml:"keys"`
	Database   string                     `yaml:"database"`
	LogLevel   string                     `yaml:"log_level"`
	Budgets    map[string]Budget          `yaml:"budgets"`
	Tools      ToolsConfig                `yaml:"tools"`
	RateLimits map[string]RateLimitConfig `yaml:"rate_limits"`
	Failover   FailoverConfig             `yaml:"failover"`
	Routing    RoutingConfig              `yaml:"routing"`
	Dashboard  DashboardConfig            `yaml:"dashboard"`
	Firewall   FirewallConfig             `yaml:"firewall"`
	QualityGate QualityGateConfig         `yaml:"quality_gate"`
	Cache       CacheConfig               `yaml:"cache"`
	Compression CompressionConfig         `yaml:"compression"`
	Experiments     []ExperimentConfig        `yaml:"experiments"`
	PromptTemplates PromptTemplateConfig      `yaml:"prompt_templates"`
	Tracing         TracingConfig             `yaml:"tracing"`
	Audit            AuditConfig               `yaml:"audit"`
	SessionOverrides SessionOverrideConfig     `yaml:"session_overrides"`
	Webhooks         WebhookConfig             `yaml:"webhooks"`
	Bundles          []string                  `yaml:"bundles"`
}

// WebhookConfig defines generic webhook endpoint settings.
type WebhookConfig struct {
	Enabled     bool                          `yaml:"enabled"`
	Definitions map[string]WebhookDefinition  `yaml:"definitions"`
}

// WebhookDefinition defines a single webhook endpoint.
type WebhookDefinition struct {
	Secret         string `yaml:"secret"`
	Model          string `yaml:"model"`
	PromptTemplate string `yaml:"prompt_template"`
	CallbackURL    string `yaml:"callback_url"`
}

// SessionOverrideConfig defines session-level config override settings.
type SessionOverrideConfig struct {
	Enabled    bool   `yaml:"enabled"`
	DefaultTTL string `yaml:"default_ttl"` // e.g. "1h", "24h"
}

// AuditConfig defines audit logging settings.
type AuditConfig struct {
	Enabled        bool     `yaml:"enabled"`
	ContentLog     bool     `yaml:"content_log"`
	DangerousTools []string `yaml:"dangerous_tools"`
}

// TracingConfig defines request tracing settings.
type TracingConfig struct {
	Enabled    bool    `yaml:"enabled"`
	SampleRate float64 `yaml:"sample_rate"`
}

// PromptTemplateConfig defines prompt template injection settings.
type PromptTemplateConfig struct {
	Enabled  bool              `yaml:"enabled"`
	Global   string            `yaml:"global"`
	Agents   map[string]string `yaml:"agents"`
	Position string            `yaml:"position"` // "prepend" or "append", default "prepend"
}

// FirewallConfig defines the prompt firewall settings.
type FirewallConfig struct {
	Enabled bool               `yaml:"enabled"`
	Rules   []FirewallRule     `yaml:"rules"`
}

// FirewallRule defines a firewall rule in config.
type FirewallRule struct {
	Name     string `yaml:"name"`
	Category string `yaml:"category"`
	Pattern  string `yaml:"pattern"`
	Action   string `yaml:"action"`
}

// ExperimentConfig defines an A/B test experiment.
type ExperimentConfig struct {
	Name         string `yaml:"name"`
	Enabled      bool   `yaml:"enabled"`
	ControlModel string `yaml:"control_model"`
	VariantModel string `yaml:"variant_model"`
	TrafficPct   int    `yaml:"traffic_pct"`
}

// CompressionConfig defines context compressor settings.
type CompressionConfig struct {
	Enabled         bool   `yaml:"enabled"`
	ThresholdTokens int    `yaml:"threshold_tokens"`
	KeepRecent      int    `yaml:"keep_recent"`
	SummaryModel    string `yaml:"summary_model"`
}

// CacheConfig defines semantic cache settings.
type CacheConfig struct {
	Enabled             bool    `yaml:"enabled"`
	SimilarityThreshold float64 `yaml:"similarity_threshold"`
	TTLMinutes          int     `yaml:"ttl_minutes"`
}

// QualityGateConfig defines quality gate settings.
type QualityGateConfig struct {
	Enabled     bool   `yaml:"enabled"`
	MaxRetries  int    `yaml:"max_retries"`
	OnEmpty     string `yaml:"on_empty"`
	OnTruncated string `yaml:"on_truncated"`
	OnRefusal   string `yaml:"on_refusal"`
}

// DashboardConfig defines the web dashboard settings.
type DashboardConfig struct {
	Enabled bool `yaml:"enabled"`
}

// RoutingConfig defines smart routing.
type RoutingConfig struct {
	Enabled  bool                          `yaml:"enabled"`
	Tiers    map[string]RoutingTier        `yaml:"tiers"`
	ModelMap map[string]map[string]string   `yaml:"model_map"`
}

// RoutingTier defines criteria for classifying a request.
type RoutingTier struct {
	MaxMessageTokens int      `yaml:"max_message_tokens"`
	MaxMessages      int      `yaml:"max_messages"`
	KeywordsAbsent   []string `yaml:"keywords_absent"`
}

// FailoverConfig defines multi-provider failover.
type FailoverConfig struct {
	MaxRetries int                 `yaml:"max_retries"`
	Chains     map[string][]string `yaml:"chains"`
}

// RateLimitConfig defines per-agent rate limits.
type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
	RequestsPerHour   int `yaml:"requests_per_hour"`
}

// Budget represents a spending budget for an agent.
type Budget struct {
	DailyLimitUSD   float64 `yaml:"daily_limit_usd"`
	MonthlyLimitUSD float64 `yaml:"monthly_limit_usd"`
	AlertAtPercent  float64 `yaml:"alert_at_percent"`
	AlertWebhook    string  `yaml:"alert_webhook"`
}

// ToolsConfig holds shared MCP tool configuration.
type ToolsConfig struct {
	MaxIterations int                    `yaml:"max_iterations"`
	Servers       map[string]MCPServer   `yaml:"servers"`
	Agents        map[string]AgentTools  `yaml:"agents"`
}

// MCPServer defines an MCP server to spawn.
type MCPServer struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Env     []string `yaml:"env"`
}

// AgentTools defines per-agent tool access control.
type AgentTools struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

// DefaultConfigDir returns the default configuration directory (~/.agix).
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".agix"), nil
}

// DefaultConfigPath returns the path to the default config file.
func DefaultConfigPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// DefaultDBPath returns the path to the default database file.
func DefaultDBPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agix.db"), nil
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	dbPath, _ := DefaultDBPath()
	return Config{
		Port:     8080,
		Keys:     map[string]string{},
		Database: dbPath,
		LogLevel: "info",
		Budgets:  map[string]Budget{},
		Tools: ToolsConfig{
			MaxIterations: 10,
		},
	}
}

// Load reads a config file from disk.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return &cfg, nil
}

// Save writes the config to disk, creating directories as needed.
func Save(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// SaveWithComments writes the config to disk with helpful comments for empty sections.
// Used by `init` to generate a self-documenting config file.
func SaveWithComments(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	data = addConfigComments(data)

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// addConfigComments inserts # comments into marshaled YAML for user guidance.
// Only annotates empty default sections; user-populated sections are left untouched.
func addConfigComments(data []byte) []byte {
	var result []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		indent := line[:len(line)-len(trimmed)]

		switch {
		case trimmed == "keys: {}":
			result = append(result,
				indent+"# LLM provider API keys (the proxy injects these — agents never see them):",
				indent+"#   keys:",
				indent+"#     openai: \"sk-...\"",
				indent+"#     anthropic: \"sk-ant-...\"",
				indent+"#     deepseek: \"sk-...\"",
				line,
			)

		case trimmed == "budgets: {}":
			result = append(result,
				indent+"# Per-agent spending limits (agents exceeding limits get 429 responses):",
				indent+"#   budgets:",
				indent+"#     my-agent:",
				indent+"#       daily_limit_usd: 10.0",
				indent+"#       monthly_limit_usd: 200.0",
				indent+"#       alert_at_percent: 80",
				line,
			)

		case strings.HasPrefix(trimmed, "max_iterations:") && !strings.Contains(line, "#"):
			result = append(result, line+" # max tool execution rounds per request")

		case trimmed == "servers: {}":
			result = append(result,
				indent+"# MCP servers to spawn (each provides tools to agents via stdio JSON-RPC):",
				indent+"#   servers:",
				indent+"#     filesystem:",
				indent+"#       command: npx",
				indent+"#       args: [\"-y\", \"@modelcontextprotocol/server-filesystem\", \"/tmp\"]",
				indent+"#     github:",
				indent+"#       command: npx",
				indent+"#       args: [\"-y\", \"@modelcontextprotocol/server-github\"]",
				indent+"#       env: [\"GITHUB_TOKEN=ghp_xxx\"]",
				line,
			)

		case trimmed == "agents: {}":
			result = append(result,
				indent+"# Per-agent tool access control (agents not listed get all tools):",
				indent+"#   agents:",
				indent+"#     my-agent:",
				indent+"#       allow: [\"read_file\", \"list_directory\"]  # whitelist",
				indent+"#     another-agent:",
				indent+"#       deny: [\"write_file\"]                     # blacklist",
				line,
			)

		case trimmed == "rate_limits: {}":
			result = append(result,
				indent+"# Per-agent request throttling (returns 429 + Retry-After when exceeded):",
				indent+"#   rate_limits:",
				indent+"#     my-agent:",
				indent+"#       requests_per_minute: 10",
				indent+"#       requests_per_hour: 100",
				line,
			)

		case trimmed == "chains: {}":
			result = append(result,
				indent+"# Model fallback chains (auto-retry on 5xx with next model in chain):",
				indent+"#   chains:",
				indent+"#     gpt-4o: [claude-sonnet-4-20250514, deepseek-chat]",
				indent+"#     claude-opus-4-6: [gpt-5]",
				line,
			)

		case trimmed == "tiers: {}":
			result = append(result,
				indent+"# Request complexity tiers (simple requests → cheaper models):",
				indent+"#   tiers:",
				indent+"#     simple:",
				indent+"#       max_message_tokens: 500   # total user message tokens",
				indent+"#       max_messages: 3            # max conversation messages",
				indent+"#       keywords_absent: [analyze, refactor, explain]",
				line,
			)

		case trimmed == "model_map: {}":
			result = append(result,
				indent+"# Model mapping per tier (which model to use for each tier):",
				indent+"#   model_map:",
				indent+"#     gpt-4o:           { simple: gpt-4o-mini }",
				indent+"#     claude-opus-4-6: { simple: claude-haiku-4-5-20251001 }",
				line,
			)

		case trimmed == "enabled: false" && isDashboardSection(result):
			result = append(result,
				indent+"# Web dashboard at http://localhost:<port>/dashboard",
				line,
			)

		case trimmed == "rules: []":
			result = append(result,
				indent+"# Regex rules to scan user messages (block → 403, warn → header, log → stdout):",
				indent+"#   rules:",
				indent+"#     - { name: block_jailbreak, category: injection,",
				indent+"#         pattern: \"(?i)ignore.*instructions\", action: block }",
				indent+"#     - { name: ssn, category: pii,",
				indent+"#         pattern: \"\\\\b\\\\d{3}-\\\\d{2}-\\\\d{4}\\\\b\", action: warn }",
				indent+"# Built-in rules are always active: injection_ignore (block),",
				indent+"# injection_pretend (warn), pii_ssn (warn), pii_credit_card (warn).",
				line,
			)

		case trimmed == "on_empty: \"\"":
			result = append(result,
				indent+"# Actions: retry (re-send request), warn (add X-Quality-Warning header),",
				indent+"#          reject (return 422). Defaults: on_empty=retry, on_truncated=warn,",
				indent+"#          on_refusal=warn. max_retries defaults to 2. Non-streaming only.",
				line,
			)

		case trimmed == "similarity_threshold: 0":
			result = append(result,
				indent+"# Cosine similarity threshold for semantic match (0-1, default 0.95).",
				indent+"# Requires an OpenAI API key for embeddings (text-embedding-3-small).",
				indent+"# Exact SHA-256 match is always tried first (no API call needed).",
				line,
			)

		case trimmed == "ttl_minutes: 0":
			result = append(result,
				indent+"# Cache entry TTL in minutes (default 60). Non-streaming only.",
				line,
			)

		case trimmed == "threshold_tokens: 0":
			result = append(result,
				indent+"# Token threshold before compressing (default 50000, estimated as words × 1.3).",
				indent+"# When exceeded, older messages are summarized and replaced.",
				line,
			)

		case trimmed == "keep_recent: 0":
			result = append(result,
				indent+"# Number of recent messages to keep uncompressed (default 10).",
				line,
			)

		case trimmed == "summary_model: \"\"":
			result = append(result,
				indent+"# Model used for summarization (default gpt-4o-mini). Leave empty for",
				indent+"# extractive fallback (no LLM call, just truncates old messages).",
				line,
			)

		case trimmed == "position: \"\"" && isPromptTemplatesSection(result):
			result = append(result,
				indent+"# Position: \"prepend\" (before existing system prompt) or \"append\" (after).",
				indent+"# Global template is injected for all agents. Per-agent templates add to it.",
				indent+"#   prompt_templates:",
				indent+"#     enabled: true",
				indent+"#     global: \"Always respond in valid JSON.\"",
				indent+"#     agents:",
				indent+"#       code-reviewer: \"You are a senior code reviewer. Be concise.\"",
				indent+"#       docs-writer: \"You are a technical writer. Use simple language.\"",
				indent+"#     position: prepend",
				line,
			)

		case trimmed == "experiments: []":
			result = append(result,
				indent+"# A/B test model variants. Traffic is split using consistent hashing",
				indent+"# (same agent always gets same variant). Use 'agix experiment list' to view.",
				indent+"#   experiments:",
				indent+"#     - name: sonnet-vs-haiku",
				indent+"#       enabled: true",
				indent+"#       control_model: claude-sonnet-4-20250514",
				indent+"#       variant_model: claude-haiku-4-5-20251001",
				indent+"#       traffic_pct: 20  # 20% of agents get the variant",
				line,
			)

		default:
			result = append(result, line)
		}
	}
	return []byte(strings.Join(result, "\n"))
}

// isPromptTemplatesSection checks if the previous non-empty line in result is "prompt_templates:" or a child of it.
func isPromptTemplatesSection(result []string) bool {
	for i := len(result) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(result[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return trimmed == "prompt_templates:" || trimmed == "enabled: false" || trimmed == "global: \"\"" || strings.HasPrefix(trimmed, "agents:")
	}
	return false
}

// isDashboardSection checks if the previous non-empty line in result is "dashboard:".
func isDashboardSection(result []string) bool {
	for i := len(result) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(result[i])
		if trimmed == "" {
			continue
		}
		return trimmed == "dashboard:"
	}
	return false
}

// LoadOrCreate loads the config from the default path, or creates it with defaults.
func LoadOrCreate() (*Config, string, error) {
	path, err := DefaultConfigPath()
	if err != nil {
		return nil, "", err
	}

	cfg, err := Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			def := DefaultConfig()
			if saveErr := Save(path, &def); saveErr != nil {
				return nil, "", fmt.Errorf("create default config: %w", saveErr)
			}
			return &def, path, nil
		}
		return nil, "", err
	}

	return cfg, path, nil
}
