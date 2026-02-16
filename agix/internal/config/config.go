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

		default:
			result = append(result, line)
		}
	}
	return []byte(strings.Join(result, "\n"))
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
