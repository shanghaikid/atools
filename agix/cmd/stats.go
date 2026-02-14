package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/agent-platform/agix/internal/store"
	"github.com/agent-platform/agix/internal/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	statsPeriod  string
	statsGroupBy string
	statsFormat  string
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "View usage statistics",
	Long: `Display aggregated usage statistics including total requests, tokens, and costs.

Examples:
  agix stats                    # Today's stats
  agix stats --period 7d        # Last 7 days
  agix stats --period 30d       # Last 30 days
  agix stats --group-by agent   # Group by agent
  agix stats --group-by model   # Group by model
  agix stats --group-by day     # Group by day`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		st, err := store.New(cfg.Database)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer st.Close()

		since, until := parsePeriod(statsPeriod)

		switch statsGroupBy {
		case "agent":
			return showAgentStats(st, since, until)
		case "model":
			return showModelStats(st, since, until)
		case "day":
			return showDailyStats(st, since, until)
		default:
			return showOverallStats(st, since, until)
		}
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().StringVarP(&statsPeriod, "period", "P", "today", "time period: today, 7d, 30d, all")
	statsCmd.Flags().StringVarP(&statsGroupBy, "group-by", "g", "", "group by: agent, model, day")
	statsCmd.Flags().StringVarP(&statsFormat, "format", "f", "table", "output format: table, json")
}

func parsePeriod(period string) (time.Time, time.Time) {
	now := time.Now().UTC()
	until := now

	switch period {
	case "today":
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), until
	case "7d", "week":
		return now.AddDate(0, 0, -7), until
	case "30d", "month":
		return now.AddDate(0, 0, -30), until
	case "all":
		return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), until
	default:
		// Try to parse as YYYY-MM
		if t, err := time.Parse("2006-01", period); err == nil {
			end := t.AddDate(0, 1, 0).Add(-time.Second)
			return t, end
		}
		// Default to today
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), until
	}
}

func periodLabel(period string) string {
	switch period {
	case "today":
		return "Today"
	case "7d", "week":
		return "Last 7 days"
	case "30d", "month":
		return "Last 30 days"
	case "all":
		return "All time"
	default:
		return period
	}
}

func showOverallStats(st *store.Store, since, until time.Time) error {
	stats, err := st.QueryStats(since, until)
	if err != nil {
		return err
	}

	if stats.TotalRequests == 0 {
		fmt.Println(ui.Dimf("No requests recorded for this period."))
		return nil
	}

	fmt.Println(ui.Boldf("Usage Summary") + ui.Dimf(" (%s)", periodLabel(statsPeriod)))
	fmt.Println()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Metric", "Value"})
	table.SetBorder(false)
	table.SetColumnSeparator("  ")
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)

	table.Append([]string{"Total Requests", fmt.Sprintf("%d", stats.TotalRequests)})
	table.Append([]string{"Input Tokens", formatTokens(stats.TotalInput)})
	table.Append([]string{"Output Tokens", formatTokens(stats.TotalOutput)})
	table.Append([]string{"Total Cost", ui.CostColor(stats.TotalCostUSD)})
	table.Append([]string{"Avg Latency", fmt.Sprintf("%.0fms", stats.AvgDurationMS)})
	table.Append([]string{"Unique Models", fmt.Sprintf("%d", stats.UniqueModels)})
	table.Append([]string{"Unique Agents", fmt.Sprintf("%d", stats.UniqueAgents)})

	table.Render()
	return nil
}

func showAgentStats(st *store.Store, since, until time.Time) error {
	agents, err := st.QueryStatsByAgent(since, until)
	if err != nil {
		return err
	}

	if len(agents) == 0 {
		fmt.Println(ui.Dimf("No requests recorded for this period."))
		return nil
	}

	fmt.Println(ui.Boldf("Cost by Agent") + ui.Dimf(" (%s)", periodLabel(statsPeriod)))
	fmt.Println()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Agent", "Requests", "Input Tokens", "Output Tokens", "Cost"})
	table.SetBorder(false)
	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
	})

	var totalCost float64
	for _, a := range agents {
		totalCost += a.CostUSD
		table.Append([]string{
			ui.Cyanf("%s", a.AgentName),
			fmt.Sprintf("%d", a.Requests),
			formatTokens(a.InputTokens),
			formatTokens(a.OutputTokens),
			ui.CostColor(a.CostUSD),
		})
	}

	table.SetFooter([]string{"", "", "", "Total", ui.CostColor(totalCost)})
	table.Render()
	return nil
}

func showModelStats(st *store.Store, since, until time.Time) error {
	models, err := st.QueryStatsByModel(since, until)
	if err != nil {
		return err
	}

	if len(models) == 0 {
		fmt.Println(ui.Dimf("No requests recorded for this period."))
		return nil
	}

	fmt.Println(ui.Boldf("Cost by Model") + ui.Dimf(" (%s)", periodLabel(statsPeriod)))
	fmt.Println()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Model", "Provider", "Requests", "Input Tokens", "Output Tokens", "Cost"})
	table.SetBorder(false)
	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
	})

	var totalCost float64
	for _, m := range models {
		totalCost += m.CostUSD
		table.Append([]string{
			m.Model,
			ui.Dimf("%s", m.Provider),
			fmt.Sprintf("%d", m.Requests),
			formatTokens(m.InputTokens),
			formatTokens(m.OutputTokens),
			ui.CostColor(m.CostUSD),
		})
	}

	table.SetFooter([]string{"", "", "", "", "Total", ui.CostColor(totalCost)})
	table.Render()
	return nil
}

func showDailyStats(st *store.Store, since, until time.Time) error {
	daily, err := st.QueryDailyCosts(since, until)
	if err != nil {
		return err
	}

	if len(daily) == 0 {
		fmt.Println(ui.Dimf("No requests recorded for this period."))
		return nil
	}

	fmt.Println(ui.Boldf("Daily Costs") + ui.Dimf(" (%s)", periodLabel(statsPeriod)))
	fmt.Println()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Date", "Requests", "Cost"})
	table.SetBorder(false)
	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
	})

	var totalCost float64
	for _, d := range daily {
		totalCost += d.CostUSD
		table.Append([]string{
			d.Date,
			fmt.Sprintf("%d", d.Requests),
			ui.CostColor(d.CostUSD),
		})
	}

	table.SetFooter([]string{"", "Total", ui.CostColor(totalCost)})
	table.Render()
	return nil
}

func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
