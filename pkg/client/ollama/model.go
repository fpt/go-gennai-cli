package ollama

import "strings"

type OllamaModel struct {
	Name string `json:"name"`

	// Tool indicates whether the model supports native tool calling
	Tool bool `json:"tool"`

	// Think indicates whether the model supports thinking
	Think bool `json:"think"`

	// Vision indicates whether the model supports image input (multimodal)
	Vision bool `json:"vision"`

	// Context indicates the context length of the model
	Context int `json:"context"`
}

// This is from https://ollama.com/search
// List must be kept in sync with the Ollama models by human.
var ollamaModels = []OllamaModel{
	{
		Name:    "gpt-oss:latest",
		Tool:    true,   // ✅ Confirmed: supports native tool calling perfectly
		Think:   true,   // ✅ Confirmed: shows thinking tokens in CLI and API
		Vision:  false,  // Unknown vision capability
		Context: 128000, // Conservative context estimate
	},
	{
		Name:    "gpt-oss:120b",
		Tool:    true,
		Think:   true,
		Vision:  false,
		Context: 128000,
	},
}

// IsToolCapableModel checks if a model supports native tool calling
func IsToolCapableModel(model string) bool {
	modelLower := strings.ToLower(model)

	// Check against the structured model list
	for _, ollamaModel := range ollamaModels {
		if strings.Contains(modelLower, strings.ToLower(ollamaModel.Name)) {
			return ollamaModel.Tool
		}
	}

	return false
}

// IsThinkingCapableModel checks if a model supports thinking/reasoning
func IsThinkingCapableModel(model string) bool {
	modelLower := strings.ToLower(model)

	// Check against the structured model list
	for _, ollamaModel := range ollamaModels {
		if strings.Contains(modelLower, strings.ToLower(ollamaModel.Name)) {
			return ollamaModel.Think
		}
	}

	return false
}

// IsVisionCapableModel checks if a model supports vision/image input
func IsVisionCapableModel(model string) bool {
	modelLower := strings.ToLower(model)

	// Check against the structured model list
	for _, ollamaModel := range ollamaModels {
		if strings.Contains(modelLower, strings.ToLower(ollamaModel.Name)) {
			return ollamaModel.Vision
		}
	}

	return false
}

// IsModelInKnownList checks if a model is in our known models list
func IsModelInKnownList(model string) bool {
	modelLower := strings.ToLower(model)

	// Check against the structured model list
	for _, ollamaModel := range ollamaModels {
		if strings.Contains(modelLower, strings.ToLower(ollamaModel.Name)) {
			return true
		}
	}

	return false
}
