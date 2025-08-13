package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fpt/go-gennai-cli/internal/config"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// TodoItem represents a single todo item
type TodoItem struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Status   string `json:"status"`   // pending, in_progress, done
	Priority string `json:"priority"` // high, medium, low
	Created  string `json:"created"`
	Updated  string `json:"updated"`
}

// TodoState represents the current state of all todos
type TodoState struct {
	Items    []TodoItem `json:"items"`
	Updated  string     `json:"updated"`
	mu       sync.RWMutex
	filePath string
}

// TodoToolManager provides task management capabilities
type TodoToolManager struct {
	tools     map[message.ToolName]message.Tool
	todoState *TodoState
}

// NewTodoToolManager creates a new todo tool manager using user config
func NewTodoToolManager(projectPath string) *TodoToolManager {
	// Get user configuration - this must succeed
	userConfig, err := config.DefaultUserConfig()
	if err != nil {
		logger.ErrorWithIcon("❌", "Failed to create user config directory", "error", err)
		logger.ErrorWithIcon("❌", "gennai requires $HOME/.gennai/ directory access")
		os.Exit(1)
	}

	// Get project-specific todo file - this must also succeed
	todoFilePath, err := userConfig.GetProjectTodoFile(projectPath)
	if err != nil {
		logger.ErrorWithIcon("❌", "Failed to get project todo file", "error", err)
		logger.ErrorWithIcon("❌", "Cannot create project directory in $HOME/.gennai/projects/")
		os.Exit(1)
	}

	return NewTodoToolManagerWithPath(todoFilePath)
}

// NewTodoToolManagerWithPath creates a new todo tool manager with a specific file path
func NewTodoToolManagerWithPath(todoFilePath string) *TodoToolManager {
	manager := &TodoToolManager{
		tools: make(map[message.ToolName]message.Tool),
		todoState: &TodoState{
			Items:    make([]TodoItem, 0),
			filePath: todoFilePath,
		},
	}

	// Load existing todos
	_ = manager.todoState.loadFromFile()

	// Register todo tools
	manager.registerTodoTools()

	return manager
}

// NewInMemoryTodoToolManager creates a new todo tool manager that only stores data in memory (no persistence)
func NewInMemoryTodoToolManager() *TodoToolManager {
	manager := &TodoToolManager{
		tools: make(map[message.ToolName]message.Tool),
		todoState: &TodoState{
			Items:    make([]TodoItem, 0),
			filePath: "", // Empty path means no persistence
		},
	}

	// No file loading for in-memory manager

	// Register todo tools
	manager.registerTodoTools()

	return manager
}

// Implement domain.ToolManager interface
func (m *TodoToolManager) GetTool(name message.ToolName) (message.Tool, bool) {
	tool, exists := m.tools[name]
	return tool, exists
}

func (m *TodoToolManager) GetTools() map[message.ToolName]message.Tool {
	return m.tools
}

func (m *TodoToolManager) CallTool(ctx context.Context, name message.ToolName, args message.ToolArgumentValues) (message.ToolResult, error) {
	tool, exists := m.tools[name]
	if !exists {
		return message.NewToolResultError(fmt.Sprintf("tool %s not found", name)), nil
	}

	handler := tool.Handler()
	return handler(ctx, args)
}

func (m *TodoToolManager) RegisterTool(name message.ToolName, description message.ToolDescription, args []message.ToolArgument, handler func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)) {
	tool := &todoTool{
		name:        name,
		description: description,
		arguments:   args,
		handler:     handler,
	}
	m.tools[name] = tool
}

// registerTodoTools registers all todo management tools
func (m *TodoToolManager) registerTodoTools() {
	// TodoWrite - Write/update todo list (Claude Code-style)
	// Note: TodoRead removed - todos are injected into prompt context instead
	m.RegisterTool("todo_write", "Write or update the todo list with tasks and their status. IMPORTANT: Keep todos to 5 items or fewer for focus and clarity. Only use for complex multi-step tasks.",
		[]message.ToolArgument{
			{
				Name:        "todos",
				Description: "Array of todo items with content, status, priority, and id",
				Required:    true,
				Type:        "array",
			},
		},
		m.handleTodoWrite)
}

// TodoWrite handler - Claude Code style
func (m *TodoToolManager) handleTodoWrite(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	todosArg, ok := args["todos"]
	if !ok {
		return message.NewToolResultError("todos parameter is required"), nil
	}

	// Handle different input formats
	var todoItems []TodoItem

	// Try to parse as JSON array first
	if todosJSON, ok := todosArg.(string); ok {
		if err := json.Unmarshal([]byte(todosJSON), &todoItems); err != nil {
			return message.NewToolResultError(fmt.Sprintf("failed to parse todos JSON: %v", err)), nil
		}
	} else if todosSlice, ok := todosArg.([]interface{}); ok {
		// Handle slice of interfaces (from structured LLM output)
		for _, item := range todosSlice {
			if todoMap, ok := item.(map[string]interface{}); ok {
				todoItem := TodoItem{
					Created: time.Now().Format(time.RFC3339),
					Updated: time.Now().Format(time.RFC3339),
				}

				if id, ok := todoMap["id"].(string); ok {
					todoItem.ID = id
				}
				if content, ok := todoMap["content"].(string); ok {
					todoItem.Content = content
				}
				if status, ok := todoMap["status"].(string); ok {
					todoItem.Status = status
				}
				if priority, ok := todoMap["priority"].(string); ok {
					todoItem.Priority = priority
				}

				// Validate required fields
				if todoItem.ID == "" || todoItem.Content == "" || todoItem.Status == "" || todoItem.Priority == "" {
					return message.NewToolResultError("all todo items must have id, content, status, and priority"), nil
				}

				todoItems = append(todoItems, todoItem)
			}
		}
	} else {
		return message.NewToolResultError("todos parameter must be a JSON array or array of objects"), nil
	}

	// Enforce maximum of 5 todos for focus and clarity
	if len(todoItems) > 5 {
		return message.NewToolResultError("Too many todo items. Please limit to 5 items or fewer for better focus and management."), nil
	}

	// Validate todo items
	for _, item := range todoItems {
		if item.ID == "" || item.Content == "" {
			return message.NewToolResultError("all todo items must have id and content"), nil
		}
		if item.Status != "pending" && item.Status != "in_progress" && item.Status != "done" {
			return message.NewToolResultError(fmt.Sprintf("invalid status '%s', must be pending, in_progress, or done", item.Status)), nil
		}
		if item.Priority != "high" && item.Priority != "medium" && item.Priority != "low" {
			return message.NewToolResultError(fmt.Sprintf("invalid priority '%s', must be high, medium, or low", item.Priority)), nil
		}
	}

	// Update todo state
	m.todoState.mu.Lock()
	defer m.todoState.mu.Unlock()

	// Replace all todos with new list
	m.todoState.Items = todoItems
	m.todoState.Updated = time.Now().Format(time.RFC3339)

	// Save to file
	if err := m.todoState.saveToFile(); err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to save todos: %v", err)), nil
	}

	// Generate summary response
	statusCounts := make(map[string]int)
	priorityCounts := make(map[string]int)

	for _, item := range todoItems {
		statusCounts[item.Status]++
		priorityCounts[item.Priority]++
	}

	summary := fmt.Sprintf("Successfully updated todo list with %d items:\n", len(todoItems))
	summary += fmt.Sprintf("- Status: %d pending, %d in_progress, %d done\n",
		statusCounts["pending"], statusCounts["in_progress"], statusCounts["done"])
	summary += fmt.Sprintf("- Priority: %d high, %d medium, %d low",
		priorityCounts["high"], priorityCounts["medium"], priorityCounts["low"])

	return message.NewToolResultText(summary), nil
}

// GetTodosForPrompt returns formatted todos for injection into prompt context (Claude Code-style)
func (m *TodoToolManager) GetTodosForPrompt() string {
	m.todoState.mu.RLock()
	defer m.todoState.mu.RUnlock()

	if len(m.todoState.Items) == 0 {
		return ""
	}

	// Format todos for prompt injection
	var result string
	result += fmt.Sprintf("Current Todo List (%d items):\n\n", len(m.todoState.Items))

	// Group by status for better readability
	statusGroups := map[string][]TodoItem{
		"in_progress": {},
		"pending":     {},
		"done":        {},
	}

	for _, item := range m.todoState.Items {
		statusGroups[item.Status] = append(statusGroups[item.Status], item)
	}

	// Display in_progress first, then pending, then done
	for _, status := range []string{"in_progress", "pending", "done"} {
		items := statusGroups[status]
		if len(items) == 0 {
			continue
		}

		result += fmt.Sprintf("## %s (%d items):\n", status, len(items))
		for _, item := range items {
			// Use raw API format for LLM consumption (not display format with emojis)
			// This ensures LLM uses correct format when calling todo_write tool
			result += fmt.Sprintf("- [%s] %s - %s (ID: %s)\n", item.Priority, item.Content, item.Status, item.ID)
		}
		result += "\n"
	}

	result += fmt.Sprintf("Last updated: %s", m.todoState.Updated)

	return result
}

// File persistence methods
func (ts *TodoState) loadFromFile() error {
	if ts.filePath == "" {
		return fmt.Errorf("no file path specified")
	}

	data, err := os.ReadFile(ts.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, start with empty state
			return nil
		}
		return err
	}

	return json.Unmarshal(data, ts)
}

func (ts *TodoState) saveToFile() error {
	if ts.filePath == "" {
		// In-memory mode - no persistence, silently skip saving
		return nil
	}

	data, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(ts.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(ts.filePath, data, 0644)
}

// todoTool is a helper struct for todo tool registration
type todoTool struct {
	name        message.ToolName
	description message.ToolDescription
	arguments   []message.ToolArgument
	handler     func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)
}

func (t *todoTool) RawName() message.ToolName {
	return t.name
}

func (t *todoTool) Name() message.ToolName {
	return t.name
}

func (t *todoTool) Description() message.ToolDescription {
	return t.description
}

func (t *todoTool) Arguments() []message.ToolArgument {
	return t.arguments
}

func (t *todoTool) Handler() func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	return t.handler
}
