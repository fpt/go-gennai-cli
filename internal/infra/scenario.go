package infra

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fpt/go-gennai-cli/internal/repository"
	domain "github.com/fpt/go-gennai-cli/internal/repository"
	"gopkg.in/yaml.v3"
)

// ScenarioConfig represents a scenario configuration from YAML
type ScenarioConfig struct {
	name        string `yaml:"-"` // Set during loading
	tools       string `yaml:"tools"`
	description string `yaml:"description"`
	prompt      string `yaml:"prompt"`
}

func NewScenarioConfig(name, tools, description, prompt string) *ScenarioConfig {
	return &ScenarioConfig{
		name:        name,
		tools:       tools,
		description: description,
		prompt:      prompt,
	}
}

// Implement repository.Scenario interface methods
func (s *ScenarioConfig) Name() string {
	return s.name
}

func (s *ScenarioConfig) Tools() string {
	return s.tools
}

func (s *ScenarioConfig) Description() string {
	return s.description
}

func (s *ScenarioConfig) Prompt() string {
	return s.prompt
}

// GetToolScope parses the tools field and returns which tool managers to use
func (s *ScenarioConfig) GetToolScope() domain.ToolScope {
	scope := domain.ToolScope{
		MCPTools: make([]string, 0),
	}

	// Split tools by comma and parse each one
	toolList := strings.Split(s.tools, ",")
	for _, tool := range toolList {
		tool = strings.TrimSpace(tool)
		toolLower := strings.ToLower(tool)

		if toolLower == "filesystem" {
			scope.UseFilesystem = true
		} else if toolLower == "default" {
			scope.UseDefault = true
		} else if toolLower == "todo" {
			scope.UseTodo = true
		} else if toolLower == "bash" {
			scope.UseBash = true
		} else if toolLower == "solver" {
			scope.UseSolver = true
		} else if strings.HasPrefix(toolLower, "mcp:") {
			// Extract MCP tool name (remove "mcp:" prefix, preserve case)
			mcpName := strings.TrimPrefix(tool, "mcp:")
			if mcpName == "" {
				mcpName = strings.TrimPrefix(tool, "MCP:")
			}

			if mcpName != "" {
				scope.MCPTools = append(scope.MCPTools, mcpName)
			}
		}
	}

	// Default to using default tools if nothing specified
	if !scope.UseFilesystem && !scope.UseDefault && !scope.UseTodo && !scope.UseBash && !scope.UseSolver && len(scope.MCPTools) == 0 {
		scope.UseDefault = true
	}

	return scope
}

// RenderPrompt replaces template variables in the prompt with actual values
func (s *ScenarioConfig) RenderPrompt(userInput, scenarioReason, workingDir string) string {
	prompt := s.prompt

	// Replace template variables
	prompt = strings.ReplaceAll(prompt, "{{userInput}}", userInput)
	prompt = strings.ReplaceAll(prompt, "{{scenarioReason}}", scenarioReason)
	prompt = strings.ReplaceAll(prompt, "{{workingDir}}", workingDir)

	return prompt
}

// LoadBuiltinScenariosFunc is a function variable that can be overridden to load built-in scenarios
var LoadBuiltinScenariosFunc func() (ScenarioMap, error) = defaultLoadBuiltinScenarios

// defaultLoadBuiltinScenarios is the default implementation (fallback when embedded scenarios not available)
func defaultLoadBuiltinScenarios() (ScenarioMap, error) {
	return make(ScenarioMap), fmt.Errorf("built-in scenarios not available - embedded scenarios package not imported")
}

// LoadBuiltinScenarios loads built-in scenarios using the current implementation
func LoadBuiltinScenarios() (ScenarioMap, error) {
	return LoadBuiltinScenariosFunc()
}

// LoadScenariosFromPath loads scenarios from a specific file or directory path
func LoadScenariosFromPath(path string) (ScenarioMap, error) {
	scenarios := make(ScenarioMap)

	// Check if path is a file or directory
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to access path %s: %w", path, err)
	}

	if info.IsDir() {
		// Load from directory
		err = filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip non-YAML files
			if !strings.HasSuffix(strings.ToLower(fileInfo.Name()), ".yaml") &&
				!strings.HasSuffix(strings.ToLower(fileInfo.Name()), ".yml") {
				return nil
			}

			return loadScenarioFile(filePath, scenarios)
		})
	} else {
		// Load single file
		err = loadScenarioFile(path, scenarios)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load scenarios from %s: %w", path, err)
	}

	return scenarios, nil
}

// loadScenarioFile loads scenarios from a single YAML file
func loadScenarioFile(filePath string, scenarios ScenarioMap) error {
	// Read YAML file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read scenario file %s: %w", filePath, err)
	}

	// Parse YAML
	var fileScenarios map[string]ScenarioConfig
	if err := yaml.Unmarshal(data, &fileScenarios); err != nil {
		return fmt.Errorf("failed to parse scenario file %s: %w", filePath, err)
	}

	// Add scenarios to the map, setting the name and normalizing keys to uppercase for case-insensitive lookup
	for scenarioName, scenarioConfig := range fileScenarios {
		scenarioConfig.name = scenarioName              // Keep original name for display
		normalizedName := strings.ToUpper(scenarioName) // Normalize key for case-insensitive lookup
		scenarios[normalizedName] = &scenarioConfig
	}

	return nil
}

// LoadScenarios loads scenarios with built-ins and optional additional paths
func LoadScenarios(additionalPaths ...string) (ScenarioMap, error) {
	// Start with built-in scenarios (will be overridden when scenarios package is imported)
	scenarios, _ := LoadBuiltinScenarios()

	// Load additional scenarios (these override built-ins)
	for _, path := range additionalPaths {
		if path == "" {
			continue
		}

		additionalScenarios, err := LoadScenariosFromPath(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load additional scenarios from %s: %w", path, err)
		}

		// Merge additional scenarios, overriding built-ins
		for name, scenario := range additionalScenarios {
			scenarios[name] = scenario
		}
	}

	return scenarios, nil
}

// LoadLegacyScenarios loads scenarios from a directory path (backward compatibility)
func LoadLegacyScenarios(scenarioDir string) (ScenarioMap, error) {
	return LoadScenariosFromPath(scenarioDir)
}

// ScenarioMap represents all scenarios loaded from YAML files
type ScenarioMap map[string]repository.Scenario

// GetScenario retrieves a specific scenario configuration
func (s ScenarioMap) GetScenario(name string) (repository.Scenario, bool) {
	scenario, exists := s[name]
	return scenario, exists
}

// GetAvailableScenarios returns a list of all available scenario names
func (s ScenarioMap) GetAvailableScenarios() []string {
	scenarios := make([]string, 0, len(s))
	for name := range s {
		scenarios = append(scenarios, name)
	}
	return scenarios
}

// DefaultScenarioDirectory returns the default path to scenario configurations (legacy function)
// Note: This function is deprecated since scenarios are now embedded in the binary
func DefaultScenarioDirectory() string {
	// Try to find the project root by looking for go.mod
	dir, err := os.Getwd()
	if err != nil {
		return "scenarios" // fallback to scenarios directory
	}

	// Walk up the directory tree to find go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "custom-scenarios") // suggest custom scenarios directory
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, fallback to relative path
			break
		}
		dir = parent
	}

	return "custom-scenarios"
}
