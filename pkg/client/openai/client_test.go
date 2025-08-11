package openai

import (
	"testing"
)

func TestGetOpenAIModel(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"gpt-4o", "gpt-4o"},
		{"gpt-4o-mini", "gpt-4o-mini"},
		{"gpt-5", "gpt-5"},
		{"gpt-5-mini", "gpt-5-mini"},
		{"unknown-model", "gpt-4o"}, // default fallback
	}

	for _, tc := range testCases {
		result := getOpenAIModel(tc.input)
		if result != tc.expected {
			t.Errorf("getOpenAIModel(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestGetModelCapabilities(t *testing.T) {
	testCases := []struct {
		model                string
		expectedVision       bool
		expectedToolCalling  bool
		expectedStructured   bool
		expectedSystemPrompt bool
	}{
		{"gpt-4o", true, true, true, true},
		{"gpt-4o-mini", true, true, true, true},
		{"gpt-5", true, true, true, true},
		{"gpt-5-mini", true, true, true, true},
		{"unknown-model", false, true, true, true}, // default capabilities
	}

	for _, tc := range testCases {
		caps := getModelCapabilities(tc.model)

		if caps.SupportsVision != tc.expectedVision {
			t.Errorf("Model %s vision support: got %v, expected %v", tc.model, caps.SupportsVision, tc.expectedVision)
		}

		if caps.SupportsToolCalling != tc.expectedToolCalling {
			t.Errorf("Model %s tool calling support: got %v, expected %v", tc.model, caps.SupportsToolCalling, tc.expectedToolCalling)
		}

		if caps.SupportsStructured != tc.expectedStructured {
			t.Errorf("Model %s structured output support: got %v, expected %v", tc.model, caps.SupportsStructured, tc.expectedStructured)
		}

		if caps.SupportsSystemPrompt != tc.expectedSystemPrompt {
			t.Errorf("Model %s system prompt support: got %v, expected %v", tc.model, caps.SupportsSystemPrompt, tc.expectedSystemPrompt)
		}
	}
}

func TestValidateModelForCapability(t *testing.T) {
	testCases := []struct {
		model      string
		capability string
		shouldErr  bool
	}{
		{"gpt-4o", "vision", false},
		{"gpt-4o", "tool_calling", false},
		{"gpt-4o", "structured_output", false},
		{"gpt-4o", "system_prompt", false},
		{"gpt-4o-mini", "vision", false},
		{"gpt-4o-mini", "tool_calling", false},
		{"unknown-model", "tool_calling", false}, // default supports tool calling
	}

	for _, tc := range testCases {
		err := validateModelForCapability(tc.model, tc.capability)

		if tc.shouldErr && err == nil {
			t.Errorf("validateModelForCapability(%s, %s) expected error but got none", tc.model, tc.capability)
		}

		if !tc.shouldErr && err != nil {
			t.Errorf("validateModelForCapability(%s, %s) expected no error but got: %v", tc.model, tc.capability, err)
		}
	}
}

// Test that the client can be created without API key (will fail at runtime, but should compile and validate)
func TestNewOpenAIClient_NoAPIKey(t *testing.T) {
	// This test assumes OPENAI_API_KEY is not set in the test environment
	// In a real environment, this would check for the environment variable
	_, err := NewOpenAIClient("gpt-4o")
	if err == nil {
		// If no error, the API key was set in the environment, which is fine for testing
		t.Skip("OPENAI_API_KEY is set in environment, skipping test")
	}

	expectedErr := "OPENAI_API_KEY environment variable not set"
	if err.Error() != expectedErr {
		t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
	}
}

// Test the new IsToolCapable method
func TestIsToolCapable(t *testing.T) {
	// Test with a mock client to avoid requiring API key
	core := &OpenAICore{
		model: "gpt-4o", // This model supports tool calling
	}
	client := NewOpenAIClientFromCore(core)
	
	// Cast back to concrete type to access IsToolCapable method
	concreteClient, ok := client.(*OpenAIClient)
	if !ok {
		t.Fatal("Expected *OpenAIClient type")
	}
	
	if !concreteClient.IsToolCapable() {
		t.Error("Expected gpt-4o to support tool calling")
	}
}

