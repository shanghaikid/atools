package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/agent-platform/agix/internal/audit"
	"github.com/agent-platform/agix/internal/store"
	"github.com/agent-platform/agix/internal/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View audit events",
	Long:  "View and query the audit event log for security-relevant events.",
}

var auditListN int
var auditListType string
var auditListAgent string

var auditListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent audit events",
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

		logger := audit.New(st.DB(), true, st.Dialect())
		defer logger.Close()

		events, err := logger.QueryRecent(auditListN, auditListType, auditListAgent)
		if err != nil {
			return fmt.Errorf("query events: %w", err)
		}

		if len(events) == 0 {
			fmt.Println(ui.Dimf("No audit events found."))
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Time", "Type", "Agent", "Details"})
		table.SetBorder(false)
		table.SetColumnSeparator(" ")
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)

		for _, e := range events {
			ts := e.Timestamp.Format("15:04:05")
			details := summarizeDetails(e.EventType, e.Details)
			table.Append([]string{
				fmt.Sprintf("%d", e.ID),
				ts,
				colorEventType(e.EventType),
				e.AgentName,
				details,
			})
		}

		table.Render()
		return nil
	},
}

func colorEventType(t string) string {
	switch t {
	case audit.EventFirewallBlock:
		return ui.Redf("block")
	case audit.EventFirewallWarn:
		return ui.Yellowf("warn")
	case audit.EventToolCall:
		return ui.Greenf("tool")
	case audit.EventContentLog:
		return ui.Dimf("content")
	default:
		return t
	}
}

func summarizeDetails(eventType string, raw json.RawMessage) string {
	switch eventType {
	case audit.EventToolCall:
		var d audit.ToolCallDetails
		if json.Unmarshal(raw, &d) == nil {
			s := fmt.Sprintf("%s (%s) %dms", d.Tool, d.Server, d.DurationMS)
			if d.Dangerous {
				s += " [DANGEROUS]"
			}
			return s
		}
	case audit.EventFirewallBlock, audit.EventFirewallWarn:
		var d audit.FirewallDetails
		if json.Unmarshal(raw, &d) == nil {
			return fmt.Sprintf("rule=%s cat=%s", d.Rule, d.Category)
		}
	case audit.EventContentLog:
		var d audit.ContentLogDetails
		if json.Unmarshal(raw, &d) == nil {
			body := d.Body
			if len(body) > 80 {
				body = body[:80] + "..."
			}
			return fmt.Sprintf("%s %s", d.Direction, d.Model)
		}
	}
	return string(raw)
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.AddCommand(auditListCmd)
	auditListCmd.Flags().IntVarP(&auditListN, "number", "n", 20, "number of events to show")
	auditListCmd.Flags().StringVarP(&auditListType, "type", "t", "", "filter by event type (tool_call, firewall_block, firewall_warn, content_log)")
	auditListCmd.Flags().StringVarP(&auditListAgent, "agent", "a", "", "filter by agent name")
}
