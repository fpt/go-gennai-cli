package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fpt/go-gennai-cli/pkg/message"
)

// SerializableMessage represents a message in a serializable format
type SerializableMessage struct {
	ID        string                `json:"id"`
	Type      message.MessageType   `json:"type"`
	Content   string                `json:"content"`
	Thinking  string                `json:"thinking,omitempty"`
	Images    []string              `json:"images,omitempty"`
	Timestamp time.Time             `json:"timestamp"`
	Source    message.MessageSource `json:"source"`

	// For tool messages
	ToolName string                 `json:"tool_name,omitempty"`
	Args     map[string]interface{} `json:"args,omitempty"`
	Result   string                 `json:"result,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

// SerializableState is the serializable version of MessageState
type SerializableState struct {
	Messages []SerializableMessage `json:"messages"`
	Metadata map[string]any        `json:"metadata,omitempty"`
}

// Chat context with conversation history
type MessageState struct {
    Messages []message.Message `json:"-"` // Don't serialize directly
    Metadata map[string]any    `json:"-"` // Don't serialize directly

    // Token counters snapshot for telemetry (not serialized)
    tokenInput  int
    tokenOutput int
    tokenTotal  int
}

// NewMessageState creates a new message state
func NewMessageState() *MessageState {
	return &MessageState{
		Messages: make([]message.Message, 0),
		Metadata: make(map[string]interface{}),
	}
}

func (c *MessageState) GetMessages() []message.Message {
	return c.Messages
}

// AddMessage adds a message to the context
func (c *MessageState) AddMessage(msg message.Message) {
	c.Messages = append(c.Messages, msg)
}

// GetLastMessage returns the last message in the context
func (c *MessageState) GetLastMessage() message.Message {
	if len(c.Messages) == 0 {
		return nil
	}
	return c.Messages[len(c.Messages)-1]
}

// Clear clears all messages from the context
func (c *MessageState) Clear() {
    c.Messages = make([]message.Message, 0)
}

// ResetTokenCounters clears the internal token counters snapshot
func (c *MessageState) ResetTokenCounters() {
    c.tokenInput, c.tokenOutput, c.tokenTotal = 0, 0, 0
}

// RecalculateTokenCountersFromMessages recomputes counters by summing input+output across messages
func (c *MessageState) RecalculateTokenCountersFromMessages() {
    in, out, _ := c.GetTotalTokenUsage()
    c.tokenInput = in
    c.tokenOutput = out
    c.tokenTotal = in + out
}

// TokenCountersSnapshot returns the last computed counters (input, output, total)
func (c *MessageState) TokenCountersSnapshot() (int, int, int) {
    return c.tokenInput, c.tokenOutput, c.tokenTotal
}

// RemoveMessagesBySource removes all messages with the specified source
// Returns the number of messages removed
func (c *MessageState) RemoveMessagesBySource(source message.MessageSource) int {
	filteredMessages := make([]message.Message, 0, len(c.Messages))
	removedCount := 0

	for _, msg := range c.Messages {
		if msg.Source() == source {
			removedCount++
			continue // Skip messages with the specified source
		}
		filteredMessages = append(filteredMessages, msg)
	}

	if removedCount > 0 {
		c.Messages = filteredMessages
	}

	return removedCount
}

// GetValidConversationHistory returns recent messages while ensuring tool call/result pairs are kept together
// This prevents API validation errors when including conversation history in requests
func (c *MessageState) GetValidConversationHistory(maxMessages int) []message.Message {
	if len(c.Messages) == 0 {
		return nil
	}

	// Simple approach: work backwards and collect messages, but skip orphaned tool calls/results
	var validMessages []message.Message

	// First pass: identify all complete tool call/result pairs
	toolPairs := make(map[string]bool) // Maps tool call IDs to whether they have complete pairs
	for i := 0; i < len(c.Messages); i++ {
		if c.Messages[i].Type() == message.MessageTypeToolCall {
			toolID := c.Messages[i].ID()
			// Look ahead for the corresponding result
			for j := i + 1; j < len(c.Messages); j++ {
				if c.Messages[j].Type() == message.MessageTypeToolResult && c.Messages[j].ID() == toolID {
					toolPairs[toolID] = true
					break
				}
			}
		}
	}

	// Second pass: collect messages from the end, including only complete tool pairs
	for i := len(c.Messages) - 1; i >= 0 && len(validMessages) < maxMessages; i-- {
		msg := c.Messages[i]

		switch msg.Type() {
		case message.MessageTypeToolCall:
			// Only include if it has a complete pair
			if toolPairs[msg.ID()] {
				validMessages = append([]message.Message{msg}, validMessages...)
			}
		case message.MessageTypeToolResult:
			// Only include if its corresponding call has a complete pair
			if toolPairs[msg.ID()] {
				validMessages = append([]message.Message{msg}, validMessages...)
			}
		case message.MessageTypeUser, message.MessageTypeAssistant, message.MessageTypeSystem:
			// Regular messages are always safe to include
			validMessages = append([]message.Message{msg}, validMessages...)
		}
	}

	return validMessages
}

// messageToSerializable converts a Message interface to SerializableMessage
func messageToSerializable(msg message.Message) SerializableMessage {
	if msg == nil {
		return SerializableMessage{}
	}

	serializable := SerializableMessage{
		ID:        msg.ID(),
		Type:      msg.Type(),
		Content:   msg.Content(),
		Thinking:  msg.Thinking(),
		Images:    msg.Images(),
		Timestamp: msg.Timestamp(),
		Source:    msg.Source(),
	}

	// Handle tool-specific fields if it's a tool call or result message
	switch msg.Type() {
	case message.MessageTypeToolCall:
		// Try to cast to ToolCallMessage to extract tool info
		if toolCall, ok := msg.(*message.ToolCallMessage); ok {
			serializable.ToolName = string(toolCall.ToolName())
			args := make(map[string]interface{})
			for k, v := range toolCall.ToolArguments() {
				args[k] = v
			}
			serializable.Args = args
		}
	case message.MessageTypeToolResult:
		// Try to cast to ToolResultMessage to extract result/error info
		if toolResult, ok := msg.(*message.ToolResultMessage); ok {
			serializable.Result = toolResult.Result
			serializable.Error = toolResult.Error
		} else {
			// Fallback: store content as result
			serializable.Result = msg.Content()
		}
	}

	return serializable
}

// serializableToMessage converts a SerializableMessage back to Message interface
func serializableToMessage(s SerializableMessage) message.Message {
	switch s.Type {
	case message.MessageTypeToolCall:
		// Create tool call message with proper types and original ID
		toolName := message.ToolName(s.ToolName)
		args := make(message.ToolArgumentValues)
		for k, v := range s.Args {
			args[k] = v
		}
		return message.NewToolCallMessageWithID(s.ID, toolName, args, s.Timestamp)
	case message.MessageTypeToolResult:
		// Create tool result message
		if s.Error != "" {
			return message.NewToolResultMessage(s.ID, "", s.Error)
		}
		return message.NewToolResultMessage(s.ID, s.Result, "")
	case message.MessageTypeSystem:
		// Handle system messages with different sources
		switch s.Source {
		case message.MessageSourceAligner:
			return message.NewAlignerSystemMessage(s.Content)
		case message.MessageSourceSummary:
			return message.NewSummarySystemMessage(s.Content)
		default:
			return message.NewSystemMessage(s.Content)
		}
	default:
		// Create regular chat message - we'll lose some metadata like custom ID and timestamp
		// but this is better than crashing
		if s.Thinking != "" {
			return message.NewChatMessageWithThinking(s.Type, s.Content, s.Thinking)
		}
		if len(s.Images) > 0 {
			return message.NewChatMessageWithImages(s.Type, s.Content, s.Images)
		}
		return message.NewChatMessage(s.Type, s.Content)
	}
}

// toSerializableState converts MessageState to SerializableState
func (c *MessageState) toSerializableState() SerializableState {
	serializableMessages := make([]SerializableMessage, len(c.Messages))
	for i, msg := range c.Messages {
		serializableMessages[i] = messageToSerializable(msg)
	}

	return SerializableState{
		Messages: serializableMessages,
		Metadata: c.Metadata,
	}
}

// fromSerializableState converts SerializableState back to MessageState
func (c *MessageState) fromSerializableState(s SerializableState) {
	c.Messages = make([]message.Message, len(s.Messages))
	for i, serializableMsg := range s.Messages {
		c.Messages[i] = serializableToMessage(serializableMsg)
	}
	c.Metadata = s.Metadata
	if c.Metadata == nil {
		c.Metadata = make(map[string]interface{})
	}
}

// Serialize serializes the message state to JSON bytes
func (c *MessageState) Serialize() ([]byte, error) {
	serializableState := c.toSerializableState()
	return json.MarshalIndent(serializableState, "", "  ")
}

// Deserialize deserializes JSON bytes into the message state
func (c *MessageState) Deserialize(data []byte) error {
	var serializableState SerializableState
	if err := json.Unmarshal(data, &serializableState); err != nil {
		return err
	}
	c.fromSerializableState(serializableState)
	return nil
}

// SaveToFile saves the message state to a file
func (c *MessageState) SaveToFile(filePath string) error {
	data, err := c.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file %s: %w", filePath, err)
	}

	return nil
}

// LoadFromFile loads the message state from a file
func (c *MessageState) LoadFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start with empty state
			c.Messages = make([]message.Message, 0)
			c.Metadata = make(map[string]interface{})
			return nil
		}
		return fmt.Errorf("failed to read state file %s: %w", filePath, err)
	}

	if err := c.Deserialize(data); err != nil {
		return fmt.Errorf("failed to deserialize state from %s: %w", filePath, err)
	}

	return nil
}
