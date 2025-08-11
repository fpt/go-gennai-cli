package client

import (
	"fmt"

	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/client/anthropic"
	"github.com/fpt/go-gennai-cli/pkg/client/gemini"
	"github.com/fpt/go-gennai-cli/pkg/client/ollama"
	"github.com/fpt/go-gennai-cli/pkg/client/openai"
)

// NewClientWithToolManager creates a tool calling client appropriate for the underlying LLM client
// Takes a base LLM client and adds tool management capabilities to it
func NewClientWithToolManager(client domain.LLM, toolManager domain.ToolManager) (domain.ToolCallingLLM, error) {
	// Check if the client is already a tool calling client
	if toolCallingClient, ok := client.(domain.ToolCallingLLM); ok {
		// Set the tool manager and return
		toolCallingClient.SetToolManager(toolManager)
		return toolCallingClient, nil
	}

	// Determine the appropriate tool calling client based on the client type
	switch c := client.(type) {
	case *ollama.OllamaClient:
		// For Ollama clients, use the embedded OllamaCore to create a new tool calling client
		// This will automatically choose between native tool calling or schema-based based on model capabilities
		toolClient := ollama.NewOllamaClientFromCore(c.OllamaCore)
		toolClient.SetToolManager(toolManager)
		return toolClient, nil
	case *anthropic.AnthropicClient:
		// For Anthropic clients, use the embedded AnthropicCore to create a new tool calling client
		toolClient := anthropic.NewAnthropicClientFromCore(c.AnthropicCore)
		toolClient.SetToolManager(toolManager)
		return toolClient, nil
	case *openai.OpenAIClient:
		// For OpenAI clients, use the embedded OpenAICore to create a new tool calling client
		toolClient := openai.NewOpenAIClientFromCore(c.OpenAICore)
		toolClient.SetToolManager(toolManager)
		return toolClient, nil
	case *gemini.GeminiClient:
		// For Gemini clients, use the embedded GeminiCore to create a new tool calling client
		toolClient := gemini.NewGeminiClientFromCore(c.GeminiCore)
		toolClient.SetToolManager(toolManager)
		return toolClient, nil
	default:
		// For unknown clients, we cannot create a tool calling client
		// since we need specific core implementations
		return nil, fmt.Errorf("unsupported client type for tool calling: %T", client)
	}
}
