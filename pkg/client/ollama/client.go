package ollama

import (
	"context"
	"fmt"
	"strings"

	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/message"
	"github.com/ollama/ollama/api"
)

// OllamaCore contains shared Ollama client resources and core functionality
// This allows efficient resource sharing between different Ollama client types
type OllamaCore struct {
	client    *api.Client
	model     string
	maxTokens int
}

// NewOllamaCore creates a new Ollama core with shared resources
func NewOllamaCore(model string) (*OllamaCore, error) {
	return NewOllamaCoreWithTokens(model, 0) // 0 = use default
}

// NewOllamaCoreWithTokens creates a new Ollama core with configurable maxTokens
func NewOllamaCoreWithTokens(model string, maxTokens int) (*OllamaCore, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama client: %w", err)
	}

	// Use default maxTokens if not specified
	if maxTokens <= 0 {
		maxTokens = 4096 // Default for Ollama models
	}

	return &OllamaCore{
		client:    client,
		model:     model,
		maxTokens: maxTokens,
	}, nil
}

// Client returns the underlying Ollama API client
func (c *OllamaCore) Client() *api.Client {
	return c.client
}

// Model returns the model name
func (c *OllamaCore) Model() string {
	return c.model
}

// OllamaClient implements tool calling and thinking capabilities for Ollama
// Implements domain.ToolCallingLLMWithThinking interface when both capabilities are available
type OllamaClient struct {
	*OllamaCore
	toolManager domain.ToolManager // For native tool calling
}

// NewOllamaClient creates a new Ollama client with appropriate capabilities based on the model
// Returns either OllamaClient (for native tool calling) or SchemaBasedToolCallingClient (for universal tool calling)
func NewOllamaClient(model string) (domain.ToolCallingLLM, error) {
	return NewOllamaClientWithTokens(model, 0) // 0 = use default
}

// NewOllamaClientWithTokens creates a new Ollama client with configurable maxTokens
func NewOllamaClientWithTokens(model string, maxTokens int) (domain.ToolCallingLLM, error) {
	core, err := NewOllamaCoreWithTokens(model, maxTokens)
	if err != nil {
		return nil, err
	}

	// Check if this model supports native tool calling
	if IsToolCapableModel(model) {
		return &OllamaClient{
			OllamaCore: core,
		}, nil
	} else {
		// For models that don't support tool calling, still create a client but with limited functionality
		// The main application will handle capability checking and user warnings
		return &OllamaClient{
			OllamaCore: core,
		}, nil
	}
}

// NewOllamaClientFromCore creates a new Ollama client from shared core
// Returns OllamaClient for tool-capable models, or nil for unsupported models
func NewOllamaClientFromCore(core *OllamaCore) domain.ToolCallingLLM {
	// Check if this model supports native tool calling
	if IsToolCapableModel(core.model) {
		return &OllamaClient{
			OllamaCore: core,
		}
	} else {
		// Return nil for models that don't support native tool calling (ollama_gbnf has been removed)
		// This will cause a panic if used, but maintains interface compatibility
		// Callers should check IsToolCapableModel before calling this function
		return nil
	}
}

// IsToolCapable checks if the current model supports native tool calling
func (c *OllamaClient) IsToolCapable() bool {
	capable := IsToolCapableModel(c.model)
	return capable
}

// SetToolManager sets the tool manager for native tool calling
func (c *OllamaClient) SetToolManager(toolManager domain.ToolManager) {
	c.toolManager = toolManager
}

// IsToolCapableModel checks if the current model supports native tool calling
// Deprecated: Use IsToolCapable() instead
func (c *OllamaClient) IsToolCapableModel() bool {
	return c.IsToolCapable()
}

// SupportsVision checks if the current model supports vision/image analysis
func (c *OllamaClient) SupportsVision() bool {
	capable := IsVisionCapableModel(c.model)
	return capable
}

// ChatWithToolChoice sends a message to Ollama with tool choice control
func (c *OllamaClient) ChatWithToolChoice(ctx context.Context, messages []message.Message, toolChoice domain.ToolChoice) (message.Message, error) {
	// Convert to Ollama format
	ollamaMessages := ToOllamaMessages(messages)

	chatRequest := &api.ChatRequest{
		Model:    c.model,
		Messages: ollamaMessages,
		Options: map[string]any{
			"temperature": 0.1,
			"num_predict": c.maxTokens, // Max output tokens for Ollama
		},
	}

	// Handle tool choice for tool-capable models
	if c.IsToolCapable() && c.toolManager != nil {
		tools := convertToOllamaTools(c.toolManager.GetTools())
		if len(tools) > 0 {
			// Apply tool choice logic
			switch toolChoice.Type {
			case domain.ToolChoiceNone:
				// Don't add any tools
			case domain.ToolChoiceAuto:
				// Add tools with encouraging system message
				chatRequest.Tools = tools
				addToolUsageSystemMessage(&ollamaMessages, "You are a helpful assistant. When the user asks you to perform tasks, you should use the available tools to help them. Always use the appropriate tool when one is available for the task at hand.")
			case domain.ToolChoiceAny:
				// Add tools with stronger encouragement
				chatRequest.Tools = tools
				addToolUsageSystemMessage(&ollamaMessages, "You are a helpful assistant. You MUST use at least one of the available tools to help the user with their request. Do not provide a response without using a tool.")
			case domain.ToolChoiceTool:
				chatRequest.Tools = tools
				addToolUsageSystemMessage(&ollamaMessages, fmt.Sprintf("You are a helpful assistant. You MUST use the '%s' tool to help the user with their request. Do not provide a response without using this specific tool.", toolChoice.Name))
			}
		}
	}

	var allContent strings.Builder
	var allToolCalls []api.ToolCall

	// Create a channel to signal when we should stop streaming
	done := make(chan struct{})

	err := c.client.Chat(ctx, chatRequest, func(resp api.ChatResponse) error {
		// Check if we should stop streaming
		select {
		case <-done:
			// Streaming cancelled - first tool call received
			return fmt.Errorf("streaming cancelled")
		default:
		}

		allContent.WriteString(resp.Message.Content)
		allToolCalls = append(allToolCalls, resp.Message.ToolCalls...)

		// Streaming response received

		// If we got a tool call, signal to stop streaming
		if len(resp.Message.ToolCalls) > 0 {
			close(done)
		}

		return nil
	})

	if err != nil && !strings.Contains(err.Error(), "streaming cancelled") {
		return nil, fmt.Errorf("ollama chat error: %w", err)
	}

	// Debug: Log approximate token usage for Ollama (estimation based on content length)
	contentLength := allContent.Len()
	estimatedInputTokens := 0
	for _, msg := range ollamaMessages {
		estimatedInputTokens += len(strings.Fields(msg.Content)) // Rough word count approximation
	}
	estimatedOutputTokens := len(strings.Fields(allContent.String())) // Rough word count approximation
	maxTokens := 8192                                                 // From num_predict option
	utilizationPct := float64(estimatedOutputTokens) / float64(maxTokens) * 100

	fmt.Printf("DEBUG: Ollama API Usage - Input: ~%d tokens, Output: ~%d tokens, Content Length: %d chars, Model: %s\n",
		estimatedInputTokens, estimatedOutputTokens, contentLength, c.model)
	fmt.Printf("DEBUG: Token Utilization - %.1f%% of max output tokens (~%d/%d)\n",
		utilizationPct, estimatedOutputTokens, maxTokens)

	// Warn if we're approaching the limit
	if utilizationPct > 90 {
		fmt.Printf("⚠️  WARNING: Very high token usage (%.1f%%) - potential truncation risk!\n", utilizationPct)
	} else if utilizationPct > 80 {
		fmt.Printf("⚠️  WARNING: High token usage (%.1f%%) - approaching limit\n", utilizationPct)
	}

	// If we have tool calls, return the first one
	if len(allToolCalls) > 0 {
		firstToolCall := allToolCalls[0]
		return message.NewToolCallMessage(
			message.ToolName(firstToolCall.Function.Name),
			message.ToolArgumentValues(firstToolCall.Function.Arguments),
		), nil
	}

	// Return the text content
	return message.NewChatMessage(message.MessageTypeAssistant, allContent.String()), nil
}

// ChatWithThinking sends a message to Ollama with thinking control
func (c *OllamaClient) ChatWithThinking(ctx context.Context, messages []message.Message, enableThinking bool) (message.Message, error) {
	// Convert to Ollama format
	ollamaMessages := ToOllamaMessages(messages)

	chatRequest := &api.ChatRequest{
		Model:    c.model,
		Messages: ollamaMessages,
		Options: map[string]any{
			"temperature": 0.1,
			"num_predict": c.maxTokens, // Max output tokens for Ollama
		},
	}

	// Set thinking parameter if supported
	if IsThinkingCapableModel(c.model) {
		chatRequest.Think = &enableThinking
	} else {
	}

	// Add tools if this is a tool-capable model and tool manager is available
	if c.IsToolCapable() && c.toolManager != nil {
		tools := convertToOllamaTools(c.toolManager.GetTools())
		if len(tools) > 0 {
			chatRequest.Tools = tools

			// Add a system message to encourage tool usage
			if len(ollamaMessages) > 0 && ollamaMessages[0].Role != "system" {
				systemMessage := api.Message{
					Role:    "system",
					Content: "You are a helpful assistant. When the user asks you to perform tasks, you should use the available tools to help them. Always use the appropriate tool when one is available for the task at hand.",
				}
				ollamaMessages = append([]api.Message{systemMessage}, ollamaMessages...)
			}
		}
	}

	var allContent strings.Builder
	var allToolCalls []api.ToolCall
	var thinkingContent strings.Builder

	// Create a channel to signal when we should stop streaming
	done := make(chan struct{})

	err := c.client.Chat(ctx, chatRequest, func(resp api.ChatResponse) error {
		// Check if we should stop streaming
		select {
		case <-done:
			// Streaming cancelled - first tool call received
			return fmt.Errorf("streaming cancelled")
		default:
		}

		allContent.WriteString(resp.Message.Content)
		allToolCalls = append(allToolCalls, resp.Message.ToolCalls...)
		thinkingContent.WriteString(resp.Message.Thinking)

		// Streaming response with thinking received

		// If we got a tool call, signal to stop streaming
		if len(resp.Message.ToolCalls) > 0 {
			close(done)
		}

		return nil
	})

	if err != nil && !strings.Contains(err.Error(), "streaming cancelled") {
		return nil, fmt.Errorf("ollama chat error: %w", err)
	}

	// Debug: Log approximate token usage for Ollama (estimation based on content length)
	contentLength := allContent.Len()
	thinkingLength := thinkingContent.Len()
	estimatedInputTokens := 0
	for _, msg := range ollamaMessages {
		estimatedInputTokens += len(strings.Fields(msg.Content)) // Rough word count approximation
	}
	estimatedOutputTokens := len(strings.Fields(allContent.String())) + len(strings.Fields(thinkingContent.String())) // Include thinking tokens
	maxTokens := 8192                                                                                                 // From num_predict option
	utilizationPct := float64(estimatedOutputTokens) / float64(maxTokens) * 100

	fmt.Printf("DEBUG: Ollama API Usage (Thinking) - Input: ~%d tokens, Output: ~%d tokens, Content: %d chars, Thinking: %d chars, Model: %s\n",
		estimatedInputTokens, estimatedOutputTokens, contentLength, thinkingLength, c.model)
	fmt.Printf("DEBUG: Token Utilization - %.1f%% of max output tokens (~%d/%d)\n",
		utilizationPct, estimatedOutputTokens, maxTokens)

	// Warn if we're approaching the limit
	if utilizationPct > 90 {
		fmt.Printf("⚠️  WARNING: Very high token usage (%.1f%%) - potential truncation risk!\n", utilizationPct)
	} else if utilizationPct > 80 {
		fmt.Printf("⚠️  WARNING: High token usage (%.1f%%) - approaching limit\n", utilizationPct)
	}

	// If we have tool calls, return the first one
	if len(allToolCalls) > 0 {
		firstToolCall := allToolCalls[0]
		return message.NewToolCallMessage(
			message.ToolName(firstToolCall.Function.Name),
			message.ToolArgumentValues(firstToolCall.Function.Arguments),
		), nil
	}

	// Return the text content with thinking if available
	if thinkingContent.Len() > 0 {
		return message.NewChatMessageWithThinking(message.MessageTypeAssistant, allContent.String(), thinkingContent.String()), nil
	}

	// Return the text content
	return message.NewChatMessage(message.MessageTypeAssistant, allContent.String()), nil
}

// Chat sends a message to Ollama and returns the response
func (c *OllamaClient) Chat(ctx context.Context, messages []message.Message) (message.Message, error) {
	// Convert to Ollama format
	ollamaMessages := ToOllamaMessages(messages)

	chatRequest := &api.ChatRequest{
		Model:    c.model,
		Messages: ollamaMessages,
		Options: map[string]any{
			"temperature": 0.1,
			"num_predict": c.maxTokens, // Max output tokens for Ollama
		},
	}

	// Add tools if this is a tool-capable model and tool manager is available
	if c.IsToolCapable() && c.toolManager != nil {
		tools := convertToOllamaTools(c.toolManager.GetTools())
		if len(tools) > 0 {
			chatRequest.Tools = tools

			// Add a system message to encourage tool usage (similar to LangChain's approach)
			if len(ollamaMessages) > 0 && ollamaMessages[0].Role != "system" {
				systemMessage := api.Message{
					Role:    "system",
					Content: "You are a helpful assistant. When the user asks you to perform tasks, you should use the available tools to help them. Always use the appropriate tool when one is available for the task at hand.",
				}
				ollamaMessages = append([]api.Message{systemMessage}, ollamaMessages...)
			}
		}
	}

	var allContent strings.Builder
	var allToolCalls []api.ToolCall

	// Create a channel to signal when we should stop streaming
	done := make(chan struct{})

	err := c.client.Chat(ctx, chatRequest, func(resp api.ChatResponse) error {
		// Check if we should stop streaming
		select {
		case <-done:
			// Streaming cancelled - first tool call received
			return fmt.Errorf("streaming cancelled")
		default:
		}

		allContent.WriteString(resp.Message.Content)

		// Accumulate tool calls from all streaming chunks
		if len(resp.Message.ToolCalls) > 0 {
			allToolCalls = append(allToolCalls, resp.Message.ToolCalls...)

			// Signal to stop streaming on first tool call
			close(done)
			return fmt.Errorf("first tool call received, stopping stream")
		}

		// Streaming response received
		return nil
	})

	if err != nil {
		// Check if this is our intentional cancellation due to tool call
		if strings.Contains(err.Error(), "first tool call received") || strings.Contains(err.Error(), "streaming cancelled") {
		} else {
			return nil, fmt.Errorf("ollama chat error: %w", err)
		}
	}

	// Debug: Log approximate token usage for Ollama (estimation based on content length)
	contentLength := allContent.Len()
	estimatedInputTokens := 0
	for _, msg := range ollamaMessages {
		estimatedInputTokens += len(strings.Fields(msg.Content)) // Rough word count approximation
	}
	estimatedOutputTokens := len(strings.Fields(allContent.String())) // Rough word count approximation
	maxTokens := 8192                                                 // From num_predict option
	utilizationPct := float64(estimatedOutputTokens) / float64(maxTokens) * 100

	fmt.Printf("DEBUG: Ollama API Usage - Input: ~%d tokens, Output: ~%d tokens, Content Length: %d chars, Model: %s\n",
		estimatedInputTokens, estimatedOutputTokens, contentLength, c.model)
	fmt.Printf("DEBUG: Token Utilization - %.1f%% of max output tokens (~%d/%d)\n",
		utilizationPct, estimatedOutputTokens, maxTokens)

	// Warn if we're approaching the limit
	if utilizationPct > 90 {
		fmt.Printf("⚠️  WARNING: Very high token usage (%.1f%%) - potential truncation risk!\n", utilizationPct)
	} else if utilizationPct > 80 {
		fmt.Printf("⚠️  WARNING: High token usage (%.1f%%) - approaching limit\n", utilizationPct)
	}

	// If we have accumulated tool calls, return them, otherwise return the content
	if len(allToolCalls) > 0 {
		// Create a synthetic message with accumulated tool calls
		syntheticMessage := api.Message{
			Role:      "assistant",
			Content:   allContent.String(),
			ToolCalls: allToolCalls,
		}
		return fromOllamaMessage(syntheticMessage), nil
	} else {
		// Return the accumulated content as a regular message
		return message.NewChatMessage(message.MessageTypeAssistant, allContent.String()), nil
	}
}
