package ollama

import (
	"encoding/base64"
	"fmt"

	pkgLogger "github.com/fpt/go-gennai-cli/pkg/logger"
	"github.com/fpt/go-gennai-cli/pkg/message"
	"github.com/ollama/ollama/api"
)

// Package-level logger for Ollama utility operations
var logger = pkgLogger.NewComponentLogger("ollama-util")

const roleSystem = "system"

// fromOllamaMessage converts a single Ollama message to domain format
func fromOllamaMessage(msg api.Message) message.Message {

	// Handle tool calls from the model
	if len(msg.ToolCalls) > 0 {
		// For now, return the first tool call
		// TODO: Handle multiple tool calls
		toolCall := msg.ToolCalls[0]

		return message.NewToolCallMessage(
			message.ToolName(toolCall.Function.Name),
			message.ToolArgumentValues(toolCall.Function.Arguments),
		)
	}

	// Handle regular messages
	var msgType message.MessageType
	switch msg.Role {
	case "user":
		msgType = message.MessageTypeUser
	case "assistant":
		msgType = message.MessageTypeAssistant
	case roleSystem:
		msgType = message.MessageTypeSystem
	default:
		msgType = message.MessageTypeUser
	}

	// Check if message has images
	if len(msg.Images) > 0 {
		// Convert images to Base64 strings
		images := make([]string, len(msg.Images))
		for i, imgData := range msg.Images {
			images[i] = string(imgData)
		}
		return message.NewChatMessageWithImages(msgType, msg.Content, images)
	}

	// Check if message has thinking content
	if msg.Thinking != "" {
		return message.NewChatMessageWithThinking(msgType, msg.Content, msg.Thinking)
	}

	return message.NewChatMessage(msgType, msg.Content)
}

// ToOllamaMessages converts neutral messages to Ollama format
func ToOllamaMessages(messages []message.Message) []api.Message {
	var ollamaMessages []api.Message

	for _, msg := range messages {
		switch msg.Type() {
		case message.MessageTypeUser, message.MessageTypeAssistant, message.MessageTypeSystem:
			ollamaMsg := api.Message{
				Role:    msg.Type().String(),
				Content: msg.Content(),
			}

			// Add images if present
			if images := msg.Images(); len(images) > 0 {
				ollamaMsg.Images = make([]api.ImageData, len(images))
				for i, imageData := range images {
					// Always assume Base64 data and decode to raw binary
					if decodedData, err := base64.StdEncoding.DecodeString(imageData); err == nil {
						ollamaMsg.Images[i] = api.ImageData(decodedData) // Use raw binary data
						logger.DebugWithIcon("🖼️", "Using Base64 image data", "decoded_bytes", len(decodedData))
					} else {
						logger.WarnWithIcon("⚠️", "Failed to decode Base64 image data", "error", err)
						// Fallback to treating as raw data (though this probably won't work)
						ollamaMsg.Images[i] = api.ImageData(imageData)
					}
				}
			}

			// Add thinking if present
			if thinking := msg.Thinking(); thinking != "" {
				ollamaMsg.Thinking = thinking
			}

			ollamaMessages = append(ollamaMessages, ollamaMsg)
		case message.MessageTypeToolCall:
			// Check if this is a ToolCallMessage
			if toolCallMsg, ok := msg.(*message.ToolCallMessage); ok {
				// Use native tool calling format

				ollamaMessages = append(ollamaMessages, api.Message{
					Role:    "assistant",
					Content: "", // Content can be empty for tool calls
					ToolCalls: []api.ToolCall{
						{
							Function: api.ToolCallFunction{
								Name:      string(toolCallMsg.ToolName()),
								Arguments: api.ToolCallFunctionArguments(toolCallMsg.ToolArguments()),
							},
						},
					},
				})
			}
		case message.MessageTypeToolResult:
			// Check if this is a ToolResultMessage
			if toolResultMsg, ok := msg.(*message.ToolResultMessage); ok {
				// For native tool calling, tool results should be sent as a special message
				// with the tool name indicating which tool the result is for
				content := toolResultMsg.Result
				if toolResultMsg.Error != "" {
					content = fmt.Sprintf("Error: %s", toolResultMsg.Error)
				}

				// For now, just send as a regular user message
				// TODO: Implement proper tool result handling when API supports it
				ollamaMessages = append(ollamaMessages, api.Message{
					Role:    "user", // Tool results come from user/tool perspective
					Content: content,
				})
			}
		}
	}

	return ollamaMessages
}

// convertToOllamaTools converts domain tools to Ollama API tool format
func convertToOllamaTools(tools map[message.ToolName]message.Tool) api.Tools {
	var ollamaTools api.Tools

	for _, tool := range tools {
		// Create parameter definitions for the tool
		properties := make(map[string]struct {
			Type        api.PropertyType `json:"type"`
			Items       any              `json:"items,omitempty"`
			Description string           `json:"description"`
			Enum        []any            `json:"enum,omitempty"`
		})
		var required []string

		for _, arg := range tool.Arguments() {
			properties[string(arg.Name)] = struct {
				Type        api.PropertyType `json:"type"`
				Items       any              `json:"items,omitempty"`
				Description string           `json:"description"`
				Enum        []any            `json:"enum,omitempty"`
			}{
				Type:        api.PropertyType{arg.Type},
				Description: string(arg.Description),
			}
			if arg.Required {
				required = append(required, string(arg.Name))
			}
		}

		toolFunction := api.ToolFunction{
			Name:        string(tool.Name()),
			Description: tool.Description().String(),
			Parameters: struct {
				Type       string   `json:"type"`
				Defs       any      `json:"$defs,omitempty"`
				Items      any      `json:"items,omitempty"`
				Required   []string `json:"required"`
				Properties map[string]struct {
					Type        api.PropertyType `json:"type"`
					Items       any              `json:"items,omitempty"`
					Description string           `json:"description"`
					Enum        []any            `json:"enum,omitempty"`
				} `json:"properties"`
			}{
				Type:       "object",
				Properties: properties,
				Required:   required,
			},
		}

		ollamaTool := api.Tool{
			Type:     "function",
			Function: toolFunction,
		}

		ollamaTools = append(ollamaTools, ollamaTool)
	}

	return ollamaTools
}

// addToolUsageSystemMessage adds a system message to encourage or force tool usage
func addToolUsageSystemMessage(messages *[]api.Message, systemContent string) {
	if len(*messages) > 0 && (*messages)[0].Role != roleSystem {
		systemMessage := api.Message{
			Role:    roleSystem,
			Content: systemContent,
		}
		*messages = append([]api.Message{systemMessage}, *messages...)
	}
}

// filterToolsByName filters Ollama tools to only include the specified tool
func filterToolsByName(tools []api.Tool, toolName message.ToolName) []api.Tool {
	var filteredTools []api.Tool
	for _, tool := range tools {
		if tool.Function.Name == string(toolName) {
			filteredTools = append(filteredTools, tool)
			break
		}
	}
	return filteredTools
}
