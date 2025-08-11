package scenarios

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed *.yaml
var embeddedFiles embed.FS

// ScenarioConfig represents a scenario configuration from YAML (duplicate to avoid import cycle)
type ScenarioConfig struct {
	Name        string `yaml:"-"` // Set during loading
	Tools       string `yaml:"tools"`
	Description string `yaml:"description"`
	Prompt      string `yaml:"prompt"`
}

// ScenarioConfigMap represents all scenarios loaded from YAML files
type ScenarioConfigMap map[string]ScenarioConfig

// LoadBuiltinScenarios loads built-in scenarios from embedded files
func LoadBuiltinScenarios() (ScenarioConfigMap, error) {
	scenarios := make(ScenarioConfigMap)

	// Read all embedded scenario files
	entries, err := embeddedFiles.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded scenarios: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip non-YAML files
		name := entry.Name()
		if !isYAMLFile(name) {
			continue
		}

		// Read embedded file
		data, err := embeddedFiles.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded scenario file %s: %w", name, err)
		}

		// Parse YAML
		var fileScenarios map[string]ScenarioConfig
		if err := yaml.Unmarshal(data, &fileScenarios); err != nil {
			return nil, fmt.Errorf("failed to parse embedded scenario file %s: %w", name, err)
		}

		// Add scenarios to the map, setting the name
		for scenarioName, scenarioConfig := range fileScenarios {
			scenarioConfig.Name = scenarioName
			scenarios[scenarioName] = scenarioConfig
		}
	}

	return scenarios, nil
}

func isYAMLFile(name string) bool {
	return len(name) > 5 && (name[len(name)-5:] == ".yaml" || (len(name) > 4 && name[len(name)-4:] == ".yml"))
}
