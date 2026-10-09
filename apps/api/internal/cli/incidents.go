package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newIncidentsCmd() *cobra.Command {
	incidentsCmd := &cobra.Command{
		Use:     "incidents",
		Aliases: []string{"inc"},
		Short:   "Manage and inspect triage incidents",
	}

	incidentsCmd.AddCommand(newListIncidentsCmd())
	incidentsCmd.AddCommand(newGetIncidentCmd())
	incidentsCmd.AddCommand(newCreateIncidentCmd())
	incidentsCmd.AddCommand(newResolveIncidentCmd())

	return incidentsCmd
}

func newListIncidentsCmd() *cobra.Command {
	var status, service, severity string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List incidents with filtering",
		Run: func(cmd *cobra.Command, args []string) {
			client := GetClient()
			ctx := context.Background()
			incidents, err := client.ListIncidents(ctx, status, service, severity, limit)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			PrintOutput(incidents, RenderIncidentsTable(incidents))
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by incident status (open, investigating, remediating, resolved)")
	cmd.Flags().StringVar(&service, "service", "", "Filter by service name")
	cmd.Flags().StringVar(&severity, "severity", "", "Filter by severity (sev1, sev2, sev3, sev4)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of incidents to return")

	return cmd
}

func newGetIncidentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get incident details by ID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := GetClient()
			ctx := context.Background()
			inc, err := client.GetIncident(ctx, args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			PrintOutput(inc, RenderIncidentDetail(inc))
		},
	}
}

func newCreateIncidentCmd() *cobra.Command {
	var title, service, severity, description string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Manually report or trigger a new incident",
		Run: func(cmd *cobra.Command, args []string) {
			if title == "" || service == "" {
				fmt.Fprintf(os.Stderr, "Error: --title and --service are required\n")
				os.Exit(1)
			}
			if severity == "" {
				severity = "sev2"
			}

			client := GetClient()
			ctx := context.Background()
			inc, err := client.CreateIncident(ctx, title, service, severity, description)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating incident: %v\n", err)
				os.Exit(1)
			}

			msg := fmt.Sprintf("Successfully created incident %s [%s] (%s)\n", inc.ID, inc.Severity, inc.Title)
			PrintOutput(inc, msg)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Incident title summary")
	cmd.Flags().StringVar(&service, "service", "", "Target affected service name")
	cmd.Flags().StringVar(&severity, "severity", "sev2", "Severity tier (sev1, sev2, sev3, sev4)")
	cmd.Flags().StringVar(&description, "desc", "", "Detailed failure description")

	return cmd
}

func newResolveIncidentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resolve <id>",
		Short: "Mark an incident as resolved",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := GetClient()
			ctx := context.Background()
			if err := client.ResolveIncident(ctx, args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error resolving incident: %v\n", err)
				os.Exit(1)
			}
			msg := fmt.Sprintf("Incident %s successfully marked as resolved.\n", args[0])
			PrintOutput(map[string]string{"id": args[0], "status": "resolved"}, msg)
		},
	}
}
