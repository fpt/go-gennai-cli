package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fpt/go-gennai-cli/pkg/message"
)

func TestNewMessageState(t *testing.T) {
	state := NewMessageState()
	if state == nil {
		t.Fatal("NewMessageState() returned nil")
	}
	if state.Messages == nil {
		t.Fatal("Messages slice should be initialized")
	}
	if state.Metadata == nil {
		t.Fatal("Metadata map should be initialized")
	}
	if len(state.Messages) != 0 {
		t.Fatalf("Expected empty messages slice, got %d messages", len(state.Messages))
	}
}

func TestAddMessage(t *testing.T) {
	state := NewMessageState()
	msg := message.NewChatMessage(message.MessageTypeUser, "Hello")

	state.AddMessage(msg)

	if len(state.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(state.Messages))
	}
	if state.Messages[0].Content() != "Hello" {
		t.Fatalf("Expected 'Hello', got '%s'", state.Messages[0].Content())
	}
}

func TestGetLastMessage(t *testing.T) {
	state := NewMessageState()

	// Test empty state
	lastMsg := state.GetLastMessage()
	if lastMsg != nil {
		t.Fatal("Expected nil for empty state")
	}

	// Test with messages
	msg1 := message.NewChatMessage(message.MessageTypeUser, "First")
	msg2 := message.NewChatMessage(message.MessageTypeAssistant, "Second")

	state.AddMessage(msg1)
	state.AddMessage(msg2)

	lastMsg = state.GetLastMessage()
	if lastMsg == nil {
		t.Fatal("Expected non-nil last message")
	}
	if lastMsg.Content() != "Second" {
		t.Fatalf("Expected 'Second', got '%s'", lastMsg.Content())
	}
}

func TestClear(t *testing.T) {
	state := NewMessageState()
	state.AddMessage(message.NewChatMessage(message.MessageTypeUser, "Test"))

	if len(state.Messages) != 1 {
		t.Fatal("Message should have been added")
	}

	state.Clear()

	if len(state.Messages) != 0 {
		t.Fatalf("Expected empty messages after Clear(), got %d messages", len(state.Messages))
	}
}

func TestRemoveMessagesBySource(t *testing.T) {
	state := NewMessageState()

	// Add messages with different sources
	alignerMsg := message.NewAlignerSystemMessage("Aligner guidance")
	summaryMsg := message.NewSummarySystemMessage("Previous conversation summary")

	regularMsg := message.NewChatMessage(message.MessageTypeUser, "Regular message")
	// regularMsg has MessageSourceDefault by default

	state.AddMessage(alignerMsg)
	state.AddMessage(summaryMsg)
	state.AddMessage(regularMsg)

	if len(state.Messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(state.Messages))
	}

	// Remove aligner messages
	removed := state.RemoveMessagesBySource(message.MessageSourceAligner)

	if removed != 1 {
		t.Fatalf("Expected 1 message removed, got %d", removed)
	}
	if len(state.Messages) != 2 {
		t.Fatalf("Expected 2 messages remaining, got %d", len(state.Messages))
	}

	// Check that the correct messages remain
	foundSummary := false
	foundRegular := false
	for _, msg := range state.Messages {
		if msg.Source() == message.MessageSourceSummary {
			foundSummary = true
		}
		if msg.Source() == message.MessageSourceDefault {
			foundRegular = true
		}
	}
	if !foundSummary || !foundRegular {
		t.Fatal("Wrong messages were removed")
	}
}

func TestGetValidConversationHistory(t *testing.T) {
	state := NewMessageState()

	// Add various message types
	userMsg := message.NewChatMessage(message.MessageTypeUser, "Hello")
	assistantMsg := message.NewChatMessage(message.MessageTypeAssistant, "Hi there")

	// Add complete tool call/result pair
	toolCall := message.NewToolCallMessage("test_tool", message.ToolArgumentValues{"arg": "value"})
	toolResult := message.NewToolResultMessage(toolCall.ID(), "Tool executed successfully", "")

	// Add orphaned tool call (no result)
	orphanedCall := message.NewToolCallMessage("orphaned_tool", message.ToolArgumentValues{"arg": "value"})

	state.AddMessage(userMsg)
	state.AddMessage(assistantMsg)
	state.AddMessage(toolCall)
	state.AddMessage(toolResult)
	state.AddMessage(orphanedCall)

	// Get valid conversation history
	validMsgs := state.GetValidConversationHistory(10)

	// Should exclude the orphaned tool call
	if len(validMsgs) != 4 {
		t.Fatalf("Expected 4 valid messages (excluding orphaned call), got %d", len(validMsgs))
	}

	// Check that the orphaned call is not included
	for _, msg := range validMsgs {
		if msg.Type() == message.MessageTypeToolCall && msg.ID() == orphanedCall.ID() {
			t.Fatal("Orphaned tool call should not be included in valid history")
		}
	}
}

func TestSerialization(t *testing.T) {
	state := NewMessageState()
	state.Metadata["test_key"] = "test_value"

	// Add different types of messages
	userMsg := message.NewChatMessage(message.MessageTypeUser, "Hello")
	state.AddMessage(userMsg)

	thinkingMsg := message.NewChatMessageWithThinking(message.MessageTypeAssistant, "Response", "I need to think about this")
	state.AddMessage(thinkingMsg)

	imageMsg := message.NewChatMessageWithImages(message.MessageTypeUser, "Look at this", []string{"base64data"})
	state.AddMessage(imageMsg)

	toolCall := message.NewToolCallMessage("test_tool", message.ToolArgumentValues{"arg": "value"})
	state.AddMessage(toolCall)

	toolResult := message.NewToolResultMessage(toolCall.ID(), "Success", "")
	state.AddMessage(toolResult)

	// Serialize
	data, err := state.Serialize()
	if err != nil {
		t.Fatalf("Serialization failed: %v", err)
	}

	// Validate JSON structure
	var serializable SerializableState
	if err := json.Unmarshal(data, &serializable); err != nil {
		t.Fatalf("Serialized data is not valid JSON: %v", err)
	}

	if len(serializable.Messages) != 5 {
		t.Fatalf("Expected 5 serialized messages, got %d", len(serializable.Messages))
	}

	// Deserialize
	newState := NewMessageState()
	if err := newState.Deserialize(data); err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}

	// Verify deserialized state
	if len(newState.Messages) != 5 {
		t.Fatalf("Expected 5 deserialized messages, got %d", len(newState.Messages))
	}

	if newState.Metadata["test_key"] != "test_value" {
		t.Fatalf("Expected metadata to be preserved, got %v", newState.Metadata)
	}

	// Check message types and content
	if newState.Messages[0].Type() != message.MessageTypeUser {
		t.Fatal("First message type not preserved")
	}
	if newState.Messages[0].Content() != "Hello" {
		t.Fatal("First message content not preserved")
	}

	if newState.Messages[1].Thinking() != "I need to think about this" {
		t.Fatal("Thinking content not preserved")
	}

	if len(newState.Messages[2].Images()) != 1 {
		t.Fatal("Images not preserved")
	}

	if newState.Messages[3].Type() != message.MessageTypeToolCall {
		t.Fatal("Tool call type not preserved")
	}

	if newState.Messages[4].Type() != message.MessageTypeToolResult {
		t.Fatal("Tool result type not preserved")
	}
}

func TestFileOperations(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_state.json")

	// Create state with test data
	originalState := NewMessageState()
	originalState.Metadata["test"] = "data"
	originalState.AddMessage(message.NewChatMessage(message.MessageTypeUser, "Test message"))

	// Save to file
	if err := originalState.SaveToFile(filePath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("File was not created")
	}

	// Load from file
	loadedState := NewMessageState()
	if err := loadedState.LoadFromFile(filePath); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Verify loaded state
	if len(loadedState.Messages) != 1 {
		t.Fatalf("Expected 1 loaded message, got %d", len(loadedState.Messages))
	}

	if loadedState.Messages[0].Content() != "Test message" {
		t.Fatal("Message content not preserved in file operations")
	}

	if loadedState.Metadata["test"] != "data" {
		t.Fatal("Metadata not preserved in file operations")
	}
}

func TestLoadFromNonExistentFile(t *testing.T) {
	state := NewMessageState()
	nonExistentPath := filepath.Join(t.TempDir(), "nonexistent.json")

	// Should not return error for non-existent file, just empty state
	if err := state.LoadFromFile(nonExistentPath); err != nil {
		t.Fatalf("LoadFromFile should not fail for non-existent file: %v", err)
	}

	if len(state.Messages) != 0 {
		t.Fatal("State should be empty for non-existent file")
	}

	if state.Metadata == nil {
		t.Fatal("Metadata should be initialized for non-existent file")
	}
}

func TestGetMessages(t *testing.T) {
	state := NewMessageState()
	msg := message.NewChatMessage(message.MessageTypeUser, "Test")
	state.AddMessage(msg)

	messages := state.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Content() != "Test" {
		t.Fatal("GetMessages returned incorrect content")
	}
}

func TestMessageSourceHandling(t *testing.T) {
	state := NewMessageState()

	// Test default source
	defaultMsg := message.NewChatMessage(message.MessageTypeUser, "Default")
	state.AddMessage(defaultMsg)

	// Test aligner source
	alignerMsg := message.NewAlignerSystemMessage("Aligner")
	state.AddMessage(alignerMsg)

	// Test summary source
	summaryMsg := message.NewSummarySystemMessage("Summary")
	state.AddMessage(summaryMsg)

	// Verify sources are preserved through serialization
	data, err := state.Serialize()
	if err != nil {
		t.Fatalf("Serialization failed: %v", err)
	}

	newState := NewMessageState()
	if err := newState.Deserialize(data); err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}

	// Test removing by source still works
	removed := newState.RemoveMessagesBySource(message.MessageSourceAligner)
	if removed != 1 {
		t.Fatalf("Expected 1 aligner message removed, got %d", removed)
	}

	if len(newState.Messages) != 2 {
		t.Fatalf("Expected 2 messages after removing aligner, got %d", len(newState.Messages))
	}
}

func TestComplexToolScenario(t *testing.T) {
	state := NewMessageState()

	// Simulate a complex conversation with multiple tool calls
	state.AddMessage(message.NewChatMessage(message.MessageTypeUser, "Please help me with multiple tasks"))

	// Tool call 1 - complete pair
	tool1 := message.NewToolCallMessage("task1", message.ToolArgumentValues{"param": "value1"})
	state.AddMessage(tool1)
	state.AddMessage(message.NewToolResultMessage(tool1.ID(), "Task 1 completed", ""))

	// Tool call 2 - complete pair
	tool2 := message.NewToolCallMessage("task2", message.ToolArgumentValues{"param": "value2"})
	state.AddMessage(tool2)
	state.AddMessage(message.NewToolResultMessage(tool2.ID(), "Task 2 completed", ""))

	// Tool call 3 - orphaned (no result)
	tool3 := message.NewToolCallMessage("task3", message.ToolArgumentValues{"param": "value3"})
	state.AddMessage(tool3)

	// Assistant response
	state.AddMessage(message.NewChatMessage(message.MessageTypeAssistant, "I've completed the available tasks"))

	// Test valid conversation history excludes orphaned tool call
	validMsgs := state.GetValidConversationHistory(20)

	// Should have: user message, tool1, result1, tool2, result2, assistant message (6 total)
	// Should exclude: tool3 (orphaned)
	if len(validMsgs) != 6 {
		t.Fatalf("Expected 6 valid messages, got %d", len(validMsgs))
	}

	// Verify no orphaned tool calls are present
	for _, msg := range validMsgs {
		if msg.Type() == message.MessageTypeToolCall && msg.ID() == tool3.ID() {
			t.Fatal("Orphaned tool call should not be in valid conversation history")
		}
	}

	// Verify complete pairs are preserved
	foundTool1 := false
	foundTool2 := false
	for _, msg := range validMsgs {
		if msg.Type() == message.MessageTypeToolCall {
			if msg.ID() == tool1.ID() {
				foundTool1 = true
			}
			if msg.ID() == tool2.ID() {
				foundTool2 = true
			}
		}
	}
	if !foundTool1 || !foundTool2 {
		t.Fatal("Complete tool call pairs should be preserved")
	}
}

func TestSerializationRoundTrip(t *testing.T) {
	// Create a comprehensive test state
	originalState := NewMessageState()

	// Add metadata
	originalState.Metadata["session_id"] = "test_session_123"
	originalState.Metadata["user_id"] = 456
	originalState.Metadata["config"] = map[string]interface{}{
		"temperature": 0.7,
		"max_tokens":  1000,
	}

	// Add various message types
	messages := []message.Message{
		message.NewChatMessage(message.MessageTypeUser, "Start conversation"),
		message.NewChatMessage(message.MessageTypeAssistant, "Hello! How can I help?"),
		message.NewChatMessageWithThinking(message.MessageTypeAssistant, "Let me think", "This requires careful consideration"),
		message.NewChatMessageWithImages(message.MessageTypeUser, "Look at this", []string{"image1", "image2"}),
	}

	// Add tool interactions
	toolCall := message.NewToolCallMessage("complex_tool", message.ToolArgumentValues{
		"string_param": "test",
		"number_param": 42,
		"bool_param":   true,
		"array_param":  []interface{}{"a", "b", "c"},
		"object_param": map[string]interface{}{
			"nested": "value",
			"count":  10,
		},
	})
	messages = append(messages, toolCall)

	toolResult := message.NewToolResultMessage(toolCall.ID(), "Complex tool executed successfully", "")
	messages = append(messages, toolResult)

	// Add error case
	errorTool := message.NewToolCallMessage("error_tool", message.ToolArgumentValues{"param": "fail"})
	messages = append(messages, errorTool)

	errorResult := message.NewToolResultMessage(errorTool.ID(), "", "Tool execution failed: invalid parameter")
	messages = append(messages, errorResult)

	// Add all messages to state
	for _, msg := range messages {
		originalState.AddMessage(msg)
	}

	// Serialize and deserialize
	data, err := originalState.Serialize()
	if err != nil {
		t.Fatalf("Serialization failed: %v", err)
	}

	deserializedState := NewMessageState()
	if err := deserializedState.Deserialize(data); err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}

	// Verify all data is preserved
	if len(deserializedState.Messages) != len(originalState.Messages) {
		t.Fatalf("Message count mismatch: expected %d, got %d",
			len(originalState.Messages), len(deserializedState.Messages))
	}

	// Verify metadata
	if deserializedState.Metadata["session_id"] != "test_session_123" {
		t.Fatal("Session ID not preserved")
	}
	// JSON unmarshaling converts numbers to float64, so we need to check that
	if userID, ok := deserializedState.Metadata["user_id"].(float64); !ok || userID != 456 {
		t.Fatalf("User ID not preserved, got %v (%T)", deserializedState.Metadata["user_id"], deserializedState.Metadata["user_id"])
	}

	// Verify complex nested metadata
	config, ok := deserializedState.Metadata["config"].(map[string]interface{})
	if !ok {
		t.Fatal("Config metadata not preserved as map")
	}
	if temp, ok := config["temperature"].(float64); !ok || temp != 0.7 {
		t.Fatalf("Nested config temperature not preserved, got %v (%T)", config["temperature"], config["temperature"])
	}

	// Verify message content preservation
	for i, originalMsg := range originalState.Messages {
		deserializedMsg := deserializedState.Messages[i]

		if originalMsg.Type() != deserializedMsg.Type() {
			t.Fatalf("Message %d type mismatch", i)
		}
		if originalMsg.Content() != deserializedMsg.Content() {
			t.Fatalf("Message %d content mismatch", i)
		}
		if originalMsg.Thinking() != deserializedMsg.Thinking() {
			t.Fatalf("Message %d thinking mismatch", i)
		}

		// Check images
		originalImages := originalMsg.Images()
		deserializedImages := deserializedMsg.Images()
		if len(originalImages) != len(deserializedImages) {
			t.Fatalf("Message %d images count mismatch", i)
		}
		for j, img := range originalImages {
			if img != deserializedImages[j] {
				t.Fatalf("Message %d image %d mismatch", i, j)
			}
		}
	}

	// Verify tool call arguments are preserved correctly
	toolCallMsg := deserializedState.Messages[4] // Should be the complex tool call
	if toolCallMsg.Type() != message.MessageTypeToolCall {
		t.Fatal("Tool call message type not preserved")
	}

	if toolCall, ok := toolCallMsg.(*message.ToolCallMessage); ok {
		args := toolCall.ToolArguments()
		if args["string_param"] != "test" {
			t.Fatal("String parameter not preserved")
		}
		if numParam, ok := args["number_param"].(float64); !ok || numParam != 42 {
			t.Fatalf("Number parameter not preserved, got %v (%T)", args["number_param"], args["number_param"])
		}
		if args["bool_param"] != true {
			t.Fatal("Boolean parameter not preserved")
		}

		// Check array parameter
		if arrayParam, ok := args["array_param"].([]interface{}); ok {
			if len(arrayParam) != 3 || arrayParam[0] != "a" {
				t.Fatal("Array parameter not preserved correctly")
			}
		} else {
			t.Fatal("Array parameter type not preserved")
		}

		// Check object parameter
		if objectParam, ok := args["object_param"].(map[string]interface{}); ok {
			if objectParam["nested"] != "value" {
				t.Fatal("Object parameter nested value not preserved correctly")
			}
			if count, ok := objectParam["count"].(float64); !ok || count != 10 {
				t.Fatalf("Object parameter count not preserved correctly, got %v (%T)", objectParam["count"], objectParam["count"])
			}
		} else {
			t.Fatal("Object parameter type not preserved")
		}
	} else {
		t.Fatal("Tool call message could not be cast back to ToolCallMessage")
	}
}
