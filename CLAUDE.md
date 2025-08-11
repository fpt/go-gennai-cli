# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

**Build and Run:**
```bash
# Interactive mode (default)
go run gennai/main.go                         # Start interactive REPL
go run gennai/main.go -b anthropic            # Interactive with Anthropic

# One-shot mode
go run gennai/main.go "your requirements"     # Run with requirements

# Build commands
go build -o gennai ./gennai                # Build binary
go build ./...                                       # Build all packages
go mod tidy                                          # Download/update dependencies
```

**Interactive Mode Usage:**
```bash
# Start interactive mode
go run gennai/main.go

# Then use slash commands or natural language:
> Create a HTTP server with health check endpoint
> Analyze the current codebase structure  
> Write unit tests for the ReactAgent
> List files in the current directory
> Run go build and fix any errors
> /help    # Show available commands
> /clear   # Clear conversation history
> /quit    # Exit interactive mode
```

**Interactive Mode Features:**
- **Readline Support**: Full cursor movement, arrow keys, and command history
- **Autocomplete**: Tab completion for slash commands and file paths
- **Unified Command Handler**: All commands start with `/` to avoid conflicts
- **Session Persistence**: Conversation history preserved between sessions

**One-shot Mode Usage:**
```bash
go run gennai/main.go "Create a HTTP server with health check endpoint"
go run gennai/main.go "List files in current directory" 
go run gennai/main.go "Run go build on this project"

# Multi-backend support
go run gennai/main.go -b anthropic "Analyze this codebase"
go run gennai/main.go -b openai -m gpt-4o "Create a REST API"
go run gennai/main.go -b gemini -m gemini-2.5-flash-lite "Optimize this code"
go run gennai/main.go -b ollama -m gpt-oss "Write unit tests"

# Custom scenarios
go run gennai/main.go --scenarios ./custom-scenarios.yaml "Use custom scenarios"
go run gennai/main.go --scenarios ./custom-scenarios/ "Use scenarios from directory"
```

**Advanced Usage Examples:**
```bash
# Code analysis with ReAct reasoning
go run gennai/main.go "Analyze the ReAct pattern implementation and suggest improvements"

# Code generation with tool usage
go run gennai/main.go "Create a REST API with user authentication using Go"

# Testing and validation
go run gennai/main.go "Write comprehensive unit tests for the ReactAgent"

# Refactoring with dependency injection
go run gennai/main.go "Refactor this code to use proper dependency injection"

# Multi-step development workflow
go run gennai/main.go "Create a new microservice with Docker support, tests, and documentation"
```

**Testing:**
```bash
go test ./...                      # Run all tests
go test -v ./...                   # Run tests with verbose output
go test ./internal/agent           # Test specific package
```

**Code Quality:**
```bash
go fmt ./...                       # Format code
go vet ./...                       # Static analysis
go mod tidy                        # Clean up dependencies
```

## User Configuration

**gennai maintains per-user configuration and project data in `$HOME/.gennai/`:**

```
$HOME/.gennai/
├── projects/                    # Project-specific data
│   └── {project-name-hash}/    # Each project gets its own directory
│       ├── project_info.txt   # Original project path and metadata
│       ├── todos.json         # Project-specific todo list
│       └── session.json       # Conversation history and context
├── todos/                      # Global todos directory
│   └── global_todos.json      # Global todo list (if needed)
└── config.json                 # User preferences (future use)
```

**Key Features:**
- **Project Isolation**: Each project gets its own todo list, session data, and storage
- **Session Persistence**: Conversation history is automatically saved and restored between runs (like Claude Code)
- **Safe Directory Names**: Project paths are converted to safe directory names with hash suffixes
- **Cross-Session Persistence**: Todos, sessions, and project data persist between gennai runs
- **Clean Project Structure**: No configuration files clutter your project directories
- **Automatic Creation**: User directories are created automatically when first needed
- **No Fallbacks**: Always uses `$HOME/.gennai/` - no local file fallbacks

**Project Directory Naming:**
Projects are stored in directories using the pattern: `{project-basename}-{path-hash}`
- Example: `/Users/you/dev/my-app` → `my-app-a1b2c3d4/`
- Handles name collisions and special characters safely

## Architecture

This is a Go-based scenario-driven coding agent that uses YAML-configured scenarios with the ReAct (Reason and Act) pattern. It supports multiple LLM backends, secure tool management, and interactive mode. The codebase follows a clean architecture with dependency injection and scenario-based execution:

**Core Structure:**
- `gennai/main.go` - Application entry point with interactive REPL
- `internal/agent/` - Scenario-based agent implementation:
  - `scenario_runner.go` - Main ScenarioRunner orchestrating planner → ReAct workflow
  - `planner_agent.go` - PlannerAgent for scenario selection using StructuredLLM
- `internal/config/` - YAML-based scenario configuration:
  - `scenario.go` - Scenario configuration loading and template rendering
  - `filesystem.go` - File system security configuration
- `internal/tool/` - Tool management with security features:
  - `simple_tool_manager.go` - Basic tool manager with MCP support
  - `filesystem_tool_manager.go` - Secure filesystem tools with read→write semantics
  - `composite_tool_manager.go` - Combines multiple tool managers
- `internal/scenarios/` - Embedded YAML scenario definitions:
  - `filesystem.yaml`, `generate.yaml`, `analyze-code.yaml` - Individual scenario configs
  - `research.yaml`, `respond.yaml`, etc. - Additional scenario definitions
  - `embedded.go` - Go embed integration for built-in scenarios
- `pkg/llmclient/` - Clean LLM client abstraction layer:
  - `domain/` - Domain interfaces and types
  - `message/` - Message handling and state management with vision support
  - `react/` - ReAct pattern implementation with vision message truncation
  - `model/` - Tool definitions
- `pkg/client/` - Client factory, structured output creation, and tool wrapper:
  - `structured_factory.go` - Factory for creating structured clients
  - `withtool.go` - ClientWithTool wrapper for tool management
  - LLM client implementations (Ollama, Anthropic)

**Key Types:**
- `ScenarioRunner` - Main orchestrator managing planner → ReAct workflow with scenario-specific tools
- `PlannerAgent` - Scenario selection using StructuredLLM with YAML-based action descriptions
- `ScenarioConfig` - YAML-based scenario definition with tools, description, and prompt template
- `FileSystemToolManager` - Secure filesystem tools with read→write semantics and allowlist/blacklist
- `CompositeToolManager` - Combines multiple tool managers (e.g., filesystem + default tools)
- `react.ReAct` - Core ReAct implementation with vision message truncation
- `client.ClientWithTool` - Auto-detecting wrapper for native vs text-based tool calling
- `domain.LLM` - Base interface for LLM clients
- `domain.StructuredLLM[T any]` - Generic interface for type-safe structured output
- `domain.ToolManager` - Interface for tool management with security controls

**YAML-Driven Scenario System:**
The system is fully driven by YAML configurations in `asset/scenario/`:

**Available Scenarios:**
- `CODE` - Comprehensive coding assistant for all development tasks (filesystem, default, todo, bash, mcp:godevmcp)
- `FILESYSTEM` - File system exploration and analysis (filesystem, default, todo, mcp:godevmcp)
- `RESEARCH` - Web research and information gathering (default, mcp:serverB)
- `RESPOND` - Direct knowledge-based responses, todo management, and tool usage (default tools only)

**Scenario Configuration Structure:**
```yaml
SCENARIO_NAME:
  tools: "filesystem, default"  # or "default" only
  description: "Brief description for planner selection"
  prompt: |
    Template prompt with variables:
    User Request: {{userInput}}
    Reason: {{scenarioReason}}
    Working Directory: {{workingDir}}
```

**Security-First Tool Management:**
- **Scenario-Based Tool Isolation**: Different scenarios get different tool access
- **Read→Write Semantics**: Files must be read before writing, with timestamp validation
- **Directory Allowlist**: Filesystem access restricted to approved directories
- **File Blacklist**: Prevents reading sensitive files (.env, .secret, etc.)
- **Composite Tool Strategy**: Filesystem scenarios get both secure filesystem + default tools

**Workflow Architecture:**
1. **YAML Loading** → Load built-in embedded scenarios + optional custom scenarios
2. **Planner Selection** → StructuredLLM selects scenario using YAML descriptions
3. **Tool Manager Selection** → Choose tools based on scenario's tool specification
4. **Prompt Rendering** → Render YAML template with user input variables
5. **ReAct Execution** → Execute scenario with appropriate tools and rendered prompt

**Key Improvements:**
- **Embedded + Custom YAML Configuration**: Built-in scenarios embedded in binary + optional custom overrides
- **Security Isolation**: Scenario-specific tool access prevents unauthorized filesystem operations
- **Template-Based Prompts**: Dynamic prompt generation with variable substitution
- **Composite Tool Architecture**: Flexible tool combination based on scenario requirements
- **Type-Safe Scenario Selection**: Structured LLM output ensures consistent scenario selection

**Tool Support:**
The system supports unified tool calling with automatic detection of model capabilities and sophisticated tool schema handling:

**Native Tool Calling:**
- Used by Anthropic Claude and tool-capable Ollama models (gpt-oss, etc.)
- API-level tool definitions with structured tool_use/tool_result blocks
- More efficient and reliable tool execution
- Automatically detected via `IsToolCapable()` interface method


**Tool Schema Enhancement:**
The system includes comprehensive tool schema handling:
```go
// Enhanced tool descriptions with parameter schemas
toolDesc := fmt.Sprintf("- %s: %s", name, tool.Description())
if len(args) > 0 {
    toolDesc += "\n  Parameters:"
    for _, arg := range args {
        required := ""
        if arg.Required {
            required = " (required)"
        }
        toolDesc += fmt.Sprintf("\n    - %s (%s)%s: %s", 
            arg.Name, arg.Type, required, arg.Description)
    }
} else {
    toolDesc += "\n  Parameters: none"
}
```

**Schema-as-Tool Pattern (Native Tool Calling for Structured Output):**
- Used by tool-capable Ollama models for structured output (gpt-oss, etc.)
- Creates a "respond" tool where target schema becomes tool parameters
- Prompt enhancement instructs models to use the "respond" tool
- JSON Schema validation ensures type-safe structured responses

**Capability-Based Design:**

The system uses a **capability-based design** with clean interface segregation:

**Interface Hierarchy:**
- `domain.LLM` - Base interface for basic chat functionality
- `domain.ToolCallingLLM` - Extends LLM with tool calling capabilities  
- `domain.ThinkingLLM` - Extends LLM with thinking capabilities
- `domain.StructuredLLM[T any]` - Extends LLM with type-safe structured output
- `domain.ToolCallingLLMWithThinking` - Combines tool calling + thinking
- `domain.StructuredLLMWithThinking[T any]` - Combines structured output + thinking

**Capability Detection:**
Capabilities are determined using Go's type assertion pattern rather than boolean methods:

```go
// Check for thinking capability
if thinkingClient, ok := client.(domain.ThinkingLLM); ok {
    // Use thinking-specific methods
    response, err := thinkingClient.ChatWithThinking(ctx, messages, true)
}

// Check for tool calling capability  
if toolClient, ok := client.(domain.ToolCallingLLM); ok {
    // Use tool calling methods
    response, err := toolClient.ChatWithToolChoice(ctx, messages, toolChoice)
}

// Check for structured output capability
if structuredClient, ok := client.(domain.StructuredLLM[MyType]); ok {
    // Use structured output methods
    result, err := structuredClient.InvokeStructuredOutput(ctx, messages)
}
```

**Benefits of Type Assertion Approach:**
- **Type Safety**: Compile-time guarantees that capabilities exist
- **Clean Interfaces**: No redundant boolean methods cluttering interfaces
- **Go Idioms**: Follows standard Go patterns for capability detection
- **Maintainability**: Capabilities are self-documenting through interface compliance

**Available Tools:**

**Default Tools (all scenarios):**
- MCP tools (tree_dir, get_github_content, search_local_files, etc.)
- `go_build` - Build Go packages
- `go_run` - Run Go programs
- Web tools (fetch_web, wikipedia_search, duckduckgo_search)
  - `fetch_web` - HTML to markdown conversion for text analysis

**Secure Filesystem Tools (filesystem scenarios only):**
- `read_file` - Read file contents (with timestamp tracking)
- `write_file` - Write content to files (requires prior read, validates timestamps)
- `list_directory` - List directory contents (allowlist restricted)
- `run_grep` - Search patterns in files (allowlist restricted)
- `edit_file` - Edit files with exact string replacement (read→write semantics)

**Tool Binding and Security:**
Tools are bound to the LLM client with scenario-specific security controls:
```go
// Get scenario-specific tool manager
toolManager := scenarioRunner.getToolManagerForScenario(selectedAction)

// Bind tools to LLM client
llmWithTools, err := client.NewClientWithToolManager(llmClient, toolManager)
reactClient := react.NewReAct(llmWithTools, toolManager, sharedState)
```

**Tool Manager Selection Logic:**
```go
func (s *ScenarioRunner) getToolManagerForScenario(scenario string) domain.ToolManager {
    if scenarioConfig, exists := s.scenarios[scenario]; exists {
        toolScope := scenarioConfig.GetToolScope()
        
        if toolScope.UseFilesystem && toolScope.UseDefault {
            // Combine secure filesystem + default tools
            return tool.NewCompositeToolManager(s.defaultToolManager, s.filesystemManager)
        } else if toolScope.UseFilesystem {
            // Only secure filesystem tools
            return s.filesystemManager
        }
    }
    // Default tools only (safe for web/research scenarios)
    return s.defaultToolManager
}
```

The `ClientWithTool` wrapper automatically detects whether the underlying LLM supports native tool calling via the `IsToolCapable()` method and routes to the appropriate implementation.

**Structured Output with Generics:**
The system provides type-safe structured output using Go generics:

```go
// Create a structured client with type safety
type MyResponse struct {
    Summary string `json:"summary" description:"Brief summary"`
    Steps   []Step `json:"steps" description:"List of steps"`
}

structuredClient, err := NewStructuredClient[MyResponse](baseClient)
// structuredClient is StructuredLLM[MyResponse]

// Get typed results without casting
result, err := structuredClient.InvokeStructuredOutput(ctx, messages)
// result is already typed as MyResponse, no casting needed!

// Access schema with compile-time type safety
mySchema := structuredClient.GetSchema() // Returns MyResponse, not any
```

**Provider-Specific Implementations:**
- **Ollama Tool**: Uses schema-as-tool pattern with native tool calling for structured output
- **Anthropic**: Uses schema-as-tool pattern with native tool calling
- **OpenAI/Gemini**: Uses native structured output with JSON Schema validation
- **Automatic Detection**: Factory function chooses optimal approach per provider and model capabilities

**Thinking Support:**
Both Ollama and Anthropic support thinking capabilities:
- **Ollama**: Reasoning models (gpt-oss) support the thinking parameter
- **Anthropic**: Claude models support ThinkingBlock responses for reasoning visibility
- Automatically enabled for thinking-capable models via type assertion
- Provides visible reasoning process for debugging and transparency
- Thinking content captured in message with `Thinking()` method

**Dependency Injection:**
The architecture uses clean dependency injection with scenario-based configuration:
- YAML scenario configurations loaded at startup
- Scenario-specific tool managers created based on security requirements
- LLM clients wrapped with appropriate tool managers per scenario
- PlannerAgent and ScenarioRunner injected with shared state
- Enhanced LLM clients injected into ReAct for execution
- No tight coupling between components

**Interactive Mode:**
The application supports two modes:
1. **Interactive Mode (default)**: REPL-style interface with conversation memory
2. **One-shot Mode**: Single command execution when arguments provided

Interactive mode supports built-in slash commands:
- `/help` - Show available commands and usage information
- `/clear` - Clear conversation history and start fresh
- `/quit` or `/exit` - Exit interactive mode gracefully

## Message State Management

The system includes sophisticated message state management with intelligent compaction and summary replacement:

**Message Sources:**
Messages are categorized by source for intelligent filtering and management:
- `MessageSourceDefault` - Standard user/assistant messages
- `MessageSourceAligner` - Alignment guidance messages (removed after each iteration)
- `MessageSourceSummary` - Conversation summary messages (replaced, not accumulated)

**Summary Replacement Strategy:**
The system maintains at most one summary message in conversation history:
- **Previous Summary Removal**: Old summary messages are automatically removed before creating new ones
- **Intelligent Compaction**: When conversation exceeds 50 messages, older messages are summarized by the LLM
- **Context Preservation**: Recent messages (last 10) are always preserved for immediate context
- **Tool Chain Safety**: Compaction respects tool call/result pairs to maintain API compatibility

**Message Compaction Features:**
- **LLM-Generated Summaries**: Uses the active LLM to create intelligent conversation summaries
- **Vision Content Truncation**: Older images are removed to save tokens while preserving recent visual context
- **Tool Call Preservation**: Never splits tool call/result chains during compaction
- **Safe Split Points**: Finds boundaries that don't break conversation flow or tool interactions
- **Fallback Summaries**: Basic statistical summaries when LLM summarization fails

**Implementation Details:**
```go
// Remove previous summaries before creating new ones
previousSummariesRemoved := state.RemoveMessagesBySource(message.MessageSourceSummary)

// Create new summary with correct source
summaryMsg := message.NewSummarySystemMessage(
    fmt.Sprintf("# Previous Conversation Summary\n%s\n\n# Current Conversation Continues", summary))

// Ensure only one summary exists in history
state.AddMessage(summaryMsg)
```

**Benefits:**
- **Token Efficiency**: Keeps conversation within LLM context limits
- **Context Preservation**: Maintains conversation continuity through intelligent summarization
- **Memory Management**: Prevents memory bloat in long-running sessions
- **API Compatibility**: Maintains tool call/result pairing required by LLM APIs

## YAML Scenario System

The system uses a hybrid approach combining embedded built-in scenarios with optional custom scenarios for maximum flexibility.

**Built-in Embedded Scenarios:**
The system includes 9 built-in scenarios embedded in the binary:
- `INVESTIGATE_FILESYSTEM` - File system exploration and analysis
- `GENERATE` - Code and content generation, file creation, project scaffolding
- `ANALYZE_CODE` - Code structure and architecture analysis  
- `RESEARCH` - Web research and information gathering
- `RESPOND` - Direct knowledge-based responses
- `DEBUG` - Debugging and error investigation
- `TEST` - Test creation and execution
- `REFACTOR` - Code refactoring and improvement

**Custom Scenario Files:**
You can override or extend built-in scenarios using the `--scenarios` flag:

```bash
# Single custom scenario file
gennai --scenarios ./my-scenarios.yaml "Custom request"

# Directory with multiple scenario files  
gennai --scenarios ./custom-scenarios/ "Custom request"

# Multiple scenario sources (later ones override earlier ones)
gennai --scenarios ./base-scenarios.yaml --scenarios ./override-scenarios.yaml "Custom request"
```

**Priority Order:**
1. Built-in embedded scenarios (lowest priority)
2. Additional scenario files/directories (higher priority - override built-ins)

**Example YAML Configuration:**
```yaml
GENERATE:
  tools: "filesystem, default, mcp:godevmcp"
  description: "Code and content generation, file creation, project scaffolding"
  prompt: |
    Generate code or content for the following request:
    
    User Request: {{userInput}}
    Reasoning: {{scenarioReason}}
    Working Directory: {{workingDir}}
    
    Instructions:
    - Create files, write code, or generate content as requested
    - Follow best practices and include proper error handling
    - Consider the project structure and existing patterns
    - Use secure filesystem tools for file operations
    - Use MCP godevmcp tools for Go-specific analysis and operations

ANALYZE_CODE:
  tools: "filesystem, default, mcp:godevmcp"
  description: "Code structure and architecture analysis"
  prompt: |
    Analyze code for the following request:
    
    User Request: {{userInput}}
    Reason: {{scenarioReason}}
    Working Directory: {{workingDir}}
    
    Instructions:
    - Use MCP tools to extract call graphs and dependencies
    - Analyze code structure using godevmcp tools
    - Use secure filesystem access to read source files safely
```

**Template Variables:**
- `{{userInput}}` - The user's original request
- `{{scenarioReason}}` - Planner's reasoning for selecting this scenario  
- `{{workingDir}}` - Current working directory path

**Tool Scope Options:**
- `"default"` - Only default tools (built-in tools like go_build, go_run)
- `"filesystem"` - Only secure filesystem tools (unusual)
- `"filesystem, default"` - Both secure filesystem and default tools
- `"mcp:godevmcp"` - Specific MCP tool manager (e.g., godevmcp for Go development)
- `"mcp:serverB"` - Another MCP tool manager (e.g., serverB for general development)
- `"default, mcp:godevmcp, mcp:serverB"` - Combine default tools with multiple MCP tools
- `"filesystem, default, mcp:godevmcp"` - All tool types combined

**MCP Tool Management:**
- **Graceful Degradation**: If an MCP tool manager is not available, the system prints a warning but continues running with other available tools
- **Tool Isolation**: MCP tools are isolated per scenario - only scenarios that explicitly request them get access
- **Multiple MCP Support**: A single scenario can use multiple MCP tool managers (e.g., `mcp:godevmcp, mcp:serverB`)
- **Backward Compatibility**: Existing scenarios without MCP tools continue to work unchanged

**MCP Tool Error Handling:**
```
✅ Added MCP tool manager: godevmcp
⚠️  Warning: MCP tool manager 'serverB' not available, skipping
```

**Custom Scenario Loading:**
```
📋 Additional scenarios loaded from: [./custom-scenarios.yaml]
📋 Using built-in scenarios (use --scenarios to add custom scenarios)
```

**Creating Custom Scenarios:**
Create a YAML file to override built-in scenarios or add new ones:

```yaml
# Override built-in GENERATE scenario
GENERATE:
  tools: "filesystem, default, mcp:godevmcp, mcp:serverB"
  description: "ENHANCED - Advanced code generation with multiple MCP tools"
  prompt: |
    Enhanced code generation mode with advanced MCP integration.
    
    User Request: {{userInput}}
    Reasoning: {{scenarioReason}}
    Working Directory: {{workingDir}}
    
    Instructions:
    - Use multiple MCP tool managers for enhanced capabilities
    - Apply advanced generation techniques
    - Include comprehensive error handling and testing

# Add completely new scenario
CUSTOM_ANALYSIS:
  tools: "default, mcp:godevmcp"
  description: "Custom analysis workflow for specialized tasks"
  prompt: |
    Custom analysis scenario for specialized requirements.
    
    User Request: {{userInput}}
    Reasoning: {{scenarioReason}}
    Working Directory: {{workingDir}}
    
    Instructions:
    - Apply custom analysis techniques
    - Use specialized tool combinations
    - Provide detailed insights and recommendations
```

## Module Information

- Module: `github.com/fpt/go-gennai-cli`
- Go Version: 1.24.4
- Dependencies: 
  - `github.com/ollama/ollama v0.5.3`
  - `github.com/anthropics/anthropic-sdk-go v1.5.0`
  - `github.com/openai/openai-go v1.12.0`
  - `google.golang.org/genai v1.19.0`
  - `github.com/chzyer/readline v1.5.1` - Terminal interaction with cursor movement and autocomplete

## Prerequisites

**For Ollama (default):**
- Ollama must be installed and running locally
- Set `OLLAMA_HOST` environment variable if using non-default host
- Ensure the model is available in Ollama

**For Anthropic/Claude:**
- Set `ANTHROPIC_API_KEY` environment variable with your API key
- API key can be obtained from https://console.anthropic.com/

**For OpenAI:**
- Set `OPENAI_API_KEY` environment variable with your API key
- API key can be obtained from https://platform.openai.com/api-keys
- Optional: Set `OPENAI_BASE_URL` for Azure OpenAI or custom endpoints

**For Google Gemini:**
- Set `GEMINI_API_KEY` environment variable with your API key  
- API key can be obtained from https://makersuite.google.com/app/apikey

## Model Performance

**Ollama Models:**

**Native Tool Calling:**
- `gpt-oss:latest` - Supports native Ollama tool calling API with thinking

**Anthropic Models (Native Tool Calling):**
- `claude-3-7-sonnet-latest` - Default Claude model
- `claude-3-5-haiku-latest` - Faster Claude model
- `claude-sonnet-4-20250514` - Latest Claude Sonnet 4

**OpenAI Models (Native Tool Calling + Structured Output):**
- `gpt-4o` - Latest GPT-4 Omni (vision, tool calling, structured output)
- `gpt-4o-mini` - Smaller, faster GPT-4 Omni
- `gpt-3.5-turbo` - Fast and cost-effective for most tasks

**Google Gemini Models (Native Schema + Structured Output):**
- `gemini-2.5-flash-lite` - **Recommended** - Latest, fastest, most efficient
- `gemini-1.5-pro` - High capability model for complex reasoning
- `gemini-2.0-flash` - Latest experimental features

## Testing Practices

### Unit Tests
The project has comprehensive unit tests with 99% coverage:

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./pkg/llmclient/react/

# Run specific test suites
go test -v ./pkg/llmclient/react/ -run TestReAct_Invoke
go test -v ./pkg/client/ollama/ -run TestToolSchemaIntegration

# Test tool schema handling specifically
go test ./pkg/client/ollama/ -v
```

### Integration Testing Scenarios
Run these scenarios to evaluate gennai's scenario-based system:

**Scenario Testing (One-shot Mode):**
1. **GENERATE Scenario**: `go run gennai/main.go "Create a new Go HTTP server with health check endpoint"`
2. **ANALYZE_CODE Scenario**: `go run gennai/main.go "Analyze the architecture and dependencies of this codebase"`
3. **INVESTIGATE_FILESYSTEM Scenario**: `go run gennai/main.go "List all Go files and their purposes in this project"`
4. **DEBUG Scenario**: `go run gennai/main.go "Find and fix any compilation errors in this project"`
5. **TEST Scenario**: `go run gennai/main.go "Create comprehensive unit tests for the ScenarioRunner"`
6. **REFACTOR Scenario**: `go run gennai/main.go "Refactor the scenario system for better maintainability"`
7. **RESEARCH Scenario**: `go run gennai/main.go "Research best practices for Go dependency injection patterns"`
8. **RESPOND Scenario**: `go run gennai/main.go "Explain the difference between channels and mutexes in Go"`

**Security Testing:**
1. **Filesystem Access Control**: Try generating files outside working directory
2. **Read→Write Semantics**: Verify files must be read before writing
3. **Tool Isolation**: Confirm web scenarios can't access filesystem tools
4. **Allowlist Validation**: Test directory access restrictions

**Tool Schema Testing:**
1. **Native Tool Calling**: `go run gennai/main.go -b ollama -m gpt-oss:latest "List files and analyze the code"`
2. **Parameter Handling**: Test tools with complex parameter schemas (tree_dir, search_local_files)
3. **Schema Validation**: Verify proper parameter type mapping and validation
4. **Tool Selection**: Test automatic routing for native tool calling based on model capabilities

**Interactive Mode Testing:**
1. **Start Interactive**: `go run gennai/main.go`
2. **Terminal Features**: Test arrow keys, cursor movement, command history
3. **Slash Commands**: Test `/help`, `/clear`, `/quit` functionality with tab completion
4. **Multi-turn Scenarios**: Test conversation context in scenario selection
5. **Tool Usage**: Test secure filesystem operations vs. default tool usage
6. **State Persistence**: Verify conversation history affects scenario selection
7. **Message Compaction**: Test automatic summary replacement in long conversations
8. **YAML Configuration**: Confirm built-in embedded scenarios load correctly
9. **Template Rendering**: Verify {{userInput}}, {{scenarioReason}}, {{workingDir}} substitution

### Unit Testing Coverage
The project includes comprehensive unit tests for:

**Scenario System Testing:**
- YAML scenario loading and parsing
- Template variable substitution
- Tool scope configuration parsing
- Scenario selection logic

**Security Feature Testing:**
- FileSystemToolManager read→write semantics
- Directory allowlist enforcement
- File blacklist validation
- Timestamp-based concurrent modification detection

**ReAct Pattern Testing:**
- Message state management with scenario context
- Tool call handling across different scenarios
- Error scenarios and recovery
- JSON parsing and structured output

**Message State Management Testing:**
- Message source filtering and removal by source type
- Summary replacement (ensuring only 0 or 1 summary exists)
- Conversation compaction with safe split points
- Tool call chain preservation during compaction
- Vision content truncation for token efficiency

**Configuration Testing:**
- Embedded YAML scenario loading from internal/scenarios/
- ScenarioConfig template rendering and variable substitution
- FileSystem security configuration validation
- Composite tool manager composition

**Tool Schema Testing:**
Comprehensive test suites for tool schema handling:

**Native Tool Schema Testing (pkg/client/ollama/):**
- "Respond" tool creation with target schemas as parameters
- Schema-as-tool pattern validation
- JSON Schema to API format conversion
- Required fields extraction and handling
- Tool serialization/deserialization for API communication
- Prompt enhancement for tool-based structured output

**Test Coverage Features:**
- Mock tool managers with realistic tool definitions
- Complete flow testing from schema generation to tool execution
- Type safety validation (Go types → JSON Schema → API format)
- Error handling and edge case coverage
- Integration tests with serialization/deserialization
- Parameter schema consistency across different model types

**Integration Testing:**
Tests use mocked dependencies to ensure:
- Clean separation between planner and executor
- Scenario-specific tool isolation
- YAML-driven prompt generation
- Secure filesystem access patterns
- Tool schema consistency across different model capabilities
- Proper routing between native tool calling and schema-as-tool patterns
