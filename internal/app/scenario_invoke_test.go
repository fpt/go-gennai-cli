package app

import (
	"context"
	"testing"
	"time"

	"github.com/fpt/go-gennai-cli/internal/infra"
	"github.com/fpt/go-gennai-cli/internal/tool"
	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/agent/state"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// mockToolCallingLLM is a mock LLM that implements ToolCallingLLM interface
type mockToolCallingLLM struct {
	toolManager domain.ToolManager
}

func (m *mockToolCallingLLM) Chat(ctx context.Context, messages []message.Message, enableThinking bool, thinkingChan chan<- string) (message.Message, error) {
	return message.NewChatMessage(message.MessageTypeAssistant, "mock response"), nil
}

func (m *mockToolCallingLLM) ChatWithToolChoice(ctx context.Context, messages []message.Message, toolChoice domain.ToolChoice, enableThinking bool, thinkingChan chan<- string) (message.Message, error) {
	return message.NewChatMessage(message.MessageTypeAssistant, "mock response"), nil
}

func (m *mockToolCallingLLM) SetToolManager(toolManager domain.ToolManager) {
	m.toolManager = toolManager
}

func (m *mockToolCallingLLM) GetToolManager() domain.ToolManager {
	return m.toolManager
}

// TestInvokeWithScenario tests the new direct scenario invocation method
func TestInvokeWithScenario(t *testing.T) {
	// Create a mock LLM
	llmClient := &mockToolCallingLLM{}

	// Create mock scenarios for testing
	mockScenarios := make(infra.ScenarioMap)
	mockScenarios["CODE"] = infra.NewScenarioConfig(
		"CODE", "filesystem, default", "Comprehensive coding assistant", "Test prompt: {{userInput}}")
	mockScenarios["RESEARCH"] = infra.NewScenarioConfig(
		"RESEARCH", "default", "Research", "Research prompt: {{userInput}}")

	// Create scenario runner with mock scenarios
	workingDir := "/tmp/test"
	todoManager := tool.NewTodoToolManager(workingDir)
	fsConfig := infra.DefaultFileSystemConfig(workingDir)
	filesystemManager := tool.NewFileSystemToolManager(fsConfig, workingDir)
	bashConfig := tool.BashConfig{WorkingDir: workingDir, MaxDuration: 120 * time.Second}
	bashManager := tool.NewBashToolManager(bashConfig)
	universalManager := tool.NewCompositeToolManager(todoManager, filesystemManager, bashManager)
	webManager := tool.NewWebToolManager()

	runner := &ScenarioRunner{
		llmClient:        llmClient,
		universalManager: universalManager,
		todoToolManager:  todoManager,
		webToolManager:   webManager.(*tool.WebToolManager),
		sharedState:      state.NewMessageState(),
		workingDir:       workingDir,
		scenarios:        mockScenarios,
		sessionFilePath:  "", // No session persistence in tests
	}

	t.Run("Valid scenario", func(t *testing.T) {
		ctx := context.Background()
		userInput := "Create a hello world function"

		// This should work without error
		result, err := runner.Invoke(ctx, userInput, "CODE")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if result == nil {
			t.Fatal("Expected result, got nil")
		}

		// Verify it's the mock response
		if result.Content() != "mock response" {
			t.Errorf("Expected 'mock response', got '%s'", result.Content())
		}
	})

	t.Run("Invalid scenario", func(t *testing.T) {
		ctx := context.Background()
		userInput := "Create a hello world function"

		// This should fail with scenario not found error
		_, err := runner.Invoke(ctx, userInput, "INVALID_SCENARIO")
		if err == nil {
			t.Fatal("Expected error for invalid scenario, got nil")
		}

		expectedError := "scenario 'INVALID_SCENARIO' not found"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("Different scenarios use different tools", func(t *testing.T) {
		ctx := context.Background()
		userInput := "Test input"

		// Test CODE scenario (should have universal + web tools)
		_, err := runner.Invoke(ctx, userInput, "CODE")
		if err != nil {
			t.Fatalf("Expected no error for CODE scenario, got: %v", err)
		}

		// Test RESEARCH scenario (should have web tools only)
		_, err = runner.Invoke(ctx, userInput, "RESEARCH")
		if err != nil {
			t.Fatalf("Expected no error for RESEARCH scenario, got: %v", err)
		}
	})
}
