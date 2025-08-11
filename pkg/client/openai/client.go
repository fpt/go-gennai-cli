package openai

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"

	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// OpenAICore holds shared resources for OpenAI clients
type OpenAICore struct {
	client    *openai.Client
	model     string
	maxTokens int
}

// OpenAIClient implements ToolCallingLLM and VisionLLM interfaces
type OpenAIClient struct {
	*OpenAICore
	toolManager domain.ToolManager
}

// NewOpenAIClient creates a new OpenAI client with the specified model
func NewOpenAIClient(model string) (*OpenAIClient, error) {
	return NewOpenAIClientWithTokens(model, 0) // 0 = use default
}

// NewOpenAIClientWithTokens creates a new OpenAI client with configurable maxTokens
func NewOpenAIClientWithTokens(model string, maxTokens int) (*OpenAIClient, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	// Setup client options
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}

	// Support custom base URL (for Azure OpenAI, etc.)
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(opts...)

	// Validate and map model name
	openaiModel := getOpenAIModel(model)

	// Use default maxTokens if not specified
	if maxTokens <= 0 {
		maxTokens = getModelCapabilities(openaiModel).MaxTokens
	}

	core := &OpenAICore{
		client:    &client,
		model:     openaiModel,
		maxTokens: maxTokens,
	}

	return &OpenAIClient{
		OpenAICore: core,
	}, nil
}

// NewOpenAIClientFromCore creates a new client instance from existing core (for factory pattern)
func NewOpenAIClientFromCore(core *OpenAICore) domain.ToolCallingLLM {
	return &OpenAIClient{
		OpenAICore: core,
	}
}

// Chat implements the basic LLM interface
func (c *OpenAIClient) Chat(ctx context.Context, messages []message.Message) (message.Message, error) {
	// Convert internal messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessageParamUnion, 0)

	for _, msg := range messages {
		switch msg.Type() {
		case message.MessageTypeUser:
			openaiMessages = append(openaiMessages, openai.UserMessage(msg.Content()))
		case message.MessageTypeAssistant:
			openaiMessages = append(openaiMessages, openai.AssistantMessage(msg.Content()))
		case message.MessageTypeSystem:
			openaiMessages = append(openaiMessages, openai.SystemMessage(msg.Content()))
			// Skip tool call and result messages for basic chat
		}
	}

	completion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages:             openaiMessages,
		Model:                shared.ChatModel(c.model),
		MaxCompletionTokens: openai.Int(int64(c.maxTokens)),
	})
	if err != nil {
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	// Create response message
	response := completion.Choices[0].Message.Content
	responseMessage := message.NewChatMessage(message.MessageTypeAssistant, response)
	
	// Set token usage from OpenAI API response
	responseMessage.SetTokenUsage(
		int(completion.Usage.PromptTokens),
		int(completion.Usage.CompletionTokens),
		int(completion.Usage.TotalTokens),
	)

	// Optional debug logging
	if os.Getenv("DEBUG_TOKENS") == "1" {
		maxTokens := c.maxTokens
		outputTokens := completion.Usage.CompletionTokens
		utilizationPct := float64(outputTokens) / float64(maxTokens) * 100

		fmt.Printf("DEBUG: OpenAI API Usage - Input: %d tokens, Output: %d tokens, Total: %d tokens, Model: %s\n",
			completion.Usage.PromptTokens, outputTokens, completion.Usage.TotalTokens, c.model)
		fmt.Printf("DEBUG: Token Utilization - %.1f%% of max output tokens (%d/%d)\n",
			utilizationPct, outputTokens, maxTokens)

		// Warn if we're approaching the limit
		if utilizationPct > 90 {
			fmt.Printf("⚠️  WARNING: Very high token usage (%.1f%%) - potential truncation risk!\n", utilizationPct)
		} else if utilizationPct > 80 {
			fmt.Printf("⚠️  WARNING: High token usage (%.1f%%) - approaching limit\n", utilizationPct)
		}
	}

	return responseMessage, nil
}

// SetToolManager implements ToolCallingLLM interface
func (c *OpenAIClient) SetToolManager(toolManager domain.ToolManager) {
	c.toolManager = toolManager
}

// IsToolCapable checks if the OpenAI client supports native tool calling
func (c *OpenAIClient) IsToolCapable() bool {
	// Check if the current model supports tool calling
	caps := getModelCapabilities(c.model)
	return caps.SupportsToolCalling
}

// ChatWithToolChoice implements ToolCallingLLM interface with native OpenAI tool calling
func (c *OpenAIClient) ChatWithToolChoice(ctx context.Context, messages []message.Message, toolChoice domain.ToolChoice) (message.Message, error) {
	// Convert internal messages to OpenAI format with tool support
	openaiMessages := toOpenAIMessages(messages)

	// Use the provided model or default
	openaiModel := getOpenAIModel(c.model)

	// Create completion params
	params := openai.ChatCompletionNewParams{
		Messages:             openaiMessages,
		Model:                shared.ChatModel(openaiModel),
		MaxCompletionTokens: openai.Int(int64(c.maxTokens)),
	}

	// Add tools if available
	if c.toolManager != nil {
		domainTools := c.toolManager.GetTools()
		tools := convertToolsToOpenAI(domainTools)
		
		if len(tools) > 0 {
			params.Tools = tools
		}
	}

	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	choice := completion.Choices[0]
	
	// Check if there are tool calls in the response
	if len(choice.Message.ToolCalls) > 0 {
		// Return the first tool call
		toolCall := choice.Message.ToolCalls[0]
		toolArgs := convertOpenAIArgsToToolArgs(toolCall.Function.Arguments)
		
		// Create tool call message with the specific OpenAI tool call ID for proper pairing
		toolCallMessage := message.NewToolCallMessageWithID(
			toolCall.ID, // Use OpenAI's tool call ID
			message.ToolName(toolCall.Function.Name),
			toolArgs,
			time.Now(),
		)
		
		// Set token usage for tool call message
		toolCallMessage.SetTokenUsage(
			int(completion.Usage.PromptTokens),
			int(completion.Usage.CompletionTokens),
			int(completion.Usage.TotalTokens),
		)
		
		// Optional debug logging
		if os.Getenv("DEBUG_TOKENS") == "1" {
			maxTokens := c.maxTokens
			outputTokens := completion.Usage.CompletionTokens
			utilizationPct := float64(outputTokens) / float64(maxTokens) * 100

			fmt.Printf("DEBUG: OpenAI API Usage (Tool Call) - Input: %d tokens, Output: %d tokens, Total: %d tokens, Model: %s\n",
				completion.Usage.PromptTokens, outputTokens, completion.Usage.TotalTokens, openaiModel)
			fmt.Printf("DEBUG: Token Utilization - %.1f%% of max output tokens (%d/%d)\n",
				utilizationPct, outputTokens, maxTokens)
		}
		
		return toolCallMessage, nil
	}

	// Return regular text response
	response := choice.Message.Content
	responseMessage := message.NewChatMessage(message.MessageTypeAssistant, response)
	
	// Set token usage for regular response
	responseMessage.SetTokenUsage(
		int(completion.Usage.PromptTokens),
		int(completion.Usage.CompletionTokens),
		int(completion.Usage.TotalTokens),
	)

	// Optional debug logging
	if os.Getenv("DEBUG_TOKENS") == "1" {
		maxTokens := c.maxTokens
		outputTokens := completion.Usage.CompletionTokens
		utilizationPct := float64(outputTokens) / float64(maxTokens) * 100

		fmt.Printf("DEBUG: OpenAI API Usage - Input: %d tokens, Output: %d tokens, Total: %d tokens, Model: %s\n",
			completion.Usage.PromptTokens, outputTokens, completion.Usage.TotalTokens, openaiModel)
		fmt.Printf("DEBUG: Token Utilization - %.1f%% of max output tokens (%d/%d)\n",
			utilizationPct, outputTokens, maxTokens)

		// Warn if we're approaching the limit
		if utilizationPct > 90 {
			fmt.Printf("⚠️  WARNING: Very high token usage (%.1f%%) - potential truncation risk!\n", utilizationPct)
		} else if utilizationPct > 80 {
			fmt.Printf("⚠️  WARNING: High token usage (%.1f%%) - approaching limit\n", utilizationPct)
		}
	}
	
	return responseMessage, nil
}

// SupportsVision implements VisionLLM interface
func (c *OpenAIClient) SupportsVision() bool {
	// GPT-4V models support vision
	return strings.Contains(c.model, "gpt-4") && (strings.Contains(c.model, "vision") || strings.Contains(c.model, "gpt-4o"))
}

