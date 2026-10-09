package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/seraph/triage/apps/api/internal/eval"
	"github.com/seraph/triage/apps/api/internal/simulate"
)

func main() {
	scenarioName := flag.String("scenario", "rabbitmq-backlog", "Fault-injection scenario name ('all' for entire suite)")
	chaosLevel := flag.Int("chaos", 0, "Chaos fault injection level (0=clean, 1=noise, 2=jitter, 3=adversarial)")
	seed := flag.Int64("seed", 1234, "Deterministic random seed for reproducible chaos")
	scenariosDir := flag.String("scenarios", "scenarios", "Directory containing eval scenario YAML fixtures")
	targetURL := flag.String("target", "", "Target triage API base URL (e.g. http://localhost:8080)")
	secret := flag.String("secret", "", "Webhook HMAC-SHA256 signature secret")
	dryRun := flag.Bool("dry-run", false, "Execute in local dry-run simulation mode without firing network requests")
	flag.Parse()

	resolvedDir := eval.ResolveScenariosDir(*scenariosDir)

	// Default to dry-run if no target URL is provided
	isDryRun := *dryRun || *targetURL == ""

	cfg := simulate.SimulationConfig{
		ScenarioName:  *scenarioName,
		ScenariosDir:  resolvedDir,
		ChaosLevel:    simulate.ChaosLevel(*chaosLevel),
		Seed:          *seed,
		TargetURL:     *targetURL,
		WebhookSecret: *secret,
		DryRun:        isDryRun,
	}

	ctx := context.Background()
	dispatcher := simulate.NewDispatcher(nil)

	if *scenarioName == "all" {
		scenarios, err := eval.LoadScenarios(resolvedDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading scenarios: %v\n", err)
			os.Exit(1)
		}

		for _, sc := range scenarios {
			cfg.ScenarioName = sc.ID
			res, err := dispatcher.Run(ctx, cfg, sc)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running scenario %s: %v\n", sc.ID, err)
				continue
			}
			fmt.Print(simulate.RenderSimulationReport(res))
		}
		os.Exit(0)
	}

	sc, err := eval.LoadScenarioByID(resolvedDir, *scenarioName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: scenario %q not found: %v\n", *scenarioName, err)
		os.Exit(1)
	}

	res, err := dispatcher.Run(ctx, cfg, sc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: simulation execution failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(simulate.RenderSimulationReport(res))
	os.Exit(0)
}
