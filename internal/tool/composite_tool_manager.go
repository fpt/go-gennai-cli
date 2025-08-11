package tool

import (
	"context"
	"fmt"

	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// CompositeToolManager combines multiple tool managers into one
type CompositeToolManager struct {
	managers []domain.ToolManager
	toolsMap map[message.ToolName]message.Tool
}

// NewCompositeToolManager creates a new composite tool manager from multiple managers
func NewCompositeToolManager(managers ...domain.ToolManager) *CompositeToolManager {
	composite := &CompositeToolManager{
		managers: managers,
		toolsMap: make(map[message.ToolName]message.Tool),
	}

	// Build unified tools map from all managers
	for _, manager := range managers {
		tools := manager.GetTools()
		for _, tool := range tools {
			composite.toolsMap[tool.Name()] = tool
		}
	}

	return composite
}

// GetTool returns a tool by name from any of the managed tool managers
func (c *CompositeToolManager) GetTool(name message.ToolName) (message.Tool, bool) {
	tool, exists := c.toolsMap[name]
	return tool, exists
}

// GetTools returns all tools from all managed tool managers
func (c *CompositeToolManager) GetTools() map[message.ToolName]message.Tool {
	return c.toolsMap
}

// CallTool executes a tool from any of the managed tool managers
func (c *CompositeToolManager) CallTool(ctx context.Context, name message.ToolName, args message.ToolArgumentValues) (message.ToolResult, error) {
	tool, exists := c.toolsMap[name]
	if !exists {
		return message.NewToolResultError(fmt.Sprintf("tool %s not found", name)), nil
	}

	handler := tool.Handler()
	return handler(ctx, args)
}

// RegisterTool is not supported on composite managers since tools should be registered on the underlying managers
func (c *CompositeToolManager) RegisterTool(name message.ToolName, description message.ToolDescription, args []message.ToolArgument, handler func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)) {
	panic("RegisterTool not supported on CompositeToolManager - register on underlying managers instead")
}
