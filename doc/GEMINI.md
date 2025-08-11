# Gemini Integration

This document describes the complete Google Gemini integration with gennai, featuring advanced structured output and comprehensive model support.

## Setup

1. **Install gennai:**
   ```bash
   go build -o gennai ./gennai
   ```

2. **Set your Gemini API key:**
   ```bash
   export GEMINI_API_KEY="your-api-key-here"
   ```

## Usage

### Interactive Mode

```bash
# Using Gemini 2.5 Flash Lite (default)
gennai -b gemini

# Using Gemini 2.5 Flash Lite explicitly
gennai -b gemini -m gemini-2.5-flash-lite

# Using Gemini 1.5 Pro for advanced tasks
gennai -b gemini -m gemini-1.5-pro

# Using Gemini 2.0 Flash for latest features
gennai -b gemini -m gemini-2.0-flash
```

### One-shot Mode

```bash
# Generate code with Gemini
gennai -b gemini "Create a simple HTTP server in Go with health check endpoint"

# Analyze code with Gemini 1.5 Pro
gennai -b gemini -m gemini-1.5-pro "Analyze this codebase architecture"

# Use with custom scenarios  
gennai -b gemini --scenarios ./my-scenarios.yaml "Custom analysis task"
```

### Configuration File

Create a `settings.json` file:

```json
{
  "llm": {
    "backend": "gemini",
    "model": "gemini-2.5-flash-lite", 
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

### Gemini 2.5 Models (Latest)
- `gemini-2.5-flash-lite` - **Preferred model** - Latest, fastest, and most efficient
- `gemini-2.5-lite` - Alias for gemini-2.5-flash-lite

### Gemini 2.0 Models  
- `gemini-2.0-flash-exp` - Experimental Gemini 2.0 model
- `gemini-2.0-flash` - Stable Gemini 2.0 model
- `gemini-2.0` - Alias for gemini-2.0-flash

### Gemini 1.5 Models
- `gemini-1.5-pro` - High capability model for complex tasks
- `gemini-1.5-flash` - Fast and capable model
- `gemini-1.5-flash-8b` - Efficient 8B parameter model

### Gemini 1.0 Models  
- `gemini-1.0-pro` - Original Gemini Pro model
- `gemini-1.0-pro-vision` - Vision-enabled model
- `gemini-pro` - Alias for gemini-1.0-pro

## Features

### Current Support
- ✅ **Basic Chat** - Works with all models
- ✅ **Model Validation** - Automatic model name mapping and validation
- ✅ **Vision Support** - Available on supported models (2.5, 2.0, 1.5-pro)
- ✅ **Advanced Structured Output** - Full schema validation with `ResponseSchema`
- ✅ **Factory Integration** - Complete integration with tool calling and structured output factories  
- ✅ **Scenario Planning** - Works perfectly with scenario selection system
- ✅ **Schema Generation** - Automatic Go struct → Gemini Schema conversion using reflection

### Enhanced Implementation
- ✅ **Native Schema Support** - Uses Gemini's `ResponseSchema` for structured output
- ✅ **Reflection-Based Schema** - Automatically converts Go types to Gemini-compatible schemas
- ✅ **JSON Tag Support** - Respects `json:` tags and field ordering
- ✅ **Description Support** - Uses `description:` struct tags for field documentation
- ✅ **System Instructions** - Proper handling of system prompts and instructions
- ✅ **Multimodal Support** - Ready for image and multimedia input

### Model Capabilities

| Model | Vision | Tool Calling | Structured Output | Max Tokens | Use Case |
|-------|--------|--------------|-------------------|------------|----------|
| gemini-2.5-flash-lite | ✅ | ✅ | ✅ | 8192 | **Preferred** - Fast, efficient |  
| gemini-2.0-flash | ✅ | ✅ | ✅ | 8192 | Latest experimental features |
| gemini-1.5-pro | ✅ | ✅ | ✅ | 8192 | Complex reasoning tasks |
| gemini-1.5-flash | ✅ | ✅ | ✅ | 8192 | Balanced speed/capability |
| gemini-1.0-pro | ❌ | ✅ | ✅ | 4096 | Basic text tasks |

### Coming Soon
- 🚧 **Native Tool Calling** - Full Gemini function calling (currently falls back to chat)
- 🚧 **Vision Input** - Image processing capabilities
- 🚧 **Streaming** - Real-time response streaming

## Environment Variables

- `GEMINI_API_KEY` - Your Google AI API key (required)
- No additional configuration needed

## Error Handling

The Gemini client includes comprehensive error handling:

- **API Key Validation** - Checks for `GEMINI_API_KEY` at startup
- **Model Capability Validation** - Ensures models support requested features
- **Schema Generation Errors** - Detailed error messages for schema conversion issues
- **Graceful Fallbacks** - Tool calling falls back to basic chat mode
- **Response Validation** - Handles malformed responses and parsing errors

## Performance

### Model Recommendations

**For Speed & Efficiency (Recommended):**
- `gemini-2.5-flash-lite` - Fastest, most cost-effective, preferred choice

**For Complex Tasks:**  
- `gemini-1.5-pro` - Best for reasoning and analysis
- `gemini-2.0-flash` - Latest features and capabilities

**For Vision Tasks:**
- `gemini-2.5-flash-lite` - Best overall vision performance
- `gemini-1.5-pro` - Advanced vision understanding

**For Legacy Support:**
- `gemini-1.0-pro` - Basic text-only tasks

## Structured Output Examples

### Basic Usage
```go
type CodeResponse struct {
    Language string   `json:"language" description:"Programming language"`
    Code     string   `json:"code" description:"Generated code"`  
    Steps    []string `json:"steps,omitempty" description:"Implementation steps"`
}

// Automatic schema generation and validation
client := gemini.NewGeminiStructuredClient[CodeResponse](model)
result, err := client.InvokeStructuredOutput(ctx, messages)
// result is guaranteed to have CodeResponse structure
```

### Advanced Schema Features
```go
type AnalysisResult struct {
    Summary     string            `json:"summary" description:"Brief analysis summary"`
    Issues      []Issue           `json:"issues" description:"Identified issues"`  
    Metrics     map[string]int    `json:"metrics,omitempty" description:"Code metrics"`
    Confidence  float64           `json:"confidence" description:"Analysis confidence (0-1)"`
    Recommended bool              `json:"recommended" description:"Whether changes are recommended"`
}

type Issue struct {
    Type        string `json:"type" description:"Issue category"`
    Severity    string `json:"severity" description:"Issue severity level"`
    Description string `json:"description" description:"Detailed description"`
    Line        int    `json:"line,omitempty" description:"Line number if applicable"`
}
```

## Integration with Scenarios

All Gemini models work seamlessly with gennai's scenario system:

```bash
# Code generation with structured output
gennai -b gemini "Create a REST API with user authentication"

# Complex analysis with reasoning
gennai -b gemini -m gemini-1.5-pro "Analyze this architecture and suggest improvements"

# File system operations with universal tools
gennai -b gemini "List all Go files and create a project structure diagram" 

# Research and web integration
gennai -b gemini "Research Go best practices for microservices"
```

## API Compatibility

The integration uses the official Google GenAI Go SDK (`google.golang.org/genai`) with:
- `genai.NewClient()` with proper ClientConfig
- `client.Models.GenerateContent()` for content generation  
- `ResponseSchema` for structured output validation
- Proper content formatting with role-based messages

## Comparison with Other Providers

| Feature | Gemini | OpenAI | Anthropic | Ollama |
|---------|--------|---------|-----------|---------|
| Structured Output | ✅ Native Schema | ✅ JSON Schema | ✅ Schema-as-Tool | ✅ BNF Grammar |
| Schema Generation | ✅ Automatic | ✅ Automatic | ✅ Automatic | ✅ Automatic |
| Vision Support | ✅ Built-in | ✅ GPT-4V+ | ✅ Claude 3+ | ✅ LLaVA models |
| Speed | ✅ Very Fast | ✅ Fast | ✅ Fast | ✅ Local |
| Cost | ✅ Competitive | 💰 Higher | 💰 Higher | ✅ Free |

Gemini provides an excellent balance of performance, features, and cost-effectiveness, making it ideal for production scenarios requiring reliable structured output.