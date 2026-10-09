package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	globalConfig Config
)

// GetClient returns a configured Client using global persistent flags or environment variables.
func GetClient() *Client {
	cfg := globalConfig
	if cfg.APIURL == "" {
		cfg.APIURL = os.Getenv("TRIAGE_API_URL")
		if cfg.APIURL == "" {
			cfg.APIURL = "http://localhost:8080"
		}
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("TRIAGE_API_KEY")
	}
	if cfg.Token == "" {
		cfg.Token = os.Getenv("TRIAGE_TOKEN")
	}
	if cfg.OrgID == "" {
		cfg.OrgID = os.Getenv("TRIAGE_ORG_ID")
		if cfg.OrgID == "" {
			cfg.OrgID = "00000000-0000-0000-0000-000000000001"
		}
	}
	return NewClient(cfg, nil)
}

// PrintOutput prints either JSON or human-readable text depending on global flags.
func PrintOutput(data any, text string) {
	if globalConfig.JSON {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
			return
		}
		fmt.Println(string(b))
	} else {
		fmt.Print(text)
	}
}

// NewRootCmd constructs the base triage CLI command with persistent flags.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "triage",
		Short: "triage - autonomous multi-tenant incident triage CLI",
		Long: `triage is an enterprise CLI for managing autonomous incident investigations,
inspecting hypotheses with evidence citations, and approving Human-in-the-Loop remediations.`,
	}

	rootCmd.PersistentFlags().StringVar(&globalConfig.APIURL, "api-url", "", "Triage API server URL (default: http://localhost:8080 or TRIAGE_API_URL)")
	rootCmd.PersistentFlags().StringVar(&globalConfig.APIKey, "api-key", "", "Organization API key (or TRIAGE_API_KEY)")
	rootCmd.PersistentFlags().StringVar(&globalConfig.Token, "token", "", "JWT Bearer authentication token (or TRIAGE_TOKEN)")
	rootCmd.PersistentFlags().StringVar(&globalConfig.OrgID, "org-id", "", "Active Tenant Organization UUID (or TRIAGE_ORG_ID)")
	rootCmd.PersistentFlags().BoolVar(&globalConfig.JSON, "json", false, "Output results in JSON format")

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newWhoamiCmd())
	rootCmd.AddCommand(newIncidentsCmd())
	rootCmd.AddCommand(newRunsCmd())
	rootCmd.AddCommand(newRemediationsCmd())
	rootCmd.AddCommand(newToolsCmd())

	return rootCmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print triage CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			PrintOutput(map[string]string{
				"version": "v1.0.0",
				"build":   "Phase 10",
			}, "triage CLI v1.0.0 (Phase 10 Release)\n")
		},
	}
}

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Display active authentication identity and tenant context",
		Run: func(cmd *cobra.Command, args []string) {
			client := GetClient()
			data := map[string]string{
				"api_url": client.cfg.APIURL,
				"org_id":  client.cfg.OrgID,
				"auth":    "api_key",
			}
			if client.cfg.Token != "" {
				data["auth"] = "jwt_bearer"
			}
			msg := fmt.Sprintf("API URL: %s\nOrg ID:  %s\nAuth:    %s\n",
				client.cfg.APIURL, client.cfg.OrgID, data["auth"])
			PrintOutput(data, msg)
		},
	}
}
