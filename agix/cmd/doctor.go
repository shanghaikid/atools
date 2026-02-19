package cmd

import (
	"os"

	"github.com/agent-platform/agix/internal/doctor"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check configuration and dependencies",
	Long: `Runs a comprehensive health check on your agix setup:

  - Config file permissions (should be 0600)
  - API key validity (lightweight models list request)
  - Budget configuration sanity (daily < monthly)
  - Firewall rule regex syntax
  - SQLite database integrity`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, cfgPath, err := loadConfig()
		if err != nil {
			return err
		}
		fails := doctor.Run(os.Stdout, cfg, cfgPath)
		if fails > 0 {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
