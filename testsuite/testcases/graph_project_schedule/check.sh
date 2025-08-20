#!/bin/bash

# Extract the result from the solver output
result_file="$1"

echo "=== Checking Graph Project Schedule Solver Results ==="

# Check if solve_topological_sort tool was used
if ! grep -q "solve_topological_sort" "$result_file"; then
    echo "❌ FAIL: solve_topological_sort tool was not used"
    exit 1
fi

# Check if the solver found a solution (no cycles)
if grep -q "CYCLE DETECTED" "$result_file"; then
    echo "❌ FAIL: Cycle detected in project dependencies - this indicates a planning error"
    exit 1
fi

# Check if topological order was found
if ! grep -q "TOPOLOGICAL ORDER FOUND" "$result_file"; then
    echo "❌ FAIL: No topological order solution found"
    exit 1
fi

# Check if execution order is present
if ! grep -q "Execution Order:" "$result_file"; then
    echo "❌ FAIL: Execution order not displayed"
    exit 1
fi

# Extract the execution order
execution_order=$(grep -A 15 "Execution Order:" "$result_file" | grep -E "^[0-9]+\." | sed 's/^[0-9]*\. //')

echo "Found execution order:"
echo "$execution_order"

# Convert to array for easier checking
order_list=($execution_order)

# Verify we have exactly 4 tasks
expected_tasks=("Plan" "Design" "Build" "Test")
if [ ${#order_list[@]} -ne 4 ]; then
    echo "❌ FAIL: Expected 4 tasks, but found ${#order_list[@]}"
    exit 1
fi

# Function to find position of task in execution order
find_position() {
    local task="$1"
    for i in "${!order_list[@]}"; do
        if [ "${order_list[$i]}" = "$task" ]; then
            echo $i
            return
        fi
    done
    echo -1
}

# Check that Plan is first
if [ "${order_list[0]}" != "Plan" ]; then
    echo "❌ FAIL: Plan should be first task, but found: ${order_list[0]}"
    exit 1
fi

# Check that Test is last
last_index=$((${#order_list[@]}-1))
if [ "${order_list[$last_index]}" != "Test" ]; then
    echo "❌ FAIL: Test should be last task, but found: ${order_list[$last_index]}"
    exit 1
fi

# Verify all expected tasks are present
for task in "${expected_tasks[@]}"; do
    pos=$(find_position "$task")
    if [ $pos -eq -1 ]; then
        echo "❌ FAIL: Task '$task' not found in execution order"
        exit 1
    fi
done

# Get positions of all tasks
plan_pos=$(find_position "Plan")
design_pos=$(find_position "Design")
build_pos=$(find_position "Build")
test_pos=$(find_position "Test")

# Verify dependency relationships
# Plan → Design
if [ $plan_pos -ge $design_pos ]; then
    echo "❌ FAIL: Plan should come before Design"
    exit 1
fi

# Design → Build
if [ $design_pos -ge $build_pos ]; then
    echo "❌ FAIL: Design should come before Build"
    exit 1
fi

# Build → Test
if [ $build_pos -ge $test_pos ]; then
    echo "❌ FAIL: Build should come before Test"
    exit 1
fi

echo "✅ PASS: All project dependencies satisfied"
echo "✅ PASS: Plan is first task, Test is last task"
echo "✅ PASS: Graph project schedule solver test passed"
echo "✅ PASS: Topological sort successfully created valid project timeline"