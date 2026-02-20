package cmd

import (
	"fmt"
	"os"

	"github.com/agent-platform/agix/internal/store"
	"github.com/agent-platform/agix/internal/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Manage generic webhooks",
}

var webhookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured webhooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		if !cfg.Webhooks.Enabled || len(cfg.Webhooks.Definitions) == 0 {
			fmt.Println(ui.Dimf("No webhooks configured."))
			fmt.Println(ui.Dimf("Add webhooks under the 'webhooks.definitions' section in config.yaml"))
			return nil
		}

		fmt.Printf("\n%s %d webhook(s)\n\n", ui.Boldf("Configured"), len(cfg.Webhooks.Definitions))

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Name", "Model", "Callback", "Secret"})
		table.SetBorder(false)
		table.SetColumnSeparator(" ")
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)

		for name, def := range cfg.Webhooks.Definitions {
			callback := def.CallbackURL
			if callback == "" {
				callback = ui.Dimf("(none)")
			}
			secret := ui.Dimf("(none)")
			if def.Secret != "" {
				secret = def.Secret[:4] + "..."
			}
			table.Append([]string{name, def.Model, callback, secret})
		}
		table.Render()
		fmt.Println()

		return nil
	},
}

var webhookHistoryName string
var webhookHistoryLimit int

var webhookHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show webhook execution history",
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

		execs, err := st.QueryWebhookExecutions(webhookHistoryLimit, webhookHistoryName)
		if err != nil {
			return fmt.Errorf("query webhook executions: %w", err)
		}

		if len(execs) == 0 {
			fmt.Println(ui.Dimf("No webhook executions found."))
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Time", "Webhook", "Status", "Duration", "Callback"})
		table.SetBorder(false)
		table.SetColumnSeparator(" ")
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)

		for _, e := range execs {
			status := e.Status
			switch status {
			case "completed":
				status = ui.Greenf("%s", status)
			case "failed", "callback_failed":
				status = ui.Redf("%s", status)
			case "running":
				status = ui.Yellowf("%s", status)
			}

			duration := fmt.Sprintf("%dms", e.DurationMS)
			callback := "-"
			if e.CallbackCode > 0 {
				callback = fmt.Sprintf("%d", e.CallbackCode)
			}

			table.Append([]string{
				fmt.Sprintf("%d", e.ID),
				e.Timestamp.Format("2006-01-02 15:04:05"),
				e.WebhookName,
				status,
				duration,
				callback,
			})
		}
		table.Render()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(webhookCmd)
	webhookCmd.AddCommand(webhookListCmd)
	webhookCmd.AddCommand(webhookHistoryCmd)
	webhookHistoryCmd.Flags().StringVar(&webhookHistoryName, "name", "", "filter by webhook name")
	webhookHistoryCmd.Flags().IntVarP(&webhookHistoryLimit, "n", "n", 20, "number of executions to show")
}
