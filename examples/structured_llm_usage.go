package main

import (
	"context"
	"fmt"
	"log"

	"github.com/fpt/go-gennai-cli/pkg/client"
	"github.com/fpt/go-gennai-cli/pkg/client/ollama"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// Example response structure for structured output
// Using jsonschema tags to add validation constraints
type TaskAnalysis struct {
	Summary    string   `json:"summary" jsonschema:"title=Task Summary,description=Brief summary of the task,minLength=1,maxLength=200"`
	Priority   int      `json:"priority" jsonschema:"minimum=1,maximum=5,description=Priority level from 1 (low) to 5 (high)"`
	Categories []string `json:"categories" jsonschema:"description=List of relevant categories,uniqueItems=true"`
	Completed  bool     `json:"completed" jsonschema:"description=Whether the task is completed"`
}

// Example of a more complex nested structure with jsonschema constraints
type ProjectStatus struct {
	Project struct {
		Name        string `json:"name" jsonschema:"minLength=1,maxLength=100"`
		Description string `json:"description" jsonschema:"maxLength=500"`
	} `json:"project"`
	Tasks []struct {
		ID      int    `json:"id" jsonschema:"minimum=1"`
		Title   string `json:"title" jsonschema:"minLength=1,maxLength=200"`
		Status  string `json:"status" jsonschema:"enum=pending,enum=in-progress,enum=completed,enum=blocked"`
		DueDate string `json:"due_date,omitempty" jsonschema:"format=date"`
	} `json:"tasks" jsonschema:"minItems=0,maxItems=50"`
	Overall struct {
		Progress float64 `json:"progress" jsonschema:"minimum=0,maximum=100"`
		Health   string  `json:"health" jsonschema:"enum=green,enum=yellow,enum=red"`
		Risk     string  `json:"risk" jsonschema:"enum=low,enum=medium,enum=high"`
	} `json:"overall"`
}

func main() {
	fmt.Println("=== StructuredLLM: JSON Schema & Tool Calling Example ===")

	// Example 1: Simple structured output (JSON Schema)
	fmt.Println("\n1. JSON Schema structured output (gemma3):")
	err := simpleStructuredExample()
	if err != nil {
		log.Printf("JSON Schema example failed: %v", err)
	}

	// Example 2: Tool calling structured output (gpt-oss)
	fmt.Println("\n2. Tool calling structured output (gpt-oss):")
	err = toolCallingStructuredExample()
	if err != nil {
		log.Printf("Tool calling example failed: %v", err)
	}

	// Example 3: Complex nested structure
	fmt.Println("\n3. Complex nested structure:")
	err = complexStructuredExample()
	if err != nil {
		log.Printf("Complex example failed: %v", err)
	}

	// Example 4: Model capability detection
	fmt.Println("\n4. Model capability detection:")
	capabilityExample()
}

func simpleStructuredExample() error {
	// Create an Ollama client for a JSON Schema-capable model
	// Note: gpt-oss has native tool calling, so we use gemma3 for JSON Schema
	baseClient, err := ollama.NewOllamaClient("gemma3:latest", 2000, false)
	if err != nil {
		return fmt.Errorf("failed to create Ollama client: %w", err)
	}

	// Create a structured client for TaskAnalysis type
	structuredClient, err := client.NewStructuredClient[TaskAnalysis](baseClient)
	if err != nil {
		return fmt.Errorf("failed to create structured client: %w", err)
	}

	// Create messages for the conversation
	messages := []message.Message{
		message.NewSystemMessage("You are a helpful assistant that analyzes tasks and provides structured responses."),
		message.NewChatMessage(message.MessageTypeUser, "Analyze this task: 'Review the Q3 financial reports and prepare summary for board meeting'"),
	}

	// Get structured response
	result, err := structuredClient.ChatWithStructure(context.Background(), messages, false, nil)
	if err != nil {
		return fmt.Errorf("structured chat failed: %w", err)
	}

	// The result is automatically typed as TaskAnalysis
	fmt.Printf("Task Analysis:\n")
	fmt.Printf("  Summary: %s\n", result.Summary)
	fmt.Printf("  Priority: %d\n", result.Priority)
	fmt.Printf("  Categories: %v\n", result.Categories)
	fmt.Printf("  Completed: %t\n", result.Completed)

	return nil
}

func toolCallingStructuredExample() error {
	// Create an Ollama client for a tool-calling capable model
	// gpt-oss supports native tool calling, perfect for tool-calling based structured output
	baseClient, err := ollama.NewOllamaClient("gpt-oss:latest", 2000, false)
	if err != nil {
		return fmt.Errorf("failed to create Ollama client: %w", err)
	}

	// Create a structured client for TaskAnalysis type
	structuredClient, err := client.NewStructuredClient[TaskAnalysis](baseClient)
	if err != nil {
		return fmt.Errorf("failed to create structured client: %w", err)
	}

	// Create messages for the conversation
	messages := []message.Message{
		message.NewSystemMessage("You are a helpful assistant that analyzes tasks and provides structured responses using tools."),
		message.NewChatMessage(message.MessageTypeUser, "Analyze this task: 'Implement user authentication system with OAuth2 and JWT tokens'"),
	}

	// Get structured response using tool calling
	result, err := structuredClient.ChatWithStructure(context.Background(), messages, false, nil)
	if err != nil {
		return fmt.Errorf("structured chat failed: %w", err)
	}

	// The result is automatically typed as TaskAnalysis
	fmt.Printf("Task Analysis (via Tool Calling):\n")
	fmt.Printf("  Summary: %s\n", result.Summary)
	fmt.Printf("  Priority: %d\n", result.Priority)
	fmt.Printf("  Categories: %v\n", result.Categories)
	fmt.Printf("  Completed: %t\n", result.Completed)

	return nil
}

func complexStructuredExample() error {
	// Create an Ollama client for a JSON Schema-capable model
	baseClient, err := ollama.NewOllamaClient("gemma3:latest", 3000, false)
	if err != nil {
		return fmt.Errorf("failed to create Ollama client: %w", err)
	}

	// Create a structured client for ProjectStatus type
	structuredClient, err := client.NewStructuredClient[ProjectStatus](baseClient)
	if err != nil {
		return fmt.Errorf("failed to create structured client: %w", err)
	}

	// Create messages for the conversation
	messages := []message.Message{
		message.NewSystemMessage("You are a project management assistant that provides detailed project status reports."),
		message.NewChatMessage(message.MessageTypeUser, "Generate a project status report for a web application development project with 3 tasks in various stages"),
	}

	// Get structured response
	result, err := structuredClient.ChatWithStructure(context.Background(), messages, false, nil)
	if err != nil {
		return fmt.Errorf("structured chat failed: %w", err)
	}

	// The result is automatically typed as ProjectStatus
	fmt.Printf("Project Status Report:\n")
	fmt.Printf("  Project: %s - %s\n", result.Project.Name, result.Project.Description)
	fmt.Printf("  Tasks:\n")
	for _, task := range result.Tasks {
		fmt.Printf("    #%d: %s (%s)\n", task.ID, task.Title, task.Status)
	}
	fmt.Printf("  Overall Progress: %.1f%%\n", result.Overall.Progress)
	fmt.Printf("  Health: %s\n", result.Overall.Health)
	fmt.Printf("  Risk: %s\n", result.Overall.Risk)

	return nil
}

func capabilityExample() {
	models := []string{
		"gpt-oss:latest", // Native tool calling - no JSON Schema needed
		"gemma3:latest",  // Known model - JSON Schema support
	}

	fmt.Println("Model capability detection:")
	for _, model := range models {
		toolCapable := ollama.IsToolCapableModel(model)
		schemaCapable := ollama.IsJSONSchemaCapableModel(model)

		fmt.Printf("  %s:\n", model)
		fmt.Printf("    Tool calling: %t\n", toolCapable)
		fmt.Printf("    JSON Schema: %t\n", schemaCapable)

		if schemaCapable {
			fmt.Printf("    → Use StructuredLLM with JSON Schema\n")
		} else if toolCapable {
			fmt.Printf("    → Use native tool calling for structured output\n")
		} else {
			fmt.Printf("    → No structured output support\n")
		}
	}
}
