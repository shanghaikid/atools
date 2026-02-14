package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/agent-platform/agix/internal/store"
	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportOutput string
	exportPeriod string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export usage data to CSV or JSON",
	Long: `Export recorded API usage data for analysis or reporting.

Examples:
  agix export                          # CSV to stdout
  agix export --format json            # JSON to stdout
  agix export -o costs.csv             # CSV to file
  agix export --period 30d -o report.json --format json`,
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

		since, until := parsePeriod(exportPeriod)
		records, err := st.ExportCSV(since, until)
		if err != nil {
			return fmt.Errorf("export data: %w", err)
		}

		if len(records) == 0 {
			fmt.Fprintln(os.Stderr, "No records found for this period.")
			return nil
		}

		// Determine output destination
		var out *os.File
		if exportOutput != "" {
			f, err := os.Create(exportOutput)
			if err != nil {
				return fmt.Errorf("create output file: %w", err)
			}
			defer f.Close()
			out = f
		} else {
			out = os.Stdout
		}

		switch exportFormat {
		case "csv":
			return exportCSV(out, records)
		case "json":
			return exportJSON(out, records)
		default:
			return fmt.Errorf("unsupported format: %s (use csv or json)", exportFormat)
		}
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "csv", "output format: csv, json")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "output file (default: stdout)")
	exportCmd.Flags().StringVarP(&exportPeriod, "period", "P", "all", "time period: today, 7d, 30d, all")
}

func exportCSV(out *os.File, records []store.Record) error {
	w := csv.NewWriter(out)
	defer w.Flush()

	// Header
	if err := w.Write([]string{
		"id", "timestamp", "agent_name", "model", "provider",
		"input_tokens", "output_tokens", "cost_usd", "duration_ms", "status_code",
	}); err != nil {
		return err
	}

	for _, r := range records {
		if err := w.Write([]string{
			fmt.Sprintf("%d", r.ID),
			r.Timestamp.Format("2006-01-02T15:04:05Z"),
			r.AgentName,
			r.Model,
			r.Provider,
			fmt.Sprintf("%d", r.InputTokens),
			fmt.Sprintf("%d", r.OutputTokens),
			fmt.Sprintf("%.6f", r.CostUSD),
			fmt.Sprintf("%d", r.DurationMS),
			fmt.Sprintf("%d", r.StatusCode),
		}); err != nil {
			return err
		}
	}

	return nil
}

func exportJSON(out *os.File, records []store.Record) error {
	type jsonRecord struct {
		ID           int64   `json:"id"`
		Timestamp    string  `json:"timestamp"`
		AgentName    string  `json:"agent_name"`
		Model        string  `json:"model"`
		Provider     string  `json:"provider"`
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
		CostUSD      float64 `json:"cost_usd"`
		DurationMS   int64   `json:"duration_ms"`
		StatusCode   int     `json:"status_code"`
	}

	output := make([]jsonRecord, len(records))
	for i, r := range records {
		output[i] = jsonRecord{
			ID:           r.ID,
			Timestamp:    r.Timestamp.Format("2006-01-02T15:04:05Z"),
			AgentName:    r.AgentName,
			Model:        r.Model,
			Provider:     r.Provider,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			CostUSD:      r.CostUSD,
			DurationMS:   r.DurationMS,
			StatusCode:   r.StatusCode,
		}
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}
