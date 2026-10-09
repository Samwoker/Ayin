package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	provider := flag.String("provider", "mock", "LLM provider to use for eval (mock, openai, anthropic)")
	scenariosDir := flag.String("scenarios", "scenarios", "Directory containing eval scenario YAML fixtures")
	compareRunID := flag.String("compare", "", "Optional run ID to compare against")
	flag.Parse()

	fmt.Printf("Triage Evaluation Runner (Phase 1 CLI)\n")
	fmt.Printf("Provider: %s\n", *provider)
	fmt.Printf("Scenarios: %s\n", *scenariosDir)
	if *compareRunID != "" {
		fmt.Printf("Comparing with run: %s\n", *compareRunID)
	}
	fmt.Printf("Scaffold verified. Evals engine wired in Phase 7.\n")
	os.Exit(0)
}
