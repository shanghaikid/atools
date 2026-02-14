package pricing

import "strings"

// ModelPricing holds per-token pricing for a model.
type ModelPricing struct {
	Provider    string
	InputPer1M  float64 // USD per 1M input tokens
	OutputPer1M float64 // USD per 1M output tokens
}

// Known model pricing table (USD per 1M tokens).
var models = map[string]ModelPricing{
	// OpenAI — GPT-5 family
	"gpt-5.2":      {Provider: "openai", InputPer1M: 1.75, OutputPer1M: 14.00},
	"gpt-5.1":      {Provider: "openai", InputPer1M: 1.25, OutputPer1M: 10.00},
	"gpt-5":        {Provider: "openai", InputPer1M: 1.25, OutputPer1M: 10.00},
	"gpt-5-mini":   {Provider: "openai", InputPer1M: 0.25, OutputPer1M: 2.00},
	"gpt-5-nano":   {Provider: "openai", InputPer1M: 0.05, OutputPer1M: 0.40},
	// OpenAI — GPT-4 family
	"gpt-4.1":      {Provider: "openai", InputPer1M: 2.00, OutputPer1M: 8.00},
	"gpt-4.1-mini": {Provider: "openai", InputPer1M: 0.40, OutputPer1M: 1.60},
	"gpt-4.1-nano": {Provider: "openai", InputPer1M: 0.10, OutputPer1M: 0.40},
	"gpt-4o":       {Provider: "openai", InputPer1M: 2.50, OutputPer1M: 10.00},
	"gpt-4o-mini":  {Provider: "openai", InputPer1M: 0.15, OutputPer1M: 0.60},
	// OpenAI — reasoning models
	"o1":       {Provider: "openai", InputPer1M: 15.00, OutputPer1M: 60.00},
	"o3":       {Provider: "openai", InputPer1M: 2.00, OutputPer1M: 8.00},
	"o3-mini":  {Provider: "openai", InputPer1M: 1.10, OutputPer1M: 4.40},
	"o4-mini":  {Provider: "openai", InputPer1M: 1.10, OutputPer1M: 4.40},

	// Anthropic — current models
	"claude-opus-4-6":            {Provider: "anthropic", InputPer1M: 5.00, OutputPer1M: 25.00},
	"claude-opus-4-5-20251101":   {Provider: "anthropic", InputPer1M: 5.00, OutputPer1M: 25.00},
	"claude-opus-4-1-20250805":   {Provider: "anthropic", InputPer1M: 15.00, OutputPer1M: 75.00},
	"claude-opus-4-20250514":     {Provider: "anthropic", InputPer1M: 15.00, OutputPer1M: 75.00},
	"claude-sonnet-4-5-20250929": {Provider: "anthropic", InputPer1M: 3.00, OutputPer1M: 15.00},
	"claude-sonnet-4-20250514":   {Provider: "anthropic", InputPer1M: 3.00, OutputPer1M: 15.00},
	"claude-haiku-4-5-20251001":  {Provider: "anthropic", InputPer1M: 1.00, OutputPer1M: 5.00},
	// Anthropic — legacy models
	"claude-3-5-haiku-20241022":  {Provider: "anthropic", InputPer1M: 0.80, OutputPer1M: 4.00},
	"claude-3-haiku-20240307":    {Provider: "anthropic", InputPer1M: 0.25, OutputPer1M: 1.25},

	// DeepSeek
	"deepseek-chat":     {Provider: "deepseek", InputPer1M: 0.27, OutputPer1M: 1.10},
	"deepseek-reasoner": {Provider: "deepseek", InputPer1M: 0.55, OutputPer1M: 2.19},
}

// Lookup returns the pricing for a model. Returns nil if unknown.
func Lookup(model string) *ModelPricing {
	model = strings.ToLower(model)
	if p, ok := models[model]; ok {
		return &p
	}
	// Try prefix matching for versioned models (e.g. gpt-4o-2024-08-06).
	// Use longest prefix match to avoid "gpt-4" matching before "gpt-4o".
	var bestName string
	var bestPricing ModelPricing
	for name, p := range models {
		if strings.HasPrefix(model, name) && len(name) > len(bestName) {
			bestName = name
			bestPricing = p
		}
	}
	if bestName != "" {
		return &bestPricing
	}
	return nil
}

// CalculateCost returns the cost in USD for a given number of tokens.
func CalculateCost(model string, inputTokens, outputTokens int) float64 {
	p := Lookup(model)
	if p == nil {
		return 0
	}
	inputCost := float64(inputTokens) / 1_000_000 * p.InputPer1M
	outputCost := float64(outputTokens) / 1_000_000 * p.OutputPer1M
	return inputCost + outputCost
}

// ProviderForModel returns the provider name for a model based on prefix.
func ProviderForModel(model string) string {
	model = strings.ToLower(model)
	switch {
	case strings.HasPrefix(model, "gpt-"), strings.HasPrefix(model, "o1"), strings.HasPrefix(model, "o3"), strings.HasPrefix(model, "o4"):
		return "openai"
	case strings.HasPrefix(model, "claude-"):
		return "anthropic"
	case strings.HasPrefix(model, "deepseek-"):
		return "deepseek"
	default:
		// Try lookup table
		if p := Lookup(model); p != nil {
			return p.Provider
		}
		return "unknown"
	}
}

// ListModels returns all known model names.
func ListModels() []string {
	result := make([]string, 0, len(models))
	for name := range models {
		result = append(result, name)
	}
	return result
}
