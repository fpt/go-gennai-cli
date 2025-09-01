package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fpt/go-gennai-cli/internal/infra"
	"github.com/fpt/go-gennai-cli/internal/tool"
	"github.com/fpt/go-gennai-cli/pkg/agent/state"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

func TestActionSelectionResponse_JSONSerialization(t *testing.T) {
	// Test that ActionSelectionResponse can be serialized and deserialized correctly
	original := ActionSelectionResponse{
		Action:    "INVESTIGATE_FILESYSTEM",
		Reasoning: "User wants to explore the project structure",
	}

	// Serialize
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal ActionSelectionResponse: %v", err)
	}

	// Deserialize
	var deserialized ActionSelectionResponse
	err = json.Unmarshal(jsonData, &deserialized)
	if err != nil {
		t.Fatalf("Failed to unmarshal ActionSelectionResponse: %v", err)
	}

	// Compare
	if deserialized.Action != original.Action {
		t.Errorf("Expected action %s, got %s", original.Action, deserialized.Action)
	}
	if deserialized.Reasoning != original.Reasoning {
		t.Errorf("Expected reasoning %s, got %s", original.Reasoning, deserialized.Reasoning)
	}
}

// mockLLM is a simple mock for testing
type mockLLM struct{}

func (m *mockLLM) Chat(ctx context.Context, messages []message.Message, enableThinking bool, thinkingChan chan<- string) (message.Message, error) {
	return message.NewChatMessage(message.MessageTypeAssistant, "mock response"), nil
}

// TestScenarioBasedToolSelection tests that different scenarios get different tool managers
func TestScenarioBasedToolSelection(t *testing.T) {
	// Create a mock LLM
	llmClient := &mockLLM{}

	// Create scenario runner with safe subdirectory
	workingDir := "/tmp/gennai-test"

	// Create mock scenarios for testing
	mockScenarios := make(infra.ScenarioMap)
	mockScenarios["INVESTIGATE_FILESYSTEM"] = infra.NewScenarioConfig(
		"INVESTIGATE_FILESYSTEM", "filesystem", "Filesystem investigation", "Mock prompt")
	mockScenarios["CODE"] = infra.NewScenarioConfig(
		"CODE", "filesystem", "Comprehensive coding assistant", "Mock prompt")
	mockScenarios["RESEARCH"] = infra.NewScenarioConfig(
		"RESEARCH", "default", "Research", "Mock prompt")
	mockScenarios["RESPOND"] = infra.NewScenarioConfig(
		"RESPOND", "default", "Direct response", "Mock prompt")

	// Create universal tool manager components
	todoManager := tool.NewTodoToolManager(workingDir)
	fsConfig := infra.DefaultFileSystemConfig(workingDir)
	filesystemManager := tool.NewFileSystemToolManager(fsConfig, workingDir)
	bashConfig := tool.BashConfig{WorkingDir: workingDir, MaxDuration: 120 * time.Second}
	bashManager := tool.NewBashToolManager(bashConfig)

	// Create search manager for Glob/Grep tools
	searchConfig := tool.SearchConfig{WorkingDir: workingDir}
	searchManager := tool.NewSearchToolManager(searchConfig)

	// Create universal manager (always available tools)
	universalManager := tool.NewCompositeToolManager(todoManager, filesystemManager, bashManager, searchManager)

	// Create web manager (optional for web scenarios)
	webManager := tool.NewWebToolManager()

	// Create the scenario runner with new architecture
	runner := &ScenarioRunner{
		llmClient:        llmClient,
		universalManager: universalManager,
		webToolManager:   webManager.(*tool.WebToolManager),
		sharedState:      state.NewMessageState(),
		workingDir:       workingDir,
		scenarios:        mockScenarios,
	}

	testCases := []struct {
		scenario       string
		expectWebTools bool
		description    string
	}{
		{
			scenario:       "INVESTIGATE_FILESYSTEM",
			expectWebTools: false,
			description:    "Filesystem investigation should have universal tools only",
		},
		{
			scenario:       "CODE",
			expectWebTools: false,
			description:    "Code generation should have universal tools only",
		},
		{
			scenario:       "CODE",
			expectWebTools: false,
			description:    "Code analysis should have universal tools only",
		},
		{
			scenario:       "RESEARCH",
			expectWebTools: true,
			description:    "Research should have web tools + universal tools",
		},
		{
			scenario:       "RESPOND",
			expectWebTools: true,
			description:    "Direct response should have web tools + universal tools",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.scenario, func(t *testing.T) {
			toolManager := runner.getToolManagerForScenario(tc.scenario)
			tools := toolManager.GetTools()

			// ALL scenarios should have universal tools (todos, filesystem, bash, grep)
			universalTools := []string{"todo_write", "Read", "Write", "LS", "Grep"}
			for _, toolName := range universalTools {
				if _, exists := tools[message.ToolName(toolName)]; !exists {
					t.Errorf("%s: Expected universal tool '%s' but didn't find it", tc.description, toolName)
				}
			}

			// Check for web tools (only for "default" scenarios)
			webTools := []string{"WebFetch", "WebSearch"}
			hasWebTools := false
			for _, toolName := range webTools {
				if _, exists := tools[message.ToolName(toolName)]; exists {
					hasWebTools = true
					break
				}
			}

			if tc.expectWebTools && !hasWebTools {
				t.Errorf("%s: Expected web tools but didn't find them", tc.description)
			}

			if !tc.expectWebTools && hasWebTools {
				t.Errorf("%s: Did not expect web tools but found them", tc.description)
			}

			// All scenarios should have some tools
			if len(tools) == 0 {
				t.Errorf("%s: Expected some tools but got none", tc.description)
			}
		})
	}
}

// TestCompositeToolManager tests the composite tool manager functionality
func TestCompositeToolManager(t *testing.T) {
	// Create individual tool managers with safe subdirectory
	testWorkDir := "/tmp/gennai-composite-test"
	todoManager := tool.NewTodoToolManager(testWorkDir)
	fsConfig := infra.DefaultFileSystemConfig(".")
	fsManager := tool.NewFileSystemToolManager(fsConfig, testWorkDir)
	bashConfig := tool.BashConfig{WorkingDir: testWorkDir, MaxDuration: 120 * time.Second}
	bashManager := tool.NewBashToolManager(bashConfig)

	// Create composite manager
	composite := tool.NewCompositeToolManager(todoManager, fsManager, bashManager)

	// Test that we get tools from both managers
	tools := composite.GetTools()

	// Should have tools from bash manager
	if _, exists := tools[message.ToolName("bash")]; !exists {
		t.Error("Expected to find bash tool from bash manager")
	}

	// Should have tools from filesystem manager
	if _, exists := tools[message.ToolName("Read")]; !exists {
		t.Error("Expected to find Read tool from filesystem manager")
	}

	// Should have tools from todo manager
	if _, exists := tools[message.ToolName("todo_write")]; !exists {
		t.Error("Expected to find todo_write tool from todo manager")
	}

	// Should be able to get individual tools
	if tool, exists := composite.GetTool(message.ToolName("Read")); !exists {
		t.Error("Expected to be able to get Read tool")
	} else if tool == nil {
		t.Error("Got nil tool when expecting Read")
	}
}

// TestMCPToolIntegration tests MCP tool integration with scenario configurations
func TestMCPToolIntegration(t *testing.T) {
	// Test tool scope parsing with MCP tools
	testCases := []struct {
		toolsString      string
		expectMCPTools   []string
		expectFilesystem bool
		expectDefault    bool
		description      string
	}{
		{
			toolsString:      "filesystem, default, mcp:serverA",
			expectMCPTools:   []string{"serverA"},
			expectFilesystem: true,
			expectDefault:    true,
			description:      "Filesystem, default, and MCP serverA",
		},
		{
			toolsString:      "mcp:serverA, mcp:serverB",
			expectMCPTools:   []string{"serverA", "serverB"},
			expectFilesystem: false,
			expectDefault:    false,
			description:      "Multiple MCP tools only",
		},
		{
			toolsString:      "default, mcp:serverA",
			expectMCPTools:   []string{"serverA"},
			expectFilesystem: false,
			expectDefault:    true,
			description:      "Default and single MCP tool",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Create a test scenario configuration using the constructor
			scenarioConfig := infra.NewScenarioConfig(
				"test", tc.toolsString, "Test scenario", "Test prompt")

			// Parse the tool scope
			scope := scenarioConfig.GetToolScope()

			// Verify filesystem tools
			if scope.UseFilesystem != tc.expectFilesystem {
				t.Errorf("Expected UseFilesystem=%v, got %v", tc.expectFilesystem, scope.UseFilesystem)
			}

			// Verify default tools
			if scope.UseDefault != tc.expectDefault {
				t.Errorf("Expected UseDefault=%v, got %v", tc.expectDefault, scope.UseDefault)
			}

			// Verify MCP tools
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
