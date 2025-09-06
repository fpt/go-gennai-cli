package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	pkgErrors "github.com/pkg/errors"

	"github.com/manifoldco/promptui"

	"github.com/fpt/go-gennai-cli/internal/config"
	"github.com/fpt/go-gennai-cli/internal/infra"
	"github.com/fpt/go-gennai-cli/internal/repository"
	"github.com/fpt/go-gennai-cli/internal/scenarios"
	"github.com/fpt/go-gennai-cli/internal/tool"
	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/agent/events"
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
	llmClient        domain.LLM                      // Base LLM client
	universalManager *tool.CompositeToolManager      // Universal tools (always available: todos, filesystem, bash, grep)
	todoToolManager  *tool.TodoToolManager           // Direct access to TodoToolManager for aligner
	webToolManager   *tool.WebToolManager            // Optional web tools for web scenarios
	mcpToolManagers  map[string]domain.ToolManager   // MCP tool managers by name
	fsRepo           repository.FilesystemRepository // Shared filesystem repository instance
	workingDir       string
	sharedState      domain.State      // Shared state for all agents
	scenarios        infra.ScenarioMap // Loaded YAML scenarios
	sessionFilePath  string            // Path to session state file for persistence
	settings         *config.Settings  // Application settings for configuration
	logger           *pkgLogger.Logger // Structured logger for this component
	out              io.Writer         // Output writer for streaming/printing
	thinkingStarted  bool              // Track if thinking has started for emoji handling
	alwaysApprove    bool              // Track if user selected "Always" approve for this session
}

// WorkingDir returns the scenario runner's working directory
func (s *ScenarioRunner) WorkingDir() string { return s.workingDir }

// FilesystemRepository returns the shared filesystem repository instance
func (s *ScenarioRunner) FilesystemRepository() repository.FilesystemRepository { return s.fsRepo }

// NewScenarioRunner creates a new ScenarioRunner with MCP tools, settings, and additional scenario paths
func NewScenarioRunner(llmClient domain.LLM, workingDir string, mcpToolManagers map[string]domain.ToolManager, settings *config.Settings, logger *pkgLogger.Logger, out io.Writer, additionalScenarioPaths ...string) *ScenarioRunner {
	fsRepo := infra.NewOSFilesystemRepository()
	return NewScenarioRunnerWithOptions(llmClient, workingDir, mcpToolManagers, settings, logger, out, false, true, fsRepo, additionalScenarioPaths...)
}

// NewScenarioRunnerWithOptions creates a new ScenarioRunner with session control options
func NewScenarioRunnerWithOptions(llmClient domain.LLM, workingDir string, mcpToolManagers map[string]domain.ToolManager, settings *config.Settings, logger *pkgLogger.Logger, out io.Writer, skipSessionRestore bool, isInteractiveMode bool, fsRepo repository.FilesystemRepository, additionalScenarioPaths ...string) *ScenarioRunner {
	// Create individual managers for universal tool manager
	// Only create persistent todo manager in interactive mode
	var todoToolManager *tool.TodoToolManager
	alwaysApprove := false
	if isInteractiveMode {
		todoToolManager = tool.NewTodoToolManager(workingDir)
	} else {
		// For one-shot mode, create an in-memory-only todo manager
		todoToolManager = tool.NewInMemoryTodoToolManager()
		// Auto-approve in one-shot mode
		alwaysApprove = true
	}

	fsConfig := infra.DefaultFileSystemConfig(workingDir)
	filesystemManager := tool.NewFileSystemToolManager(fsRepo, fsConfig, workingDir)

	bashConfig := tool.BashConfig{
		WorkingDir:  workingDir,
		MaxDuration: 2 * time.Minute,
	}
	bashToolManager := tool.NewBashToolManager(bashConfig)

	// Create search tool manager (Glob/Grep)
	searchToolManager := tool.NewSearchToolManager(tool.SearchConfig{WorkingDir: workingDir})

	// Create universal tool manager (always available tools)
	universalManager := tool.NewCompositeToolManager(todoToolManager, filesystemManager, bashToolManager, searchToolManager)

	// Create optional web tool manager for web scenarios
	webToolManager := tool.NewWebToolManager()

	// Load scenario configurations (built-in + additional)
	scenarios, err := infra.LoadScenarios(additionalScenarioPaths...)
	if err != nil {
		logger.Warn("Failed to load scenarios, using empty fallback",
			"error", err, "paths", additionalScenarioPaths)
		scenarios = make(infra.ScenarioMap) // Use empty map as fallback
	}

	// Create or restore shared message state with session persistence
	var sharedState domain.State
	var sessionFilePath string

	// Only handle session persistence in interactive mode
	if isInteractiveMode {
		// Try to get session file path for persistence
		if userConfig, err := config.DefaultUserConfig(); err == nil {
			if sessionPath, err := userConfig.GetProjectSessionFile(workingDir); err == nil {
				sessionFilePath = sessionPath
				// Create repository and inject it into MessageState
				messageRepo := infra.NewMessageHistoryRepository(sessionFilePath)
				sharedState = state.NewMessageStateWithRepository(messageRepo)

				// Only restore session if not skipped (for -f flag isolation)
				if !skipSessionRestore {
					// Try to load existing session state
					if err := sharedState.LoadFromFile(); err != nil {
						logger.DebugWithIntention(pkgLogger.IntentionStatus, "Starting with new session",
							"reason", "could not load existing session", "error", err)
					} else {
						logger.DebugWithIntention(pkgLogger.IntentionStatus, "Restored session state",
							"message_count", len(sharedState.GetMessages()), "session_file", sessionFilePath)
					}
				} else {
					logger.DebugWithIntention(pkgLogger.IntentionStatus, "Starting with clean session",
						"reason", "session restore skipped for file mode")
				}
			} else {
				logger.Warn("Could not get session file path", "error", err)
				// Fallback to in-memory state
				sharedState = state.NewMessageState()
			}
		} else {
			logger.Warn("Could not access user config for session persistence", "error", err)
			// Fallback to in-memory state
			sharedState = state.NewMessageState()
		}
	} else {
		// One-shot mode: no session persistence, always start clean
		sharedState = state.NewMessageState()
		logger.DebugWithIntention(pkgLogger.IntentionStatus, "Starting with clean session", "reason", "one-shot mode")
	}

	return &ScenarioRunner{
		llmClient:        llmClient,
		universalManager: universalManager,
		todoToolManager:  todoToolManager,
		webToolManager:   webToolManager.(*tool.WebToolManager),
		mcpToolManagers:  mcpToolManagers,
		fsRepo:           fsRepo,
		workingDir:       workingDir,
		sharedState:      sharedState,
		scenarios:        scenarios,
		sessionFilePath:  sessionFilePath,
		settings:         settings,
		logger:           logger.WithComponent("scenario-runner"),
		out:              out,
		alwaysApprove:    alwaysApprove,
	}
}

// Invoke directly executes a specified scenario from CLI
func (s *ScenarioRunner) Invoke(ctx context.Context, userInput string, scenarioName string) (message.Message, error) {
	// Validate that the scenario exists
	if _, exists := s.scenarios[scenarioName]; !exists {
		return nil, fmt.Errorf("scenario '%s' not found", scenarioName)
	}

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
	// Create ReAct client which returns its own event emitter, then set up event handlers
	reactClient, eventEmitter := react.NewReAct(llmWithTools, toolManager, s.sharedState, aligner, maxIterations)
	s.setupEventHandlers(eventEmitter)

	// Step 2: Execute the scenario through ReAct
	actionResp := &ActionSelectionResponse{
		Action:    scenarioName,
		Reasoning: reasoning,
	}

	// Prepare a stable system prompt (scenario header) and insert only when changed
	// Render with empty userInput so the header remains stable across turns;
	// include workingDir and reasoning so those parts remain accurate.
	if scenarioConfig, exists := s.scenarios[actionResp.Action]; exists {
		systemPrompt := scenarioConfig.RenderPrompt("", actionResp.Reasoning, s.workingDir)
		if systemPrompt != "" {
			// Use a discoverable marker so we can detect previous insertion
			marker := fmt.Sprintf("[[SCENARIO_PROMPT:%s]]\n", actionResp.Action)
			candidate := marker + systemPrompt

			// Find the most recent matching marker message
			var lastMatched string
			for _, msg := range s.sharedState.GetMessages() {
				if msg.Type() == message.MessageTypeSystem && strings.HasPrefix(msg.Content(), marker) {
					lastMatched = msg.Content()
				}
			}

			if lastMatched == "" || lastMatched != candidate {
				s.sharedState.AddMessage(message.NewSystemMessage(candidate))
			}
		}
	}

	// Build the user-facing prompt content (raw request + current todos)
	userPrompt := userInput
	if s.todoToolManager != nil {
		if todosContext := s.todoToolManager.GetTodosForPrompt(); todosContext != "" {
			userPrompt = fmt.Sprintf("%s\n\n## Current Todos:\n%s\n\nUse TodoWrite tool to update todos as you progress.", userPrompt, todosContext)
		}
	}

	// Expand line-based includes in the user prompt: lines starting with @filename
	if strings.Contains(userPrompt, "@") {
		lines := strings.Split(userPrompt, "\n")
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "@") {
				rel := strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
				if rel == "" {
					// Drop empty includes
					continue
				}
				var fullPath string
				if filepath.IsAbs(rel) {
					fullPath = rel
				} else {
					fullPath = filepath.Join(s.workingDir, rel)
				}
				if data, err := os.ReadFile(fullPath); err == nil {
					out = append(out,
						"----- BEGIN "+rel+" -----",
						string(data),
						"----- END "+rel+" -----",
					)
					continue
				}
				// Unreadable file; drop directive
				continue
			}
			out = append(out, line)
		}
		userPrompt = strings.Join(out, "\n")
	}

	result, err := reactClient.Run(ctx, userPrompt)

	// Handle multiple approval workflows in sequence
	var approvalErrors []error
	for err != nil && pkgErrors.Is(err, react.ErrWaitingForApproval) {
		result, err = s.handleApprovalWorkflow(ctx, reactClient)
		if err != nil && !pkgErrors.Is(err, react.ErrWaitingForApproval) {
			approvalErrors = append(approvalErrors, err)
		}
	}

	if err != nil {
		if len(approvalErrors) > 0 {
			return nil, fmt.Errorf("action execution failed: %w", errors.Join(append(approvalErrors, err)...))
		}
		return nil, fmt.Errorf("action execution failed: %w", err)
	}
	defer reactClient.Close()

	// Save session state after successful interaction
	if s.sessionFilePath != "" {
		if saveErr := s.sharedState.SaveToFile(); saveErr != nil {
			s.logger.Warn("Failed to save session state",
				"session_file", s.sessionFilePath, "error", saveErr)
		}
	}

	return result, nil
}

// handleApprovalWorkflow handles the write confirmation workflow when the agent is waiting for approval
func (s *ScenarioRunner) handleApprovalWorkflow(ctx context.Context, reactClient domain.ReAct) (message.Message, error) {
	writer := s.OutWriter()

	// If "Always" was previously selected, auto-approve
	if s.alwaysApprove {
		fmt.Fprintf(writer, "✅ Proceeding (Always selected)...\n\n")
		return reactClient.Resume(ctx)
	}

	// Get the pending tool call details
	lastMessage := reactClient.GetLastMessage()

	// Check if we can interact with the user (has a proper terminal)
	stat, err := os.Stdin.Stat()
	if err != nil || (stat.Mode()&os.ModeCharDevice) == 0 {
		// Not interactive mode - auto-approve
		fmt.Fprintf(writer, "\n📝 About to write file(s):\n")
		fmt.Fprintf(writer, "📋 %s\n", lastMessage.TruncatedString())
		fmt.Fprintf(writer, "✅ Proceeding (non-interactive mode)...\n\n")
		return reactClient.Resume(ctx)
	}

	// Display the pending action
	fmt.Fprintf(writer, "\n📝 About to write file(s):\n")
	fmt.Fprintf(writer, "📋 %s\n\n", lastMessage.TruncatedString())

	// Create promptui select with horizontal-style options
	prompt := promptui.Select{
		Label: "Proceed with this action?",
		Items: []string{"Yes", "Always", "No"},
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "▶ {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "{{ \"✓\" | green }} {{ . }}",
		},
		Size: 3,
	}

	_, result, err := prompt.Run()
	if err != nil {
		// If promptui fails, fall back to auto-approve
		fmt.Fprintf(writer, "✅ Input error, proceeding...\n\n")
		return reactClient.Resume(ctx)
	}

	switch result {
	case "Yes":
		fmt.Fprintf(writer, "✅ Proceeding...\n\n")
		return reactClient.Resume(ctx)

	case "Always":
		s.alwaysApprove = true // Set flag for this session
		fmt.Fprintf(writer, "✅ Proceeding (will auto-approve future file operations this session)...\n\n")
		return reactClient.Resume(ctx)

	case "No":
		fmt.Fprintf(writer, "⏸️  Cancelled.\n")

		// Cancel the pending tool call (this will add declined result message and reset status)
		reactClient.CancelPendingToolCall()
		return reactClient.Resume(ctx)

	default:
		// Fallback - should not happen
		fmt.Fprintf(writer, "✅ Proceeding...\n\n")
		return reactClient.Resume(ctx)
	}
}

// ClearHistory clears the conversation history
func (s *ScenarioRunner) ClearHistory() {
	// Clear the shared state which affects all scenarios
	s.sharedState.Clear()

	// Save cleared state to session file
	if s.sessionFilePath != "" {
		if saveErr := s.sharedState.SaveToFile(); saveErr != nil {
			s.logger.Warn("Failed to save cleared session state",
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
					s.logger.DebugWithIntention(pkgLogger.IntentionSuccess, "Added MCP tool manager (wildcard)",
						"scenario", scenario, "mcp_name", availableMCPName)
				}
			} else if mcpManager, exists := s.mcpToolManagers[mcpName]; exists {
				managers = append(managers, mcpManager)
				s.logger.DebugWithIntention(pkgLogger.IntentionSuccess, "Added MCP tool manager",
					"scenario", scenario, "mcp_name", mcpName)
			} else {
				s.logger.Warn("MCP tool manager not available, skipping",
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
	// Create ReAct client which returns its own event emitter, then set up event handlers
	reactClient, eventEmitter := react.NewReAct(llmWithTools, s.universalManager, s.sharedState, aligner, maxIterations)
	s.setupEventHandlers(eventEmitter)

	result, err := reactClient.Run(ctx, prompt)

	// Handle multiple approval workflows in sequence
	var approvalErrors []error
	for err != nil && pkgErrors.Is(err, react.ErrWaitingForApproval) {
		result, err = s.handleApprovalWorkflow(ctx, reactClient)
		if err != nil && !pkgErrors.Is(err, react.ErrWaitingForApproval) {
			approvalErrors = append(approvalErrors, err)
		}
	}

	if err != nil {
		if len(approvalErrors) > 0 {
			return nil, errors.Join(append(approvalErrors, err)...)
		}
		return nil, err
	}

	return result, err
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

// GetMessageState returns the shared message state for context calculations
func (s *ScenarioRunner) GetMessageState() domain.State {
	return s.sharedState
}

// GetLLMClient returns the LLM client for context window estimation
func (s *ScenarioRunner) GetLLMClient() domain.LLM {
	return s.llmClient
}

// OutWriter returns the output writer used for streaming thinking/log lines
func (s *ScenarioRunner) OutWriter() io.Writer {
	if s.out != nil {
		return s.out
	}
	return os.Stdout
}

// setupEventHandlers configures event handlers to convert events back to output format
func (s *ScenarioRunner) setupEventHandlers(emitter events.EventEmitter) {
	emitter.AddHandler(func(event events.AgentEvent) {
		writer := s.OutWriter()
		if writer == nil {
			return
		}

		switch event.Type {
		case events.EventTypeToolCallStart:
			if data, ok := event.Data.(events.ToolCallStartData); ok {
				fmt.Fprintf(writer, "🔧 Running tool %s %v\n", data.ToolName, data.Arguments)
			}

		case events.EventTypeToolResult:
			if data, ok := event.Data.(events.ToolResultData); ok {
				if data.Content == "" {
					fmt.Fprintln(writer, "↳ (no output)")
				} else if data.IsError {
					// Show error messages with error icon
					lines := strings.Split(data.Content, "\n")
					for _, line := range lines {
						fmt.Fprintf(writer, "❌ %s\n", line)
					}
				} else {
					// Show successful results with arrow
					lines := strings.Split(data.Content, "\n")
					maxLines := 5
					if len(lines) > maxLines {
						fmt.Fprintf(writer, "↳ ...(%d more lines)\n", len(lines)-maxLines)
						lines = lines[len(lines)-maxLines:]
					}
					for _, line := range lines {
						if len(line) > 80 {
							line = line[:77] + "..."
						}
						fmt.Fprintf(writer, "↳ %s\n", line)
					}
				}
			}

		case events.EventTypeThinkingChunk:
			if data, ok := event.Data.(events.ThinkingChunkData); ok {
				// First content triggers header
				if !s.thinkingStarted {
					fmt.Fprint(writer, "\x1b[90m💭 ") // Gray thinking emoji
					s.thinkingStarted = true
				}
				// Print content in gray without reset
				fmt.Fprintf(writer, "\x1b[90m%s", data.Content)
			}

		case events.EventTypeResponse:
			// Reset thinking state when response is complete
			if s.thinkingStarted {
				fmt.Fprint(writer, "\x1b[0m\n") // Reset color and newline
				s.thinkingStarted = false
			}

		case events.EventTypeError:
			if data, ok := event.Data.(events.ErrorData); ok {
				fmt.Fprintf(writer, "❌ Error: %v\n", data.Error)
			}
		}
	})
}
