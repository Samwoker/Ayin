package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func newRunsCmd() *cobra.Command {
	runsCmd := &cobra.Command{
		Use:   "runs",
		Short: "Inspect autonomous investigation agent runs",
	}

	runsCmd.AddCommand(newListRunsCmd())
	runsCmd.AddCommand(newGetRunCmd())
	runsCmd.AddCommand(newTailRunsCmd())

	return runsCmd
}

func newListRunsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <incident_id>",
		Short: "List all investigation runs for an incident",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := GetClient()
			ctx := context.Background()
			runs, err := client.ListRuns(ctx, args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			PrintOutput(runs, RenderRunsTable(runs))
		},
	}
}

func newGetRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <run_id>",
		Short: "Get details and metrics of an investigation run",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := GetClient()
			ctx := context.Background()
			run, err := client.GetRun(ctx, args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			PrintOutput(run, fmt.Sprintf("Run ID:      %s\nIncident:    %s\nStatus:      %s\nSteps:       %d\nTokens:      %d\nCost (USD):  $%.4f\nCreated At:  %s\n",
				run.ID, run.IncidentID, run.Status, run.StepsCount, run.TotalTokens, run.TotalCostUSD, run.CreatedAt))
		},
	}
}

func newTailRunsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tail <incident_id>",
		Short: "Stream real-time SSE diagnostic steps and agent reasoning for an incident",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			incidentID := args[0]
			client := GetClient()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Println("\nStopping live stream...")
				cancel()
			}()

			fmt.Printf("Streaming real-time investigation events for incident %s (Ctrl+C to quit)...\n\n", incidentID)

			err := client.StreamIncident(ctx, incidentID, func(eventType, data string) {
				var payload map[string]any
				_ = json.Unmarshal([]byte(data), &payload)

				switch eventType {
				case "run_started", "run.started":
					fmt.Printf("[START] Investigation started for service: %v\n", payload["service"])
				case "run_step", "run.step":
					stepNum := payload["step_number"]
					tool := payload["tool_name"]
					fmt.Printf("[STEP %v] Tool: %-18s Latency: %vms\n", stepNum, tool, payload["latency_ms"])
				case "run_completed", "run.completed":
					fmt.Printf("[COMPLETED] Investigation completed! Status: %v (Tokens: %v, Cost: $%v)\n",
						payload["status"], payload["total_tokens"], payload["total_cost_usd"])
				case "remediation_proposed", "remediation.proposed":
					fmt.Printf("[REMEDIATION] Proposed action: %v on target: %v (Risk: %v)\n",
						payload["action"], payload["target"], payload["risk_tier"])
				case "remediation_approved", "remediation.approved":
					fmt.Printf("[APPROVED] Remediation %v approved by responder\n", payload["remediation_id"])
				default:
					if eventType != "" {
						fmt.Printf("[%s] %s\n", eventType, data)
					}
				}
			})

			if err != nil && err != context.Canceled {
				fmt.Fprintf(os.Stderr, "Streaming disconnected: %v\n", err)
			}
		},
	}
}
