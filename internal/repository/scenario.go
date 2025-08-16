package repository

// ToolScope represents which tool managers to use for a scenario
type ToolScope struct {
	UseFilesystem bool
	UseDefault    bool
	UseTodo       bool
	UseBash       bool
	UseSolver     bool
	MCPTools      []string // List of MCP tool manager names (e.g., ["serverA", "serverB"])
}

// Scenario represents a scenario configuration from YAML
type Scenario interface {
	Name() string
	Tools() string
	Description() string
	Prompt() string
	GetToolScope() ToolScope
	RenderPrompt(userInput, scenarioReason, workingDir string) string
}
