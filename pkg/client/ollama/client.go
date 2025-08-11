package ollama

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/message"
	"github.com/ollama/ollama/api"
	"github.com/pkg/errors"
)

const temperature = 0.1 // Default temperature for Ollama chat requests

// OllamaCore contains shared Ollama client resources and core functionality
// This allows efficient resource sharing between different Ollama client types
type OllamaCore struct {
	client    *api.Client
	model     string
	maxTokens int
	thinking  bool // Settings-based thinking control
}

// NewOllamaCore creates a new Ollama core with shared resources
func NewOllamaCore(model string) (*OllamaCore, error) {
	return NewOllamaCoreWithOptions(model, 0, true) // 0 = use default, true = enable thinking
}

// NewOllamaCoreWithTokens creates a new Ollama core with configurable maxTokens
func NewOllamaCoreWithTokens(model string, maxTokens int) (*OllamaCore, error) {
	return NewOllamaCoreWithOptions(model, maxTokens, true) // true = enable thinking
}

// NewOllamaCoreWithOptions creates a new Ollama core with configurable maxTokens and thinking
func NewOllamaCoreWithOptions(model string, maxTokens int, thinking bool) (*OllamaCore, error) {
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
		thinking:  thinking,
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

func (c *OllamaCore) chat(ctx context.Context, chatRequest *api.ChatRequest) (api.Message, error) {
	var result api.Message
	var contentBuilder strings.Builder
	var thinkingBuilder strings.Builder
	var hasShownThinkingHeader bool

	err := c.client.Chat(ctx, chatRequest, func(resp api.ChatResponse) error {
		// Accumulate content and thinking from streaming responses
		if resp.Message.Content != "" {
			contentBuilder.WriteString(resp.Message.Content)
		}

		if resp.Message.Thinking != "" {
			// Show thinking header only once when first thinking token arrives
			if !hasShownThinkingHeader && shouldShowThinking(c.thinking, chatRequest.Think) {
				fmt.Print("\x1b[90m💭 ") // Light gray color + thinking emoji
				hasShownThinkingHeader = true
			}

			// Display thinking tokens progressively in light gray (only if thinking is enabled)
			if shouldShowThinking(c.thinking, chatRequest.Think) {
				fmt.Printf("\x1b[90m%s", resp.Message.Thinking) // Light gray (keep color active)
				os.Stdout.Sync()                                // Ensure immediate display of thinking tokens
			}

			thinkingBuilder.WriteString(resp.Message.Thinking)
		}

		if resp.Done {
			// Add newline after thinking if it was shown
			if hasShownThinkingHeader {
				fmt.Print("\x1b[0m\n") // Reset color and newline
			}

			// Combine accumulated content and thinking
			result = api.Message{
				Role:     resp.Message.Role,
				Content:  contentBuilder.String(),
				Thinking: thinkingBuilder.String(),
			}
			// Copy other fields from the final response
			if len(resp.Message.ToolCalls) > 0 {
				result.ToolCalls = resp.Message.ToolCalls
			}
		}

		return nil
	})

	return result, errors.Wrap(err, "ollama chat error")
}

// shouldShowThinking determines if thinking should be displayed based on settings and request parameters
func shouldShowThinking(settingsThinking bool, requestThink *bool) bool {
	if requestThink != nil {
		// Explicit parameter takes precedence (from ChatWithThinking)
		return *requestThink
	}
	// Use settings-based thinking control (from Chat)
	return settingsThinking
}

// OllamaClient implements tool calling and thinking capabilities for Ollama
// Implements domain.ToolCallingLLMWithThinking interface when both capabilities are available
type OllamaClient struct {
	*OllamaCore
	toolManager domain.ToolManager // For native tool calling
}

// NewOllamaClient creates a new Ollama client with configurable maxTokens and thinking
func NewOllamaClient(model string, maxTokens int, thinking bool) (domain.ToolCallingLLM, error) {
	core, err := NewOllamaCoreWithOptions(model, maxTokens, thinking)
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
			"temperature": temperature,
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

	result, err := c.chat(ctx, chatRequest)
	if err != nil {
		return nil, fmt.Errorf("ollama chat error: %w", err)
	}

	// If we have tool calls, return the first one
	if len(result.ToolCalls) > 0 {
		firstToolCall := result.ToolCalls[0]
		return message.NewToolCallMessage(
			message.ToolName(firstToolCall.Function.Name),
			message.ToolArgumentValues(firstToolCall.Function.Arguments),
		), nil
	}

	// Return the text content
	return message.NewChatMessage(message.MessageTypeAssistant, result.Content), nil
}

// chatWithOptions is a private helper that consolidates chat logic
func (c *OllamaClient) chatWithOptions(ctx context.Context, messages []message.Message, enableThinking *bool) (message.Message, error) {
	// Convert to Ollama format
	ollamaMessages := ToOllamaMessages(messages)

	chatRequest := &api.ChatRequest{
		Model:    c.model,
		Messages: ollamaMessages,
		Options: map[string]any{
			"temperature": temperature,
			"num_predict": c.maxTokens, // Max output tokens for Ollama
		},
	}

	// Set thinking parameter if supported
	if IsThinkingCapableModel(c.model) {
		if enableThinking != nil {
			// Use provided thinking setting (from ChatWithThinking)
			chatRequest.Think = enableThinking
		} else {
			// Use settings-based thinking control (from Chat)
			chatRequest.Think = &c.thinking
		}
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

	result, err := c.chat(ctx, chatRequest)
	if err != nil {
		return nil, fmt.Errorf("ollama chat error: %w", err)
	}

	// If we have tool calls, return the first one
	if len(result.ToolCalls) > 0 {
		firstToolCall := result.ToolCalls[0]
		return message.NewToolCallMessage(
			message.ToolName(firstToolCall.Function.Name),
			message.ToolArgumentValues(firstToolCall.Function.Arguments),
		), nil
	}

	// Return the text content with thinking if available and enabled
	if len(result.Thinking) > 0 {
		// For ChatWithThinking, use the explicit enableThinking parameter
		if enableThinking != nil {
			if *enableThinking {
				return message.NewChatMessageWithThinking(
					message.MessageTypeAssistant,
					result.Content,
					result.Thinking,
				), nil
			}
		} else {
			// For Chat, use the settings-based thinking control
			if c.thinking {
				return message.NewChatMessageWithThinking(
					message.MessageTypeAssistant,
					result.Content,
					result.Thinking,
				), nil
			}
		}
		// If thinking is disabled, fall through to return content only
	}

	// Return the text content
	return message.NewChatMessage(message.MessageTypeAssistant, result.Content), nil
}

// Chat sends a message to Ollama with thinking control
func (c *OllamaClient) Chat(ctx context.Context, messages []message.Message, enableThinking bool) (message.Message, error) {
	return c.chatWithOptions(ctx, messages, &enableThinking)
}
