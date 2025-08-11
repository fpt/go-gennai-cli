package state

import (
	"context"
	"fmt"
	"strings"

	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	pkgLogger "github.com/fpt/go-gennai-cli/pkg/logger"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// Package-level logger for state management operations
var logger = pkgLogger.NewComponentLogger("state-manager")

// CleanupMandatory performs mandatory cleanup without compaction
// - Removes aligner messages (mandatory for context purity)
// - Truncates vision content in older messages (mandatory for token efficiency)
// - Does NOT perform message compaction/summarization
func (c *MessageState) CleanupMandatory() error {
	// Remove any previous summary messages first to get accurate count
	previousSummariesRemoved := c.RemoveMessagesBySource(message.MessageSourceSummary)
	if previousSummariesRemoved > 0 {
		logger.DebugWithIcon("🧹", "Removed previous summary messages during cleanup",
			"removed_count", previousSummariesRemoved)
	}

	// Remove aligner messages (mandatory for context purity)
	alignerRemoved := c.RemoveMessagesBySource(message.MessageSourceAligner)
	if alignerRemoved > 0 {
		logger.DebugWithIcon("🧹", "Removed aligner messages during mandatory cleanup",
			"removed_count", alignerRemoved)
	}

	// Apply vision content truncation to older messages (keep recent 10 messages with images)
	messages := c.Messages
	if len(messages) > 10 {
		for i, msg := range messages[:len(messages)-10] {
			if len(msg.Images()) > 0 {
				// Create new message without images to save tokens
				switch typedMsg := msg.(type) {
				case *message.ChatMessage:
					// Create new message without images
					newMsg := message.NewChatMessage(msg.Type(), msg.Content())
					newMsg.SetTokenUsage(msg.InputTokens(), msg.OutputTokens(), msg.TotalTokens())
					c.Messages[i] = newMsg
				case *message.ToolResultMessage:
					// Create new tool result without images
					newMsg := message.NewToolResultMessage(msg.ID(), typedMsg.Result, typedMsg.Error)
					newMsg.SetTokenUsage(msg.InputTokens(), msg.OutputTokens(), msg.TotalTokens())
					c.Messages[i] = newMsg
				}
				logger.DebugWithIcon("🖼️", "Truncated vision content for token efficiency",
					"message_id", msg.ID(), "position", "older_message")
			}
		}
	}

	return nil
}

// CompactIfNeeded performs compaction only if token usage exceeds threshold
func (c *MessageState) CompactIfNeeded(ctx context.Context, llm domain.LLM, maxTokens int, thresholdPercent float64) error {
	if maxTokens <= 0 {
		return nil // No token limit specified
	}

	_, _, totalTokens := c.GetTotalTokenUsage()
	threshold := float64(maxTokens) * (thresholdPercent / 100.0)
	usagePercent := (float64(totalTokens) / float64(maxTokens)) * 100

	logger.DebugWithIcon("📊", "Token usage check",
		"current_tokens", totalTokens,
		"max_tokens", maxTokens,
		"usage_percent", fmt.Sprintf("%.1f%%", usagePercent),
		"threshold_percent", fmt.Sprintf("%.1f%%", thresholdPercent),
		"threshold_tokens", int(threshold))

	if float64(totalTokens) < threshold {
		logger.DebugWithIcon("📊", "Token usage below threshold, skipping compaction",
			"usage_percent", fmt.Sprintf("%.1f%%", usagePercent),
			"threshold_percent", fmt.Sprintf("%.1f%%", thresholdPercent))
		return nil // Below threshold, no compaction needed
	}

	logger.InfoWithIcon("📊", "Token usage exceeds threshold, performing compaction",
		"usage_percent", fmt.Sprintf("%.1f%%", usagePercent),
		"threshold_percent", fmt.Sprintf("%.1f%%", thresholdPercent))

	// Perform the full compaction process
	return c.performCompaction(ctx, llm)
}

// GetTotalTokenUsage returns the total token usage across all messages
func (c *MessageState) GetTotalTokenUsage() (inputTokens, outputTokens, totalTokens int) {
	for _, msg := range c.Messages {
		inputTokens += msg.InputTokens()
		outputTokens += msg.OutputTokens()
		totalTokens += msg.TotalTokens()
	}
	return inputTokens, outputTokens, totalTokens
}

// performCompaction contains the original compaction logic
func (c *MessageState) performCompaction(ctx context.Context, llm domain.LLM) error {
	messages := c.Messages

	// Simple compaction strategy: keep recent messages, summarize older ones
	const maxMessages = 50
	const preserveRecent = 10

	if len(messages) <= maxMessages {
		return nil // No compaction needed
	}

	// Find a safe split point that doesn't break tool call chains
	splitPoint := findSafeSplitPoint(messages, preserveRecent)
	if splitPoint <= 0 {
		return nil // Not enough messages to compact safely
	}

	olderMessages := messages[:splitPoint]
	recentMessages := messages[splitPoint:]

	// Create an LLM-generated summary of older messages (with vision truncation applied)
	summary, err := c.createLLMSummary(ctx, llm, olderMessages)
	if err != nil {
		logger.WarnWithIcon("🤖", "Failed to create LLM summary, using fallback",
			"error", err, "message_count", len(olderMessages))
		summary = createBasicMessageSummary(olderMessages)
	}

	// Create new message state with summary + recent messages
	c.Clear()

	// Add new summary as system message
	summaryMsg := message.NewSummarySystemMessage(
		fmt.Sprintf("# Previous Conversation Summary\n%s\n\n# Current Conversation Continues", summary))
	c.AddMessage(summaryMsg)

	// Add back recent messages, filtering out alignment messages
	skippedAlignment := 0
	for _, msg := range recentMessages {
		// Skip alignment messages injected by Aligner during compaction
		if isAlignmentMessage(msg) {
			skippedAlignment++
			continue
		}
		c.AddMessage(msg)
	}

	if skippedAlignment > 0 {
		logger.DebugWithIcon("🧹", "Skipped alignment messages during compaction",
			"skipped_count", skippedAlignment)
	}
	logger.InfoWithIcon("📝", "Message compaction completed",
		"before_count", len(messages),
		"after_count", len(c.Messages),
		"compression_ratio", fmt.Sprintf("%.1f%%", float64(len(c.Messages))/float64(len(messages))*100))

	return nil
}

// findSafeSplitPoint finds a split point that doesn't break tool call chains
// This is critical for Anthropic API compatibility which requires tool calls and results to be paired
func findSafeSplitPoint(messages []message.Message, preserveRecent int) int {
	desiredSplitPoint := len(messages) - preserveRecent

	// Work backwards from the desired split point to find a safe boundary
	// A safe boundary is one where we don't split tool call/result pairs
	for i := desiredSplitPoint; i >= 0; i-- {
		if isSafeSplitPoint(messages, i) {
			return i
		}
	}

	// If no safe split point found, don't compact
	return 0
}

// isSafeSplitPoint checks if splitting at this point would break tool call chains
func isSafeSplitPoint(messages []message.Message, splitPoint int) bool {
	if splitPoint <= 0 || splitPoint >= len(messages) {
		return false
	}

	// Check if we're splitting in the middle of a tool call chain
	// Rule: Don't split if there's an unpaired tool call before the split point
	// or an unpaired tool result after the split point

	// Count unpaired tool calls before split point (looking backwards)
	unpairedToolCalls := 0
	for i := splitPoint - 1; i >= 0; i-- {
		switch messages[i].Type() {
		case message.MessageTypeToolCall:
			unpairedToolCalls++
		case message.MessageTypeToolResult:
			if unpairedToolCalls > 0 {
				unpairedToolCalls--
			}
		}
	}

	// If there are unpaired tool calls before the split, it's not safe
	if unpairedToolCalls > 0 {
		return false
	}

	// Check for orphaned tool results after split point
	unpairedToolResults := 0
	for i := splitPoint; i < len(messages); i++ {
		switch messages[i].Type() {
		case message.MessageTypeToolResult:
			unpairedToolResults++
		case message.MessageTypeToolCall:
			if unpairedToolResults > 0 {
				unpairedToolResults--
			}
		}
	}

	// If there are unpaired tool results after the split, it's not safe
	if unpairedToolResults > 0 {
		return false
	}

	return true
}

// createLLMSummary creates an intelligent summary using LLM
func (c *MessageState) createLLMSummary(ctx context.Context, llm domain.LLM, messages []message.Message) (string, error) {
	if len(messages) == 0 {
		return "No previous conversation.", nil
	}

	// Build conversation text for summarization
	var conversationBuilder strings.Builder
	conversationBuilder.WriteString("Previous conversation to summarize:\n\n")

	for _, msg := range messages {
		switch msg.Type() {
		case message.MessageTypeUser:
			conversationBuilder.WriteString(fmt.Sprintf("User: %s\n", msg.Content()))
		case message.MessageTypeAssistant:
			// Only include actual responses, not tool calls
			if len(msg.Content()) > 0 && !strings.HasPrefix(msg.Content(), "Tool call:") {
				conversationBuilder.WriteString(fmt.Sprintf("Assistant: %s\n", msg.Content()))
			}
		case message.MessageTypeToolCall:
			if toolMsg, ok := msg.(*message.ToolCallMessage); ok {
				conversationBuilder.WriteString(fmt.Sprintf("Tool used: %s\n", toolMsg.ToolName()))
			}
		case message.MessageTypeToolResult:
			if toolResult, ok := msg.(*message.ToolResultMessage); ok {
				result := toolResult.Result
				if len(result) > 200 {
					result = result[:200] + "..."
				}

				// Drop all images from older messages to save tokens - recent messages keep the latest images
				if len(msg.Images()) > 0 {
					conversationBuilder.WriteString(fmt.Sprintf("Tool result: %s [Image data truncated for token efficiency]\n", result))
				} else {
					conversationBuilder.WriteString(fmt.Sprintf("Tool result: %s\n", result))
				}
			}
		}
	}

	// Create summarization prompt
	summaryPrompt := fmt.Sprintf(`Please create a concise summary of the following conversation. Focus on:
1. Main topics discussed
2. Key findings or results
3. Important context that should be preserved
4. Any ongoing tasks or decisions

Keep the summary under 200 words and preserve essential context for continuing the conversation.

%s

Summary:`, conversationBuilder.String())

	// Use LLM to create summary
	summaryMessage := message.NewChatMessage(message.MessageTypeUser, summaryPrompt)
	response, err := llm.Chat(ctx, []message.Message{summaryMessage})
	if err != nil {
		return "", fmt.Errorf("failed to generate LLM summary: %w", err)
	}

	return response.Content(), nil
}

// createBasicMessageSummary creates a simple fallback summary of messages
func createBasicMessageSummary(messages []message.Message) string {
	if len(messages) == 0 {
		return "No previous conversation."
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Summary of %d previous messages:\n\n", len(messages)))

	userQuestions := 0
	toolCalls := 0
	topics := make(map[string]int)
	hasVisionContent := false

	for _, msg := range messages {
		switch msg.Type() {
		case message.MessageTypeUser:
			userQuestions++
			content := strings.ToLower(msg.Content())
			if strings.Contains(content, "analyze") || strings.Contains(content, "analysis") {
				topics["code_analysis"]++
			}
			if strings.Contains(content, "function") || strings.Contains(content, "declaration") {
				topics["function_analysis"]++
			}
			if strings.Contains(content, "dependency") || strings.Contains(content, "import") {
				topics["dependency_analysis"]++
			}
		case message.MessageTypeToolCall:
			toolCalls++
		case message.MessageTypeToolResult:
			// Track if we had vision content (now truncated)
			if len(msg.Images()) > 0 {
				hasVisionContent = true
			}
		}
	}

	summary.WriteString(fmt.Sprintf("- User questions/requests: %d\n", userQuestions))
	summary.WriteString(fmt.Sprintf("- Tool calls executed: %d\n", toolCalls))

	if hasVisionContent {
		summary.WriteString("- Visual content: preserved most recent images for context\n")
	}

	if len(topics) > 0 {
		summary.WriteString("\nMain topics discussed:\n")
		for topic, count := range topics {
			summary.WriteString(fmt.Sprintf("- %s: %d occurrences\n",
				strings.ReplaceAll(topic, "_", " "), count))
		}
	}

	summary.WriteString("\n*This is a simplified summary. Full conversation history was compressed to manage context length.*")
	return summary.String()
}

// isAlignmentMessage checks if a message was injected by the Aligner and should be removed during compaction
func isAlignmentMessage(msg message.Message) bool {
	return msg.Source() == message.MessageSourceAligner
}

// truncateContent truncates content to specified length with ellipsis
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen-3] + "..."
}
