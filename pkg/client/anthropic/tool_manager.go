package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// emptyToolManager is a no-op tool manager
type emptyToolManager struct{}

func (etm *emptyToolManager) GetTools() map[message.ToolName]message.Tool {
	return make(map[message.ToolName]message.Tool)
}

// Note: ToolManager interface doesn't have Execute method

func (etm *emptyToolManager) CallTool(ctx context.Context, name message.ToolName, args message.ToolArgumentValues) (message.ToolResult, error) {
	return message.NewToolResultError(fmt.Sprintf("tool '%s' not found", name)), nil
}

func (etm *emptyToolManager) RegisterTool(name message.ToolName, description message.ToolDescription, arguments []message.ToolArgument, handler func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)) {
	// No-op
}

// mergedToolManager combines schema tool with existing tools
type mergedToolManager struct {
	schemaTool      message.Tool
	existingManager domain.ToolManager
}

func (mtm *mergedToolManager) GetTools() map[message.ToolName]message.Tool {
	// Start with existing tools
	allTools := make(map[message.ToolName]message.Tool)
	for name, tool := range mtm.existingManager.GetTools() {
		allTools[name] = tool
	}

	// Add schema tool
	allTools[mtm.schemaTool.Name()] = mtm.schemaTool

	return allTools
}

// Note: ToolManager interface doesn't have Execute method
// The Execute method is only needed for schemaToolManager compatibility

func (mtm *mergedToolManager) CallTool(ctx context.Context, name message.ToolName, args message.ToolArgumentValues) (message.ToolResult, error) {
	// If it's the schema tool, handle it specially
	if name == mtm.schemaTool.Name() {
		// For schema-as-tool, we don't actually execute the tool
		// We just return the arguments as JSON
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return message.NewToolResultError(fmt.Sprintf("failed to marshal tool arguments: %v", err)), nil
		}

		return message.NewToolResultText(string(argsJSON)), nil
	}

	// Otherwise, delegate to existing manager
	return mtm.existingManager.CallTool(ctx, name, args)
}

func (mtm *mergedToolManager) RegisterTool(name message.ToolName, description message.ToolDescription, arguments []message.ToolArgument, handler func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)) {
	// Delegate to existing manager
	mtm.existingManager.RegisterTool(name, description, arguments, handler)
}
