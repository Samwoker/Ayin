package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newToolsCmd() *cobra.Command {
	toolsCmd := &cobra.Command{
		Use:   "tools",
		Short: "Inspect and execute diagnostic read-only investigation tools",
	}

	toolsCmd.AddCommand(newListToolsCmd())
	toolsCmd.AddCommand(newExecToolCmd())

	return toolsCmd
}

func newListToolsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered diagnostic tools available to the organization",
		Run: func(cmd *cobra.Command, args []string) {
			client := GetClient()
			ctx := context.Background()
			tools, err := client.ListTools(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			PrintOutput(tools, RenderToolsTable(tools))
		},
	}
}

func newExecToolCmd() *cobra.Command {
	var paramsRaw string

	cmd := &cobra.Command{
		Use:   "exec <tool_name>",
		Short: "Directly execute a diagnostic tool within tenant security boundaries",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			toolName := args[0]
			client := GetClient()
			ctx := context.Background()

			var inputJSON []byte
			if paramsRaw != "" {
				inputJSON = []byte(paramsRaw)
			} else {
				inputJSON = []byte("{}")
			}

			res, err := client.ExecuteTool(ctx, toolName, inputJSON)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error executing tool %s: %v\n", toolName, err)
				os.Exit(1)
			}

			b, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error formatting JSON output: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(b))
		},
	}

	cmd.Flags().StringVar(&paramsRaw, "params", "", "JSON string input parameters for the tool (e.g. '{\"service\":\"checkout-worker\"}')")

	return cmd
}
