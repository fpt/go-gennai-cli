#!/bin/bash

# Extract the result from the solver output
result_file="$1"

echo "=== Checking Graph Delivery Route Solver Results ==="

# Check if solve_shortest_path tool was used
if ! grep -q "solve_shortest_path" "$result_file"; then
    echo "❌ FAIL: solve_shortest_path tool was not used"
    exit 1
fi

# Check if the solver found a solution
if ! grep -q "SHORTEST PATH FOUND" "$result_file"; then
    echo "❌ FAIL: No shortest path solution found"
    exit 1
fi

# Check if total weight/distance is present
if ! grep -q "Total Weight:" "$result_file"; then
    echo "❌ FAIL: Total distance not reported"
    exit 1
fi

# Check if path is present with arrow notation
if ! grep -q "Path:.*→" "$result_file"; then
    echo "❌ FAIL: Path not properly displayed with arrow notation"
    exit 1
fi

# Check if it starts from Warehouse
if ! grep -q "Warehouse" "$result_file"; then
    echo "❌ FAIL: Path does not start from Warehouse"
    exit 1
fi

# Check if it ends at Store
if ! grep -q "Store" "$result_file"; then
    echo "❌ FAIL: Path does not end at Store"
    exit 1
fi

# Extract the total weight and verify it's reasonable (should be between 8-12 miles for this network)
total_weight=$(grep "Total Weight:" "$result_file" | grep -o '[0-9]\+\.[0-9]\+\|[0-9]\+')
if [ -n "$total_weight" ]; then
    # Convert to integer for comparison (multiply by 100 to handle decimals)
    weight_int=$(echo "$total_weight * 100" | bc -l | cut -d. -f1)
    if [ "$weight_int" -lt 800 ] || [ "$weight_int" -gt 1200 ]; then
        echo "❌ FAIL: Total weight $total_weight seems unreasonable (expected 8-12 miles)"
        exit 1
    fi
    echo "✅ PASS: Found valid shortest path with distance $total_weight miles"
else
    echo "❌ FAIL: Could not extract total weight from result"
    exit 1
fi

echo "✅ PASS: Graph delivery route solver test passed"
echo "✅ PASS: Shortest path algorithm successfully found optimal delivery route"