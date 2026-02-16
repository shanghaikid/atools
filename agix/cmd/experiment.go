package cmd

import (
	"fmt"
	"os"

	"github.com/agent-platform/agix/internal/experiment"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var experimentCmd = &cobra.Command{
	Use:   "experiment",
	Short: "Manage A/B test experiments",
	Long:  `View and manage A/B test experiments configured in the gateway.`,
}

var experimentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured experiments",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		if len(cfg.Experiments) == 0 {
			fmt.Println("No experiments configured.")
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Name", "Enabled", "Control", "Variant", "Traffic %"})
		table.SetBorder(false)
		table.SetColumnSeparator(" ")

		for _, e := range cfg.Experiments {
			enabled := "no"
			if e.Enabled {
				enabled = "yes"
			}
			table.Append([]string{
				e.Name,
				enabled,
				e.ControlModel,
				e.VariantModel,
				fmt.Sprintf("%d%%", e.TrafficPct),
			})
		}

		table.Render()
		return nil
	},
}

var experimentCheckCmd = &cobra.Command{
	Use:   "check [agent] [model]",
	Short: "Check which variant an agent would receive",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]
		model := args[1]

		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		var exps []experiment.Config
		for _, e := range cfg.Experiments {
			exps = append(exps, experiment.Config{
				Name:         e.Name,
				Enabled:      e.Enabled,
				ControlModel: e.ControlModel,
				VariantModel: e.VariantModel,
				TrafficPct:   e.TrafficPct,
			})
		}

		em := experiment.New(exps)
		if em == nil {
			fmt.Println("No enabled experiments.")
			return nil
		}

		assignment := em.Assign(agentName, model)
		if assignment == nil {
			fmt.Printf("No experiment matches model %q\n", model)
			return nil
		}

		fmt.Printf("Experiment: %s\n", assignment.ExperimentName)
		fmt.Printf("Variant:    %s\n", assignment.Variant)
		fmt.Printf("Model:      %s\n", assignment.Model)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(experimentCmd)
	experimentCmd.AddCommand(experimentListCmd)
	experimentCmd.AddCommand(experimentCheckCmd)
}
