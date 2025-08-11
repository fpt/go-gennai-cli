package openai

import (
	"encoding/json"
	"strings"

	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/message"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

// convertToolsToOpenAI converts domain tools to OpenAI function format
func convertToolsToOpenAI(tools map[message.ToolName]message.Tool) []openai.ChatCompletionToolParam {
	var openaiTools []openai.ChatCompletionToolParam

	// Convert domain tools to OpenAI format
	for _, tool := range tools {
		// Create properties from tool arguments
		properties := make(map[string]interface{})
		var required []string

		for _, arg := range tool.Arguments() {
			// Convert tool argument to OpenAI property schema  
			property := convertArgumentToProperty(arg)
			properties[string(arg.Name)] = property

			if arg.Required {
				required = append(required, string(arg.Name))
			}
		}

		// Ensure required is never nil (OpenAI expects an array, even if empty)
		if required == nil {
			required = []string{}
		}

		// Create OpenAI function parameters
		params := shared.FunctionParameters{
			"type":       "object",
			"properties": properties,
		}
		
		// Only add required if we have required fields
		if len(required) > 0 {
			params["required"] = required
		}

		// Create OpenAI function tool
		openaiTool := openai.ChatCompletionToolParam{
			Type: "function",
			Function: shared.FunctionDefinitionParam{
				Name:        string(tool.Name()),
				Description: openai.String(tool.Description().String()),
				Parameters:  params,
			},
		}

		openaiTools = append(openaiTools, openaiTool)
	}

	return openaiTools
}

// convertToolChoiceToOpenAI converts domain ToolChoice to OpenAI format
func convertToolChoiceToOpenAI(toolChoice domain.ToolChoice) *string {
	switch toolChoice.Type {
	case domain.ToolChoiceAuto:
		autoChoice := "auto"
		return &autoChoice
	case domain.ToolChoiceAny:
		requiredChoice := "required"
		return &requiredChoice
	case domain.ToolChoiceTool:
		// For specific tool choice, we'll use auto for now
		// TODO: Implement specific tool selection
		autoChoice := "auto"
		return &autoChoice
	case domain.ToolChoiceNone:
		noneChoice := "none"
		return &noneChoice
	default:
		// Default to auto
		autoChoice := "auto"
		return &autoChoice
	}
}

// toOpenAIMessages converts domain messages to OpenAI format, handling tool calls and results
func toOpenAIMessages(messages []message.Message) []openai.ChatCompletionMessageParamUnion {
	var openaiMessages []openai.ChatCompletionMessageParamUnion

	for _, msg := range messages {
		switch msg.Type() {
		case message.MessageTypeUser:
			openaiMessages = append(openaiMessages, openai.UserMessage(msg.Content()))
		case message.MessageTypeAssistant:
			openaiMessages = append(openaiMessages, openai.AssistantMessage(msg.Content()))
		case message.MessageTypeSystem:
			openaiMessages = append(openaiMessages, openai.SystemMessage(msg.Content()))
		case message.MessageTypeToolCall:
			// For conversation history, represent tool calls as assistant messages
			// The actual tool calling happens via the OpenAI native API
			if toolCallMsg, ok := msg.(interface{ ToolName() message.ToolName; ToolArguments() message.ToolArgumentValues }); ok {
				argsJSON := convertToolArgsToJSON(toolCallMsg.ToolArguments())
				toolCallText := "[Called tool: " + string(toolCallMsg.ToolName()) + "(" + argsJSON + ")]"
				openaiMessages = append(openaiMessages, openai.AssistantMessage(toolCallText))
			}
		case message.MessageTypeToolResult:
			// Represent tool results as user messages (as if the user provided the information)
			if toolResultMsg, ok := msg.(interface{ Content() string }); ok {
				resultText := "[Tool result: " + toolResultMsg.Content() + "]"
				openaiMessages = append(openaiMessages, openai.UserMessage(resultText))
			}
		}
	}

	return openaiMessages
}

// convertArgumentToProperty converts a ToolArgument to OpenAI property schema
// This provides generic JSON schema inference from ToolArgument metadata
func convertArgumentToProperty(arg message.ToolArgument) map[string]interface{} {
	property := map[string]interface{}{
		"type":        arg.Type,
		"description": arg.Description.String(),
	}

	desc := arg.Description.String()
	
	// Extract enum values if present in description  
	if enumValues := extractEnumFromDescription(desc); len(enumValues) > 0 {
		property["enum"] = enumValues
	}

	// Handle array types - infer item schema from description patterns
	if arg.Type == "array" {
		if itemSchema := inferArrayItemSchema(desc); itemSchema != nil {
			property["items"] = itemSchema
		}
		
		// Extract array constraints like maxItems
		if maxItems := extractMaxItems(desc); maxItems > 0 {
			property["maxItems"] = maxItems
		}
	}

	// TODO: Future enhancement - detect and use raw JSON schema if available
	// This could check if the tool has access to original schema information:
	// if schemaProvider, ok := tool.(interface{ GetJSONSchema(argName string) map[string]interface{} }); ok {
	//     if rawSchema := schemaProvider.GetJSONSchema(string(arg.Name)); rawSchema != nil {
	//         return rawSchema
	//     }
	// }

	return property
}

// inferArrayItemSchema attempts to infer the schema for array items from description
func inferArrayItemSchema(desc string) map[string]interface{} {
	// Look for common patterns in descriptions
	lowerDesc := strings.ToLower(desc)
	
	// Generic object with properties mentioned in description
	if strings.Contains(lowerDesc, "object") && strings.Contains(lowerDesc, "properties") {
		return map[string]interface{}{
			"type": "object",
		}
	}
	
	// String array
	if strings.Contains(lowerDesc, "string") || strings.Contains(lowerDesc, "text") {
		return map[string]interface{}{
			"type": "string",
		}
	}
	
	// Number array  
	if strings.Contains(lowerDesc, "number") || strings.Contains(lowerDesc, "numeric") {
		return map[string]interface{}{
			"type": "number",
		}
	}
	
	// Default to object for complex arrays
	return map[string]interface{}{
		"type": "object",
	}
}

// extractMaxItems extracts maxItems constraint from description
func extractMaxItems(desc string) int {
	// Look for patterns like "maxItems: 5" or "5 items or fewer" 
	if strings.Contains(desc, "maxItems:") {
		if idx := strings.Index(desc, "maxItems:"); idx >= 0 {
			remaining := desc[idx+9:] // Skip "maxItems:"
			if len(remaining) > 0 && remaining[0] >= '0' && remaining[0] <= '9' {
				return int(remaining[0] - '0') // Simple single digit parsing
			}
		}
	}
	
	// Look for "X items or fewer" pattern
	if strings.Contains(desc, "items or fewer") || strings.Contains(desc, "or fewer items") {
		// Simple pattern matching for common cases
		if strings.Contains(desc, "5 items") {
			return 5
		}
	}
	
	return 0
}

// extractEnumFromDescription extracts enum values from description text
func extractEnumFromDescription(desc string) []string {
	if len(desc) < 10 {
		return nil
	}
	
	// Look for "one of:" or "enum:" pattern
	var enumStart int = -1
	for i := 0; i < len(desc)-6; i++ {
		if desc[i:i+7] == "one of:" || (i < len(desc)-5 && desc[i:i+5] == "enum:") {
			if desc[i:i+7] == "one of:" {
				enumStart = i + 7
			} else {
				enumStart = i + 5
			}
			break
		}
	}
	
	if enumStart == -1 {
		return nil
	}
	
	enumStr := desc[enumStart:]
	if len(enumStr) == 0 {
		return nil
	}
	
	// Skip whitespace
	for len(enumStr) > 0 && enumStr[0] == ' ' {
		enumStr = enumStr[1:]
	}
	
	if len(enumStr) == 0 {
		return nil
	}
	
	// Simple split by comma
	parts := make([]string, 0)
	current := ""
	for i := 0; i < len(enumStr); i++ {
		if enumStr[i] == ',' {
			trimmed := ""
			// Trim spaces from current
			for j := 0; j < len(current); j++ {
				if current[j] != ' ' {
					trimmed = current[j:]
					break
				}
			}
			for len(trimmed) > 0 && trimmed[len(trimmed)-1] == ' ' {
				trimmed = trimmed[:len(trimmed)-1]
			}
			if len(trimmed) > 0 {
				parts = append(parts, trimmed)
			}
			current = ""
		} else {
			current += string(enumStr[i])
		}
	}
	
	// Add final part
	trimmed := ""
	for j := 0; j < len(current); j++ {
		if current[j] != ' ' {
			trimmed = current[j:]
			break
		}
	}
	for len(trimmed) > 0 && trimmed[len(trimmed)-1] == ' ' {
		trimmed = trimmed[:len(trimmed)-1]
	}
	if len(trimmed) > 0 {
		parts = append(parts, trimmed)
	}
	
	if len(parts) > 1 {
		return parts
	}
	return nil
}

// convertOpenAIArgsToToolArgs converts OpenAI function arguments JSON to tool argument values
func convertOpenAIArgsToToolArgs(argsJSON string) message.ToolArgumentValues {
	result := make(message.ToolArgumentValues)

	if argsJSON == "" {
		return result
	}

	// Parse JSON arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		// If parsing fails, return empty map
		return result
	}

	// Convert interface{} values to proper types
	for key, value := range args {
		result[key] = value
	}

	return result
}

// convertToolArgsToJSON converts tool argument values to JSON string
func convertToolArgsToJSON(args message.ToolArgumentValues) string {
	if len(args) == 0 {
		return "{}"
	}

	jsonBytes, err := json.Marshal(args)
	if err != nil {
		// If marshaling fails, return empty object
		return "{}"
	}

	return string(jsonBytes)
}