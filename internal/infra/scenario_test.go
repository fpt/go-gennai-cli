package infra

import (
	"testing"

	"github.com/fpt/go-gennai-cli/internal/scenarios"
)

func init() {
	// Override for testing
	LoadBuiltinScenariosFunc = func() (ScenarioMap, error) {
		embeddedScenarios, err := scenarios.LoadBuiltinScenarios()
		if err != nil {
			return nil, err
		}

		// Convert between types
		configScenarios := make(ScenarioMap)
		for name, scenario := range embeddedScenarios {
			configScenarios[name] = NewScenarioConfig(
				scenario.Name,
				scenario.Tools,
				scenario.Description,
				scenario.Prompt,
			)
		}

		return configScenarios, nil
	}
}

func TestLoadBuiltinScenarios(t *testing.T) {
	// Test loading built-in embedded scenarios
	scenarios, err := LoadBuiltinScenarios()
	if err != nil {
		t.Fatalf("Failed to load built-in scenarios: %v", err)
	}

	// Check that we have the expected scenarios
	expectedScenarios := []string{
		"CODE",
		"RESPOND",
	}

	for _, expected := range expectedScenarios {
		if _, exists := scenarios[expected]; !exists {
			t.Errorf("Expected scenario %s not found", expected)
		}
	}

	t.Logf("Loaded %d built-in scenarios", len(scenarios))
}

func TestLoadScenariosWithOverride(t *testing.T) {
	// Test loading built-in scenarios with additional override
	scenarios, err := LoadScenarios()
	if err != nil {
		t.Fatalf("Failed to load scenarios: %v", err)
	}

	// Should have all built-in scenarios
	expectedScenarios := []string{
		"CODE",
		"RESPOND",
	}

	for _, expected := range expectedScenarios {
		if _, exists := scenarios[expected]; !exists {
			t.Errorf("Expected scenario %s not found", expected)
		}
	}

	t.Logf("Loaded %d scenarios", len(scenarios))
}

func TestScenarioConfig_GetToolScope(t *testing.T) {
	testCases := []struct {
		tools            string
		expectFilesystem bool
		expectDefault    bool
		expectMCPTools   []string
		description      string
	}{
		{
			tools:            "filesystem, default",
			expectFilesystem: true,
			expectDefault:    true,
			expectMCPTools:   []string{},
			description:      "Both filesystem and default tools",
		},
		{
			tools:            "default",
			expectFilesystem: false,
			expectDefault:    true,
			expectMCPTools:   []string{},
			description:      "Only default tools",
		},
		{
			tools:            "filesystem",
			expectFilesystem: true,
			expectDefault:    false,
			expectMCPTools:   []string{},
			description:      "Only filesystem tools",
		},
		{
			tools:            "filesystem, default, mcp:serverA",
			expectFilesystem: true,
			expectDefault:    true,
			expectMCPTools:   []string{"serverA"},
			description:      "Filesystem, default, and MCP tools",
		},
		{
			tools:            "mcp:serverA, mcp:serverB",
			expectFilesystem: false,
			expectDefault:    false,
			expectMCPTools:   []string{"serverA", "serverB"},
			description:      "Multiple MCP tools only",
		},
		{
			tools:            "default, mcp:serverA",
			expectFilesystem: false,
			expectDefault:    true,
			expectMCPTools:   []string{"serverA"},
			description:      "Default and single MCP tool",
		},
		{
			tools:            "",
			expectFilesystem: false,
			expectDefault:    true,
			expectMCPTools:   []string{},
			description:      "Empty tools should default to default tools",
		},
		{
			tools:            "filesystem, default, todo, bash, mcp:*",
			expectFilesystem: true,
			expectDefault:    true,
			expectMCPTools:   []string{"*"},
			description:      "MCP wildcard pattern should be parsed correctly",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			config := NewScenarioConfig("test", tc.tools, "Test scenario", "Test prompt")

			scope := config.GetToolScope()

			if scope.UseFilesystem != tc.expectFilesystem {
				t.Errorf("Expected UseFilesystem=%v, got %v", tc.expectFilesystem, scope.UseFilesystem)
			}

			if scope.UseDefault != tc.expectDefault {
				t.Errorf("Expected UseDefault=%v, got %v", tc.expectDefault, scope.UseDefault)
			}

			// Check MCP tools
			if len(scope.MCPTools) != len(tc.expectMCPTools) {
				t.Errorf("Expected %d MCP tools, got %d", len(tc.expectMCPTools), len(scope.MCPTools))
			} else {
				for i, expected := range tc.expectMCPTools {
					if i < len(scope.MCPTools) && scope.MCPTools[i] != expected {
						t.Errorf("Expected MCP tool[%d]=%s, got %s", i, expected, scope.MCPTools[i])
					}
				}
			}
		})
	}
}

func TestScenarioConfig_RenderPrompt(t *testing.T) {
	config := NewScenarioConfig("test", "default", "Test scenario",
		`Test prompt with {{userInput}} and {{scenarioReason}} and {{workingDir}}`)

	rendered := config.RenderPrompt("create file", "user requested it", "/tmp")

	expected := "Test prompt with create file and user requested it and /tmp"
	if rendered != expected {
		t.Errorf("Expected: %s\nGot: %s", expected, rendered)
	}
}

func TestScenarioConfigMap_GetScenario(t *testing.T) {
	scenarios := ScenarioMap{
		"TEST_SCENARIO": NewScenarioConfig("TEST_SCENARIO", "default", "Test scenario", "Test prompt"),
	}

	// Test existing scenario
	scenario, exists := scenarios.GetScenario("TEST_SCENARIO")
	if !exists {
		t.Error("Expected to find TEST_SCENARIO")
	}
	if scenario.Name() != "TEST_SCENARIO" {
		t.Errorf("Expected name TEST_SCENARIO, got %s", scenario.Name())
	}

	// Test non-existing scenario
	_, exists = scenarios.GetScenario("NON_EXISTENT")
	if exists {
		t.Error("Expected NON_EXISTENT scenario to not exist")
	}
}

func TestScenarioConfigMap_GetAvailableScenarios(t *testing.T) {
	scenarios := ScenarioMap{
		"SCENARIO_1": NewScenarioConfig("SCENARIO_1", "default", "Test scenario 1", "Test prompt 1"),
		"SCENARIO_2": NewScenarioConfig("SCENARIO_2", "default", "Test scenario 2", "Test prompt 2"),
		"SCENARIO_3": NewScenarioConfig("SCENARIO_3", "default", "Test scenario 3", "Test prompt 3"),
	}

	available := scenarios.GetAvailableScenarios()

	if len(available) != 3 {
		t.Errorf("Expected 3 scenarios, got %d", len(available))
	}

	// Check that all expected scenarios are present
	scenarioMap := make(map[string]bool)
	for _, name := range available {
		scenarioMap[name] = true
	}

	for expectedName := range scenarios {
		if !scenarioMap[expectedName] {
			t.Errorf("Expected scenario %s not found in available list", expectedName)
		}
	}
}
