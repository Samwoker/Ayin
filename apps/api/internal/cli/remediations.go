package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newRemediationsCmd() *cobra.Command {
	remsCmd := &cobra.Command{
		Use:     "remediations",
		Aliases: []string{"rem"},
		Short:   "Review and execute Human-in-the-Loop remediations",
	}

	remsCmd.AddCommand(newListRemediationsCmd())
	remsCmd.AddCommand(newGetRemediationCmd())
	remsCmd.AddCommand(newApproveRemediationCmd())
	remsCmd.AddCommand(newRejectRemediationCmd())
	remsCmd.AddCommand(newRollbackRemediationCmd())

	return remsCmd
}

func newListRemediationsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <incident_id>",
		Short: "List all remediations proposed or executed for an incident",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := GetClient()
			ctx := context.Background()
			rems, err := client.ListRemediations(ctx, args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			PrintOutput(rems, RenderRemediationsTable(rems))
		},
	}
}

func newGetRemediationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get details and parameters of a remediation",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := GetClient()
			ctx := context.Background()
			rem, err := client.GetRemediation(ctx, args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			PrintOutput(rem, RenderRemediationDetail(rem))
		},
	}
}

func newApproveRemediationCmd() *cobra.Command {
	var reason string
	var skipConfirm bool

	cmd := &cobra.Command{
		Use:   "approve <id>",
		Short: "Approve and execute a proposed remediation action",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			client := GetClient()
			ctx := context.Background()

			if !skipConfirm {
				fmt.Printf("CAUTION: Approving remediation %s will execute mutating operations on infrastructure.\n", id)
			}

			if reason == "" {
				reason = "Approved via CLI"
			}

			if err := client.ApproveRemediation(ctx, id, reason); err != nil {
				fmt.Fprintf(os.Stderr, "Error approving remediation: %v\n", err)
				os.Exit(1)
			}

			msg := fmt.Sprintf("Remediation %s approved and dispatched to executor.\n", id)
			PrintOutput(map[string]string{"id": id, "status": "approved"}, msg)
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Responder rationale for approval")
	cmd.Flags().BoolVarP(&skipConfirm, "yes", "y", false, "Skip confirmation prompt")

	return cmd
}

func newRejectRemediationCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "reject <id>",
		Short: "Reject a proposed remediation action",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			client := GetClient()
			ctx := context.Background()

			if reason == "" {
				reason = "Rejected via CLI"
			}

			if err := client.RejectRemediation(ctx, id, reason); err != nil {
				fmt.Fprintf(os.Stderr, "Error rejecting remediation: %v\n", err)
				os.Exit(1)
			}

			msg := fmt.Sprintf("Remediation %s rejected.\n", id)
			PrintOutput(map[string]string{"id": id, "status": "rejected"}, msg)
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Responder rationale for rejection")

	return cmd
}

func newRollbackRemediationCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "rollback <id>",
		Short: "Trigger rollback or compensation for a previously executed remediation",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			client := GetClient()
			ctx := context.Background()

			if reason == "" {
				reason = "Rollback triggered via CLI"
			}

			if err := client.RollbackRemediation(ctx, id, reason); err != nil {
				fmt.Fprintf(os.Stderr, "Error rolling back remediation: %v\n", err)
				os.Exit(1)
			}

			msg := fmt.Sprintf("Rollback for remediation %s dispatched to executor.\n", id)
			PrintOutput(map[string]string{"id": id, "status": "rolled_back"}, msg)
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Responder rationale for rollback")

	return cmd
}
