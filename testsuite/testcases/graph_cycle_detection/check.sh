#!/bin/bash

# Extract the result from the solver output
result_file="$1"

echo "=== Checking Graph Cycle Detection Solver Results ==="

# Check if solve_topological_sort tool was used
if ! grep -q "solve_topological_sort" "$result_file"; then
    echo "❌ FAIL: solve_topological_sort tool was not used"
    exit 1
fi

# Check if cycle was detected (this is what we expect for this test case)
if ! grep -q "CYCLE DETECTED" "$result_file"; then
    echo "❌ FAIL: Expected cycle detection, but solver did not detect cycles"
    exit 1
fi

# Check if the solver properly rejected the topological sort
if grep -q "TOPOLOGICAL ORDER FOUND" "$result_file"; then
    echo "❌ FAIL: Solver should not find a topological order when cycles exist"
    exit 1
fi

# Check if detected cycles are reported
if ! grep -q "Detected cycles:" "$result_file"; then
    echo "❌ FAIL: Detected cycles should be listed in the output"
    exit 1
fi

# Verify that the cycle involves the expected tasks
cycle_output=$(grep -A 10 "Detected cycles:" "$result_file")

# Check if key tasks appear in the cycle description
expected_tasks=("Design" "Development" "Testing" "Review")
found_tasks=0

for task in "${expected_tasks[@]}"; do
    if echo "$cycle_output" | grep -q "$task"; then
        ((found_tasks++))
    fi
done

if [ $found_tasks -lt 3 ]; then
    echo "❌ FAIL: Expected to find at least 3 of the 4 tasks in the cycle description, but found only $found_tasks"
    exit 1
fi

# Check that the output explains the problem
if ! grep -q "Cannot perform topological sort" "$result_file"; then
    echo "❌ FAIL: Output should explain that topological sort cannot be performed with cycles"
    exit 1
fi

echo "✅ PASS: Cycle detected correctly"
echo "✅ PASS: Topological sort properly rejected due to cycles"
echo "✅ PASS: Cycle details provided in output"
echo "✅ PASS: Graph cycle detection solver test passed"
echo "✅ PASS: Solver successfully identified circular dependencies in project plan"