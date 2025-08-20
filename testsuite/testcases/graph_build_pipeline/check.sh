#!/bin/bash

# Extract the result from the solver output
result_file="$1"

echo "=== Checking Graph Build Pipeline Solver Results ==="

# Check if solve_topological_sort tool was used
if ! grep -q "solve_topological_sort" "$result_file"; then
    echo "❌ FAIL: solve_topological_sort tool was not used"
    exit 1
fi

# Check if the solver found a solution (no cycles)
if grep -q "CYCLE DETECTED" "$result_file"; then
    echo "❌ FAIL: Cycle detected in build dependencies - this should not happen"
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

# Extract the execution order and verify key constraints
execution_order=$(grep -A 20 "Execution Order:" "$result_file" | grep -E "^[0-9]+\." | sed 's/^[0-9]*\. //')

echo "Found execution order:"
echo "$execution_order"

# Convert to numbered list for easier checking
order_list=($execution_order)

# Check that we have exactly 4 components
if [ ${#order_list[@]} -ne 4 ]; then
    echo "❌ FAIL: Expected 4 components, but found ${#order_list[@]}"
    exit 1
fi

# Check that Database is first
if [ "${order_list[0]}" != "Database" ]; then
    echo "❌ FAIL: Database should be built first, but found: ${order_list[0]}"
    exit 1
fi

# Check that Tests is last
last_index=$((${#order_list[@]}-1))
if [ "${order_list[$last_index]}" != "Tests" ]; then
    echo "❌ FAIL: Tests should be built last, but found: ${order_list[$last_index]}"
    exit 1
fi

# Function to find position of component in build order
find_position() {
    local component="$1"
    for i in "${!order_list[@]}"; do
        if [ "${order_list[$i]}" = "$component" ]; then
            echo $i
            return
        fi
    done
    echo -1
}

# Verify all expected components are present
expected_components=("Database" "Library" "Service" "Tests")
for component in "${expected_components[@]}"; do
    pos=$(find_position "$component")
    if [ $pos -eq -1 ]; then
        echo "❌ FAIL: Component '$component' not found in execution order"
        exit 1
    fi
done

# Verify dependency relationships
database_pos=$(find_position "Database")
library_pos=$(find_position "Library")
service_pos=$(find_position "Service")
tests_pos=$(find_position "Tests")

# Check: Database → Library
if [ $database_pos -ge $library_pos ]; then
    echo "❌ FAIL: Database should be built before Library"
    exit 1
fi

# Check: Library → Service
if [ $library_pos -ge $service_pos ]; then
    echo "❌ FAIL: Library should be built before Service"
    exit 1
fi

# Check: Service → Tests
if [ $service_pos -ge $tests_pos ]; then
    echo "❌ FAIL: Service should be built before Tests"
    exit 1
fi

echo "✅ PASS: All dependency relationships satisfied"
echo "✅ PASS: Database built first, Tests built last"
echo "✅ PASS: Graph build pipeline solver test passed"
echo "✅ PASS: Topological sort successfully resolved build dependencies"