package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/message"
	"github.com/gnboorse/centipede"
)

// SolverToolManager provides simple CSP solver-related tools
type SolverToolManager struct {
	tools map[message.ToolName]message.Tool
}

// NewSolverToolManager creates a new solver tool manager with all solver-related tools
func NewSolverToolManager() domain.ToolManager {
	m := &SolverToolManager{
		tools: make(map[message.ToolName]message.Tool),
	}

	// Register all solver-related tools
	m.registerSolverTools()
	return m
}

func (m *SolverToolManager) registerSolverTools() {
	m.RegisterTool("solve_csp", "Solve a Constraint Satisfaction Problem (CSP) using the centipede library with backtracking and arc consistency",
		[]message.ToolArgument{
			{
				Name:        "variables",
				Description: "JSON object mapping variable names to their domains. Example: '{\"X\":[1,2,3], \"Y\":[1,2,3], \"Z\":[1,2,3]}'",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "constraints",
				Description: "JSON array of constraint expressions. Supported: 'X = Y', 'X != Y', 'X < Y', 'X <= Y', 'X > Y', 'X >= Y', 'X = 5', 'AllUnique([X,Y,Z])', arithmetic expressions like '2*X + 3*Y = 10', 'X + Y + Z >= 5'. Example: '[\"X != Y\", \"Y != Z\", \"2*X + Y = 10\"]'",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "use_arc_consistency",
				Description: "Whether to use arc consistency preprocessing (AC-3 algorithm) before backtracking. Default: true",
				Required:    false,
				Type:        "boolean",
			},
			{
				Name:        "timeout_seconds",
				Description: "Maximum time to spend solving in seconds. Default: 30",
				Required:    false,
				Type:        "number",
			},
		},
		m.handleSolveCSP)
}

// Implement domain.ToolManager interface
func (m *SolverToolManager) GetTool(name message.ToolName) (message.Tool, bool) {
	tool, exists := m.tools[name]
	return tool, exists
}

func (m *SolverToolManager) GetTools() map[message.ToolName]message.Tool {
	return m.tools
}

func (m *SolverToolManager) CallTool(ctx context.Context, name message.ToolName, args message.ToolArgumentValues) (message.ToolResult, error) {
	tool, exists := m.tools[name]
	if !exists {
		return message.NewToolResultError(fmt.Sprintf("tool '%s' not found", name)), nil
	}

	return tool.Handler()(ctx, args)
}

func (m *SolverToolManager) RegisterTool(name message.ToolName, description message.ToolDescription, arguments []message.ToolArgument, handler func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)) {
	m.tools[name] = &solverTool{
		name:        name,
		description: description,
		arguments:   arguments,
		handler:     handler,
	}
}

// handleSolveCSP solves a Constraint Satisfaction Problem using the centipede library
func (m *SolverToolManager) handleSolveCSP(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	variablesJSON, ok := args["variables"].(string)
	if !ok {
		return message.NewToolResultError("variables parameter is required and must be a string"), nil
	}
	constraintsJSON, ok := args["constraints"].(string)
	if !ok {
		return message.NewToolResultError("constraints parameter is required and must be a string"), nil
	}

	// Optional parameters
	useArcConsistency := true // default
	if arc, exists := args["use_arc_consistency"]; exists {
		if arcBool, ok := arc.(bool); ok {
			useArcConsistency = arcBool
		}
	}

	timeoutSeconds := 30.0 // default
	if timeout, exists := args["timeout_seconds"]; exists {
		if timeoutFloat, ok := timeout.(float64); ok {
			timeoutSeconds = timeoutFloat
		} else if timeoutInt, ok := timeout.(int); ok {
			timeoutSeconds = float64(timeoutInt)
		}
	}

	// Create context with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Solve the CSP
	solution, err := m.solveCSPWithCentipede(ctxWithTimeout, variablesJSON, constraintsJSON, useArcConsistency)
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("Failed to solve CSP: %v", err)), nil
	}

	return message.NewToolResultText(solution), nil
}

// solveCSPWithCentipede solves a CSP using the centipede library
func (m *SolverToolManager) solveCSPWithCentipede(ctx context.Context, variablesJSON, constraintsJSON string, useArcConsistency bool) (string, error) {
	// Parse variables from JSON
	var variableMap map[string][]int
	if err := json.Unmarshal([]byte(variablesJSON), &variableMap); err != nil {
		return "", fmt.Errorf("failed to parse variables JSON: %v", err)
	}

	// Debug: log the parsed variables
	fmt.Printf("DEBUG: Parsed variables: %+v\n", variableMap)

	// Parse constraints from JSON
	var constraintStrings []string
	if err := json.Unmarshal([]byte(constraintsJSON), &constraintStrings); err != nil {
		return "", fmt.Errorf("failed to parse constraints JSON: %v", err)
	}

	// Debug: log the parsed constraints
	fmt.Printf("DEBUG: Parsed constraints: %+v\n", constraintStrings)

	// Create centipede variables in a deterministic order
	var variables centipede.Variables[int]

	// First, collect all variable names and sort them for consistency
	var varNames []string
	for name := range variableMap {
		varNames = append(varNames, name)
	}
	sort.Strings(varNames)

	// Create variables in alphabetical order to ensure deterministic behavior
	for _, name := range varNames {
		domain := variableMap[name]
		variable := centipede.NewVariable(centipede.VariableName(name), centipede.Domain[int](domain))
		variables = append(variables, variable)
		fmt.Printf("DEBUG: Created variable %s with domain %v\n", name, domain)
	}

	// Debug: log all created variables
	fmt.Printf("DEBUG: All variables created: %d\n", len(variables))
	for i, v := range variables {
		fmt.Printf("DEBUG: Variable %d: name=%s, domain=%v\n", i, v.Name, v.Domain)
	}

	// Create constraints
	constraints, err := m.parseConstraints(constraintStrings)
	if err != nil {
		return "", fmt.Errorf("failed to parse constraints: %v", err)
	}

	// Create solver
	solver := centipede.NewBackTrackingCSPSolver(variables, constraints)

	// Apply arc consistency if requested (with panic recovery)
	if useArcConsistency {
		err := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("arc consistency detected unsatisfiable constraints: %v", r)
				}
			}()
			return solver.State.MakeArcConsistent(ctx)
		}()
		if err != nil {
			// Arc consistency detected unsatisfiability, return early
			result := strings.Builder{}
			result.WriteString("CSP Solver Result (using centipede library):\n\n")
			result.WriteString(fmt.Sprintf("Input Variables: %s\n", variablesJSON))
			result.WriteString(fmt.Sprintf("Input Constraints: %s\n", constraintsJSON))
			result.WriteString(fmt.Sprintf("Arc Consistency: %v\n\n", useArcConsistency))
			result.WriteString("❌ NO SOLUTION FOUND\n")
			result.WriteString("Arc consistency preprocessing detected that the problem is unsatisfiable.\n")
			return result.String(), nil
		}
	}

	// Solve (with panic recovery)
	solved, err := func() (solved bool, err error) {
		defer func() {
			if r := recover(); r != nil {
				solved = false
				err = fmt.Errorf("solver panic (likely unsatisfiable): %v", r)
			}
		}()
		solved, err = solver.Solve(ctx)
		return
	}()

	if err != nil {
		// Handle solver panics as unsatisfiable problems
		result := strings.Builder{}
		result.WriteString("CSP Solver Result (using centipede library):\n\n")
		result.WriteString(fmt.Sprintf("Input Variables: %s\n", variablesJSON))
		result.WriteString(fmt.Sprintf("Input Constraints: %s\n", constraintsJSON))
		result.WriteString(fmt.Sprintf("Arc Consistency: %v\n\n", useArcConsistency))
		result.WriteString("❌ NO SOLUTION FOUND\n")
		result.WriteString(fmt.Sprintf("Solver detected unsatisfiability: %v\n", err))
		return result.String(), nil
	}

	// Format result
	result := strings.Builder{}
	result.WriteString("CSP Solver Result (using centipede library):\n\n")
	result.WriteString(fmt.Sprintf("Input Variables: %s\n", variablesJSON))
	result.WriteString(fmt.Sprintf("Input Constraints: %s\n", constraintsJSON))
	result.WriteString(fmt.Sprintf("Arc Consistency: %v\n\n", useArcConsistency))

	if solved {
		result.WriteString("✅ SOLUTION FOUND:\n")
		for _, variable := range solver.State.Vars {
			if !variable.Empty {
				result.WriteString(fmt.Sprintf("- %s = %v\n", variable.Name, variable.Value))
			} else {
				result.WriteString(fmt.Sprintf("- %s = UNASSIGNED\n", variable.Name))
			}
		}
	} else {
		result.WriteString("❌ NO SOLUTION FOUND\n")
		result.WriteString("The constraint satisfaction problem has no valid solution.\n")
	}

	return result.String(), nil
}

// parseConstraints converts constraint strings to centipede constraints
func (m *SolverToolManager) parseConstraints(constraintStrings []string) (centipede.Constraints[int], error) {
	var constraints centipede.Constraints[int]

	for _, constraintStr := range constraintStrings {
		constraint, err := m.parseConstraint(strings.TrimSpace(constraintStr))
		if err != nil {
			return nil, fmt.Errorf("failed to parse constraint '%s': %v", constraintStr, err)
		}
		constraints = append(constraints, constraint...)
	}

	return constraints, nil
}

// parseConstraint parses a single constraint string
func (m *SolverToolManager) parseConstraint(constraintStr string) (centipede.Constraints[int], error) {
	// Handle AllUnique constraint
	if strings.HasPrefix(constraintStr, "AllUnique(") && strings.HasSuffix(constraintStr, ")") {
		// Extract variable names from AllUnique([X,Y,Z])
		inner := constraintStr[10 : len(constraintStr)-1] // Remove "AllUnique(" and ")"
		if strings.HasPrefix(inner, "[") && strings.HasSuffix(inner, "]") {
			inner = inner[1 : len(inner)-1] // Remove "[" and "]"
		}
		varNames := strings.Split(inner, ",")
		var centipedeVarNames []centipede.VariableName
		for _, name := range varNames {
			centipedeVarNames = append(centipedeVarNames, centipede.VariableName(strings.TrimSpace(name)))
		}
		return centipede.AllUnique[int](centipedeVarNames...), nil
	}

	// Handle arithmetic expressions (must come before simple binary constraints)
	operators := []string{"!=", "<=", ">=", "=", "<", ">"}
	for _, op := range operators {
		if strings.Contains(constraintStr, op) {
			parts := strings.Split(constraintStr, op)
			if len(parts) != 2 {
				continue
			}

			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])

			// Check if this is an arithmetic expression (contains operators like *, +, -)
			if m.isArithmeticExpression(left) || m.isArithmeticExpression(right) {
				return centipede.Constraints[int]{m.createArithmeticConstraint(left, op, right)}, nil
			}

			// Check if right side is a number (unary constraint)
			if value, err := strconv.Atoi(right); err == nil {
				switch op {
				case "=":
					return centipede.Constraints[int]{centipede.UnaryEquals[int](centipede.VariableName(left), value)}, nil
				case "!=":
					return centipede.Constraints[int]{centipede.UnaryNotEquals[int](centipede.VariableName(left), value)}, nil
				case "<":
					return centipede.Constraints[int]{m.createCustomUnaryLessThanConstraint(centipede.VariableName(left), value)}, nil
				case "<=":
					return centipede.Constraints[int]{m.createCustomUnaryLessThanOrEqualConstraint(centipede.VariableName(left), value)}, nil
				case ">":
					return centipede.Constraints[int]{m.createCustomUnaryGreaterThanConstraint(centipede.VariableName(left), value)}, nil
				case ">=":
					return centipede.Constraints[int]{m.createCustomUnaryGreaterThanOrEqualConstraint(centipede.VariableName(left), value)}, nil
				}
			} else {
				// Binary constraint between two variables
				switch op {
				case "=":
					return centipede.Constraints[int]{centipede.Equals[int](centipede.VariableName(left), centipede.VariableName(right))}, nil
				case "!=":
					return centipede.Constraints[int]{centipede.NotEquals[int](centipede.VariableName(left), centipede.VariableName(right))}, nil
				case "<":
					// Use custom constraint to avoid centipede.LessThan bug with arc consistency
					return centipede.Constraints[int]{m.createCustomLessThanConstraint(centipede.VariableName(left), centipede.VariableName(right))}, nil
				case "<=":
					// Use custom constraint to avoid centipede.LessThanOrEqualTo bug with arc consistency
					return centipede.Constraints[int]{m.createCustomLessThanOrEqualConstraint(centipede.VariableName(left), centipede.VariableName(right))}, nil
				case ">":
					// Use custom constraint to avoid centipede.GreaterThan bug with arc consistency
					return centipede.Constraints[int]{m.createCustomGreaterThanConstraint(centipede.VariableName(left), centipede.VariableName(right))}, nil
				case ">=":
					// Use custom constraint to avoid centipede.GreaterThanOrEqualTo bug with arc consistency
					return centipede.Constraints[int]{m.createCustomGreaterThanOrEqualConstraint(centipede.VariableName(left), centipede.VariableName(right))}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("unsupported constraint format: %s", constraintStr)
}

// Custom constraint creation methods to avoid centipede built-in bugs with arc consistency
// Based on the zebra puzzle example pattern from centipede library
// See: https://github.com/gnboorse/centipede/blob/v1.0.2/zebra_test.go

func (m *SolverToolManager) createCustomLessThanConstraint(var1, var2 centipede.VariableName) centipede.Constraint[int] {
	return centipede.Constraint[int]{
		Vars: centipede.VariableNames{var1, var2},
		ConstraintFunction: func(variables *centipede.Variables[int]) bool {
			if variables.Find(var1).Empty || variables.Find(var2).Empty {
				return true
			}
			v1 := variables.Find(var1).Value
			v2 := variables.Find(var2).Value
			return v1 < v2
		},
	}
}

func (m *SolverToolManager) createCustomLessThanOrEqualConstraint(var1, var2 centipede.VariableName) centipede.Constraint[int] {
	return centipede.Constraint[int]{
		Vars: centipede.VariableNames{var1, var2},
		ConstraintFunction: func(variables *centipede.Variables[int]) bool {
			if variables.Find(var1).Empty || variables.Find(var2).Empty {
				return true
			}
			v1 := variables.Find(var1).Value
			v2 := variables.Find(var2).Value
			return v1 <= v2
		},
	}
}

func (m *SolverToolManager) createCustomGreaterThanConstraint(var1, var2 centipede.VariableName) centipede.Constraint[int] {
	return centipede.Constraint[int]{
		Vars: centipede.VariableNames{var1, var2},
		ConstraintFunction: func(variables *centipede.Variables[int]) bool {
			if variables.Find(var1).Empty || variables.Find(var2).Empty {
				return true
			}
			v1 := variables.Find(var1).Value
			v2 := variables.Find(var2).Value
			return v1 > v2
		},
	}
}

func (m *SolverToolManager) createCustomGreaterThanOrEqualConstraint(var1, var2 centipede.VariableName) centipede.Constraint[int] {
	return centipede.Constraint[int]{
		Vars: centipede.VariableNames{var1, var2},
		ConstraintFunction: func(variables *centipede.Variables[int]) bool {
			if variables.Find(var1).Empty || variables.Find(var2).Empty {
				return true
			}
			v1 := variables.Find(var1).Value
			v2 := variables.Find(var2).Value
			return v1 >= v2
		},
	}
}

// Custom unary constraint creation methods for constraints with numeric values

func (m *SolverToolManager) createCustomUnaryLessThanConstraint(variable centipede.VariableName, value int) centipede.Constraint[int] {
	return centipede.Constraint[int]{
		Vars: centipede.VariableNames{variable},
		ConstraintFunction: func(variables *centipede.Variables[int]) bool {
			if variables.Find(variable).Empty {
				return true
			}
			return variables.Find(variable).Value < value
		},
	}
}

func (m *SolverToolManager) createCustomUnaryLessThanOrEqualConstraint(variable centipede.VariableName, value int) centipede.Constraint[int] {
	return centipede.Constraint[int]{
		Vars: centipede.VariableNames{variable},
		ConstraintFunction: func(variables *centipede.Variables[int]) bool {
			if variables.Find(variable).Empty {
				return true
			}
			return variables.Find(variable).Value <= value
		},
	}
}

func (m *SolverToolManager) createCustomUnaryGreaterThanConstraint(variable centipede.VariableName, value int) centipede.Constraint[int] {
	return centipede.Constraint[int]{
		Vars: centipede.VariableNames{variable},
		ConstraintFunction: func(variables *centipede.Variables[int]) bool {
			if variables.Find(variable).Empty {
				return true
			}
			return variables.Find(variable).Value > value
		},
	}
}

func (m *SolverToolManager) createCustomUnaryGreaterThanOrEqualConstraint(variable centipede.VariableName, value int) centipede.Constraint[int] {
	return centipede.Constraint[int]{
		Vars: centipede.VariableNames{variable},
		ConstraintFunction: func(variables *centipede.Variables[int]) bool {
			if variables.Find(variable).Empty {
				return true
			}
			return variables.Find(variable).Value >= value
		},
	}
}

// isArithmeticExpression checks if a string contains arithmetic operators
func (m *SolverToolManager) isArithmeticExpression(s string) bool {
	return strings.ContainsAny(s, "+-*/")
}

// createArithmeticConstraint creates a constraint that evaluates arithmetic expressions
func (m *SolverToolManager) createArithmeticConstraint(left, operator, right string) centipede.Constraint[int] {
	return centipede.Constraint[int]{
		Vars: m.extractVariableNames(left, right),
		ConstraintFunction: func(variables *centipede.Variables[int]) bool {
			// Skip evaluation if any variable is unassigned
			for _, varName := range m.extractVariableNames(left, right) {
				if variables.Find(varName).Empty {
					return true
				}
			}

			// Create environment for expression evaluation
			env := make(map[string]interface{})
			for _, varName := range m.extractVariableNames(left, right) {
				variable := variables.Find(varName)
				if !variable.Empty {
					env[string(varName)] = variable.Value
				}
			}

			// Evaluate left and right expressions
			leftValue, err := m.evaluateExpression(left, env)
			if err != nil {
				return false
			}

			rightValue, err := m.evaluateExpression(right, env)
			if err != nil {
				return false
			}

			// Apply the operator
			switch operator {
			case "=":
				return leftValue == rightValue
			case "!=":
				return leftValue != rightValue
			case "<":
				return leftValue < rightValue
			case "<=":
				return leftValue <= rightValue
			case ">":
				return leftValue > rightValue
			case ">=":
				return leftValue >= rightValue
			default:
				return false
			}
		},
	}
}

// extractVariableNames extracts all variable names from expressions
func (m *SolverToolManager) extractVariableNames(expressions ...string) centipede.VariableNames {
	varSet := make(map[string]bool)
	
	for _, expr := range expressions {
		// Simple regex-like approach: find words that aren't numbers
		words := strings.FieldsFunc(expr, func(c rune) bool {
			return c == '+' || c == '-' || c == '*' || c == '/' || c == '(' || c == ')' || c == ' '
		})
		
		for _, word := range words {
			word = strings.TrimSpace(word)
			if word != "" {
				// Check if it's not a number
				if _, err := strconv.Atoi(word); err != nil {
					varSet[word] = true
				}
			}
		}
	}
	
	var varNames centipede.VariableNames
	for varName := range varSet {
		varNames = append(varNames, centipede.VariableName(varName))
	}
	
	return varNames
}

// evaluateExpression evaluates an arithmetic expression using the expr library
func (m *SolverToolManager) evaluateExpression(expression string, env map[string]interface{}) (int, error) {
	// If it's just a number, return it directly
	if value, err := strconv.Atoi(strings.TrimSpace(expression)); err == nil {
		return value, nil
	}

	// Compile and run the expression
	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return 0, fmt.Errorf("failed to compile expression '%s': %v", expression, err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return 0, fmt.Errorf("failed to evaluate expression '%s': %v", expression, err)
	}

	// Convert result to int
	switch v := result.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("expression result is not a number: %v", result)
	}
}

// solverTool implements the domain.Tool interface for solver tools
type solverTool struct {
	name        message.ToolName
	description message.ToolDescription
	arguments   []message.ToolArgument
	handler     func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)
}

func (t *solverTool) RawName() message.ToolName {
	return t.name
}

func (t *solverTool) Name() message.ToolName {
	return t.name
}

func (t *solverTool) Description() message.ToolDescription {
	return t.description
}

func (t *solverTool) Handler() func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	return t.handler
}

func (t *solverTool) Arguments() []message.ToolArgument {
	return t.arguments
}
