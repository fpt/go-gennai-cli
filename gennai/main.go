package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/fpt/go-gennai-cli/internal/app"
	"github.com/fpt/go-gennai-cli/internal/config"
	"github.com/fpt/go-gennai-cli/internal/mcp"
	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/client/anthropic"
	"github.com/fpt/go-gennai-cli/pkg/client/gemini"
	"github.com/fpt/go-gennai-cli/pkg/client/ollama"
	"github.com/fpt/go-gennai-cli/pkg/client/openai"
	pkgLogger "github.com/fpt/go-gennai-cli/pkg/logger"
	"github.com/fpt/go-gennai-cli/pkg/message"
	"github.com/manifoldco/promptui"
)

// scenarioPathsFlag implements flag.Value for handling multiple scenario paths
type scenarioPathsFlag []string

func (s *scenarioPathsFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *scenarioPathsFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// resolveStringFlag returns the non-empty value, preferring short flag over long flag
func resolveStringFlag(shortVal, longVal string) string {
	if shortVal != "" {
		return shortVal
	}
	return longVal
}

func printUsage() {
	fmt.Println("gennai - AI-powered coding agent with scenario-based tool management")
	fmt.Println()
	fmt.Println("Available Scenarios (case-insensitive):")
	fmt.Println("  code                    Comprehensive coding assistant (generation + analysis + debug + test + refactor)")
	fmt.Println("  research                Web research, information gathering, and visual web analysis")
	fmt.Println("  respond                 Basic knowledge-based responses")
	fmt.Println("  filesystem              File system exploration and operations")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  gennai                                    # Interactive mode (code scenario)")
	fmt.Println("  gennai \"Create a HTTP server\"             # One-shot mode (code scenario)")
	fmt.Println("  gennai -s research \"Go best practices\"    # Research scenario")
	fmt.Println("  gennai -s code \"Fix compilation errors\"   # Code scenario")
	fmt.Println("  gennai -b anthropic \"Analyze this code\"  # Use Anthropic backend")
	fmt.Println("  gennai -f prompts.txt                     # Multi-turn from file (no memory)")
	fmt.Println("  gennai --custom-scenarios ./my.yaml      # Use additional scenarios")
	fmt.Println("  gennai -v \"Debug this issue\"             # Enable verbose debug logging")
	fmt.Println("  gennai -l                                # Show conversation history")
	fmt.Println()
}

func main() {
	ctx := context.Background()

	// Define command line flags
	var backend = flag.String("b", "", "LLM backend (ollama, anthropic, openai, or gemini)")
	var backendLong = flag.String("backend", "", "LLM backend (ollama, anthropic, openai, or gemini)")
	var model = flag.String("m", "", "Model name to use")
	var modelLong = flag.String("model", "", "Model name to use")
	var workdir = flag.String("workdir", "", "Working directory")
	var settingsPath = flag.String("settings", "", "Path to settings file")
	var scenario = flag.String("s", "code", "Scenario to use (default: code)")
	var scenarioLong = flag.String("scenario", "code", "Scenario to use (default: code)")
	var scenarioPaths scenarioPathsFlag
	var showLog = flag.Bool("l", false, "Print conversation message history and exit")
	var showLogLong = flag.Bool("log", false, "Print conversation message history and exit")
	var promptFile = flag.String("f", "", "File containing multi-turn prompts separated by '----' (no memory between turns)")
	var verbose = flag.Bool("v", false, "Enable verbose logging (debug level)")
	var verboseLong = flag.Bool("verbose", false, "Enable verbose logging (debug level)")
	var help = flag.Bool("h", false, "Show this help message")
	var helpLong = flag.Bool("help", false, "Show this help message")

	// Custom flag for multiple scenario paths
	flag.Var(&scenarioPaths, "custom-scenarios", "Additional scenario file or directory (can be used multiple times)")

	// Custom usage function
	flag.Usage = func() {
		printUsage()
		fmt.Println("Flags:")
		flag.PrintDefaults()
	}

	// Parse flags
	flag.Parse()

	// Handle help flag
	if *help || *helpLong {
		flag.Usage()
		return
	}

	// Resolve long/short flag conflicts (prefer the one that was set)
	resolvedBackend := resolveStringFlag(*backend, *backendLong)
	resolvedModel := resolveStringFlag(*model, *modelLong)
	resolvedScenario := resolveStringFlag(*scenario, *scenarioLong)
	resolvedShowLog := *showLog || *showLogLong
	resolvedVerbose := *verbose || *verboseLong

	// Get remaining arguments as the command
	args := flag.Args()

	// Load settings
	settings, err := config.LoadSettings(*settingsPath)
	if err != nil {
		fmt.Printf("⚠️  Warning: failed to load settings: %v\n", err)
		settings = config.GetDefaultSettings()
	}

	// Initialize structured logger based on settings
	// Override log level to debug if verbose flag is set
	logLevel := settings.Agent.LogLevel
	if resolvedVerbose {
		logLevel = "debug"
	}
	// Update global logger level so all component loggers use the new level
	pkgLogger.SetGlobalLogLevel(pkgLogger.LogLevel(logLevel))
	logger := pkgLogger.NewLogger(pkgLogger.LogLevel(logLevel))

	// Log the current log level for debugging
	if resolvedVerbose {
		logger.DebugWithIcon("📊", "Verbose logging enabled", "log_level", logLevel)
	} else {
		logger.InfoWithIcon("📊", "Standard logging enabled", "log_level", logLevel)
	}

	// Convert scenario to uppercase for case-insensitive matching with YAML files
	internalScenario := strings.ToUpper(resolvedScenario)

	// Override settings with command line arguments
	if resolvedBackend != "" {
		settings.LLM.Backend = resolvedBackend
	}
	if resolvedModel != "" {
		settings.LLM.Model = resolvedModel
	}

	// Validate settings
	if err := config.ValidateSettings(settings); err != nil {
		logger.ErrorWithIcon("❌", "Settings validation failed", "error", err)
		os.Exit(1)
	}

	// Create LLM client based on settings
	var llmClient domain.LLM
	switch settings.LLM.Backend {
	case "anthropic", "claude":
		llmClient, err = anthropic.NewAnthropicClientWithTokens(settings.LLM.Model, settings.LLM.MaxTokens)
		if err != nil {
			logger.ErrorWithIcon("❌", "Failed to create Anthropic client", "error", err)
			os.Exit(1)
		}
	case "openai":
		llmClient, err = openai.NewOpenAIClient(settings.LLM.Model, settings.LLM.MaxTokens)
		if err != nil {
			logger.ErrorWithIcon("❌", "Failed to create OpenAI client", "error", err)
			os.Exit(1)
		}
	case "gemini":
		llmClient, err = gemini.NewGeminiClientWithTokens(settings.LLM.Model, settings.LLM.MaxTokens)
		if err != nil {
			logger.ErrorWithIcon("❌", "Failed to create Gemini client", "error", err)
			os.Exit(1)
		}
	default:
		// For Ollama, check if model is in known list, if not, test capability
		if !ollama.IsModelInKnownList(settings.LLM.Model) {
			logger.WarnWithIcon("⚠️", "Model not in known capabilities list, testing tool calling capability",
				"model", settings.LLM.Model)

			// Test model capability with a 30-second timeout
			testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			hasToolCapability, testErr := ollama.DynamicCapabilityCheck(testCtx, settings.LLM.Model, false)
			if testErr != nil {
				logger.WarnWithIcon("⚠️", "Failed to test model capability, proceeding without tool support",
					"model", settings.LLM.Model, "error", testErr)
			} else if !hasToolCapability {
				logger.WarnWithIcon("⚠️", "Model does not support tool calling - limited functionality",
					"model", settings.LLM.Model, "suggestion", "consider using 'gpt-oss:latest'")
			} else {
				logger.InfoWithIcon("✅", "Model supports tool calling, proceeding with full functionality",
					"model", settings.LLM.Model)
			}
		}

		llmClient, err = ollama.NewOllamaClient(settings.LLM.Model, settings.LLM.MaxTokens, settings.LLM.Thinking)
		if err != nil {
			logger.ErrorWithIcon("❌", "Failed to create Ollama client", "error", err)
			os.Exit(1)
		}
	}

	// Determine working directory (don't change process cwd, just pass to tools)
	workingDirectory := *workdir
	if workingDirectory != "" {
		// Validate that the directory exists
		if _, err := os.Stat(workingDirectory); err != nil {
			logger.ErrorWithIcon("❌", "Working directory does not exist",
				"directory", workingDirectory, "error", err)
			os.Exit(1)
		}
		fmt.Printf("Working directory: %s\n", workingDirectory)
	} else {
		workingDirectory = "." // current directory
	}

	// Initialize MCP integration if any servers are enabled
	var mcpIntegration *mcp.Integration
	if hasEnabledMCPServers(settings.MCP.Servers) {
		fmt.Println("🔌 Initializing MCP Integration...")
		mcpIntegration = initializeMCP(ctx, settings.MCP, logger)
		if mcpIntegration != nil {
			defer mcpIntegration.Close()
		}
	}

	// Initialize the scenario runner with optional MCP tool manager and additional scenarios
	var a *app.ScenarioRunner
	// Skip session restoration for file mode (-f flag) to ensure clean isolated tests
	skipSessionRestore := (*promptFile != "")
	// Determine if we're in interactive mode (affects project directory usage)
	isInteractiveMode := len(args) == 0 && *promptFile == ""

	if mcpIntegration != nil {
		toolManager := mcpIntegration.GetToolManager()
		mcpToolManagers := map[string]domain.ToolManager{}

		// Register each connected MCP server by name for scenario configs
		serverNames := mcpIntegration.ListServers()
		for _, serverName := range serverNames {
			mcpToolManagers[serverName] = toolManager
		}

		a = app.NewScenarioRunnerWithOptions(llmClient, workingDirectory, mcpToolManagers, settings, logger, skipSessionRestore, isInteractiveMode, scenarioPaths...)
		stats := mcpIntegration.GetStats()
		fmt.Printf("✅ MCP Integration: %d servers connected, %d tools available\n", stats.ConnectedServers, stats.TotalTools)

		// Debug: List all available tools from the tool manager
		allTools := toolManager.GetTools()
		fmt.Printf("🔧 Debug: Tool Manager has %d tools loaded\n", len(allTools))
	} else {
		mcpToolManagers := make(map[string]domain.ToolManager)
		a = app.NewScenarioRunnerWithOptions(llmClient, workingDirectory, mcpToolManagers, settings, logger, skipSessionRestore, isInteractiveMode, scenarioPaths...)

		// Note: SimpleToolManager removed - tools now managed by specialized managers
	}

	// Report scenario loading
	if len(scenarioPaths) > 0 {
		fmt.Printf("📋 Additional scenarios loaded from: %v\n", scenarioPaths)
	} else {
		fmt.Printf("📋 Using built-in scenarios (use --scenarios to add custom scenarios)\n")
	}

	// Handle special command line options
	if resolvedShowLog {
		// Print conversation history and exit
		conversationHistory := a.GetConversationPreview(1000) // Get full history
		if conversationHistory != "" {
			fmt.Println("📜 Conversation History:")
			fmt.Println(strings.Repeat("=", 60))
			fmt.Print(conversationHistory)
			fmt.Println(strings.Repeat("=", 60))
		} else {
			fmt.Println("📜 No conversation history found.")
		}
		return
	}

	// Show initial configuration
	a.PrintPhaseModels()

	// Show which scenario is being used
	fmt.Printf("📋 Using scenario: %s (%s)\n", resolvedScenario, internalScenario)

	// Handle multi-turn prompt file if specified
	if *promptFile != "" {
		executeMultiTurnFile(ctx, a, *promptFile, internalScenario)
		return
	}

	// Determine if we should run in interactive mode or one-shot mode
	if len(args) > 0 {
		// One-shot mode: execute single command and exit
		userInput := strings.Join(args, " ")
		executeCommand(ctx, a, userInput, internalScenario)
	} else {
		// Interactive mode: start REPL
		startInteractiveMode(ctx, a, internalScenario)
	}
}

func executeCommand(ctx context.Context, a *app.ScenarioRunner, userInput string, scenario string) {
	fmt.Print("\n")

	var response message.Message
	var err error

	// Always use the CLI-specified scenario (defaults to "coding")
	response, err = a.Invoke(ctx, userInput, scenario)

	if err != nil {
		fmt.Printf("❌ Command execution failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Response:\n%+v\n", response.Content())
}

func executeMultiTurnFile(ctx context.Context, a *app.ScenarioRunner, filePath string, scenario string) {
	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("❌ Failed to read prompt file '%s': %v\n", filePath, err)
		os.Exit(1)
	}

	// Split prompts by "----" separator
	prompts := strings.Split(string(content), "----")

	if len(prompts) == 0 {
		fmt.Printf("❌ No prompts found in file '%s'\n", filePath)
		os.Exit(1)
	}

	fmt.Printf("🗂️  Executing %d turns from file: %s\n", len(prompts), filePath)
	fmt.Printf("📋 Each turn will use scenario: %s (memory preserved between turns)\n\n", scenario)

	// Execute each prompt as a separate turn with memory preserved
	for i, prompt := range prompts {
		prompt = strings.TrimSpace(prompt)
		if prompt == "" {
			continue // Skip empty prompts
		}

		fmt.Printf("🔄 Turn %d/%d:\n", i+1, len(prompts))
		fmt.Printf("📝 Prompt: %s\n", prompt)
		fmt.Print("\n")

		// Memory is preserved between turns - no ClearHistory() call

		// Execute the prompt
		response, err := a.Invoke(ctx, prompt, scenario)
		if err != nil {
			fmt.Printf("❌ Turn %d failed: %v\n", i+1, err)
			continue
		}

		fmt.Printf("✅ Response:\n%s\n", response.Content())
		fmt.Printf("%s\n\n", strings.Repeat("─", 60))
	}

	fmt.Println("🏁 All turns completed.")
}

func startInteractiveMode(ctx context.Context, a *app.ScenarioRunner, scenario string) {
	// Configure readline with enhanced features
	config := &readline.Config{
		Prompt:              "> ",
		HistoryFile:         "/tmp/gennai_history",
		AutoComplete:        createAutoCompleter(),
		InterruptPrompt:     "^C",
		EOFPrompt:           "exit",
		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	}

	// Create readline instance
	rl, err := readline.NewEx(config)
	if err != nil {
		fmt.Printf("❌ Failed to initialize interactive mode: %v\n", err)
		fmt.Println("💡 Please use one-shot mode instead: gennai \"your request here\"")
		return
	}
	defer rl.Close()

	fmt.Println("\n🚀 Welcome to Gennai Interactive Mode!")
	fmt.Println("💬 Commands start with '/', everything else goes to the AI agent!")
	fmt.Println("⌨️ Use arrow keys to navigate, Ctrl+R for history search, Tab for completion.")
	fmt.Println(strings.Repeat("=", 60))

	// Show conversation preview if there are existing messages
	conversationPreview := a.GetConversationPreview(6) // Show last 6 messages
	if conversationPreview != "" {
		fmt.Print("\n")
		fmt.Print(conversationPreview)
		fmt.Println()
	}

	for {
		fmt.Print("\n") // Add newline before prompt
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		userInput := strings.TrimSpace(line)

		if userInput == "" {
			continue
		}

		// Handle commands that start with /
		if strings.HasPrefix(userInput, "/") {
			if handleSlashCommand(userInput, a) {
				break // Command requested exit
			}
			continue // Command was handled, get next input
		}

		// Process the user input with ReAct agent
		// Processing user input

		var response message.Message
		var invokeErr error

		// Always use the CLI-specified scenario (defaults to "coding")
		response, invokeErr = a.Invoke(ctx, userInput, scenario)

		if invokeErr != nil {
			fmt.Printf("❌ Error: %v\n", invokeErr)
			continue
		}

		fmt.Printf("✅ Response:\n%+v\n", response.Content())
	}
}

// SlashCommand represents a command that starts with /
type SlashCommand struct {
	Name        string
	Description string
	Handler     func(*app.ScenarioRunner) bool // Returns true if should exit
}

// getSlashCommands returns all available slash commands
func getSlashCommands() []SlashCommand {
	return []SlashCommand{
		{
			Name:        "help",
			Description: "Show available commands and usage information",
			Handler: func(a *app.ScenarioRunner) bool {
				showInteractiveHelp()
				return false
			},
		},
		{
			Name:        "clear",
			Description: "Clear conversation history and start fresh",
			Handler: func(a *app.ScenarioRunner) bool {
				a.ClearHistory()
				fmt.Println("🧹 Conversation history cleared.")
				return false
			},
		},
		{
			Name:        "status",
			Description: "Show current session status and statistics",
			Handler: func(a *app.ScenarioRunner) bool {
				showStatus(a)
				return false
			},
		},
		{
			Name:        "quit",
			Description: "Exit the interactive session",
			Handler: func(a *app.ScenarioRunner) bool {
				fmt.Println("👋 Goodbye!")
				return true
			},
		},
		{
			Name:        "exit",
			Description: "Exit the interactive session (alias for quit)",
			Handler: func(a *app.ScenarioRunner) bool {
				fmt.Println("👋 Goodbye!")
				return true
			},
		},
	}
}

// handleSlashCommand processes commands that start with /
// Returns true if the command requests program exit, false otherwise
func handleSlashCommand(input string, a *app.ScenarioRunner) bool {
	// Check if this is just "/" - show command selector
	if strings.TrimSpace(input) == "/" {
		return showCommandSelector(a)
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}

	commandName := strings.TrimPrefix(parts[0], "/")
	commands := getSlashCommands()

	// Find and execute the command
	for _, cmd := range commands {
		if cmd.Name == commandName {
			return cmd.Handler(a)
		}
	}

	// Command not found - show available commands
	fmt.Printf("❌ Unknown command: /%s\n", commandName)
	fmt.Println("💡 Available commands:")
	for _, cmd := range commands {
		fmt.Printf("  /%s - %s\n", cmd.Name, cmd.Description)
	}
	fmt.Println("\n💡 Tip: Type just '/' to see an interactive command selector!")
	return false
}

// showCommandSelector shows an interactive command selector using promptui
func showCommandSelector(a *app.ScenarioRunner) bool {
	commands := getSlashCommands()

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}?",
		Active:   "▸ {{ .Name | cyan }} - {{ .Description | faint }}",
		Inactive: "  {{ .Name | cyan }} - {{ .Description | faint }}",
		Selected: "{{ .Name | red | cyan }}",
		Details: `
--------- Command Details ----------
{{ "Name:" | faint }}	{{ .Name }}
{{ "Description:" | faint }}	{{ .Description }}`,
	}

	searcher := func(input string, index int) bool {
		command := commands[index]
		name := strings.ReplaceAll(strings.ToLower(command.Name), " ", "")
		input = strings.ReplaceAll(strings.ToLower(input), " ", "")

		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Label:     "Choose a command",
		Items:     commands,
		Templates: templates,
		Size:      10,
		Searcher:  searcher,
	}

	i, _, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			fmt.Println("\nCancelled.")
			return false
		}
		fmt.Printf("Command selection failed: %v\n", err)
		return false
	}

	// Execute the selected command
	return commands[i].Handler(a)
}

// showStatus displays current session status
func showStatus(a *app.ScenarioRunner) {
	fmt.Println("\n📊 Session Status:")

	// Get conversation preview to count messages
	preview := a.GetConversationPreview(100) // Get many to count all
	if preview != "" {
		// Simple heuristic to count messages by looking for message patterns
		userMsgCount := strings.Count(preview, "👤 You:")
		assistantMsgCount := strings.Count(preview, "🤖 Assistant:")
		fmt.Printf("  💬 Messages: %d from you, %d from assistant\n", userMsgCount, assistantMsgCount)
	} else {
		fmt.Println("  💬 Messages: No conversation history")
	}

	fmt.Println("  🔧 Tools: Available and active")
	fmt.Println("  🧠 Agent: ReAct with scenario planning")
	fmt.Println("  ⚡ Status: Ready for requests")
}

// createAutoCompleter creates an autocompletion function for readline
func createAutoCompleter() *readline.PrefixCompleter {
	commands := getSlashCommands()
	var pcItems []readline.PrefixCompleterInterface

	// Add slash commands dynamically
	for _, cmd := range commands {
		pcItems = append(pcItems, readline.PcItem("/"+cmd.Name))
	}

	// Add the interactive slash command selector
	pcItems = append(pcItems, readline.PcItem("/"))

	// Add common request patterns
	commonPatterns := []string{
		"Create a", "Analyze the", "Write unit tests for", "List files in",
		"Run go build", "Fix any errors", "Explain how", "Show me",
		"Generate", "Debug", "Test", "Refactor",
	}

	for _, pattern := range commonPatterns {
		pcItems = append(pcItems, readline.PcItem(pattern))
	}

	return readline.NewPrefixCompleter(pcItems...)
}

// filterInput filters input runes to handle special keys
func filterInput(r rune) (rune, bool) {
	switch r {
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

func showInteractiveHelp() {
	commands := getSlashCommands()

	fmt.Println("\n📚 Interactive Commands:")
	fmt.Println("  /                - Show interactive command selector 🆕")
	for _, cmd := range commands {
		fmt.Printf("  /%-15s - %s\n", cmd.Name, cmd.Description)
	}

	fmt.Println("\n⌨️  Enhanced Features:")
	fmt.Println("  Ctrl+C           - Cancel current input")
	fmt.Println("  Ctrl+R           - Search command history")
	fmt.Println("  Tab              - Auto-complete commands and patterns")
	fmt.Println("  Arrow keys       - Navigate input and history")
	fmt.Println("  /                - Interactive command selector with search!")

	fmt.Println("\n💡 Example requests:")
	fmt.Println("  > Create a HTTP server with health check")
	fmt.Println("  > Analyze the current codebase structure")
	fmt.Println("  > Write unit tests for the ScenarioRunner")
	fmt.Println("  > List files in the current directory")
	fmt.Println("  > Run go build and fix any errors")

	fmt.Println("\n✨ New: Type just '/' to see a beautiful command selector!")
	fmt.Println("🔧 The agent will automatically use tools when needed!")
}

// hasEnabledMCPServers checks if there are any enabled MCP servers
func hasEnabledMCPServers(servers []domain.MCPServerConfig) bool {
	for _, server := range servers {
		if server.Enabled {
			return true
		}
	}
	return false
}

// initializeMCP initializes MCP integration with enabled servers from settings
func initializeMCP(ctx context.Context, mcpSettings config.MCPSettings, logger *pkgLogger.Logger) *mcp.Integration {
	integration := mcp.NewIntegration()

	// Add only enabled servers from settings
	var connectedServers []string
	var failedServers []string

	for _, serverConfig := range mcpSettings.Servers {
		// Skip disabled servers
		if !serverConfig.Enabled {
			continue
		}

		if err := integration.AddServer(ctx, serverConfig); err != nil {
			logger.WarnWithIcon("⚠️", "Failed to connect to MCP server",
				"server", serverConfig.Name, "error", err)
			failedServers = append(failedServers, serverConfig.Name)
		} else {
			connectedServers = append(connectedServers, serverConfig.Name)
		}
	}

	// Log connection results
	if len(connectedServers) > 0 {
		logger.InfoWithIcon("✅", "Successfully connected to MCP servers",
			"servers", connectedServers)
	}
	if len(failedServers) > 0 {
		logger.WarnWithIcon("⚠️", "Failed to connect to MCP servers",
			"servers", failedServers)
	}

	if len(connectedServers) == 0 {
		logger.WarnWithIcon("⚠️", "No MCP servers connected")
		return nil
	}

	return integration
}
