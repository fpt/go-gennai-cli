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
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
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
	// CSP Solver
	m.RegisterTool("solve_csp", `Solve a Constraint Satisfaction Problem (CSP) using the centipede library with backtracking and arc consistency.

COMPREHENSIVE CSP GUIDE:

**What is CSP?**
Constraint Satisfaction Problems involve finding values for variables that satisfy all given constraints. Perfect for scheduling, assignment, logic puzzles, and resource allocation problems.

**When to Use:**
- Staff scheduling with availability constraints
- Resource allocation with capacity limits
- Logic puzzles (N-Queens, Sudoku, graph coloring)
- Assignment problems with restrictions
- Configuration problems with rules

**CSP Modeling Process:**
1. IDENTIFY VARIABLES: What decisions need to be made?
   Example: X = person assigned to time slot, Y = room assigned to meeting
2. DEFINE DOMAINS: What values can each variable take?
   Example: X ∈ [1,2,3,4] where 1=9am, 2=10am, 3=11am, 4=12pm
3. LIST CONSTRAINTS: What restrictions must be satisfied?
   Example: X ≠ Y (different assignments), X < Y (ordering), AllUnique([X,Y,Z])
4. SOLVE: Use this tool with proper JSON formatting
5. INTERPRET: Map mathematical solution back to real-world meaning

**Supported Constraint Types:**
- EQUALITY: 'X = Y', 'X = 5' (variables equal, or equal to constant)
- INEQUALITY: 'X != Y', 'X != 3' (variables different)
- ORDERING: 'X < Y', 'X <= Y', 'X > Y', 'X >= Y' (relational constraints)
- UNIQUENESS: 'AllUnique([X,Y,Z])' (all variables different values)
- ARITHMETIC: '2*X + 3*Y = 10', 'X + Y + Z >= 15' (mathematical expressions)

**Variable Domains:**
- Use integer arrays: [1,2,3,4,5] for discrete values
- Map real-world concepts to numbers: 1=Monday, 2=Tuesday, etc.
- Keep domains small for better performance
- Consider using 0-based or 1-based indexing consistently

**Best Practices:**
- Use descriptive variable names that relate to problem context
- Start with small domains and expand if needed
- Use arc consistency (default) for better performance
- Set appropriate timeout based on problem complexity
- If no solution exists, consider relaxing some constraints

**Example Problems:**
1. SCHEDULING: Assign 4 staff to 4 shifts with availability constraints
   Variables: {"Alice":[1,2,3], "Bob":[2,3,4], "Carol":[1,3,4], "David":[1,2,4]}
   Constraints: ["AllUnique([Alice,Bob,Carol,David])", "Alice != 1"]

2. RESOURCE ALLOCATION: Distribute tasks among team members
   Variables: {"Task1":[1,2,3], "Task2":[1,2,3], "Task3":[1,2,3]}
   Constraints: ["AllUnique([Task1,Task2,Task3])", "Task1 < Task2"]

3. LOGIC PUZZLE: N-Queens problem for 4x4 board
   Variables: {"Q1":[1,2,3,4], "Q2":[1,2,3,4], "Q3":[1,2,3,4], "Q4":[1,2,3,4]}
   Constraints: ["AllUnique([Q1,Q2,Q3,Q4])", "Q1-Q2 != 1", "Q1-Q2 != -1", ...]`,
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

	// Graph Shortest Path Solver
	m.RegisterTool("solve_shortest_path", `Find the shortest path between nodes in a weighted directed graph using Dijkstra's algorithm.

COMPREHENSIVE SHORTEST PATH GUIDE:

**What is Shortest Path?**
Graph algorithms that find the optimal route between two points in a network, minimizing total distance, time, or cost. Uses Dijkstra's algorithm for guaranteed optimal solutions in weighted directed graphs.

**When to Use:**
- Delivery route optimization and logistics planning
- Network routing and traffic optimization  
- Navigation systems and GPS routing
- Supply chain and distribution planning
- Cost minimization in transportation networks
- Finding optimal paths in game worlds or simulations

**Graph Modeling Process:**
1. IDENTIFY NODES: What are the locations/points in your network?
   Example: Warehouses, customers, distribution centers, cities
2. DEFINE EDGES: How are nodes connected and what are the costs?
   Example: Roads with distances, routes with travel times, connections with costs
3. SET START/END: What are your source and destination points?
   Example: Start from warehouse, end at customer location
4. SOLVE: Use this tool with properly formatted graph data
5. INTERPRET: Convert optimal path back to real-world routing instructions

**Graph Structure Requirements:**
- NODES: Must be unique identifiers (strings recommended)
- EDGES: Directed connections with positive weights (distances/costs)
- CONNECTIVITY: Ensure path exists from start to end node
- WEIGHTS: Use consistent units (miles, minutes, cost, etc.)

**Edge Weight Considerations:**
- DISTANCE: Physical distance between locations (miles, km)
- TIME: Travel time between points (minutes, hours)  
- COST: Monetary cost of traversing the connection
- COMPOSITE: Combined metrics (time + fuel cost + tolls)
- PENALTIES: Higher weights for undesirable routes

**Algorithm Properties:**
- OPTIMAL: Dijkstra's algorithm guarantees shortest path
- DIRECTED: Supports one-way connections (A→B ≠ B→A)
- WEIGHTED: Handles different costs for different edges
- EFFICIENT: Scales well for moderate-sized networks

**Common Applications:**
1. DELIVERY OPTIMIZATION: Route packages efficiently
   Nodes: ["Warehouse", "CustomerA", "CustomerB", "CustomerC"]
   Edges: [{"from":"Warehouse","to":"CustomerA","weight":5.2}]

2. NETWORK ROUTING: Find best path through infrastructure
   Nodes: ["RouterA", "RouterB", "RouterC", "Destination"]
   Edges: [{"from":"RouterA","to":"RouterB","weight":10.5}]

3. GAME PATHFINDING: Navigate characters through game world
   Nodes: ["StartRoom", "Corridor", "TreasureRoom", "Exit"]
   Edges: [{"from":"StartRoom","to":"Corridor","weight":1.0}]

**Best Practices:**
- Use descriptive node names that reflect real locations
- Ensure all weights are positive (Dijkstra requirement)
- Verify graph connectivity before solving
- Consider bidirectional edges if movement works both ways
- Use consistent weight units throughout the graph
- Test with small examples before scaling up

**Troubleshooting:**
- NO PATH FOUND: Check if nodes are connected, verify node names match exactly
- UNEXPECTED RESULTS: Verify edge directions and weights are correct
- PERFORMANCE ISSUES: Consider reducing graph size or breaking into subproblems`,
		[]message.ToolArgument{
			{
				Name:        "nodes",
				Description: "JSON array of node IDs. Example: '[\"A\", \"B\", \"C\", \"D\"]'",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "edges",
				Description: "JSON array of weighted edges. Each edge: {\"from\": \"A\", \"to\": \"B\", \"weight\": 5.0}. Example: '[{\"from\":\"A\",\"to\":\"B\",\"weight\":3}, {\"from\":\"B\",\"to\":\"C\",\"weight\":2}]'",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "start_node",
				Description: "Starting node ID for path finding",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "end_node",
				Description: "Target node ID for path finding",
				Required:    true,
				Type:        "string",
			},
		},
		m.handleShortestPath)

	// Topological Sort Solver
	m.RegisterTool("solve_topological_sort", `Perform topological sorting on a directed acyclic graph (DAG) for dependency resolution.

COMPREHENSIVE TOPOLOGICAL SORT GUIDE:

**What is Topological Sort?**
An algorithm that orders nodes in a directed acyclic graph (DAG) such that for every directed edge (A→B), node A comes before node B in the ordering. Essential for dependency resolution, scheduling, and build systems.

**When to Use:**
- Build system dependency resolution (compile order)
- Project task scheduling with prerequisites
- Course prerequisite planning in education
- Software deployment pipelines
- Workflow and process ordering
- Detecting circular dependencies in systems

**Dependency Modeling Process:**
1. IDENTIFY TASKS/NODES: What are the items that need ordering?
   Example: Build tasks, project milestones, course modules
2. DEFINE DEPENDENCIES: What must happen before what?
   Example: "Database setup" must happen before "API development"
3. MODEL AS DAG: Create directed edges showing prerequisite relationships
   Example: Database → API → Frontend → Testing
4. SOLVE: Use this tool to get valid execution order
5. INTERPRET: Convert mathematical ordering to practical schedule

**Dependency Relationship Types:**
- PREREQUISITE: A must complete before B can start
- SEQUENTIAL: Natural ordering where A enables B
- RESOURCE: A must finish before B can access same resource
- LOGICAL: A provides information/output needed by B
- TEMPORAL: A must happen before B due to time constraints

**Graph Structure Requirements:**
- NODES: Unique identifiers for tasks/items to be ordered
- EDGES: Directed connections showing "A must come before B"
- ACYCLIC: No circular dependencies (A→B→C→A not allowed)
- DEPENDENCIES: Each edge represents a dependency relationship

**Algorithm Properties:**
- ORDERING: Provides valid linear ordering of all nodes
- CYCLE DETECTION: Automatically detects circular dependencies
- MULTIPLE SOLUTIONS: May have several valid orderings
- PARALLEL IDENTIFICATION: Shows which tasks can run concurrently

**Common Applications:**
1. BUILD SYSTEM: Order compilation tasks
   Nodes: ["Database", "SharedLibrary", "AuthService", "WebAPI"]
   Dependencies: [{"from":"Database","to":"SharedLibrary"}]

2. PROJECT MANAGEMENT: Schedule tasks with dependencies
   Nodes: ["Requirements", "Design", "Development", "Testing"]
   Dependencies: [{"from":"Requirements","to":"Design"}]

3. COURSE PLANNING: Order classes with prerequisites
   Nodes: ["Math101", "Math201", "Statistics", "DataScience"]
   Dependencies: [{"from":"Math101","to":"Math201"}]

**Understanding Output:**
- EXECUTION ORDER: Sequential list showing valid ordering
- PARALLEL OPPORTUNITIES: Tasks at same "level" can run concurrently
- CRITICAL PATH: Longest sequence of dependent tasks
- FLEXIBILITY: Some tasks may be reorderable within constraints

**Cycle Detection:**
When circular dependencies exist:
- IDENTIFICATION: Tool reports which nodes form cycles
- PROBLEM DIAGNOSIS: Shows the circular dependency chain
- RESOLUTION HINTS: Suggests which dependencies might be removed
- NO ORDERING: Cannot provide valid sequence until cycles resolved

**Best Practices:**
- Use descriptive node names reflecting actual tasks/items
- Verify all dependencies are truly necessary
- Consider granularity - not too fine, not too coarse
- Test with simple examples before complex scenarios
- Document assumptions about dependency relationships
- Plan for parallel execution opportunities

**Troubleshooting:**
- CYCLES DETECTED: Review dependency chain, remove unnecessary dependencies
- UNEXPECTED ORDER: Verify dependencies are correctly specified
- MISSING DEPENDENCIES: Add missing prerequisite relationships
- TOO RESTRICTIVE: Consider if some dependencies can be relaxed`,
		[]message.ToolArgument{
			{
				Name:        "nodes",
				Description: "JSON array of node IDs. Example: '[\"Task1\", \"Task2\", \"Task3\"]'",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "dependencies",
				Description: "JSON array of dependency edges. Each edge: {\"from\": \"Task1\", \"to\": \"Task2\"} means Task1 must complete before Task2. Example: '[{\"from\":\"Task1\",\"to\":\"Task2\"}, {\"from\":\"Task2\",\"to\":\"Task3\"}]'",
				Required:    true,
				Type:        "string",
			},
		},
		m.handleTopologicalSort)
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

// handleShortestPath finds the shortest path in a weighted directed graph
func (m *SolverToolManager) handleShortestPath(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	nodesJSON, ok := args["nodes"].(string)
	if !ok {
		return message.NewToolResultError("nodes parameter is required and must be a string"), nil
	}
	edgesJSON, ok := args["edges"].(string)
	if !ok {
		return message.NewToolResultError("edges parameter is required and must be a string"), nil
	}
	startNode, ok := args["start_node"].(string)
	if !ok {
		return message.NewToolResultError("start_node parameter is required and must be a string"), nil
	}
	endNode, ok := args["end_node"].(string)
	if !ok {
		return message.NewToolResultError("end_node parameter is required and must be a string"), nil
	}

	// Solve the shortest path problem
	solution, err := m.solveShortestPath(ctx, nodesJSON, edgesJSON, startNode, endNode)
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("Failed to solve shortest path: %v", err)), nil
	}

	return message.NewToolResultText(solution), nil
}

// handleTopologicalSort performs topological sorting on a DAG
func (m *SolverToolManager) handleTopologicalSort(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	nodesJSON, ok := args["nodes"].(string)
	if !ok {
		return message.NewToolResultError("nodes parameter is required and must be a string"), nil
	}
	dependenciesJSON, ok := args["dependencies"].(string)
	if !ok {
		return message.NewToolResultError("dependencies parameter is required and must be a string"), nil
	}

	// Solve the topological sort problem
	solution, err := m.solveTopologicalSort(ctx, nodesJSON, dependenciesJSON)
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("Failed to solve topological sort: %v", err)), nil
	}

	return message.NewToolResultText(solution), nil
}

// solveShortestPath uses gonum to find shortest path between nodes
func (m *SolverToolManager) solveShortestPath(ctx context.Context, nodesJSON, edgesJSON, startNode, endNode string) (string, error) {
	// Parse nodes
	var nodeIDs []string
	if err := json.Unmarshal([]byte(nodesJSON), &nodeIDs); err != nil {
		return "", fmt.Errorf("failed to parse nodes JSON: %v", err)
	}

	// Parse edges
	type Edge struct {
		From   string  `json:"from"`
		To     string  `json:"to"`
		Weight float64 `json:"weight"`
	}
	var edges []Edge
	if err := json.Unmarshal([]byte(edgesJSON), &edges); err != nil {
		return "", fmt.Errorf("failed to parse edges JSON: %v", err)
	}

	// Create weighted directed graph
	g := simple.NewWeightedDirectedGraph(0, 0)

	// Create node mapping
	nodeMap := make(map[string]int64)
	for i, nodeID := range nodeIDs {
		nodeMap[nodeID] = int64(i)
		g.AddNode(simple.Node(int64(i)))
	}

	// Add edges
	for _, edge := range edges {
		fromID, exists := nodeMap[edge.From]
		if !exists {
			return "", fmt.Errorf("node '%s' not found in nodes list", edge.From)
		}
		toID, exists := nodeMap[edge.To]
		if !exists {
			return "", fmt.Errorf("node '%s' not found in nodes list", edge.To)
		}
		g.SetWeightedEdge(g.NewWeightedEdge(simple.Node(fromID), simple.Node(toID), edge.Weight))
	}

	// Find node IDs for start and end
	startID, exists := nodeMap[startNode]
	if !exists {
		return "", fmt.Errorf("start node '%s' not found in nodes list", startNode)
	}
	endID, exists := nodeMap[endNode]
	if !exists {
		return "", fmt.Errorf("end node '%s' not found in nodes list", endNode)
	}

	// Use Dijkstra's algorithm
	shortest := path.DijkstraFrom(simple.Node(startID), g)
	pathNodes, weight := shortest.To(endID)

	// Format result
	result := strings.Builder{}
	result.WriteString("Shortest Path Solver Result (using gonum/Dijkstra):\n\n")
	result.WriteString(fmt.Sprintf("Graph Nodes: %s\n", nodesJSON))
	result.WriteString(fmt.Sprintf("Graph Edges: %s\n", edgesJSON))
	result.WriteString(fmt.Sprintf("Start Node: %s\n", startNode))
	result.WriteString(fmt.Sprintf("End Node: %s\n\n", endNode))

	if len(pathNodes) == 0 {
		result.WriteString("❌ NO PATH FOUND\n")
		result.WriteString(fmt.Sprintf("No path exists from '%s' to '%s'\n", startNode, endNode))
	} else {
		result.WriteString("✅ SHORTEST PATH FOUND:\n")
		result.WriteString(fmt.Sprintf("Total Weight: %.2f\n", weight))
		result.WriteString("Path: ")

		// Convert node IDs back to original names
		pathNames := make([]string, len(pathNodes))
		for i, node := range pathNodes {
			nodeID := node.ID()
			for name, id := range nodeMap {
				if id == nodeID {
					pathNames[i] = name
					break
				}
			}
		}
		result.WriteString(strings.Join(pathNames, " → "))
		result.WriteString("\n")
	}

	return result.String(), nil
}

// solveTopologicalSort uses gonum to perform topological sorting
func (m *SolverToolManager) solveTopologicalSort(ctx context.Context, nodesJSON, dependenciesJSON string) (string, error) {
	// Parse nodes
	var nodeIDs []string
	if err := json.Unmarshal([]byte(nodesJSON), &nodeIDs); err != nil {
		return "", fmt.Errorf("failed to parse nodes JSON: %v", err)
	}

	// Parse dependencies
	type Dependency struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	var dependencies []Dependency
	if err := json.Unmarshal([]byte(dependenciesJSON), &dependencies); err != nil {
		return "", fmt.Errorf("failed to parse dependencies JSON: %v", err)
	}

	// Create directed graph
	g := simple.NewDirectedGraph()

	// Create node mapping
	nodeMap := make(map[string]int64)
	for i, nodeID := range nodeIDs {
		nodeMap[nodeID] = int64(i)
		g.AddNode(simple.Node(int64(i)))
	}

	// Add dependency edges
	for _, dep := range dependencies {
		fromID, exists := nodeMap[dep.From]
		if !exists {
			return "", fmt.Errorf("node '%s' not found in nodes list", dep.From)
		}
		toID, exists := nodeMap[dep.To]
		if !exists {
			return "", fmt.Errorf("node '%s' not found in nodes list", dep.To)
		}
		g.SetEdge(g.NewEdge(simple.Node(fromID), simple.Node(toID)))
	}

	// Check for cycles
	cycles := topo.DirectedCyclesIn(g)
	if len(cycles) > 0 {
		result := strings.Builder{}
		result.WriteString("Topological Sort Solver Result (using gonum):\n\n")
		result.WriteString(fmt.Sprintf("Graph Nodes: %s\n", nodesJSON))
		result.WriteString(fmt.Sprintf("Dependencies: %s\n\n", dependenciesJSON))
		result.WriteString("❌ CYCLE DETECTED\n")
		result.WriteString("Cannot perform topological sort on a graph with cycles.\n")
		result.WriteString("Detected cycles:\n")

		for i, cycle := range cycles {
			if i >= 3 { // Limit output
				result.WriteString("... (more cycles detected)\n")
				break
			}
			cycleNames := make([]string, len(cycle))
			for j, node := range cycle {
				nodeID := node.ID()
				for name, id := range nodeMap {
					if id == nodeID {
						cycleNames[j] = name
						break
					}
				}
			}
			result.WriteString(fmt.Sprintf("- %s\n", strings.Join(cycleNames, " → ")))
		}
		return result.String(), nil
	}

	// Perform topological sort
	sortedNodes, err := topo.Sort(g)
	if err != nil {
		return "", fmt.Errorf("topological sort failed: %v", err)
	}

	// Format result
	result := strings.Builder{}
	result.WriteString("Topological Sort Solver Result (using gonum):\n\n")
	result.WriteString(fmt.Sprintf("Graph Nodes: %s\n", nodesJSON))
	result.WriteString(fmt.Sprintf("Dependencies: %s\n\n", dependenciesJSON))
	result.WriteString("✅ TOPOLOGICAL ORDER FOUND:\n")

	// Convert node IDs back to original names
	sortedNames := make([]string, len(sortedNodes))
	for i, node := range sortedNodes {
		nodeID := node.ID()
		for name, id := range nodeMap {
			if id == nodeID {
				sortedNames[i] = name
				break
			}
		}
	}

	result.WriteString("Execution Order:\n")
	for i, name := range sortedNames {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, name))
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
