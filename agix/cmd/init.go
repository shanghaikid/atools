package cmd

import (
	"fmt"
	"os"

	"github.com/agent-platform/agix/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize agix configuration",
	Long:  `Creates the configuration directory and default config file at ~/.agix/config.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := cfgFile
		if path == "" {
			var err error
			path, err = config.DefaultConfigPath()
			if err != nil {
				return fmt.Errorf("determine config path: %w", err)
			}
		}

		// If config already exists, merge with defaults to pick up new keys
		if _, err := os.Stat(path); err == nil {
			cfg, loadErr := config.Load(path)
			if loadErr != nil {
				return fmt.Errorf("load existing config: %w", loadErr)
			}
			if err := config.SaveWithComments(path, cfg); err != nil {
				return fmt.Errorf("update config: %w", err)
			}
			fmt.Printf("Configuration updated at %s (merged new defaults)\n", path)
			return nil
		}

		cfg := config.DefaultConfig()
		if err := config.SaveWithComments(path, &cfg); err != nil {
			return fmt.Errorf("create config: %w", err)
		}

		fmt.Printf("Configuration initialized at %s\n", path)
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  1. Add your API keys to the config file:")
		fmt.Printf("     %s\n", path)
		fmt.Println()
		fmt.Println("  2. Start the proxy:")
		fmt.Println("     agix start")
		fmt.Println()
		fmt.Println("  3. Point your agents to http://localhost:8080")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
