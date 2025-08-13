package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

const (
	defaultMaxTokens    = 8192
	maxTokensStopReason = "max_tokens"
)

// AnthropicCore contains shared Anthropic client resources and core functionality
// This allows efficient resource sharing between different Anthropic client types
type AnthropicCore struct {
	client    *anthropic.Client
	model     string
	maxTokens int
}

// NewAnthropicCore creates a new Anthropic core with shared resources
func NewAnthropicCore(model string) (*AnthropicCore, error) {
	return NewAnthropicCoreWithTokens(model, 0) // 0 = use default
}

// NewAnthropicCoreWithTokens creates a new Anthropic core with configurable maxTokens
func NewAnthropicCoreWithTokens(model string, maxTokens int) (*AnthropicCore, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	// Use default if maxTokens is 0 or negative
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	return &AnthropicCore{
		client:    &client,
		model:     model,
		maxTokens: maxTokens,
	}, nil
}

// AnthropicClient handles communication with Claude models
// Implements domain.ToolCallingLLM interfaces for tool calling
type AnthropicClient struct {
	*AnthropicCore
	toolManager domain.ToolManager
}

// NewAnthropicClient creates a new Anthropic client with tool calling and thinking capabilities
func NewAnthropicClient(model string) (domain.ToolCallingLLM, error) {
	return NewAnthropicClientWithTokens(model, 0) // 0 = use default
}

// NewAnthropicClientWithTokens creates a new Anthropic client with configurable maxTokens
func NewAnthropicClientWithTokens(model string, maxTokens int) (domain.ToolCallingLLM, error) {
	core, err := NewAnthropicCoreWithTokens(model, maxTokens)
	if err != nil {
		return nil, err
	}

	// Return as domain.ToolCallingLLM interface
	return &AnthropicClient{
		AnthropicCore: core,
	}, nil
}

// NewAnthropicClientFromCore creates a new Anthropic client from shared core
func NewAnthropicClientFromCore(core *AnthropicCore) domain.ToolCallingLLM {
	return &AnthropicClient{
		AnthropicCore: core,
	}
}

// IsToolCapable checks if the Anthropic client supports native tool calling
func (c *AnthropicClient) IsToolCapable() bool {
	// Anthropic API always supports native tool calling
	return true
}

// ChatWithToolChoice sends a message to Claude with tool choice control
func (c *AnthropicClient) ChatWithToolChoice(ctx context.Context, messages []message.Message, toolChoice domain.ToolChoice) (message.Message, error) {
	// Convert messages to Anthropic format
	anthropicMessages := toAnthropicMessages(messages)

	// Use the provided model or default to Claude Sonnet 4
	claudeModel := getAnthropicModel(c.model)

	// Get tools from tool manager if available
	var tools []anthropic.ToolUnionParam
	if c.toolManager != nil {
		tools = convertToolsToAnthropic(c.toolManager.GetTools())
	}

	// Create message params
	messageParams := anthropic.MessageNewParams{
		MaxTokens: int64(c.maxTokens),
		Messages:  anthropicMessages,
		Model:     claudeModel,
		Tools:     tools,
	}

	// Set tool choice based on the provided configuration
	if len(tools) > 0 {
		anthropicToolChoice := convertToolChoiceToAnthropic(toolChoice)
		messageParams.ToolChoice = anthropicToolChoice
	}

	// Check if we have tool results in the conversation history
	// If so, disable thinking to avoid Anthropic's format restrictions
	hasToolResults := false
	for _, msg := range messages {
		if msg.Type() == message.MessageTypeToolResult {
			hasToolResults = true
			break
		}
	}
	
	// Enable thinking with streaming for progressive display only if no tool results
	if !hasToolResults {
		// Add thinking configuration to the message params with minimum required budget
		messageParams.Thinking = anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: int64(2048), // Set a reasonable thinking budget (minimum 1024)
			},
		}
		
		// Use streaming with thinking
		return c.chatWithStreaming(ctx, messageParams, true)
	}
	
	// Fall back to non-streaming for tool result conversations
	msg, err := c.client.Messages.New(ctx, messageParams)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}
	
	// Handle the response similar to the old non-streaming logic
	if len(msg.Content) == 0 {
		return nil, fmt.Errorf("no content in Anthropic response")
	}
	
	// Handle different content block types
	var content string
	var toolCalls []anthropic.ToolUseBlock
	
	for _, contentBlock := range msg.Content {
		switch variant := contentBlock.AsAny().(type) {
		case anthropic.TextBlock:
			content += variant.Text
		case anthropic.ToolUseBlock:
			// Collect tool calls
			toolCalls = append(toolCalls, variant)
		}
	}
	
	// If we have tool calls, return the first one (for now)
	if len(toolCalls) > 0 {
		toolCall := toolCalls[0]
		toolArgs := make(map[string]any)
		if toolCall.Input != nil {
			// Debug: Log the raw tool call input from Claude
			fmt.Printf("DEBUG: Anthropic non-streaming tool call - name: %s, raw input: %s\n", toolCall.Name, string(toolCall.Input))

			// Parse the JSON input to map[string]any
			if err := json.Unmarshal(toolCall.Input, &toolArgs); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
			}

			fmt.Printf("DEBUG: Parsed tool args: %v\n", toolArgs)
		}

		return message.NewToolCallMessage(
			message.ToolName(toolCall.Name),
			message.ToolArgumentValues(toolArgs),
		), nil
	}
	
	return message.NewChatMessage(message.MessageTypeAssistant, content), nil
}

// SetToolManager sets the tool manager for dynamic tool definitions
func (c *AnthropicClient) SetToolManager(toolManager domain.ToolManager) {
	c.toolManager = toolManager
}

// SupportsVision checks if the Anthropic client supports vision/image analysis
func (c *AnthropicClient) SupportsVision() bool {
	// All Anthropic Claude models support vision
	return true
}

// ChatWithThinking sends a message to Claude with thinking control
func (c *AnthropicClient) Chat(ctx context.Context, messages []message.Message, enableThinking bool) (message.Message, error) {
	// Convert messages to Anthropic format
	anthropicMessages := toAnthropicMessages(messages)

	// Use the provided model or default to Claude Sonnet 4
	claudeModel := getAnthropicModel(c.model)

	// Get tools from tool manager if available
	var tools []anthropic.ToolUnionParam
	if c.toolManager != nil {
		tools = convertToolsToAnthropic(c.toolManager.GetTools())
	}

	// Create message params with thinking enabled
	messageParams := anthropic.MessageNewParams{
		MaxTokens: int64(c.maxTokens),
		Messages:  anthropicMessages,
		Model:     claudeModel,
		Tools:     tools,
	}

	// Enable thinking with streaming for progressive display
	if enableThinking {
		// Add thinking configuration to the message params with minimum required budget
		messageParams.Thinking = anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: int64(2048), // Set a reasonable thinking budget (minimum 1024)
			},
		}
		return c.chatWithStreaming(ctx, messageParams, true)
	}

	msg, err := c.client.Messages.New(ctx, messageParams)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	// Debug: Log token usage from Anthropic API response
	outputTokens := msg.Usage.OutputTokens
	totalTokens := msg.Usage.InputTokens + outputTokens
	utilizationPct := float64(outputTokens) / float64(c.maxTokens) * 100

	fmt.Printf("DEBUG: Anthropic API Usage - Input: %d tokens, Output: %d tokens, Total: %d tokens, Stop Reason: %s\n",
		msg.Usage.InputTokens, outputTokens, totalTokens, msg.StopReason)
	fmt.Printf("DEBUG: Token Utilization - %.1f%% of max output tokens (%d/%d)\n",
		utilizationPct, outputTokens, c.maxTokens)

	// Warn if we're approaching the limit or hit it
	if utilizationPct > 90 {
		fmt.Printf("⚠️  WARNING: Very high token usage (%.1f%%) - potential truncation risk!\n", utilizationPct)
	} else if utilizationPct > 80 {
		fmt.Printf("⚠️  WARNING: High token usage (%.1f%%) - approaching limit\n", utilizationPct)
	}

	// Check if response was truncated due to token limits
	if msg.StopReason == maxTokensStopReason {
		fmt.Printf("🚨 TRUNCATED: Response was cut off due to max_tokens limit!\n")
	}

	// Handle different content block types
	var content string
	var thinking string
	var toolCalls []anthropic.ToolUseBlock

	for _, contentBlock := range msg.Content {
		switch variant := contentBlock.AsAny().(type) {
		case anthropic.TextBlock:
			content += variant.Text
		case anthropic.ToolUseBlock:
			// Collect tool calls
			toolCalls = append(toolCalls, variant)
		case anthropic.ThinkingBlock:
			// Extract thinking content if present
			thinking += variant.Thinking
		case anthropic.RedactedThinkingBlock:
			// Skip redacted thinking blocks
			continue
		default:
			// For other block types, try to extract text if available
			if textBlock, ok := variant.(anthropic.TextBlock); ok {
				content += textBlock.Text
			}
		}
	}

	// If we have tool calls, return the first one (for now)
	if len(toolCalls) > 0 {
		toolCall := toolCalls[0]
		toolArgs := make(map[string]any)
		if toolCall.Input != nil {
			// Debug: Log the raw tool call input from Claude
			fmt.Printf("DEBUG: Anthropic tool call - name: %s, raw input: %s\n", toolCall.Name, string(toolCall.Input))

			// Parse the JSON input to map[string]interface{}
			if err := json.Unmarshal(toolCall.Input, &toolArgs); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
			}

			// Debug: Log the parsed arguments
			fmt.Printf("DEBUG: Parsed tool args: %+v\n", toolArgs)
		}

		return message.NewToolCallMessage(
			message.ToolName(unsanitizeToolNameFromAnthropic(toolCall.Name)),
			message.ToolArgumentValues(toolArgs),
		), nil
	}

	// Create response message with thinking content if available
	var response message.Message
	if thinking != "" {
		response = message.NewChatMessageWithThinking(message.MessageTypeAssistant, content, thinking)
	} else {
		response = message.NewChatMessage(message.MessageTypeAssistant, content)
	}

	return response, nil
}

// chatWithStreaming handles streaming generation with progressive thinking display using Message.Accumulate pattern
func (c *AnthropicClient) chatWithStreaming(ctx context.Context, messageParams anthropic.MessageNewParams, showThinking bool) (message.Message, error) {
	// Create streaming request
	stream := c.client.Messages.NewStreaming(ctx, messageParams)
	
	// Use Message.Accumulate pattern for proper streaming handling
	var acc anthropic.Message
	var thinkingBuilder strings.Builder
	hasShownThinkingHeader := false
	
	// Process streaming events
	for stream.Next() {
		event := stream.Current()
		
		// Accumulate the event into the message
		if err := acc.Accumulate(event); err != nil {
			return nil, fmt.Errorf("failed to accumulate streaming event: %w", err)
		}
		
		// Handle thinking display for progressive feedback
		switch eventData := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			if delta, ok := eventData.Delta.AsAny().(anthropic.ThinkingDelta); ok {
				// Thinking content - show progressively
				if delta.Thinking != "" && showThinking {
					// Show thinking header only once
					if !hasShownThinkingHeader {
						fmt.Print("\x1b[90m💭 ") // Light gray color + thinking emoji
						hasShownThinkingHeader = true
					}
					
					// Display progressive thinking in light gray
					fmt.Printf("\x1b[90m%s", delta.Thinking) // Light gray
					os.Stdout.Sync() // Force flush
					
					// Accumulate thinking content
					thinkingBuilder.WriteString(delta.Thinking)
				}
			}
			
		case anthropic.ContentBlockStartEvent:
			if block, ok := eventData.ContentBlock.AsAny().(anthropic.ThinkingBlock); ok {
				// Thinking block started
				if showThinking && !hasShownThinkingHeader {
					fmt.Print("\x1b[90m💭 ") // Light gray color + thinking emoji  
					hasShownThinkingHeader = true
				}
				// Add initial thinking content if present
				if block.Thinking != "" && showThinking {
					fmt.Printf("\x1b[90m%s", block.Thinking)
					os.Stdout.Sync()
					thinkingBuilder.WriteString(block.Thinking)
				}
			}
		}
	}
	
	// Check for streaming errors
	if stream.Err() != nil {
		return nil, fmt.Errorf("anthropic streaming error: %w", stream.Err())
	}
	
	// Reset color and add newline if we showed thinking
	if hasShownThinkingHeader {
		fmt.Print("\x1b[0m\n") // Reset color
	}
	
	// Now process the accumulated message like the non-streaming version
	if len(acc.Content) == 0 {
		return nil, fmt.Errorf("no content in accumulated Anthropic message")
	}
	
	// Handle different content block types from accumulated message
	var content string
	var thinking string
	var toolCalls []anthropic.ToolUseBlock
	
	for _, contentBlock := range acc.Content {
		switch variant := contentBlock.AsAny().(type) {
		case anthropic.TextBlock:
			content += variant.Text
		case anthropic.ToolUseBlock:
			// Collect tool calls from accumulated message
			toolCalls = append(toolCalls, variant)
		case anthropic.ThinkingBlock:
			// Extract thinking content if present
			thinking += variant.Thinking
		case anthropic.RedactedThinkingBlock:
			// Skip redacted thinking blocks
			continue
		}
	}
	
	// If we have tool calls, return the first one (for now)
	if len(toolCalls) > 0 {
		toolCall := toolCalls[0]
		toolArgs := make(map[string]any)
		if toolCall.Input != nil {
			// Debug: Log the raw tool call input from Claude
			fmt.Printf("DEBUG: Anthropic streaming accumulated tool call - name: %s, raw input: %s\n", toolCall.Name, string(toolCall.Input))

			// Parse the JSON input to map[string]any
			if err := json.Unmarshal(toolCall.Input, &toolArgs); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
			}

			fmt.Printf("DEBUG: Parsed tool args: %v\n", toolArgs)
		}

		return message.NewToolCallMessage(
			message.ToolName(toolCall.Name),
			message.ToolArgumentValues(toolArgs),
		), nil
	}
	
	// Create response message with thinking content if available
	if thinking != "" || thinkingBuilder.Len() > 0 {
		// Use accumulated thinking from the message or the progressive builder
		finalThinking := thinking
		if thinkingBuilder.Len() > 0 {
			finalThinking = thinkingBuilder.String()
		}
		return message.NewChatMessageWithThinking(message.MessageTypeAssistant, content, finalThinking), nil
	}
	
	return message.NewChatMessage(message.MessageTypeAssistant, content), nil
}
