package failover

import "github.com/agent-platform/agix/internal/pricing"

// Config holds failover configuration.
type Config struct {
	MaxRetries int                 `yaml:"max_retries"`
	Chains     map[string][]string `yaml:"chains"`
}

// Failover resolves fallback models for a given model.
type Failover struct {
	maxRetries int
	chains     map[string][]string
}

// New creates a Failover from config. Returns nil if config is empty.
func New(cfg Config) *Failover {
	if len(cfg.Chains) == 0 {
		return nil
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}
	return &Failover{
		maxRetries: maxRetries,
		chains:     cfg.Chains,
	}
}

// MaxRetries returns the configured max retry count.
func (f *Failover) MaxRetries() int {
	return f.maxRetries
}

// FallbackModels returns the fallback chain for a model.
// Returns nil if no chain is configured.
func (f *Failover) FallbackModels(model string) []string {
	return f.chains[model]
}

// IsRetryable returns true if the status code is retryable (5xx).
func IsRetryable(statusCode int) bool {
	return statusCode >= 500 && statusCode < 600
}

// ResolveProvider returns the provider for a given model.
func ResolveProvider(model string) string {
	return pricing.ProviderForModel(model)
}
