package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ResolveScenariosDir finds the directory containing benchmark scenarios.
func ResolveScenariosDir(dir string) string {
	candidates := []string{
		dir,
		filepath.Join(".", dir),
		filepath.Join("..", dir),
		filepath.Join("..", "..", dir),
		"scenarios",
		filepath.Join("..", "scenarios"),
		filepath.Join("..", "..", "scenarios"),
	}

	for _, c := range candidates {
		if c == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return dir
}

// LoadScenarios reads and parses all YAML scenario fixtures in the specified directory.
func LoadScenarios(dir string) ([]*Scenario, error) {
	resolvedDir := ResolveScenariosDir(dir)
	entries, err := os.ReadDir(resolvedDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read scenarios directory %q: %w", resolvedDir, err)
	}

	var scenarios []*Scenario
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(resolvedDir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read scenario file %s: %w", filePath, err)
		}

		var sc Scenario
		if err := yaml.Unmarshal(data, &sc); err != nil {
			return nil, fmt.Errorf("failed to unmarshal scenario %s: %w", filePath, err)
		}

		if err := validateScenario(&sc, filePath); err != nil {
			return nil, err
		}

		scenarios = append(scenarios, &sc)
	}

	if len(scenarios) == 0 {
		return nil, fmt.Errorf("no scenario YAML files found in directory %q", resolvedDir)
	}

	// Sort deterministically by scenario ID
	sort.Slice(scenarios, func(i, j int) bool {
		return scenarios[i].ID < scenarios[j].ID
	})

	return scenarios, nil
}

// LoadScenarioByID reads a single scenario by its ID or filename.
func LoadScenarioByID(dir, id string) (*Scenario, error) {
	scenarios, err := LoadScenarios(dir)
	if err != nil {
		return nil, err
	}
	for _, sc := range scenarios {
		if sc.ID == id || strings.TrimSuffix(sc.ID, ".yaml") == strings.TrimSuffix(id, ".yaml") {
			return sc, nil
		}
	}
	return nil, fmt.Errorf("scenario with ID %q not found", id)
}

func validateScenario(sc *Scenario, filePath string) error {
	if sc.ID == "" {
		return fmt.Errorf("scenario in %s missing required field 'id'", filePath)
	}
	if sc.Name == "" {
		return fmt.Errorf("scenario %s missing required field 'name'", sc.ID)
	}
	if sc.Service == "" {
		return fmt.Errorf("scenario %s missing required field 'service'", sc.ID)
	}
	if sc.Alert.Title == "" {
		return fmt.Errorf("scenario %s alert missing 'title'", sc.ID)
	}
	if len(sc.GroundTruth.ExpectedRootCauses) == 0 {
		return fmt.Errorf("scenario %s ground_truth must define at least one 'expected_root_causes'", sc.ID)
	}
	if sc.GroundTruth.MinConfidence <= 0 {
		sc.GroundTruth.MinConfidence = 0.70
	}
	if sc.GroundTruth.MaxSteps <= 0 {
		sc.GroundTruth.MaxSteps = 8
	}
	if sc.GroundTruth.MaxTokens <= 0 {
		sc.GroundTruth.MaxTokens = 25000
	}
	return nil
}
