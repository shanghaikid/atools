package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/agent-platform/agix/internal/store"
	"github.com/agent-platform/agix/internal/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var traceListLimit int
var traceListAgent string

var traceCmd = &cobra.Command{
	Use:   "trace [trace-id]",
	Short: "View request traces",
	Long: `View detailed per-request pipeline traces.

Examples:
  agix trace list              List recent traces
  agix trace list -n 10        Last 10 traces
  agix trace list -a my-agent  Filter by agent
  agix trace <trace-id>        Show detailed timeline for a trace`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return showTrace(args[0])
	},
}

var traceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent traces",
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

		traces, err := st.QueryRecentTraces(traceListLimit, traceListAgent)
		if err != nil {
			return fmt.Errorf("query traces: %w", err)
		}

		if len(traces) == 0 {
			fmt.Println("No traces found.")
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Trace ID", "Agent", "Model", "Spans", "Timestamp"})
		table.SetBorder(false)
		table.SetColumnSeparator(" ")

		for _, tr := range traces {
			var spans []json.RawMessage
			json.Unmarshal(tr.Spans, &spans)
			table.Append([]string{
				tr.TraceID,
				tr.AgentName,
				tr.Model,
				fmt.Sprintf("%d", len(spans)),
				tr.Timestamp.Format("2006-01-02 15:04:05"),
			})
		}

		table.Render()
		return nil
	},
}

func showTrace(traceID string) error {
	cfg, _, err := loadConfig()
	if err != nil {
		return err
	}

	st, err := store.New(cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer st.Close()

	tr, err := st.QueryTrace(traceID)
	if err != nil {
		return fmt.Errorf("query trace: %w", err)
	}
	if tr == nil {
		return fmt.Errorf("trace %q not found", traceID)
	}

	fmt.Printf("\n%s %s\n", ui.Boldf("Trace:"), tr.TraceID)
	fmt.Printf("%s %s\n", ui.Dimf("Agent:"), tr.AgentName)
	fmt.Printf("%s %s\n", ui.Dimf("Model:"), tr.Model)
	fmt.Printf("%s %s\n", ui.Dimf("Time: "), tr.Timestamp.Format(time.RFC3339))
	fmt.Println()

	var spans []struct {
		Name       string         `json:"name"`
		DurationMS int64          `json:"duration_ms"`
		Metadata   map[string]any `json:"metadata,omitempty"`
	}
	if err := json.Unmarshal(tr.Spans, &spans); err != nil {
		return fmt.Errorf("parse spans: %w", err)
	}

	if len(spans) == 0 {
		fmt.Println("  (no spans recorded)")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"#", "Span", "Duration", "Details"})
	table.SetBorder(false)
	table.SetColumnSeparator(" ")

	for i, s := range spans {
		details := ""
		if len(s.Metadata) > 0 {
			b, _ := json.Marshal(s.Metadata)
			details = string(b)
		}
		table.Append([]string{
			fmt.Sprintf("%d", i+1),
			s.Name,
			fmt.Sprintf("%dms", s.DurationMS),
			details,
		})
	}

	table.Render()
	fmt.Println()
	return nil
}

func init() {
	rootCmd.AddCommand(traceCmd)
	traceCmd.AddCommand(traceListCmd)
	traceListCmd.Flags().IntVarP(&traceListLimit, "number", "n", 20, "number of traces to show")
	traceListCmd.Flags().StringVarP(&traceListAgent, "agent", "a", "", "filter by agent name")
}
