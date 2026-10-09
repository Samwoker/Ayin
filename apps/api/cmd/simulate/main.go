package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	scenario := flag.String("scenario", "rabbitmq-backlog", "Fault-injection scenario name")
	chaosLevel := flag.Int("chaos", 0, "Chaos fault injection level (0-3)")
	seed := flag.Int64("seed", 1234, "Deterministic random seed")
	flag.Parse()

	fmt.Printf("Triage Fault-Injection Simulator (Phase 1 CLI)\n")
	fmt.Printf("Scenario: %s\n", *scenario)
	fmt.Printf("Chaos level: %d\n", *chaosLevel)
	fmt.Printf("Seed: %d\n", *seed)
	fmt.Printf("Simulator engine wired in Phase 7/18.\n")
	os.Exit(0)
}
