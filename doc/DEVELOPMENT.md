# GENNAI CLI Development Guide

This document provides detailed information for developers working on GENNAI CLI.

## Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ gennai/  │    │   internal/     │    │  pkg/llmclient/ │
│   main.go       │───▶│   agent/        │───▶│   react/        │
│                 │    │   react_agent.go│    │   react.go      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │                        │
                              ▼                        ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   internal/     │    │  pkg/llmclient/ │
                       │   tool/         │    │   client/       │
                       │   simple_tool_  │    │   ollama.go     │
                       │   manager.go    │    │   anthropic.go  │
                       └─────────────────┘    └─────────────────┘
```

### Key Components

- **ScenarioRunner**: Main orchestrator managing scenario-based ReAct execution
- **ReAct**: Core reasoning and acting implementation with unified tool calling
- **ClientWithTool**: Automatic wrapper that detects and handles native vs text-based tool calling
- **LLM Clients**: Pluggable backend support (Ollama, Anthropic, OpenAI, Gemini) with capability-based design
- **Client Factory**: LLM client creation and configuration
- **Tool Manager**: Handles tool registration and execution
- **Message State**: Manages conversation history and context
- **Interactive REPL**: Continuous conversation interface with built-in commands

### Scenario-Based ReAct Workflow

GENNAI CLI uses a simplified scenario-based approach:

```
User Request → Scenario Assignment → ReAct Execution
                      ↓                     ↓
              ┌─────────────────┐  ┌─────────────────────┐
              │   Default or    │  │  ReAct with Tool    │
              │ User-Specified  │  │  Calling Execution  │
              │    Scenario     │  │                     │
              └─────────────────┘  └─────────────────────┘
```

**Current Implementation:**
- **Default Scenario**: Uses 'code' scenario by default for comprehensive development tasks
- **Manual Override**: Users can specify scenario with `-s` flag (e.g., `-s respond`)
- **Direct Execution**: No complex scenario selection logic - simple and reliable

**Available Scenarios:**
- `code` - Comprehensive coding assistant for all development tasks (generation + analysis + debug + test + refactor)
- `respond` - Direct knowledge-based responses without tool usage

**Architecture Benefits:**
- **Simplicity**: Straightforward scenario assignment without complex selection logic
- **Predictability**: Users know exactly which scenario will be used
- **Performance**: No overhead from scenario selection algorithms
- **Reliability**: Fewer points of failure in the execution pipeline

### Capability-Based Design

The system uses a **capability-based architecture** with type assertion for clean capability detection:

**Interface Hierarchy:**
- `domain.LLM` - Base interface for basic chat functionality
- `domain.ToolCallingLLM` - Extends LLM with tool calling capabilities  
- `domain.StructuredLLM[T any]` - Extends LLM with type-safe structured output

**Type Assertion Pattern:**
```go
// Check for tool calling capability  
if toolClient, ok := client.(domain.ToolCallingLLM); ok {
    response, err := toolClient.ChatWithToolChoice(ctx, messages, toolChoice)
}
```

**Benefits:**
- **Type Safety**: Compile-time guarantees for capability existence
- **Clean Interfaces**: No redundant boolean methods
- **Go Idioms**: Follows standard Go patterns for capability detection
- **Self-Documenting**: Capabilities are clear through interface compliance

## Tool Calling Support

GENNAI CLI uses native tool calling for all supported models:

- **API-level tool definitions** with structured tool_use/tool_result blocks
- **Efficient and reliable** tool execution
- **Automatic capability detection** via dynamic testing for unknown models
- **Unified interface** across all LLM backends (Ollama, Anthropic, OpenAI, Gemini)

The system automatically tests unknown Ollama models for tool calling capability and warns users if a model lacks this support, ensuring you always know what functionality is available.

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test -v ./pkg/llmclient/react/

# Run tests with make
make test
```

### Code Quality

```bash
# Format code
go fmt ./...

# Run linter
go vet ./...

# Clean up dependencies
go mod tidy
```

### Project Structure

```
go-gennai-cli/
├── gennai/             # Main application
│   └── main.go                # CLI entry point
├── doc/                    # Documentation
│   └── DEVELOPMENT.md         # This file
├── internal/               # Internal application code
│   ├── agent/                 # ScenarioRunner implementation
│   ├── config/                # Configuration management
│   ├── infra/                 # Infrastructure components
│   ├── mcp/                   # MCP integration
│   ├── scenarios/             # Built-in scenarios
│   └── tool/                  # Tool management
├── pkg/agent/              # Reusable agent components
│   ├── domain/                # Domain interfaces
│   ├── mcp/                   # MCP client implementation
│   ├── react/                 # ReAct pattern implementation
│   └── state/                 # State management
├── pkg/client/             # Client factory and implementations
│   ├── anthropic/             # Anthropic client implementation
│   ├── gemini/                # Google Gemini client implementation  
│   ├── ollama/                # Ollama client implementation
│   └── openai/                # OpenAI client implementation
└── pkg/message/            # Message handling and types
```

### Available Models

**Ollama Models:**
- `gpt-oss:latest` - **Recommended** - Best balanced model with native tool calling

**Anthropic Models:**
- `claude-3-7-sonnet-latest` - Default Claude model with native tool calling
- `claude-3-5-haiku-latest` - Faster Claude model with native tool calling
- `claude-sonnet-4-20250514` - Latest Claude Sonnet 4 with native tool calling

**OpenAI Models:**
- `gpt-4o` - Latest GPT-4 Omni (vision, tool calling, structured output)
- `gpt-4o-mini` - Smaller, faster GPT-4 Omni
- `gpt-3.5-turbo` - Fast and cost-effective for most tasks

**Google Gemini Models:**
- `gemini-2.5-flash-lite` - **Recommended** - Latest, fastest, most efficient
- `gemini-1.5-pro` - High capability model for complex reasoning
- `gemini-2.0-flash` - Latest experimental features

### Development Tasks

**One-shot Mode Testing:**
```bash
# Code analysis
go run gennai/main.go "Analyze the architecture of this Go project"

# Code generation
go run gennai/main.go "Create a REST API with user authentication"

# Testing
go run gennai/main.go "Write unit tests for the react package"

# Refactoring
go run gennai/main.go "Refactor this code to use dependency injection"
```

**Interactive Mode Testing:**
```bash
# Start interactive mode
go run gennai/main.go

# Then use commands like:
> Create a HTTP server with health check
> Analyze the current codebase structure
> Write unit tests for the ScenarioRunner
> List files in the current directory
> Run go build and fix any errors
> /help    # Show available commands
> /clear   # Clear conversation history
> /quit    # Exit interactive mode
```

### Integration Testing

Test different scenarios to evaluate the scenario-based system:

1. **CODE Scenario (Default)**: `go run gennai/main.go "Create a new Go HTTP server with health check endpoint"`
2. **CODE Scenario (Tools)**: `go run gennai/main.go "List all Go files and analyze their purposes in this project"`
3. **RESPOND Scenario**: `go run gennai/main.go -s respond "Explain the difference between channels and mutexes in Go"`
4. **RESPOND Scenario (Knowledge)**: `go run gennai/main.go -s respond "What are Go best practices for error handling?"`

### Security Testing

1. **Configuration Validation**: Test settings.json validation and error handling
2. **MCP Server Integration**: Verify MCP server connection failures are handled gracefully
3. **Tool Capability Detection**: Test automatic detection of model tool calling capabilities
4. **API Key Management**: Ensure API keys are never logged or exposed in debug output

### Unit Testing Coverage

The project includes comprehensive unit tests for:

**Scenario System Testing:**
- YAML scenario loading and parsing
- Template variable substitution
- Tool scope configuration parsing
- Scenario assignment and execution

**Configuration Testing:**
- Settings validation and loading
- MCP server configuration validation
- Default value application

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

**Tool Schema Testing:**
Comprehensive test suites for tool schema handling:
- Schema-as-tool pattern validation
- JSON Schema to API format conversion
- Required fields extraction and handling
- Tool serialization/deserialization for API communication
- Type safety validation (Go types → JSON Schema → API format)

### Integration Testing

Tests use mocked dependencies to ensure:
- Clean scenario-based execution flow
- Scenario-specific tool isolation
- YAML-driven prompt generation
- Configuration validation and error handling
- Tool schema consistency across different model capabilities

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run unit tests and ensure they pass: `make test`
6. Run integration tests: `make integ`
7. Run code quality checks: `make lint`
8. Submit a pull request

### Integration Test Suite

The project includes comprehensive integration tests in the `testsuite/` directory:

```bash
# Run all integration tests
make integ

# Run specific integration tests
cd testsuite
./runner.sh <scenario> <backend>

# Examples:
./runner.sh planning anthropic    # Test planning with Anthropic
./runner.sh code ollama           # Test code scenario with Ollama  
./runner.sh respond openai        # Test respond scenario with OpenAI
```

**Integration Test Categories:**
- **Planning Capability**: Tests the ability to break down complex tasks
- **Scenario Execution**: Tests CODE and RESPOND scenario workflows  
- **Backend Compatibility**: Tests all LLM backends (Ollama, Anthropic, OpenAI, Gemini)
- **Tool Integration**: Tests tool calling across different scenarios
- **Configuration Management**: Tests settings.json and MCP configuration

**Before Submitting PRs:**
- Ensure all unit tests pass (`make test`)
- Run integration tests with `make integ` or test specific scenarios/backends with `./runner.sh`
- Update integration tests if you modify scenario behavior or add new backends

## Build System

### Makefile Targets

```bash
make test          # Run all unit tests
make integ         # Run all integration tests
make build         # Build the binary
make install       # Install to $GOPATH/bin
make clean         # Clean build artifacts
make deps          # Download dependencies
```

### Module Information

- **Module**: `github.com/fpt/go-gennai-cli`
- **Go Version**: 1.24.4
- **Key Dependencies**: 
  - `github.com/ollama/ollama v0.5.3`
  - `github.com/anthropics/anthropic-sdk-go v1.5.0`
  - `github.com/openai/openai-go v1.12.0`
  - `google.golang.org/genai v1.19.0`
  - `github.com/chzyer/readline v1.5.1`
  - `github.com/mark3labs/mcp-go`

## Reference

- **Anthropic SDK**: https://pkg.go.dev/github.com/anthropics/anthropic-sdk-go
- **Ollama API**: https://pkg.go.dev/github.com/ollama/ollama/api
- **OpenAI SDK**: https://pkg.go.dev/github.com/openai/openai-go
- **Google Generative AI**: https://pkg.go.dev/google.golang.org/genai
- **MCP Protocol**: https://github.com/mark3labs/mcp-go
- **ReAct Pattern**: https://arxiv.org/abs/2210.03629