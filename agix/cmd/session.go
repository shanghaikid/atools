package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/agent-platform/agix/internal/session"
	"github.com/agent-platform/agix/internal/store"
	"github.com/agent-platform/agix/internal/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage session overrides",
	Long:  "View and manage session-level config overrides (model, temperature, max_tokens).",
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active session overrides",
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

		ttl := time.Hour
		if cfg.SessionOverrides.DefaultTTL != "" {
			if d, err := time.ParseDuration(cfg.SessionOverrides.DefaultTTL); err == nil {
				ttl = d
			}
		}

		mgr, err := session.New(st.DB(), ttl, st.Dialect())
		if err != nil {
			return fmt.Errorf("init session manager: %w", err)
		}
		defer mgr.Close()

		overrides, err := mgr.ListActive()
		if err != nil {
			return fmt.Errorf("list sessions: %w", err)
		}

		if len(overrides) == 0 {
			fmt.Println(ui.Dimf("No active session overrides."))
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Session ID", "Agent", "Model", "Temp", "Max Tokens", "Expires"})
		table.SetBorder(false)
		table.SetColumnSeparator(" ")
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)

		for _, o := range overrides {
			temp := "-"
			if o.Temperature != nil {
				temp = fmt.Sprintf("%.2f", *o.Temperature)
			}
			maxTok := "-"
			if o.MaxTokens != nil {
				maxTok = fmt.Sprintf("%d", *o.MaxTokens)
			}
			model := o.Model
			if model == "" {
				model = "-"
			}
			remaining := time.Until(o.ExpiresAt).Truncate(time.Second)
			expires := fmt.Sprintf("%s (%s)", o.ExpiresAt.Format("15:04:05"), remaining)

			table.Append([]string{o.SessionID, o.AgentName, model, temp, maxTok, expires})
		}

		table.Render()
		return nil
	},
}

var sessionCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Purge expired session overrides",
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

		ttl := time.Hour
		if cfg.SessionOverrides.DefaultTTL != "" {
			if d, err := time.ParseDuration(cfg.SessionOverrides.DefaultTTL); err == nil {
				ttl = d
			}
		}

		mgr, err := session.New(st.DB(), ttl, st.Dialect())
		if err != nil {
			return fmt.Errorf("init session manager: %w", err)
		}
		defer mgr.Close()

		n, err := mgr.CleanExpired()
		if err != nil {
			return fmt.Errorf("clean expired: %w", err)
		}

		if n == 0 {
			fmt.Println(ui.Dimf("No expired sessions to clean."))
		} else {
			fmt.Printf("Cleaned %d expired session override(s).\n", n)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionCleanCmd)
}
