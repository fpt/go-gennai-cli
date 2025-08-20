package tool

import (
	"context"
	"testing"

	"github.com/fpt/go-gennai-cli/pkg/message"
)

func TestSolverToolManager_GetTools(t *testing.T) {
	manager := NewSolverToolManager()
	tools := manager.GetTools()

	expectedTools := 3 // solve_csp, solve_shortest_path, solve_topological_sort
	if len(tools) != expectedTools {
		t.Errorf("Expected %d tools, got %d", expectedTools, len(tools))
	}

	// Test that all expected tools exist
	expectedToolNames := []string{"solve_csp", "solve_shortest_path", "solve_topological_sort"}
	for _, toolName := range expectedToolNames {
		if _, exists := tools[message.ToolName(toolName)]; !exists {
			t.Errorf("Expected %s tool to exist", toolName)
		}
	}

	// Test that the solve_csp tool exists
	tool, exists := tools["solve_csp"]
	if !exists {
		t.Error("Expected solve_csp tool to exist")
	}

	if tool.Name() != "solve_csp" {
		t.Errorf("Expected tool name to be 'solve_csp', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Expected tool to have a description")
	}

	args := tool.Arguments()
	if len(args) != 4 {
		t.Errorf("Expected 4 arguments, got %d", len(args))
	}

	// Check required arguments
	hasVariables := false
	hasConstraints := false
	hasArcConsistency := false
	hasTimeout := false

	for _, arg := range args {
		switch arg.Name {
		case "variables":
			hasVariables = true
			if !arg.Required {
				t.Error("Variables argument should be required")
			}
		case "constraints":
			hasConstraints = true
			if !arg.Required {
				t.Error("Constraints argument should be required")
			}
		case "use_arc_consistency":
			hasArcConsistency = true
			if arg.Required {
				t.Error("Arc consistency argument should be optional")
			}
		case "timeout_seconds":
			hasTimeout = true
			if arg.Required {
				t.Error("Timeout argument should be optional")
			}
		}
	}

	if !hasVariables {
		t.Error("Missing variables argument")
	}
	if !hasConstraints {
		t.Error("Missing constraints argument")
	}
	if !hasArcConsistency {
		t.Error("Missing use_arc_consistency argument")
	}
	if !hasTimeout {
		t.Error("Missing timeout_seconds argument")
	}
}

func TestSolverToolManager_CallTool(t *testing.T) {
	manager := NewSolverToolManager()
	ctx := context.Background()

	// Test successful call
	args := message.ToolArgumentValues{
		"variables":           `{"X":[1,2,3], "Y":[1,2,3]}`,
		"constraints":         `["X != Y"]`,
		"use_arc_consistency": true,
		"timeout_seconds":     10.0,
	}

	result, err := manager.CallTool(ctx, "solve_csp", args)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error in result, got %s", result.Error)
	}

	if result.Text == "" {
		t.Error("Expected text result")
	}

	// Test with missing required argument
	argsIncomplete := message.ToolArgumentValues{
		"variables": `{"X":[1,2,3]}`,
		// missing constraints
	}

	result, err = manager.CallTool(ctx, "solve_csp", argsIncomplete)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result.Error == "" {
		t.Error("Expected error in result for missing constraints argument")
	}

	// Test with non-existent tool
	result, err = manager.CallTool(ctx, "nonexistent", args)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result.Error == "" {
		t.Error("Expected error in result for non-existent tool")
	}
}

func TestSolverToolManager_ToolCount(t *testing.T) {
	manager := NewSolverToolManager()
	tools := manager.GetTools()

	expectedTools := 3 // solve_csp, solve_shortest_path, solve_topological_sort
	if len(tools) != expectedTools {
		t.Errorf("Expected %d tools, got %d", expectedTools, len(tools))
	}

	expectedToolNames := []string{"solve_csp", "solve_shortest_path", "solve_topological_sort"}
	for _, toolName := range expectedToolNames {
		if _, exists := tools[message.ToolName(toolName)]; !exists {
			t.Errorf("Expected %s tool in tools map", toolName)
		}
	}
}
