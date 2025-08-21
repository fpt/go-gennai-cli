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

# Extract only the response content (after ✅ Response:)
response_content=$(./extract_response.sh "$result_file")
echo "$response_content" > ./response_only.txt

# Find the sequence of tasks as they first appear in the response only
declare -a found_tasks
declare -a line_numbers

# Get all lines with task names and their line numbers from response only
while IFS= read -r line; do
    line_num=$(echo "$line" | cut -d: -f1)
    line_content=$(echo "$line" | cut -d: -f2-)
    
    if [[ "$line_content" =~ Plan ]] && [[ ! " ${found_tasks[@]} " =~ " Plan " ]]; then
        found_tasks+=("Plan")
        line_numbers+=("$line_num")
    elif [[ "$line_content" =~ Design ]] && [[ ! " ${found_tasks[@]} " =~ " Design " ]]; then
        found_tasks+=("Design")  
        line_numbers+=("$line_num")
    elif [[ "$line_content" =~ Build ]] && [[ ! " ${found_tasks[@]} " =~ " Build " ]]; then
        found_tasks+=("Build")
        line_numbers+=("$line_num")
    elif [[ "$line_content" =~ Test ]] && [[ ! " ${found_tasks[@]} " =~ " Test " ]]; then
        found_tasks+=("Test")
        line_numbers+=("$line_num")
    fi
done <<< "$(grep -n -E "(Plan|Design|Build|Test)" ./response_only.txt)"

execution_order=$(IFS=' '; echo "${found_tasks[*]}")

echo "Found tasks in order: ${found_tasks[@]}"
echo "At line numbers: ${line_numbers[@]}"

# Check if tasks appear in adjacent lines (allowing for some spacing)
if [ ${#line_numbers[@]} -eq 4 ]; then
    for i in {0..2}; do
        current_line=${line_numbers[$i]}
        next_line=${line_numbers[$((i+1))]}
        line_diff=$((next_line - current_line))
        if [ $line_diff -gt 3 ]; then
            echo "⚠️  WARNING: Tasks may not be in adjacent lines (gap of $line_diff lines between ${found_tasks[$i]} and ${found_tasks[$((i+1))]})"
        fi
    done
fi

echo "Found execution order:"
echo "$execution_order"

# Check that we have exactly 4 tasks
if [ ${#found_tasks[@]} -ne 4 ]; then
    echo "❌ FAIL: Expected 4 tasks, but found ${#found_tasks[@]}"
    echo "Found tasks: ${found_tasks[@]}"
    exit 1
fi

# Check that Plan is first
if [ "${found_tasks[0]}" != "Plan" ]; then
    echo "❌ FAIL: Plan should be first task, but found: ${found_tasks[0]}"
    exit 1
fi

# Check that Test is last
last_index=$((${#found_tasks[@]}-1))
if [ "${found_tasks[$last_index]}" != "Test" ]; then
    echo "❌ FAIL: Test should be last task, but found: ${found_tasks[$last_index]}"
    exit 1
fi

# Verify all expected tasks are present in correct order
expected_order=("Plan" "Design" "Build" "Test")
for i in "${!expected_order[@]}"; do
    if [ "${found_tasks[$i]}" != "${expected_order[$i]}" ]; then
        echo "❌ FAIL: Expected task ${expected_order[$i]} at position $((i+1)), but found: ${found_tasks[$i]}"
        exit 1
    fi
done

echo "✅ PASS: All project dependencies satisfied"
echo "✅ PASS: Plan is first task, Test is last task"
echo "✅ PASS: Graph project schedule solver test passed"
echo "✅ PASS: Topological sort successfully created valid project timeline"