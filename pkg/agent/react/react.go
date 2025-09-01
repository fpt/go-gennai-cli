package react

import (
	"context"
	"fmt"
	"strings"

	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// ReAct is a simple ReAct implementation that uses LLM and tools
// It handles tool calls and manages the message state
//
// This implementation is designed to be simple and straightforward,
// focusing on the core functionality of ReAct with LLM and tools.

type ReAct struct {
	llmClient     domain.LLM
	state         domain.State
	toolManager   domain.ToolManager
	aligner       domain.Aligner
	maxIterations int // configurable loop limit
}

// Ensure ReAct implements domain.ReAct interface
var _ domain.ReAct = (*ReAct)(nil)

func NewReAct(llmClient domain.LLM, toolManager domain.ToolManager, sharedState domain.State, aligner domain.Aligner, maxIterations int) *ReAct {
	return &ReAct{
		llmClient:     llmClient,
		toolManager:   toolManager,
		state:         sharedState,
		aligner:       aligner,
		maxIterations: maxIterations,
	}
}

// GetLastMessage returns the last message in the conversation without exposing state
func (r *ReAct) GetLastMessage() message.Message {
	return r.state.GetLastMessage()
}

// ClearHistory clears the conversation history without exposing state
func (r *ReAct) ClearHistory() {
	r.state.Clear()
}

// GetConversationSummary returns a summary of the recent conversation for context
// This helps with action selection by providing conversational context
func (r *ReAct) GetConversationSummary() string {
	messages := r.state.GetMessages()
	if len(messages) == 0 {
		return "This is the start of a new conversation."
	}

	// Build a summary of recent user-assistant exchanges
	var summary strings.Builder
	summary.WriteString("Recent conversation:\n")

	// Get the last few messages to provide context
	start := 0
	if len(messages) > 6 { // Keep last 6 messages for context
		start = len(messages) - 6
	}

	for i := start; i < len(messages); i++ {
		msg := messages[i]
		switch msg.Type() {
		case message.MessageTypeUser:
			summary.WriteString(fmt.Sprintf("User: %s\n", msg.Content()))
		case message.MessageTypeAssistant:
			// Only include assistant responses, not tool calls/results
			if len(msg.Content()) > 0 && !strings.Contains(msg.Content(), "Tool result:") {
				content := msg.Content()
				if len(content) > 100 {
					content = content[:100] + "..."
				}
				summary.WriteString(fmt.Sprintf("Assistant: %s\n", content))
			}
		}
	}

	return summary.String()
}

// chatWithThinkingIfSupported uses thinking if the LLM client supports it
func (r *ReAct) chatWithThinkingIfSupported(ctx context.Context, messages []message.Message, thinkingChan chan<- string) (message.Message, error) {
	return r.llmClient.Chat(ctx, messages, true, thinkingChan)
}

// chatWithToolChoice uses tool choice control if the LLM client supports it
func (r *ReAct) chatWithToolChoice(ctx context.Context, messages []message.Message, toolChoice domain.ToolChoice, thinkingChan chan<- string) (message.Message, error) {
	// Check if the client supports tool calling with tool choice
	if toolClient, ok := r.llmClient.(domain.ToolCallingLLM); ok {
		return toolClient.ChatWithToolChoice(ctx, messages, toolChoice, true, thinkingChan)
	}

	// If the client doesn't support tool choice, fall back to regular chat
	// This ensures compatibility with non-tool-calling clients
	return r.llmClient.Chat(ctx, messages, true, thinkingChan)
}

// annotateAndLogUsage attaches token usage (when available) to the response message
// and prints a concise usage line for quick visibility.
func (r *ReAct) annotateAndLogUsage(resp message.Message) {
	// Get model ID if available
	modelID := ""
	if idProvider, ok := r.llmClient.(domain.ModelIdentifier); ok {
		modelID = idProvider.ModelID()
	}

	// Get token usage if available
	if usageProvider, ok := r.llmClient.(domain.TokenUsageProvider); ok {
		if usage, ok2 := usageProvider.LastTokenUsage(); ok2 {
			// Attach to message for persistence in state
			resp.SetTokenUsage(usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
			// Print concise usage summary
			if modelID != "" {
				fmt.Printf("📈 Tokens [%s]: in %d, out %d, total %d\n", modelID, usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
			} else {
				fmt.Printf("📈 Tokens: in %d, out %d, total %d\n", usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
			}
		}
	}
}

// Invoke processes input using the configured maxIterations with external thinking channel
func (r *ReAct) Invoke(ctx context.Context, input string, thinkingChan chan<- string) (message.Message, error) {
	// Use the configured maxIterations instead of options.LoopLimit
	loopLimit := r.maxIterations

	// Add user message to state (enriched with todos if available)
	userMessage := message.NewChatMessage(message.MessageTypeUser, input)
	r.state.AddMessage(userMessage)

	for i := range loopLimit {
		// Remove any previous aligner messages to avoid context contamination
		// We only want the most current alignment guidance
		if removedCount := r.state.RemoveMessagesBySource(message.MessageSourceAligner); removedCount > 0 {
			// Debug: Removed previous aligner messages to prevent context contamination
			// (using fmt.Printf since ReAct is lower level and doesn't have logger context)
			fmt.Printf("🧠 Debug: Removed %d previous aligner messages\n", removedCount)
		}

		r.aligner.InjectMessage(r.state, i, loopLimit)

		// Apply mandatory cleanup (remove images, aligner messages) every iteration
		if err := r.state.CleanupMandatory(); err != nil {
			return nil, fmt.Errorf("failed to perform mandatory cleanup: %w", err)
		}

		// Apply compaction only if token usage exceeds 70% threshold
		// This preserves conversation context until we approach token limits
		maxTokensEstimate := r.estimateContextWindow()
		const compactionThreshold = 70.0 // 70% threshold
		if err := r.state.CompactIfNeeded(ctx, r.llmClient, maxTokensEstimate, compactionThreshold); err != nil {
			return nil, fmt.Errorf("failed to compact messages when needed: %w", err)
		}
		messages := r.state.GetMessages()

		// Use tool calling if available, otherwise fall back to thinking/regular chat
		var resp message.Message
		var err error

		// Check if we have tools available and should use tool calling
		if r.toolManager != nil && len(r.toolManager.GetTools()) > 0 {
			// Use tool choice auto to let the LLM decide when to use tools
			resp, err = r.chatWithToolChoice(ctx, messages, domain.ToolChoice{Type: domain.ToolChoiceAuto}, thinkingChan)
		} else {
			// Fall back to thinking if supported, otherwise regular chat
			resp, err = r.chatWithThinkingIfSupported(ctx, messages, thinkingChan)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to get response from LLM client: %w", err)
		}

		// Clear waiting indicator and show minified response
		fmt.Print("\r                    \r") // Clear the "Thinking..." line
		// Annotate and log token usage when available
		r.annotateAndLogUsage(resp)
		r.printMinifiedResponse(resp, i)

		switch resp := resp.(type) {
		case *message.ChatMessage:
			// Add assistant response to state
			r.state.AddMessage(resp)
			// Check if this is reasoning (intermediate thinking) vs final answer
			if resp.Type() == message.MessageTypeReasoning {
				// Continue the ReAct loop for reasoning messages
				// (Debug logging removed for cleaner output - flow continues automatically)
				continue
			} else {
				// Return for final answers (MessageTypeAssistant)
				// (Debug logging removed for cleaner output - final answer reached)
				return resp, nil
			}

		case *message.ToolCallMessage:
			// Record the tool call message in state
			r.state.AddMessage(resp)
			toolCall := resp

			// Show tool execution indicator
			fmt.Printf("🔧 Running tool: %s\n", toolCall.ToolName())

			msg, err := r.handleToolCall(ctx, toolCall)
			if err != nil {
				return nil, fmt.Errorf("failed to handle tool call: %w", err)
			}

			// Show truncated tool result
			r.printTruncatedToolResult(msg)

			// Add tool result to state
			r.state.AddMessage(msg)

			// Continue to next iteration to process the tool result

		case *message.ToolCallBatchMessage:
			// Execute multiple tools within a single model turn to reduce loops
			batch := resp
			calls := batch.Calls()
			for _, call := range calls {
				// Add each tool call message to state for transcript consistency
				r.state.AddMessage(call)
				fmt.Printf("🔧 Running tool: %s\n", call.ToolName())
				msg, err := r.handleToolCall(ctx, call)
				if err != nil {
					return nil, fmt.Errorf("failed to handle tool call (batch): %w", err)
				}
				r.printTruncatedToolResult(msg)
				r.state.AddMessage(msg)
			}
			// After executing the batch, continue the loop to let the model consume results
			continue

		default:
			return nil, fmt.Errorf("unexpected response type: %T", resp)
		}
	}

	// TBD: If it exhausted with tool calls, we might want to drop it to prevent Anthropic's error.

	return nil, fmt.Errorf("exceeded maximum loop limit (%d) without a valid response", loopLimit)
}

func (r *ReAct) handleToolCall(ctx context.Context, toolCall *message.ToolCallMessage) (message.Message, error) {
	id := toolCall.ID()
	toolName := toolCall.ToolName()
	toolArgs := toolCall.ToolArguments()

	// Execute tool and get structured result
	toolResult, err := r.toolManager.CallTool(ctx, toolName, toolArgs)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %v", err)
	}

	// Handle structured tool result
	var resp message.Message
	if len(toolResult.Images) > 0 {
		resp = message.NewToolResultMessageWithImages(id, toolResult.Text, toolResult.Images, toolResult.Error)
	} else if toolResult.Error != "" {
		resp = message.NewToolResultMessage(id, "", toolResult.Error)
	} else {
		resp = message.NewToolResultMessage(id, toolResult.Text, "")
	}

	return resp, nil
}

// printMinifiedResponse shows a clean, minified version of the agent's response
func (r *ReAct) printMinifiedResponse(resp message.Message, iteration int) {
	switch msg := resp.(type) {
	case *message.ChatMessage:
		if msg.Type() == message.MessageTypeAssistant {
			// Show thinking content if available
			if thinking := msg.Thinking(); thinking != "" {
				fmt.Printf("🧠 Thinking:\n%s\n\n", thinking)
			}

			// Show first 100 characters of assistant response
			content := msg.Content()
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			fmt.Printf("💭 %s\n", content)
		} else if msg.Type() == message.MessageTypeReasoning {
			fmt.Printf("🧠 [Reasoning step %d]\n", iteration+1)

			// Show thinking content for reasoning messages too
			if thinking := msg.Thinking(); thinking != "" {
				fmt.Printf("🧠 Thinking:\n%s\n\n", thinking)
			}
		}
	case *message.ToolCallMessage:
		fmt.Printf("🔧 Calling: %s\n", msg.ToolName())
	}
}

// printTruncatedToolResult shows tool output with truncation for success, full output for errors
func (r *ReAct) printTruncatedToolResult(msg message.Message) {
	content := msg.Content()
	if content == "" {
		fmt.Println("   ↳ (no output)")
		return
	}

	// Check if this is an error message - show full error messages
	isError := strings.HasPrefix(content, "Error:")

	if isError {
		// Show full error messages without truncation
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			fmt.Printf("   ↳ %s\n", line)
		}
	} else {
		// Apply truncation for successful results (original behavior)
		lines := strings.Split(content, "\n")

		// Show last 5 lines maximum
		maxLines := 5
		startLine := 0
		if len(lines) > maxLines {
			startLine = len(lines) - maxLines
			fmt.Printf("   ↳ ...(%d more lines)\n", startLine)
		}

		for i := startLine; i < len(lines); i++ {
			line := lines[i]
			if len(line) > 80 {
				line = line[:77] + "..."
			}
			fmt.Printf("   ↳ %s\n", line)
		}
	}
}

// estimateContextWindow estimates the context window size based on common model patterns
func (r *ReAct) estimateContextWindow() int {
	// This is a conservative estimation based on common model types
	// In the future, this should be replaced with dynamic model capability detection

	// Try to get client type information if possible
	clientType := fmt.Sprintf("%T", r.llmClient)

	switch {
	case strings.Contains(clientType, "anthropic"):
		return 200000 // Claude models typically have 200k+ context windows
	case strings.Contains(clientType, "openai"):
		return 128000 // GPT-4o models have 128k context windows
	case strings.Contains(clientType, "gemini"):
		return 1000000 // Gemini models have very large context windows (1M+)
	case strings.Contains(clientType, "ollama"):
		return 128000 // Most modern Ollama models support 128k context
	default:
		return 100000 // Conservative fallback for unknown models
	}
}
