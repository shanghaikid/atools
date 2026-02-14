package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/store"
	"github.com/agent-platform/agix/internal/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	budgetAgent   string
	budgetDaily   float64
	budgetMonthly float64
)

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Manage agent budgets",
	Long: `View and manage spending budgets for agents.

Examples:
  agix budget                                          # Show all budgets
  agix budget set --agent mybot --daily 5.00           # Set daily limit
  agix budget set --agent mybot --monthly 100.00       # Set monthly limit
  agix budget remove --agent mybot                     # Remove budget`,
}

var budgetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agent budgets and current spend",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		if len(cfg.Budgets) == 0 {
			fmt.Println(ui.Dimf("No budgets configured."))
			fmt.Println(ui.Dimf("Use 'agix budget set --agent <name> --daily <amount>' to set a budget."))
			return nil
		}

		fmt.Println(ui.Boldf("Agent Budgets"))
		fmt.Println()

		st, err := store.New(cfg.Database)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer st.Close()

		now := time.Now().UTC()

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Agent", "Daily Limit", "Daily Spend", "Monthly Limit", "Monthly Spend", "Status"})
		table.SetBorder(false)

		for agent, b := range cfg.Budgets {
			dailySpend, _ := st.QueryAgentDailySpend(agent, now)
			monthlySpend, _ := st.QueryAgentMonthlySpend(agent, now.Year(), now.Month())

			status := "OK"
			if b.DailyLimitUSD > 0 && dailySpend >= b.DailyLimitUSD {
				status = "DAILY LIMIT"
			} else if b.MonthlyLimitUSD > 0 && monthlySpend >= b.MonthlyLimitUSD {
				status = "MONTHLY LIMIT"
			} else if b.AlertAtPercent > 0 {
				if b.DailyLimitUSD > 0 && dailySpend/b.DailyLimitUSD*100 >= b.AlertAtPercent {
					status = "WARN"
				}
				if b.MonthlyLimitUSD > 0 && monthlySpend/b.MonthlyLimitUSD*100 >= b.AlertAtPercent {
					status = "WARN"
				}
			}

			table.Append([]string{
				ui.Cyanf("%s", agent),
				formatUSD(b.DailyLimitUSD),
				fmt.Sprintf("$%.2f", dailySpend),
				formatUSD(b.MonthlyLimitUSD),
				fmt.Sprintf("$%.2f", monthlySpend),
				ui.BudgetStatusColor(status),
			})
		}

		table.Render()
		return nil
	},
}

var budgetSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set budget for an agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if budgetAgent == "" {
			return fmt.Errorf("--agent is required")
		}

		cfg, path, err := loadConfig()
		if err != nil {
			return err
		}

		if cfg.Budgets == nil {
			cfg.Budgets = map[string]config.Budget{}
		}

		b := cfg.Budgets[budgetAgent]
		if budgetDaily > 0 {
			b.DailyLimitUSD = budgetDaily
		}
		if budgetMonthly > 0 {
			b.MonthlyLimitUSD = budgetMonthly
		}
		if b.AlertAtPercent == 0 {
			b.AlertAtPercent = 80 // Default alert threshold
		}
		cfg.Budgets[budgetAgent] = b

		if err := config.Save(path, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Budget set for agent %q:\n", budgetAgent)
		if b.DailyLimitUSD > 0 {
			fmt.Printf("  Daily limit:  $%.2f\n", b.DailyLimitUSD)
		}
		if b.MonthlyLimitUSD > 0 {
			fmt.Printf("  Monthly limit: $%.2f\n", b.MonthlyLimitUSD)
		}
		return nil
	},
}

var budgetRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove budget for an agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if budgetAgent == "" {
			return fmt.Errorf("--agent is required")
		}

		cfg, path, err := loadConfig()
		if err != nil {
			return err
		}

		if _, ok := cfg.Budgets[budgetAgent]; !ok {
			fmt.Printf("No budget configured for agent %q\n", budgetAgent)
			return nil
		}

		delete(cfg.Budgets, budgetAgent)

		if err := config.Save(path, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Budget removed for agent %q\n", budgetAgent)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(budgetCmd)
	budgetCmd.AddCommand(budgetListCmd)
	budgetCmd.AddCommand(budgetSetCmd)
	budgetCmd.AddCommand(budgetRemoveCmd)

	// Default to list when running `agix budget` without subcommand
	budgetCmd.RunE = budgetListCmd.RunE

	budgetSetCmd.Flags().StringVarP(&budgetAgent, "agent", "a", "", "agent name")
	budgetSetCmd.Flags().Float64VarP(&budgetDaily, "daily", "d", 0, "daily spending limit in USD")
	budgetSetCmd.Flags().Float64VarP(&budgetMonthly, "monthly", "m", 0, "monthly spending limit in USD")

	budgetRemoveCmd.Flags().StringVarP(&budgetAgent, "agent", "a", "", "agent name")
}

func formatUSD(v float64) string {
	if v == 0 {
		return "-"
	}
	return fmt.Sprintf("$%.2f", v)
}
