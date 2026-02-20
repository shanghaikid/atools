package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/agent-platform/agix/internal/bundle"
	"github.com/agent-platform/agix/internal/config"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Manage MCP tool bundles",
	Long:  `Pre-packaged sets of MCP servers that can be installed with one command.`,
}

var bundleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available and installed bundles",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		infos, err := bundle.List(cfg.Bundles)
		if err != nil {
			return err
		}

		if len(infos) == 0 {
			fmt.Println("No bundles available.")
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Name", "Description", "Source", "Installed"})
		table.SetBorder(false)
		table.SetColumnSeparator(" ")

		for _, info := range infos {
			source := "built-in"
			if !info.Builtin {
				source = "user"
			}
			installed := ""
			if info.Installed {
				installed = "yes"
			}
			table.Append([]string{info.Name, info.Description, source, installed})
		}

		table.Render()
		return nil
	},
}

var bundleShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show bundle details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, err := bundle.Get(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Bundle: %s\n", b.Name)
		fmt.Printf("Description: %s\n\n", b.Description)

		fmt.Println("Servers:")
		for name, server := range b.Servers {
			fmt.Printf("  %s:\n", name)
			fmt.Printf("    command: %s\n", server.Command)
			fmt.Printf("    args: [%s]\n", strings.Join(server.Args, ", "))
			if len(server.Env) > 0 {
				fmt.Printf("    env: [%s]\n", strings.Join(server.Env, ", "))
			}
		}

		if len(b.AgentDefaults) > 0 {
			fmt.Println("\nAgent Defaults:")
			for name, tools := range b.AgentDefaults {
				fmt.Printf("  %s:\n", name)
				if len(tools.Allow) > 0 {
					fmt.Printf("    allow: [%s]\n", strings.Join(tools.Allow, ", "))
				}
				if len(tools.Deny) > 0 {
					fmt.Printf("    deny: [%s]\n", strings.Join(tools.Deny, ", "))
				}
			}
		}

		return nil
	},
}

var bundleInstallCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "Install a bundle into config",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, cfgPath, err := loadConfig()
		if err != nil {
			return err
		}

		b, err := bundle.Get(args[0])
		if err != nil {
			return err
		}

		if err := bundle.Install(cfg, b); err != nil {
			return err
		}

		if err := config.Save(cfgPath, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Installed bundle %q — added %d server(s) and %d agent default(s).\n",
			b.Name, len(b.Servers), len(b.AgentDefaults))
		return nil
	},
}

var bundleRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a bundle from config",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, cfgPath, err := loadConfig()
		if err != nil {
			return err
		}

		b, err := bundle.Get(args[0])
		if err != nil {
			return err
		}

		if err := bundle.Remove(cfg, b); err != nil {
			return err
		}

		if err := config.Save(cfgPath, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Removed bundle %q — removed %d server(s) and %d agent default(s).\n",
			b.Name, len(b.Servers), len(b.AgentDefaults))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(bundleCmd)
	bundleCmd.AddCommand(bundleListCmd)
	bundleCmd.AddCommand(bundleShowCmd)
	bundleCmd.AddCommand(bundleInstallCmd)
	bundleCmd.AddCommand(bundleRemoveCmd)
}
