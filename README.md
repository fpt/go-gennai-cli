# GENNAI CLI

A CLI-based AI coding agent supporting multiple LLM backends, using ReAct (Reason and Act) pattern and MessageState with compaction to interact with tools, maintaining context.

Default scenario focuses on coding tasks with ToDo management, various built-in tools, and user-configured tools via MCP client functionality.

The name, GENNAI comes from both 'GENeric ageNt for AI' and Gennai Hiraga, a historic inventor of Japan. In my opinion, ReAct agent acts like Karakuri.

## Features

- **Interactive Mode**: REPL-style interface for continuous interaction with conversation memory
- **Multiple LLM Backends**: Support for Ollama (gpt-oss), Anthropic Claude, OpenAI GPT, and Google Gemini (Experimental)
- **Simplified ReAct Pattern**: Streamlined reasoning and acting with single-action loops for better performance
- **Integrated Tools**: File operations, grep search, bash tools, todo tools, and simple web tools
- **MCP Server Support**: MCP Servers can be configured in settings.json
- **Conversation State Management**: Automatic handling of conversation history and context
- **Token Usage & Caching Foundations**: Per‑client token usage reporting (OpenAI supported) and provider‑native caching hints (session/prompt caching)
- **Clean Output System**: ScenarioRunner streams thinking to an injected `io.Writer`; logs use Info/Debug intentions for console icons while file logs store structured `intention` fields.

## Quick Start

### Installation

```bash
go install github.com/fpt/go-gennai-cli/gennai@latest
```

### Prerequisites

**For Ollama (default):**
- Install Ollama: https://ollama.ai/
- Pull a model: `ollama pull gpt-oss:latest`

**For Anthropic Claude:**
- Set `ANTHROPIC_API_KEY` environment variable

**For OpenAI:**
- Set `OPENAI_API_KEY` environment variable

NOTE: OpenAI is not tested at this moment.

**For Google Gemini (has limitation):**
- Set `GEMINI_API_KEY` environment variable

### Basic Usage

**Interactive Mode (default):**
```bash
# Start interactive REPL
go run gennai/main.go

# Interactive with Anthropic Claude
go run gennai/main.go -b anthropic
```

**One-shot Mode:**
```bash
# Run single command with default model
go run gennai/main.go "Create a HTTP server with health check endpoint"

# Use different backends
go run gennai/main.go -b anthropic "Analyze this codebase"
go run gennai/main.go -b openai -m gpt-4o "Create a REST API"
go run gennai/main.go -b gemini -m gemini-2.5-flash-lite "Optimize this code"

# Work in specific directory
go run gennai/main.go -workdir testbench "Create a simple web server"
```

### Build and Install

```bash
# Build the binary
go build -o gennai ./gennai

# Run the binary
./gennai "Your task description here"
```

## Configuration

### Unified Settings (settings.json)

GENNAI CLI uses a unified configuration system with settings stored in `~/.gennai/settings.json`. 

**Automatic Setup**: When you first run GENNAI, it automatically creates a default `~/.gennai/settings.json` file with example configurations that you can easily modify.

**💡 To enable MCP servers**: Change `"enabled": false` to `"enabled": true` and update the server configuration with your actual MCP server details.

### Configuration Management

**Automatic Configuration Search:**
1. `.gennai/settings.json` in current directory
2. `$HOME/.gennai/settings.json` in home directory  
3. Defaults if no configuration found

**Override with Command Line:**
```bash
# Override backend and model
gennai -b anthropic -m claude-3-7-sonnet-latest "Analyze this code"

# Use custom settings file
gennai --settings ./my-settings.json "Create a web server"
```

### MCP (Model Context Protocol) Integration

**MCP Server Configuration:**
- **stdio servers**: External processes communicating via stdin/stdout
- **SSE servers**: HTTP Server-Sent Events endpoints
- **Allowed Tools (optional)**: Limit context size by specifying only needed tools. If omitted, all tools from the server are allowed.
- **Environment Variables**: Set per-server environment

**Example MCP Servers:**

This is an example of [godevmcp](https://github.com/fpt/go-dev-mcp).

```json
{
  "mcp": {
    "servers": [
      {
        "name": "godevmcp", 
        "enabled": true,
        "type": "stdio",
        "command": "godevmcp",
        "args": ["serve"],
        // Optional: if omitted, all server tools are allowed
        "allowed_tools": ["tree_dir", "search_local_files", "read_godoc"]
      }
    ]
  }
}
```

**MCP Features:**
- **Graceful Degradation**: Continues running if MCP servers fail to connect
- **Per-Scenario Access**: Only scenarios that explicitly request MCP tools get access
- **Multiple Server Support**: Connect to multiple MCP servers simultaneously
- **Tool Filtering**: `allowed_tools` is optional; by default all tools are allowed. Use it to reduce context size and improve performance when needed.

## Supported Models

- **Anthropic (Recommended)**: `claude-sonnet-4-20250514`, `claude-opus-4-20250514`, `claude-3-7-sonnet-latest`, `claude-3-5-haiku-latest`
- **OpenAI**: `gpt-5`, `gpt-5-mini`, `gpt-4o`, `gpt-4o-mini`
- **Ollama**: `gpt-oss:latest`, `gpt-oss:120b`
- **Google Gemini (Not recommended)**: `gemini-2.5-pro`, `gemini-2.5-flash`, `gemini-2.5-flash-lite`

## Example Usage

**Scenario Selection:**
GENNAI CLI uses the 'code' scenario by default for comprehensive development tasks:

```bash
# All these use the default 'code' scenario
go run gennai/main.go "Create a HTTP server"
go run gennai/main.go "Fix compilation errors" 
go run gennai/main.go "Write unit tests"
go run gennai/main.go "Analyze this codebase"
go run gennai/main.go "List files in this directory"

# Use 'respond' scenario for knowledge-based responses
go run gennai/main.go -s respond "Explain channels in Go"
go run gennai/main.go -s respond "What are Go best practices?"
```

**Interactive Mode:**
```bash
# Start interactive mode
go run gennai/main.go

# Then use commands like:
> Create a HTTP server with health check
> Analyze the current codebase structure  
> Write unit tests for this package
> List files in the current directory
> Run go build and fix any errors
> /help    # Show available commands
> /clear   # Clear conversation history
> /quit    # Exit interactive mode
```

## Development

### Output and Logging

- **ScenarioRunner Writer**: The runner accepts an `io.Writer` and streams thinking output to it (REPL, tests, or gRPC).
- **Unified Console Writer**: The global logger can target the same writer via `SetGlobalLoggerWithConsoleWriter` so agent logs and thinking share one sink.
- **Intentions**: Only Info/Debug logs carry an `intention` tag (e.g., `tool`, `status`, `statistics`); Warn/Error rely purely on level.
- **Console vs File Logs**: Console renders icons based on intention; file logs are plain with `intention=...` as structured metadata in `~/.gennai/logs/gennai.log`.
- **Model-Facing Text**: Tool outputs returned to models avoid emojis and use plain PASS/FAIL/ERROR phrasing.

For detailed development information, architecture details, and contributing guidelines, see:

**[📖 Development Guide](doc/DEVELOPMENT.md)**

This includes:
- Architecture overview and design patterns
- Structured output system with generics
- Token usage reporting and provider‑native caching hooks
- Testing and code quality guidelines
- Project structure and contribution workflow
- Model capabilities and integration testing

## ⚠️ Important Notices

### Token Usage & Provider‑Native Caching
- GENNAI exposes a foundation for per‑client token usage monitoring and provider‑native caching hints (no local cache layer).
- OpenAI (Responses API) currently reports token usage; other clients will add support when their SDKs expose usage.
- Provider‑side prompt caching (e.g., OpenAI Prompt Caching) can be enabled via client options when supported by the SDK.
  - Reference: https://platform.openai.com/docs/guides/prompt-caching

### Responsible Use
- This tool is provided for research and development purposes
- Users are responsible for complying with LLM provider terms of service and applicable laws
- Users must ensure their API usage adheres to rate limits and usage policies
- Malicious use is strictly prohibited

### Security Best Practices
- **Never hardcode API keys** - always use environment variables:
  ```bash
  export ANTHROPIC_API_KEY="your_anthropic_key"
  export OPENAI_API_KEY="your_openai_key"  
  export GEMINI_API_KEY="your_gemini_key"
  ```
- Keep your API keys secure and rotate them regularly
- Be cautious when sharing configurations, logs, or screenshots that might contain sensitive information
- Review AI-generated code before using it in production systems

### Model Capability Warnings
gennai automatically tests unknown Ollama models for tool-calling capability:
- ✅ **Known compatible models** (like `gpt-oss:latest`) work without testing
- ⚠️ **Unknown models** are tested automatically with clear warnings about limitations
- 🚫 **Non-tool-capable models** will have limited functionality (no file operations, web search, etc.)

### Disclaimer
This software is provided "as is" under the Apache 2.0 License without warranty of any kind. The developers are not responsible for any damage, data loss, API costs, or misuse resulting from the use of this software.

## License

This project is licensed under the Apache 2.0 License.
