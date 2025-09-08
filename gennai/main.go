package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fpt/go-gennai-cli/internal/app"
	"github.com/fpt/go-gennai-cli/internal/config"
	"github.com/fpt/go-gennai-cli/internal/infra"
	"github.com/fpt/go-gennai-cli/internal/mcp"
	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/client/anthropic"
	"github.com/fpt/go-gennai-cli/pkg/client/gemini"
	"github.com/fpt/go-gennai-cli/pkg/client/ollama"
	"github.com/fpt/go-gennai-cli/pkg/client/openai"
	pkgLogger "github.com/fpt/go-gennai-cli/pkg/logger"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

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
	// Custom scenario CLI option removed
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
	var showLog = flag.Bool("l", false, "Print conversation message history and exit")
	var showLogLong = flag.Bool("log", false, "Print conversation message history and exit")
	var promptFile = flag.String("f", "", "File containing multi-turn prompts separated by '----' (no memory between turns)")
	var verbose = flag.Bool("v", false, "Enable verbose logging (debug level)")
	var verboseLong = flag.Bool("verbose", false, "Enable verbose logging (debug level)")
	var help = flag.Bool("h", false, "Show this help message")
	var helpLong = flag.Bool("help", false, "Show this help message")

	// Custom scenarios option removed

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
	// Use a single writer for console output and ScenarioRunner output
	out := os.Stdout
	pkgLogger.SetGlobalLoggerWithConsoleWriter(pkgLogger.LogLevel(logLevel), out)
	logger := pkgLogger.NewLoggerWithConsoleWriter(pkgLogger.LogLevel(logLevel), out)

	// Log the current log level for debugging
	if resolvedVerbose {
		logger.DebugWithIntention(pkgLogger.IntentionStatistics, "Verbose logging enabled", "log_level", logLevel)
	}

	// Convert scenario to uppercase for case-insensitive matching with YAML files
	internalScenario := strings.ToUpper(resolvedScenario)

	// Override settings with command line arguments
	if resolvedBackend != "" {
		// When backend is overridden, reset ALL LLM settings to defaults for that backend
		// unless specific model is also provided
		if resolvedModel == "" {
			// No specific model provided, use all defaults for the backend
			settings.LLM = config.GetDefaultLLMSettingsForBackend(resolvedBackend)
		} else {
			// Specific model provided, use backend defaults but override model
			backendDefaults := config.GetDefaultLLMSettingsForBackend(resolvedBackend)
			settings.LLM = backendDefaults
			settings.LLM.Model = resolvedModel
		}
	} else if resolvedModel != "" {
		// Only model is overridden, keep existing backend settings but change model
		settings.LLM.Model = resolvedModel
	}

	// Validate settings
	if err := config.ValidateSettings(settings); err != nil {
		logger.Error("Settings validation failed", "error", err)
		os.Exit(1)
	}

	// Create LLM client based on settings
	var llmClient domain.LLM
	switch settings.LLM.Backend {
	case "anthropic", "claude":
		llmClient, err = anthropic.NewAnthropicClientWithTokens(settings.LLM.Model, settings.LLM.MaxTokens)
		if err != nil {
			logger.Error("Failed to create Anthropic client", "error", err)
			os.Exit(1)
		}
	case "openai":
		llmClient, err = openai.NewOpenAIClient(settings.LLM.Model, settings.LLM.MaxTokens)
		if err != nil {
			logger.Error("Failed to create OpenAI client", "error", err)
			os.Exit(1)
		}
	case "gemini":
		llmClient, err = gemini.NewGeminiClientWithTokens(settings.LLM.Model, settings.LLM.MaxTokens)
		if err != nil {
			logger.Error("Failed to create Gemini client", "error", err)
			os.Exit(1)
		}
	default:
		// For Ollama, check if model is in known list, if not, test capability
		if !ollama.IsModelInKnownList(settings.LLM.Model) {
			logger.Warn("Model not in known capabilities list, testing tool calling capability",
				"model", settings.LLM.Model)

			// Test model capability with a 30-second timeout
			testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			hasToolCapability, testErr := ollama.DynamicCapabilityCheck(testCtx, settings.LLM.Model, false)
			if testErr != nil {
				logger.Warn("Failed to test model capability, proceeding without tool support",
					"model", settings.LLM.Model, "error", testErr)
			} else if !hasToolCapability {
				logger.Warn("Model does not support tool calling - limited functionality",
					"model", settings.LLM.Model, "suggestion", "consider using 'gpt-oss:latest'")
			} else {
				logger.InfoWithIntention(pkgLogger.IntentionSuccess, "Model supports tool calling, proceeding with full functionality",
					"model", settings.LLM.Model)
			}
		}

		llmClient, err = ollama.NewOllamaClient(settings.LLM.Model, settings.LLM.MaxTokens, settings.LLM.Thinking)
		if err != nil {
			logger.Error("Failed to create Ollama client", "error", err)
			os.Exit(1)
		}
	}

	// Determine working directory (don't change process cwd, just pass to tools)
	workingDirectory := *workdir
	if workingDirectory != "" {
		// Validate that the directory exists
		if _, err := os.Stat(workingDirectory); err != nil {
			logger.Error("Working directory does not exist",
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

	// Create shared FilesystemRepository instance at application level
	fsRepo := infra.NewOSFilesystemRepository()

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

		a = app.NewScenarioRunnerWithOptions(llmClient, workingDirectory, mcpToolManagers, settings, logger, out, skipSessionRestore, isInteractiveMode, fsRepo)
	} else {
		mcpToolManagers := make(map[string]domain.ToolManager)
		a = app.NewScenarioRunnerWithOptions(llmClient, workingDirectory, mcpToolManagers, settings, logger, out, skipSessionRestore, isInteractiveMode, fsRepo)

		// Note: SimpleToolManager removed - tools now managed by specialized managers
	}

	// Using built-in scenarios only

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
		app.StartInteractiveMode(ctx, a, internalScenario)
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

	// Print plain header + content via ScenarioRunner writer
	w := a.OutWriter()
	model := a.GetLLMClient().ModelID()
	app.WriteResponseHeader(w, model, false)
	fmt.Fprintln(w, response.Content())
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

		w := a.OutWriter()
		model := a.GetLLMClient().ModelID()
		app.WriteResponseHeader(w, model, false)
		fmt.Fprintln(w, response.Content())
		fmt.Fprintf(w, "%s\n\n", strings.Repeat("─", 60))
	}

	fmt.Println("🏁 All turns completed.")
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
			logger.Warn("Failed to connect to MCP server",
				"server", serverConfig.Name, "error", err)
			failedServers = append(failedServers, serverConfig.Name)
		} else {
			connectedServers = append(connectedServers, serverConfig.Name)
		}
	}

	// Log connection results
	if len(connectedServers) > 0 {
		logger.DebugWithIntention(pkgLogger.IntentionSuccess, "Successfully connected to MCP servers",
			"servers", connectedServers)
	}
	if len(failedServers) > 0 {
		logger.Warn("Failed to connect to MCP servers",
			"servers", failedServers)
	}

	if len(connectedServers) == 0 {
		logger.Warn("No MCP servers connected")
		return nil
	}

	return integration
}
