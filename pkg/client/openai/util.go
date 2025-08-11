package openai

import (
	"fmt"
)

// Model constants
const (
	modelGPT5      = "gpt-5"
	modelGPT5Mini  = "gpt-5-mini"
	modelGPT4o     = "gpt-4o"
	modelGPT4oMini = "gpt-4o-mini"
)

// getOpenAIModel maps user-friendly model names to actual OpenAI model identifiers
func getOpenAIModel(model string) string {
	// Normalize the model name
	switch model {
	case modelGPT5:
		return modelGPT5
	case modelGPT5Mini:
		return modelGPT5Mini
	case modelGPT4o:
		return modelGPT4o
	case modelGPT4oMini:
		return modelGPT4oMini

	default:
		// If it's already a valid OpenAI model name, return as-is
		if isValidOpenAIModel(model) {
			return model
		}
		// Default to gpt-4o for unknown models
		return "gpt-4o"
	}
}

// isValidOpenAIModel checks if a model name is a valid OpenAI model
func isValidOpenAIModel(model string) bool {
	validModels := map[string]bool{
		"gpt-5":       true,
		"gpt-5-mini":  true,
		"gpt-4o":      true,
		"gpt-4o-mini": true,
	}
	return validModels[model]
}

// getModelCapabilities returns capabilities for a given OpenAI model
type ModelCapabilities struct {
	SupportsVision       bool
	SupportsToolCalling  bool
	SupportsStructured   bool
	MaxTokens            int
	SupportsSystemPrompt bool
}

// getModelCapabilities returns the capabilities of a specific OpenAI model
func getModelCapabilities(model string) ModelCapabilities {
	switch model {
	case modelGPT5:
		return ModelCapabilities{
			SupportsVision:       true,
			SupportsToolCalling:  true,
			SupportsStructured:   true,
			MaxTokens:            16384,
			SupportsSystemPrompt: true,
		}
	case modelGPT5Mini:
		return ModelCapabilities{
			SupportsVision:       true,
			SupportsToolCalling:  true,
			SupportsStructured:   true,
			MaxTokens:            16384,
			SupportsSystemPrompt: true,
		}
	case modelGPT4o:
		return ModelCapabilities{
			SupportsVision:       true,
			SupportsToolCalling:  true,
			SupportsStructured:   true,
			MaxTokens:            8192,
			SupportsSystemPrompt: true,
		}
	case modelGPT4oMini:
		return ModelCapabilities{
			SupportsVision:       true,
			SupportsToolCalling:  true,
			SupportsStructured:   true,
			MaxTokens:            4096,
			SupportsSystemPrompt: true,
		}
	default:
		// Default capabilities for unknown models
		return ModelCapabilities{
			SupportsVision:       false,
			SupportsToolCalling:  true,
			SupportsStructured:   true,
			MaxTokens:            4096,
			SupportsSystemPrompt: true,
		}
	}
}

// validateModelForCapability checks if a model supports a specific capability
func validateModelForCapability(model string, capability string) error {
	caps := getModelCapabilities(model)

	switch capability {
	case "tool_calling":
		if !caps.SupportsToolCalling {
			return fmt.Errorf("model %s does not support tool calling", model)
		}
	case "vision":
		if !caps.SupportsVision {
			return fmt.Errorf("model %s does not support vision", model)
		}
	case "structured_output":
		if !caps.SupportsStructured {
			return fmt.Errorf("model %s does not support structured output", model)
		}
	case "system_prompt":
		if !caps.SupportsSystemPrompt {
			return fmt.Errorf("model %s does not support system prompts", model)
		}
	}

	return nil
}