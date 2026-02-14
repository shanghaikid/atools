package cmd

import (
	"fmt"
	"os"

	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/toolmgr"
	"github.com/agent-platform/agix/internal/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Manage shared MCP tools",
}

var toolsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available MCP tools",
	Long: `Connects to all configured MCP servers, discovers available tools,
and displays them in a table. Use this to verify your MCP server configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		if len(cfg.Tools.Servers) == 0 {
			fmt.Println(ui.Dimf("No MCP servers configured in config.yaml"))
			fmt.Println(ui.Dimf("Add servers under the 'tools.servers' section."))
			return nil
		}

		fmt.Println(ui.Dimf("Connecting to MCP servers..."))

		mgr, err := toolmgr.New(cfg.Tools)
		if err != nil {
			return fmt.Errorf("initialize tool manager: %w", err)
		}
		defer mgr.Close()

		tools := mgr.AllTools()
		if len(tools) == 0 {
			fmt.Println(ui.Yellowf("No tools discovered from %d server(s)", mgr.ServerCount()))
			return nil
		}

		fmt.Printf("\n%s %d tool(s) from %d server(s)\n\n",
			ui.Boldf("Discovered"), len(tools), mgr.ServerCount())

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Tool", "Server", "Description"})
		table.SetBorder(false)
		table.SetColumnSeparator(" ")
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)

		for _, t := range tools {
			desc := t.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			table.Append([]string{t.Name, t.Server, desc})
		}
		table.Render()

		// Show agent access summary
		if len(cfg.Tools.Agents) > 0 {
			fmt.Printf("\n%s\n", ui.Dimf("Agent access rules:"))
			for agent, acl := range cfg.Tools.Agents {
				if len(acl.Allow) > 0 {
					fmt.Printf("  %s  allow %v\n", ui.Cyanf("%s", agent), acl.Allow)
				} else if len(acl.Deny) > 0 {
					fmt.Printf("  %s  deny %v\n", ui.Cyanf("%s", agent), acl.Deny)
				}
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(toolsCmd)
	toolsCmd.AddCommand(toolsListCmd)
}

// initToolManager creates a tool manager from config. Returns nil if no servers configured.
func initToolManager(cfg *config.Config) (*toolmgr.Manager, error) {
	if len(cfg.Tools.Servers) == 0 {
		return nil, nil
	}

	mgr, err := toolmgr.New(cfg.Tools)
	if err != nil {
		return nil, err
	}

	return mgr, nil
}
