# LLM Provider Comparison

This document compares all supported LLM providers in gennai, their capabilities, and recommended use cases.

## Quick Reference

| Provider | Backend Flag | API Key | Default Model | Structured Output | Tool Calling | Vision |
|----------|--------------|---------|---------------|-------------------|--------------|---------|
| **Ollama** | `-b ollama` | None | `gpt-oss:latest` | ✅ Native Tools | ✅ Native | ✅ LLaVA |
| **Anthropic** | `-b anthropic` | `ANTHROPIC_API_KEY` | `claude-sonnet-4` | ✅ Schema-as-Tool | ✅ Native | ✅ Built-in |
| **OpenAI** | `-b openai` | `OPENAI_API_KEY` | `gpt-4o` | ✅ JSON Schema | 🚧 Fallback | ✅ GPT-4V+ |
| **Gemini** | `-b gemini` | `GEMINI_API_KEY` | `gemini-2.5-flash-lite` | ✅ Native Schema | 🚧 Fallback | ✅ Built-in |

## Detailed Comparison

### Structured Output

| Provider | Method | Schema Generation | Validation | Reliability |
|----------|--------|-------------------|------------|-------------|
| **Gemini** | `ResponseSchema` | ✅ Automatic reflection | ✅ Native validation | ⭐⭐⭐⭐⭐ |
| **OpenAI** | `ResponseFormatJSONSchema` | ✅ Automatic reflection | ✅ Strict mode | ⭐⭐⭐⭐⭐ |
| **Anthropic** | Schema-as-Tool pattern | ✅ Automatic reflection | ✅ Tool validation | ⭐⭐⭐⭐ |
| **Ollama** | Native tool calling | ✅ Automatic schema | ✅ Native validation | ⭐⭐⭐⭐ |

### Tool Calling

| Provider | Implementation | Status | Reliability |
|----------|----------------|--------|-------------|
| **Anthropic** | Native API tool calling | ✅ Complete | ⭐⭐⭐⭐⭐ |
| **Ollama** | Native API tool calling | ✅ Complete | ⭐⭐⭐⭐ |
| **OpenAI** | Native API (planned) | 🚧 Fallback to chat | ⭐⭐⭐ |
| **Gemini** | Native API (planned) | 🚧 Fallback to chat | ⭐⭐⭐ |

### Performance & Cost

| Provider | Speed | Cost | Local | API Limits |
|----------|-------|------|-------|------------|
| **Ollama** | ⭐⭐⭐ Local | ✅ Free | ✅ Yes | None |
| **Gemini** | ⭐⭐⭐⭐⭐ Very Fast | ⭐⭐⭐⭐ Low | ❌ No | Generous |
| **Anthropic** | ⭐⭐⭐⭐ Fast | ⭐⭐ Higher | ❌ No | Moderate |
| **OpenAI** | ⭐⭐⭐⭐ Fast | ⭐⭐ Higher | ❌ No | Moderate |

### Vision Capabilities

| Provider | Vision Models | Image Support | Performance |
|----------|---------------|---------------|-------------|
| **Gemini** | 2.5, 2.0, 1.5-pro | ✅ Built-in | ⭐⭐⭐⭐⭐ |
| **Anthropic** | Claude 3+ | ✅ Built-in | ⭐⭐⭐⭐ |
| **OpenAI** | GPT-4V, GPT-4o | ✅ Built-in | ⭐⭐⭐⭐ |
| **Ollama** | LLaVA models | ✅ Local | ⭐⭐⭐ |

## Use Case Recommendations

### For Development & Learning
**Recommended: Ollama**
```bash
go run gennai/main.go -b ollama -m gpt-oss "Help me learn Go"
```
- ✅ Free and private
- ✅ No API limits
- ✅ Good for experimentation
- ✅ Works offline

### For Production Applications
**Recommended: Gemini 2.5 Flash Lite**
```bash
go run gennai/main.go -b gemini -m gemini-2.5-flash-lite "Generate production code"
```
- ✅ Excellent performance/cost ratio
- ✅ Reliable structured output
- ✅ Fast responses
- ✅ Latest features

### For Complex Reasoning
**Recommended: Anthropic Claude Sonnet 4**
```bash
go run gennai/main.go -b anthropic -m claude-sonnet-4 "Analyze complex architecture"
```
- ✅ Superior reasoning capabilities
- ✅ Excellent tool calling
- ✅ Long context support
- ✅ Reliable and stable

### For Cost-Sensitive Applications
**Recommended: Gemini 2.5 Flash Lite or OpenAI GPT-4o Mini**
```bash
go run gennai/main.go -b gemini -m gemini-2.5-flash-lite "Quick coding task"
go run gennai/main.go -b openai -m gpt-4o-mini "Simple analysis"
```
- ✅ Low cost per token
- ✅ Good performance
- ✅ Reliable structured output

### For Vision Tasks
**Recommended: Gemini 2.5 Flash Lite or GPT-4o**
```bash
go run gennai/main.go -b gemini -m gemini-2.5-flash-lite "Analyze this image"
go run gennai/main.go -b openai -m gpt-4o "Describe this image"
```
- ✅ Excellent vision capabilities
- ✅ Multimodal understanding
- ✅ Good vision + reasoning combination

## Environment Setup Examples

### Multi-Provider Setup
```bash
# Set up all providers for maximum flexibility
export ANTHROPIC_API_KEY="your-anthropic-key"
export OPENAI_API_KEY="your-openai-key"  
export GEMINI_API_KEY="your-gemini-key"
# Ollama requires local installation

# Now you can switch providers easily:
gennai -b anthropic "Complex reasoning task"
gennai -b gemini "Fast structured output task"
gennai -b openai "GPT-4o specific task"
gennai -b ollama "Private local task"
```

### Development Workflow
```bash
# Local development with Ollama
gennai -b ollama -m gpt-oss "Help me prototype this feature"

# Production code generation with Gemini
gennai -b gemini "Generate production-ready REST API"

# Complex analysis with Claude
gennai -b anthropic "Review architecture and suggest improvements"

# Quick fixes with OpenAI
gennai -b openai -m gpt-4o-mini "Fix this bug quickly"
```

## Implementation Status

### ✅ Complete
- **Ollama**: Full implementation with native tool calling (gpt-oss model)
- **Anthropic**: Full implementation with native tool calling and schema-as-tool
- **Gemini**: Enhanced structured output with native schema support
- **OpenAI**: Enhanced structured output with JSON Schema validation

### 🚧 In Progress  
- **OpenAI Tool Calling**: Native function calling API integration
- **Gemini Tool Calling**: Native function calling API integration
- **Vision Input**: Image processing for all providers
- **Streaming**: Real-time response streaming

### 🎯 Future Enhancements
- **Multi-modal Input**: Audio, video support
- **Batch Processing**: Efficient bulk operations
- **Fine-tuning**: Custom model training integration
- **Cost Optimization**: Intelligent provider routing based on cost/performance

## Migration Guide

### From Ollama-only to Multi-provider
1. **Add API keys** for desired providers
2. **Update settings.json** to specify preferred backend
3. **Test scenarios** with different providers
4. **Optimize** based on use case requirements

### Provider Selection Strategy
1. **Start with Gemini** for best overall balance
2. **Use Claude** for complex reasoning tasks
3. **Use Ollama** for privacy and local development
4. **Use OpenAI** for compatibility with existing workflows

This comparison helps you choose the right provider for each use case while maintaining the flexibility to switch between them as needed.