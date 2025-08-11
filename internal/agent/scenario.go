package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fpt/go-gennai-cli/internal/config"
	"github.com/fpt/go-gennai-cli/internal/infra"
	"github.com/fpt/go-gennai-cli/internal/scenarios"
	"github.com/fpt/go-gennai-cli/internal/tool"
	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/agent/react"
	"github.com/fpt/go-gennai-cli/pkg/agent/state"
	"github.com/fpt/go-gennai-cli/pkg/client"
	pkgLogger "github.com/fpt/go-gennai-cli/pkg/logger"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// Default maximum iterations for scenario execution
const DefaultScenarioMaxIterations = 10

func init() {
	// Override the LoadBuiltinScenarios function to use embedded scenarios
	infra.LoadBuiltinScenariosFunc = func() (infra.ScenarioMap, error) {
		embeddedScenarios, err := scenarios.LoadBuiltinScenarios()
		if err != nil {
			return nil, err
		}

		// Convert between types
		configScenarios := make(infra.ScenarioMap)
		for name, scenario := range embeddedScenarios {
			configScenarios[name] = infra.NewScenarioConfig(
				scenario.Name,
				scenario.Tools,
				scenario.Description,
				scenario.Prompt,
			)
		}

		return configScenarios, nil
	}
}

// ActionSelectionResponse represents a single action selected for the user request
type ActionSelectionResponse struct {
	Action    string `json:"action" jsonschema:"title=Selected Action,description=The single best action to take for this request"`
	Reasoning string `json:"reasoning" jsonschema:"title=Reasoning,description=Brief explanation of why this specific action was selected,minLength=10"`
}

// ScenarioRunner handles scenario-based planning and sequential action execution
type ScenarioRunner struct {
	llmClient        domain.LLM                    // Base LLM client
	universalManager *tool.CompositeToolManager    // Universal tools (always available: todos, filesystem, bash, grep)
	todoToolManager  *tool.TodoToolManager         // Direct access to TodoToolManager for aligner
	webToolManager   *tool.WebToolManager          // Optional web tools for web scenarios
	mcpToolManagers  map[string]domain.ToolManager // MCP tool managers by name
	workingDir       string
	sharedState      domain.State      // Shared state for all agents
	scenarios        infra.ScenarioMap // Loaded YAML scenarios
	sessionFilePath  string            // Path to session state file for persistence
	settings         *config.Settings  // Application settings for configuration
	logger           *pkgLogger.Logger // Structured logger for this component
}

// NewScenarioRunner creates a new ScenarioRunner with MCP tools, settings, and additional scenario paths
func NewScenarioRunner(llmClient domain.LLM, workingDir string, mcpToolManagers map[string]domain.ToolManager, settings *config.Settings, logger *pkgLogger.Logger, additionalScenarioPaths ...string) *ScenarioRunner {
	return NewScenarioRunnerWithOptions(llmClient, workingDir, mcpToolManagers, settings, logger, false, additionalScenarioPaths...)
}

// NewScenarioRunnerWithOptions creates a new ScenarioRunner with session control options
func NewScenarioRunnerWithOptions(llmClient domain.LLM, workingDir string, mcpToolManagers map[string]domain.ToolManager, settings *config.Settings, logger *pkgLogger.Logger, skipSessionRestore bool, additionalScenarioPaths ...string) *ScenarioRunner {
	// Create individual managers for universal tool manager
	todoToolManager := tool.NewTodoToolManager(workingDir)

	fsConfig := infra.DevelopmentFileSystemConfig(workingDir)
	filesystemManager := tool.NewFileSystemToolManager(fsConfig, workingDir)

	bashConfig := tool.BashConfig{
		WorkingDir:  workingDir,
		MaxDuration: 2 * time.Minute,
	}
	bashToolManager := tool.NewBashToolManager(bashConfig)

	// Create universal tool manager (always available tools)
	universalManager := tool.NewCompositeToolManager(todoToolManager, filesystemManager, bashToolManager)

	// Create optional web tool manager for web scenarios
	webToolManager := tool.NewWebToolManager()

	// Load scenario configurations (built-in + additional)
	scenarios, err := infra.LoadScenarios(additionalScenarioPaths...)
	if err != nil {
		logger.WarnWithIcon("⚠️", "Failed to load scenarios, using empty fallback",
			"error", err, "paths", additionalScenarioPaths)
		scenarios = make(infra.ScenarioMap) // Use empty map as fallback
	}

	// Create or restore shared message state with session persistence
	sharedState := state.NewMessageState()
	var sessionFilePath string

	// Try to get session file path for persistence
	if userConfig, err := config.DefaultUserConfig(); err == nil {
		if sessionPath, err := userConfig.GetProjectSessionFile(workingDir); err == nil {
			sessionFilePath = sessionPath
			// Only restore session if not skipped (for -f flag isolation)
			if !skipSessionRestore {
				// Try to load existing session state
				if err := sharedState.LoadFromFile(sessionFilePath); err != nil {
					logger.DebugWithIcon("🔄", "Starting with new session",
						"reason", "could not load existing session", "error", err)
				} else {
					logger.InfoWithIcon("📚", "Restored session state",
						"message_count", len(sharedState.GetMessages()), "session_file", sessionFilePath)
				}
			} else {
				logger.DebugWithIcon("🔄", "Starting with clean session",
					"reason", "session restore skipped for file mode")
			}
		} else {
			logger.WarnWithIcon("⚠️", "Could not get session file path", "error", err)
		}
	} else {
		logger.WarnWithIcon("⚠️", "Could not access user config for session persistence", "error", err)
	}

	return &ScenarioRunner{
		llmClient:        llmClient,
		universalManager: universalManager,
		todoToolManager:  todoToolManager,
		webToolManager:   webToolManager.(*tool.WebToolManager),
		mcpToolManagers:  mcpToolManagers,
		workingDir:       workingDir,
		sharedState:      sharedState,
		scenarios:        scenarios,
		sessionFilePath:  sessionFilePath,
		settings:         settings,
		logger:           logger.WithComponent("scenario-runner"),
	}
}

// Invoke directly executes a specified scenario from CLI
func (s *ScenarioRunner) Invoke(ctx context.Context, userInput string, scenarioName string) (message.Message, error) {
	// Validate that the scenario exists
	if _, exists := s.scenarios[scenarioName]; !exists {
		return nil, fmt.Errorf("scenario '%s' not found", scenarioName)
	}

	fmt.Printf("📋 Using scenario: %s\n", scenarioName)

	// Execute scenario directly with CLI reasoning
	return s.executeScenario(ctx, userInput, scenarioName, "Scenario specified directly via CLI")
}

// executeScenario handles the common execution logic for both Invoke and InvokeWithScenario
func (s *ScenarioRunner) executeScenario(ctx context.Context, userInput string, scenarioName string, reasoning string) (message.Message, error) {
	// Step 1: Create scenario-specific tool manager and ReAct client
	toolManager := s.getToolManagerForScenario(scenarioName)

	// Create LLM client with scenario-specific tools
	llmWithTools, err := client.NewClientWithToolManager(s.llmClient, toolManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client with tools: %w", err)
	}

	aligner := NewScenarioAligner(s.todoToolManager) // Use scenario aligner for message alignment

	// Create ReAct client for tool calling execution with shared state
	maxIterations := DefaultScenarioMaxIterations // default fallback
	if s.settings != nil && s.settings.Agent.MaxIterations > 0 {
		maxIterations = s.settings.Agent.MaxIterations
	}
	reactClient := react.NewReAct(llmWithTools, toolManager, s.sharedState, aligner, maxIterations)

	// Step 2: Execute the scenario through ReAct
	actionResp := &ActionSelectionResponse{
		Action:    scenarioName,
		Reasoning: reasoning,
	}

	actionPrompt := s.createActionPrompt(userInput, actionResp)

	result, err := reactClient.Invoke(ctx, actionPrompt)
	if err != nil {
		return nil, fmt.Errorf("action execution failed: %w", err)
	}

	// Save session state after successful interaction (like Claude Code)
	if s.sessionFilePath != "" {
		if saveErr := s.sharedState.SaveToFile(s.sessionFilePath); saveErr != nil {
			s.logger.WarnWithIcon("⚠️", "Failed to save session state",
				"session_file", s.sessionFilePath, "error", saveErr)
		}
	}

	return result, nil
}

// ClearHistory clears the conversation history
func (s *ScenarioRunner) ClearHistory() {
	// Clear the shared state which affects all scenarios
	s.sharedState.Clear()

	// Save cleared state to session file
	if s.sessionFilePath != "" {
		if saveErr := s.sharedState.SaveToFile(s.sessionFilePath); saveErr != nil {
			s.logger.WarnWithIcon("⚠️", "Failed to save cleared session state",
				"session_file", s.sessionFilePath, "error", saveErr)
		}
	}
}

// getToolManagerForScenario returns the appropriate tool manager for a given scenario
func (s *ScenarioRunner) getToolManagerForScenario(scenario string) domain.ToolManager {
	// Universal manager is always included (todos, filesystem, bash, grep)
	managers := []domain.ToolManager{s.universalManager}

	// Check if we have YAML configuration for this scenario
	if scenarioConfig, exists := s.scenarios[scenario]; exists {
		toolScope := scenarioConfig.GetToolScope()

		// Add web tools if requested (for RESEARCH scenarios)
		if toolScope.UseDefault { // "default" in old system meant web tools
			managers = append(managers, s.webToolManager)
		}

		// Add MCP tools if requested
		for _, mcpName := range toolScope.MCPTools {
			if mcpName == "*" {
				// Wildcard: add all available MCP tool managers
				for availableMCPName, mcpManager := range s.mcpToolManagers {
					managers = append(managers, mcpManager)
					s.logger.DebugWithIcon("✅", "Added MCP tool manager (wildcard)",
						"scenario", scenario, "mcp_name", availableMCPName)
				}
			} else if mcpManager, exists := s.mcpToolManagers[mcpName]; exists {
				managers = append(managers, mcpManager)
				s.logger.DebugWithIcon("✅", "Added MCP tool manager",
					"scenario", scenario, "mcp_name", mcpName)
			} else {
				s.logger.WarnWithIcon("⚠️", "MCP tool manager not available, skipping",
					"scenario", scenario, "mcp_name", mcpName)
			}
		}

		// Return appropriate composite or single tool manager
		if len(managers) == 1 {
			// Only universal manager
			return managers[0]
		} else {
			// Universal + optional managers, create composite
			return tool.NewCompositeToolManager(managers...)
		}
	}

	// Fallback to universal manager only (todos, filesystem, bash, grep)
	return s.universalManager
}

// InvokeWithOptions creates a ReAct client with universal tools and configured maxIterations
// This method creates a temporary ReAct client with universal tools
func (s *ScenarioRunner) InvokeWithOptions(ctx context.Context, prompt string) (message.Message, error) {
	// Create temporary ReAct client with universal tools
	llmWithTools, err := client.NewClientWithToolManager(s.llmClient, s.universalManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client with tools: %w", err)
	}

	aligner := NewScenarioAligner(s.todoToolManager)

	maxIterations := DefaultScenarioMaxIterations // default fallback
	if s.settings != nil && s.settings.Agent.MaxIterations > 0 {
		maxIterations = s.settings.Agent.MaxIterations
	}
	reactClient := react.NewReAct(llmWithTools, s.universalManager, s.sharedState, aligner, maxIterations)

	return reactClient.Invoke(ctx, prompt)
}

// GetConversationPreview returns a formatted preview of the last few messages
func (s *ScenarioRunner) GetConversationPreview(maxMessages int) string {
	messages := s.sharedState.GetMessages()
	if len(messages) == 0 {
		return ""
	}

	// Get the last N messages
	startIdx := 0
	if len(messages) > maxMessages {
		startIdx = len(messages) - maxMessages
	}

	recentMessages := messages[startIdx:]

	var preview strings.Builder
	preview.WriteString("📚 Previous Conversation:\n")
	preview.WriteString(strings.Repeat("─", 50) + "\n")

	isFirstMessage := true
	for _, msg := range recentMessages {
		truncated := msg.TruncatedString()
		if truncated == "" {
			continue // Skip empty messages (like system messages)
		}

		// Add spacing between messages
		if !isFirstMessage {
			preview.WriteString("\n")
		}
		isFirstMessage = false

		// Use the message's TruncatedString method for clean formatting
		preview.WriteString(truncated + "\n")
	}

	preview.WriteString(strings.Repeat("─", 50) + "\n")
	return preview.String()
}

// PrintPhaseModels displays scenario runner configuration
func (s *ScenarioRunner) PrintPhaseModels() {
	fmt.Printf("\n🤖 Scenario Runner Configuration\n")
	fmt.Printf("==============================\n")
	fmt.Printf("Pattern: Scenario Planning -> Sequential Action Execution\n")
	fmt.Printf("Working Directory: %s\n", s.workingDir)
	fmt.Printf("==============================\n\n")
}

// createActionPrompt creates a detailed prompt for the selected action using YAML configurations
func (s *ScenarioRunner) createActionPrompt(userInput string, actionResp *ActionSelectionResponse) string {
	workingDir := s.workingDir
	if workingDir == "" {
		workingDir = "current directory"
	}

	// Get current todos for prompt injection (Claude Code-style)
	var todosContext string
	if s.todoToolManager != nil {
		todosContext = s.todoToolManager.GetTodosForPrompt()
	}

	// Use YAML configuration for this scenario
	if scenarioConfig, exists := s.scenarios[actionResp.Action]; exists {
		basePrompt := scenarioConfig.RenderPrompt(userInput, actionResp.Reasoning, workingDir)

		// Inject todos into prompt context if available
		if todosContext != "" {
			return fmt.Sprintf("%s\n\n## Current Todos:\n%s\n\nUse TodoWrite tool to update todos as you progress.", basePrompt, todosContext)
		}
		return basePrompt
	}

	// Should not reach here if YAML scenarios are complete
	return fmt.Sprintf("Error: No scenario configuration found for %s", actionResp.Action)
}
