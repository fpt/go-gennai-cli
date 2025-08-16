package tool

import (
	"context"
	"strings"
	"testing"

	"github.com/fpt/go-gennai-cli/pkg/message"
)

func TestSolverToolManager_IntegrationTest(t *testing.T) {
	manager := NewSolverToolManager()
	ctx := context.Background()

	// Test simple CSP: X != Y with domains [1,2]
	args := message.ToolArgumentValues{
		"variables":            `{"X":[1,2], "Y":[1,2]}`,
		"constraints":          `["X != Y"]`,
		"use_arc_consistency":  true,
		"timeout_seconds":      5.0,
	}

	result, err := manager.CallTool(ctx, "solve_csp", args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Error != "" {
		t.Fatalf("Expected no error in result, got: %s", result.Error)
	}

	if !strings.Contains(result.Text, "✅ SOLUTION FOUND") {
		t.Errorf("Expected solution to be found, got: %s", result.Text)
	}

	// Should have both variables assigned
	if !strings.Contains(result.Text, "X = ") {
		t.Error("Expected X to be assigned in solution")
	}
	if !strings.Contains(result.Text, "Y = ") {
		t.Error("Expected Y to be assigned in solution")
	}
}

func TestSolverToolManager_UnsatisfiableCSP(t *testing.T) {
	manager := NewSolverToolManager()
	ctx := context.Background()

	// Test unsatisfiable CSP: X = Y and X != Y with same domains
	// Disable arc consistency to avoid panics in the centipede library
	args := message.ToolArgumentValues{
		"variables":            `{"X":[1], "Y":[1]}`,
		"constraints":          `["X = Y", "X != Y"]`,
		"use_arc_consistency":  false, // Avoid centipede library panic
		"timeout_seconds":      5.0,
	}

	result, err := manager.CallTool(ctx, "solve_csp", args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Error != "" {
		t.Fatalf("Expected no error in result, got: %s", result.Error)
	}

	if !strings.Contains(result.Text, "❌ NO SOLUTION FOUND") {
		t.Errorf("Expected no solution for unsatisfiable CSP, got: %s", result.Text)
	}
}

func TestSolverToolManager_AllUniqueConstraint(t *testing.T) {
	manager := NewSolverToolManager()
	ctx := context.Background()

	// Test AllUnique constraint: three variables all must be different
	args := message.ToolArgumentValues{
		"variables":            `{"X":[1,2,3], "Y":[1,2,3], "Z":[1,2,3]}`,
		"constraints":          `["AllUnique([X,Y,Z])"]`,
		"use_arc_consistency":  false, // Test without arc consistency
		"timeout_seconds":      5.0,
	}

	result, err := manager.CallTool(ctx, "solve_csp", args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Error != "" {
		t.Fatalf("Expected no error in result, got: %s", result.Error)
	}

	if !strings.Contains(result.Text, "✅ SOLUTION FOUND") {
		t.Errorf("Expected solution to be found for AllUnique, got: %s", result.Text)
	}

	// All three variables should be assigned with different values
	resultText := result.Text
	hasX := strings.Contains(resultText, "X = ")
	hasY := strings.Contains(resultText, "Y = ")
	hasZ := strings.Contains(resultText, "Z = ")

	if !hasX || !hasY || !hasZ {
		t.Errorf("Expected all variables to be assigned, got: %s", resultText)
	}
}

func TestSolverToolManager_UnaryConstraints(t *testing.T) {
	manager := NewSolverToolManager()
	ctx := context.Background()

	// Test unary constraints: X = 2, Y != 2
	args := message.ToolArgumentValues{
		"variables":            `{"X":[1,2,3], "Y":[1,2,3]}`,
		"constraints":          `["X = 2", "Y != 2"]`,
		"use_arc_consistency":  true,
		"timeout_seconds":      5.0,
	}

	result, err := manager.CallTool(ctx, "solve_csp", args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Error != "" {
		t.Fatalf("Expected no error in result, got: %s", result.Error)
	}

	if !strings.Contains(result.Text, "✅ SOLUTION FOUND") {
		t.Errorf("Expected solution to be found, got: %s", result.Text)
	}

	// X should be 2, Y should be 1 or 3
	if !strings.Contains(result.Text, "X = 2") {
		t.Error("Expected X = 2 in solution")
	}

	if strings.Contains(result.Text, "Y = 2") {
		t.Error("Y should not be 2 due to constraint Y != 2")
	}
}

func TestSolverToolManager_InvalidJSON(t *testing.T) {
	manager := NewSolverToolManager()
	ctx := context.Background()

	// Test invalid JSON in variables
	args := message.ToolArgumentValues{
		"variables":            `{"X":[1,2,3], "Y":invalid}`, // invalid JSON
		"constraints":          `["X != Y"]`,
		"use_arc_consistency":  true,
		"timeout_seconds":      5.0,
	}

	result, err := manager.CallTool(ctx, "solve_csp", args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("Expected error for invalid JSON, got successful result")
	}

	if !strings.Contains(result.Error, "Failed to solve CSP") {
		t.Errorf("Expected specific error message, got: %s", result.Error)
	}
}