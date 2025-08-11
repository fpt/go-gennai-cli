# OpenAI Integration Example

This example shows how to use OpenAI models with gennai.

## Setup

1. **Install gennai:**
   ```bash
   go build -o gennai ./gennai
   ```

2. **Set your OpenAI API key:**
   ```bash
   export OPENAI_API_KEY="your-api-key-here"
   ```

## Usage

### Interactive Mode

```bash
# Using GPT-4o (default OpenAI model)
gennai -b openai

# Using GPT-4o Mini for faster/cheaper responses  
gennai -b openai -m gpt-4o-mini

# Using GPT-3.5 Turbo
gennai -b openai -m gpt-3.5-turbo
```

### One-shot Mode

```bash
# Generate code with OpenAI
gennai -b openai "Create a simple HTTP server in Go with health check endpoint"

# Analyze code with GPT-4o
gennai -b openai -m gpt-4o "Analyze this codebase and suggest improvements"

# Use with custom scenarios
gennai -b openai --scenarios ./my-scenarios.yaml "Custom analysis task"
```

### Configuration File

Create a `settings.json` file:

```json
{
  "llm": {
    "backend": "openai",
    "model": "gpt-4o",
    "thinking": false
  },
  "agent": {
    "max_iterations": 10,
    "timeout_seconds": 300,
    "log_level": "info"
  }
}
```

## Supported Models

### GPT-4 Models
- `gpt-4o` - Latest GPT-4 Omni (vision, tool calling, structured output)
- `gpt-4o-mini` - Smaller, faster GPT-4 Omni  
- `gpt-4-turbo` - GPT-4 Turbo (tool calling, structured output)
- `gpt-4` - Original GPT-4 (basic chat only)
- `gpt-4-vision-preview` - GPT-4 with vision (no tool calling)

### GPT-3.5 Models
- `gpt-3.5-turbo` - GPT-3.5 Turbo (tool calling, structured output)

### Reasoning Models
- `o1-preview` - OpenAI o1 reasoning model (structured output only)
- `o1-mini` - Smaller o1 model (structured output only)

## Features

### Current Support
- ✅ **Basic Chat** - Works with all models
- ✅ **Model Validation** - Automatic model name mapping and validation  
- ✅ **Vision Support** - Automatic detection for GPT-4V models
- ✅ **Advanced Structured Output** - Full JSON Schema validation with `ResponseFormatJSONSchemaParam`
- ✅ **Factory Integration** - Complete integration with tool calling and structured output factories
- ✅ **Schema Generation** - Automatic Go struct → OpenAI JSON Schema conversion using reflection
- ✅ **Strict Validation** - Uses OpenAI's strict mode for reliable structured output

### Enhanced Implementation
- ✅ **Reflection-Based Schema** - Automatically converts Go types to OpenAI-compatible JSON schemas
- ✅ **JSON Tag Support** - Respects `json:` tags and `omitempty` directives  
- ✅ **Description Support** - Uses `description:` struct tags for schema documentation
- ✅ **Required Fields** - Automatically determines required vs optional fields
- ✅ **Type Safety** - Full compile-time type safety with generics

### Coming Soon
- 🚧 **Tool Calling** - Native OpenAI function calling (fallback to basic chat for now)
- 🚧 **Vision Input** - Image processing capabilities
- 🚧 **Streaming** - Real-time response streaming

## Environment Variables

- `OPENAI_API_KEY` - Your OpenAI API key (required)
- `OPENAI_BASE_URL` - Custom base URL (optional, for Azure OpenAI or proxies)

## Error Handling

The OpenAI client includes robust error handling:

- **API Key Validation** - Checks for `OPENAI_API_KEY` at startup
- **Model Capability Validation** - Ensures models support requested features  
- **Graceful Fallbacks** - Tool calling falls back to basic chat mode
- **Structured Output Parsing** - Handles JSON parsing errors gracefully

## Performance

### Model Recommendations

**For Speed:**
- `gpt-4o-mini` - Fast and cost-effective
- `gpt-3.5-turbo` - Fastest for simple tasks

**For Quality:**
- `gpt-4o` - Best overall performance
- `o1-preview` - Best for complex reasoning

**For Vision:**
- `gpt-4o` - Supports images and vision tasks
- `gpt-4-vision-preview` - Vision-only model

## Integration with Scenarios

All OpenAI models work seamlessly with gennai's scenario system:

```bash
# Code generation with GPT-4o
gennai -b openai "Create a REST API with user authentication"

# Code analysis with o1-preview (reasoning model)
gennai -b openai -m o1-preview "Analyze this architecture and suggest optimizations"

# File system operations (uses universal tools)
gennai -b openai "List all Go files and create a project structure diagram"
```

The scenario runner automatically selects appropriate tools based on your request, whether you're using OpenAI, Anthropic, or Ollama models.