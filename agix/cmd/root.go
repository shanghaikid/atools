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
(OpenAI, Anthropic). It tracks token usage and costs, enforces per-agent
budgets, and provides shared MCP tools to all agents — no agent-side
changes required.

Usage:
  agix init          Initialize configuration
  agix start         Start the gateway
  agix stats         View usage statistics
  agix logs          View recent request logs
  agix budget        Manage agent budgets
  agix export        Export data to CSV/JSON
  agix tools list    List shared MCP tools`,
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
