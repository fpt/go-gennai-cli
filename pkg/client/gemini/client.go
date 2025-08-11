package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"

	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// GeminiCore holds shared resources for Gemini clients
type GeminiCore struct {
	client    *genai.Client
	model     string
	maxTokens int
}

// GeminiClient implements ToolCallingLLM and VisionLLM interfaces
type GeminiClient struct {
	*GeminiCore
	toolManager domain.ToolManager
}

// NewGeminiClient creates a new Gemini client with the specified model
func NewGeminiClient(model string) (*GeminiClient, error) {
	return NewGeminiClientWithTokens(model, 0) // 0 = use default
}

// NewGeminiClientWithTokens creates a new Gemini client with configurable maxTokens
func NewGeminiClientWithTokens(model string, maxTokens int) (*GeminiClient, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Validate and map model name
	geminiModel := getGeminiModel(model)

	// Use default maxTokens if not specified
	if maxTokens <= 0 {
		maxTokens = getModelCapabilities(geminiModel).MaxTokens
	}

	core := &GeminiCore{
		client:    client,
		model:     geminiModel,
		maxTokens: maxTokens,
	}

	return &GeminiClient{
		GeminiCore: core,
	}, nil
}

// NewGeminiClientFromCore creates a new client instance from existing core (for factory pattern)
func NewGeminiClientFromCore(core *GeminiCore) *GeminiClient {
	return &GeminiClient{
		GeminiCore: core,
	}
}

// Chat implements the basic LLM interface
func (c *GeminiClient) Chat(ctx context.Context, messages []message.Message) (message.Message, error) {
	// Convert internal messages to Gemini format
	geminiContents := make([]*genai.Content, 0)
	var systemInstruction *genai.Content

	for _, msg := range messages {
		switch msg.Type() {
		case message.MessageTypeUser:
			// Handle images if present
			if images := msg.Images(); len(images) > 0 {
				// Create parts with both text and images
				parts := []*genai.Part{}
				if content := msg.Content(); content != "" {
					parts = append(parts, &genai.Part{Text: content})
				}
				// TODO: Implement image handling with proper base64 decoding
				// For now, just add text content
				geminiContents = append(geminiContents, genai.NewContentFromParts(parts, genai.RoleUser))
			} else {
				geminiContents = append(geminiContents, genai.NewContentFromText(msg.Content(), genai.RoleUser))
			}

		case message.MessageTypeAssistant:
			// Add assistant messages as context
			geminiContents = append(geminiContents, genai.NewContentFromText(msg.Content(), genai.RoleModel))

		case message.MessageTypeSystem:
			// Use the last system message as system instruction
			systemInstruction = genai.NewContentFromText(msg.Content(), genai.RoleUser)

			// Skip tool call and result messages for basic chat
		}
	}

	// Prepare configuration
	config := &genai.GenerateContentConfig{
		MaxOutputTokens: int32(c.maxTokens),
	}
	if systemInstruction != nil {
		config.SystemInstruction = systemInstruction
	}

	// Generate content using the Models interface
	resp, err := c.client.Models.GenerateContent(ctx, c.model, geminiContents, config)
	if err != nil {
		return nil, fmt.Errorf("Gemini API call failed: %w", err)
	}

	// Debug: Log token usage from Gemini API response
	if resp.UsageMetadata != nil {
		maxTokens := c.maxTokens
		inputTokens := resp.UsageMetadata.PromptTokenCount
		outputTokens := resp.UsageMetadata.CandidatesTokenCount
		totalTokens := resp.UsageMetadata.TotalTokenCount
		utilizationPct := float64(outputTokens) / float64(maxTokens) * 100

		fmt.Printf("DEBUG: Gemini API Usage - Input: %d tokens, Output: %d tokens, Total: %d tokens, Model: %s\n",
			inputTokens, outputTokens, totalTokens, c.model)
		fmt.Printf("DEBUG: Token Utilization - %.1f%% of max output tokens (%d/%d)\n",
			utilizationPct, outputTokens, maxTokens)

		// Warn if we're approaching the limit
		if utilizationPct > 90 {
			fmt.Printf("⚠️  WARNING: Very high token usage (%.1f%%) - potential truncation risk!\n", utilizationPct)
		} else if utilizationPct > 80 {
			fmt.Printf("⚠️  WARNING: High token usage (%.1f%%) - approaching limit\n", utilizationPct)
		}
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	// Extract text content from response
	responseText := resp.Text()
	if responseText == "" {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	return message.NewChatMessage(message.MessageTypeAssistant, responseText), nil
}

// SetToolManager implements ToolCallingLLM interface
func (c *GeminiClient) SetToolManager(toolManager domain.ToolManager) {
	c.toolManager = toolManager
}

// GetToolManager implements ToolCallingLLM interface
func (c *GeminiClient) GetToolManager() domain.ToolManager {
	return c.toolManager
}

// ChatWithToolChoice implements ToolCallingLLM interface with basic functionality
func (c *GeminiClient) ChatWithToolChoice(ctx context.Context, messages []message.Message, toolChoice domain.ToolChoice) (message.Message, error) {
	// For now, fall back to regular chat - tool calling will be implemented in a future version
	// This ensures Gemini integration works immediately while tool calling can be added later
	return c.Chat(ctx, messages)
}

// SupportsVision implements VisionLLM interface
func (c *GeminiClient) SupportsVision() bool {
	// Gemini Pro Vision models support vision
	return strings.Contains(c.model, "vision") || strings.Contains(c.model, "gemini-pro-vision") ||
		strings.Contains(c.model, "gemini-2.0") || strings.Contains(c.model, "gemini-1.5")
}

// convertGeminiArgsToToolArgs converts Gemini function arguments JSON to tool argument values
func convertGeminiArgsToToolArgs(argsJSON string) message.ToolArgumentValues {
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
