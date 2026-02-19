package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "agix",
	Short: "AI agent gateway — cost, tools & more",
	Long: `agix is a local gateway between your AI agents and LLM providers
(OpenAI, Anthropic, DeepSeek). It tracks token usage and costs, enforces
per-agent budgets, and provides shared MCP tools — no agent-side changes
required.

Commands:
  agix init              Initialize configuration
  agix start             Start the gateway
  agix doctor            Check configuration and dependencies
  agix stats             View usage statistics
  agix logs              View recent request logs
  agix budget            Manage agent budgets
  agix export            Export data to CSV/JSON
  agix tools list        List shared MCP tools
  agix experiment list   List A/B test experiments
  agix experiment check  Check variant assignment for an agent
  agix trace list        List recent request traces
  agix trace <id>        Show detailed trace timeline

Features (configured in ~/.agix/config.yaml):
  rate_limits:    Per-agent request throttling (RPM/RPH)
  failover:       Automatic model failover on 5xx errors
  routing:        Smart model routing based on request complexity
  budgets:        Per-agent daily/monthly spend limits with alerts
  dashboard:      Web UI at http://localhost:<port>/dashboard
  firewall:       Regex-based prompt scanning (block/warn/log)
  quality_gate:   Post-response checks (empty/truncated/refusal)
  cache:          Semantic response caching with embeddings
  compression:    Auto-summarize long conversations
  experiments:    A/B test model variants with consistent hashing
  tracing:        Per-request pipeline tracing with timing`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.agix/config.yaml)")
}
