package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/seraph/triage/apps/api/internal/eval"
)

func main() {
	provider := flag.String("provider", "mock", "LLM provider to use for eval (mock, openai)")
	model := flag.String("model", "", "Model identifier (defaults to mock-deterministic-v1 or gpt-4o-mini)")
	scenariosDir := flag.String("scenarios", "scenarios", "Directory containing eval scenario YAML fixtures")
	compareFile := flag.String("compare", "", "Path to baseline report JSON to compare against for regressions")
	saveBaseline := flag.String("save-baseline", "", "Path to write current report as a baseline JSON fixture")
	outputFmt := flag.String("output", "text", "Output format (text, json, markdown)")
	threshold := flag.Float64("threshold", 80.0, "Minimum passing rate percentage required to succeed")
	flag.Parse()

	resolvedDir := eval.ResolveScenariosDir(*scenariosDir)

	cfg := eval.EvalConfig{
		ScenariosDir: resolvedDir,
		Provider:     *provider,
		Model:        *model,
		CompareFile:  *compareFile,
		SaveBaseline: *saveBaseline,
		PassRateGoal: *threshold,
	}

	ctx := context.Background()
	report, err := eval.RunEvaluation(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: evaluation failed: %v\n", err)
		os.Exit(1)
	}

	switch *outputFmt {
	case "json":
		bytes, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(bytes))
	case "markdown", "md":
		fmt.Print(eval.RenderMarkdownReport(report))
	default:
		fmt.Print(eval.RenderTextReport(report))
	}

	// Gate: Check threshold and regressions
	if report.PassRate < *threshold {
		fmt.Fprintf(os.Stderr, "FAILED: Pass rate %.1f%% below required threshold %.1f%%\n", report.PassRate, *threshold)
		os.Exit(1)
	}

	if report.Comparison != nil && report.Comparison.Delta.RegressionsCount > 0 {
		fmt.Fprintf(os.Stderr, "FAILED: %d regression(s) detected compared to baseline %s\n",
			report.Comparison.Delta.RegressionsCount, report.Comparison.BaselineRunID)
		os.Exit(1)
	}

	os.Exit(0)
}
