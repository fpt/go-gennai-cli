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

# Extract only the response content (after ✅ Response:)
response_content=$(./extract_response.sh "$result_file")
echo "$response_content" > ./response_only.txt

# Find the sequence of components as they first appear in the response only
declare -a found_components
declare -a line_numbers

# Get all lines with component names and their line numbers from response only
while IFS= read -r line; do
    line_num=$(echo "$line" | cut -d: -f1)
    line_content=$(echo "$line" | cut -d: -f2-)
    
    if [[ "$line_content" =~ Database ]] && [[ ! " ${found_components[@]} " =~ " Database " ]]; then
        found_components+=("Database")
        line_numbers+=("$line_num")
    elif [[ "$line_content" =~ Library ]] && [[ ! " ${found_components[@]} " =~ " Library " ]]; then
        found_components+=("Library")  
        line_numbers+=("$line_num")
    elif [[ "$line_content" =~ Service ]] && [[ ! " ${found_components[@]} " =~ " Service " ]]; then
        found_components+=("Service")
        line_numbers+=("$line_num")
    elif [[ "$line_content" =~ Tests ]] && [[ ! " ${found_components[@]} " =~ " Tests " ]]; then
        found_components+=("Tests")
        line_numbers+=("$line_num")
    fi
done <<< "$(grep -n -E "(Database|Library|Service|Tests)" ./response_only.txt)"

execution_order=$(IFS=' '; echo "${found_components[*]}")

echo "Found components in order: ${found_components[@]}"
echo "At line numbers: ${line_numbers[@]}"

# Check if components appear in adjacent lines (allowing for some spacing)
if [ ${#line_numbers[@]} -eq 4 ]; then
    for i in {0..2}; do
        current_line=${line_numbers[$i]}
        next_line=${line_numbers[$((i+1))]}
        line_diff=$((next_line - current_line))
        if [ $line_diff -gt 3 ]; then
            echo "⚠️  WARNING: Components may not be in adjacent lines (gap of $line_diff lines between ${found_components[$i]} and ${found_components[$((i+1))]})"
        fi
    done
fi

echo "Found execution order:"
echo "$execution_order"

# Check that we have exactly 4 components
if [ ${#found_components[@]} -ne 4 ]; then
    echo "❌ FAIL: Expected 4 components, but found ${#found_components[@]}"
    echo "Found components: ${found_components[@]}"
    exit 1
fi

# Check that Database is first
if [ "${found_components[0]}" != "Database" ]; then
    echo "❌ FAIL: Database should be built first, but found: ${found_components[0]}"
    exit 1
fi

# Check that Tests is last
last_index=$((${#found_components[@]}-1))
if [ "${found_components[$last_index]}" != "Tests" ]; then
    echo "❌ FAIL: Tests should be built last, but found: ${found_components[$last_index]}"
    exit 1
fi

# Verify all expected components are present in correct order
expected_order=("Database" "Library" "Service" "Tests")
for i in "${!expected_order[@]}"; do
    if [ "${found_components[$i]}" != "${expected_order[$i]}" ]; then
        echo "❌ FAIL: Expected component ${expected_order[$i]} at position $((i+1)), but found: ${found_components[$i]}"
        exit 1
    fi
done

echo "✅ PASS: All dependency relationships satisfied"
echo "✅ PASS: Database built first, Tests built last"
echo "✅ PASS: Graph build pipeline solver test passed"
echo "✅ PASS: Topological sort successfully resolved build dependencies"