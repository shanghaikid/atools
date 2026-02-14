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
	logsLimit int
	logsAgent string
	logsTail  bool
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View recent request logs",
	Long: `Display recent API requests with model, tokens, cost, and latency.

Examples:
  agix logs                  # Last 20 requests
  agix logs -n 50            # Last 50 requests
  agix logs --agent mybot    # Filter by agent
  agix logs --tail           # Watch in real-time`,
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

		if logsTail {
			return tailLogs(st)
		}

		return showLogs(st)
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().IntVarP(&logsLimit, "limit", "n", 20, "number of records to show")
	logsCmd.Flags().StringVarP(&logsAgent, "agent", "a", "", "filter by agent name")
	logsCmd.Flags().BoolVarP(&logsTail, "tail", "t", false, "watch for new requests in real-time")
}

func showLogs(st *store.Store) error {
	records, err := st.QueryRecentRequests(logsLimit, logsAgent)
	if err != nil {
		return fmt.Errorf("query logs: %w", err)
	}

	if len(records) == 0 {
		fmt.Println(ui.Dimf("No requests recorded."))
		return nil
	}

	fmt.Println(ui.Boldf("Recent Requests"))
	fmt.Println()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Time", "Agent", "Model", "Input", "Output", "Cost", "Latency", "Status"})
	table.SetBorder(false)
	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_CENTER,
	})

	for _, r := range records {
		table.Append([]string{
			ui.Dimf("%s", r.Timestamp.Format("01-02 15:04:05")),
			ui.Cyanf("%s", truncate(r.AgentName, 15)),
			truncate(r.Model, 25),
			formatTokens(r.InputTokens),
			formatTokens(r.OutputTokens),
			ui.CostColor(r.CostUSD),
			fmt.Sprintf("%dms", r.DurationMS),
			ui.StatusColor(r.StatusCode),
		})
	}

	table.Render()
	fmt.Printf("\n%s\n", ui.Dimf("Showing %d most recent requests", len(records)))
	return nil
}

func tailLogs(st *store.Store) error {
	fmt.Println(ui.Boldf("Watching for requests...") + ui.Dimf(" (Ctrl+C to stop)"))
	fmt.Println()

	// Print header
	fmt.Printf("%-19s  %-15s  %-25s  %8s  %8s  %10s  %8s  %s\n",
		ui.Dimf("TIME"), ui.Dimf("AGENT"), ui.Dimf("MODEL"),
		ui.Dimf("INPUT"), ui.Dimf("OUTPUT"), ui.Dimf("COST"),
		ui.Dimf("LATENCY"), ui.Dimf("STATUS"))
	fmt.Println(ui.Dimf("---"))

	var lastID int64

	// Get current max ID
	records, err := st.QueryRecentRequests(1, "")
	if err == nil && len(records) > 0 {
		lastID = records[0].ID
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		records, err := st.QueryRecentRequests(50, logsAgent)
		if err != nil {
			continue
		}

		// Print new records (they come in reverse order)
		var newRecords []store.Record
		for _, r := range records {
			if r.ID > lastID {
				newRecords = append(newRecords, r)
			}
		}

		// Print in chronological order
		for i := len(newRecords) - 1; i >= 0; i-- {
			r := newRecords[i]
			fmt.Printf("%-19s  %-15s  %-25s  %8s  %8s  %10s  %8s  %s\n",
				ui.Dimf("%s", r.Timestamp.Format("01-02 15:04:05")),
				ui.Cyanf("%s", truncate(r.AgentName, 15)),
				truncate(r.Model, 25),
				formatTokens(r.InputTokens),
				formatTokens(r.OutputTokens),
				ui.CostColor(r.CostUSD),
				fmt.Sprintf("%dms", r.DurationMS),
				ui.StatusColor(r.StatusCode))

			if r.ID > lastID {
				lastID = r.ID
			}
		}
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
