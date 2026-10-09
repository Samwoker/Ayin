package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "triage",
	Short: "triage - incident triage agent CLI",
	Long:  `triage is an automated incident triage and root-cause analysis agent CLI.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of triage",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("triage CLI v0.1.0 (Phase 1 scaffold)")
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Display authenticated user and active organization",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Org: dev-org (id: 00000000-0000-0000-0000-000000000001)")
		fmt.Println("Role: admin")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(whoamiCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
