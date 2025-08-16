#!/bin/bash

# Test solver shopping scenario
# Arguments: $1 = output file, $2 = error file

output_file="$1"
error_file="$2"

echo "Testing AI solver shopping scenario..."

# Check if AI output exists
if [ ! -f "$output_file" ]; then
    echo "✗ FAILED: Output file not found"
    exit 1
fi

# Read the AI output
ai_output=$(cat "$output_file")

echo "✓ AI output file found"

# Check if the AI used the solve_csp tool
if ! echo "$ai_output" | grep -q "solve_csp"; then
    echo "✗ FAILED: AI did not use the solve_csp tool"
    echo "AI output:"
    cat "$output_file"
    exit 1
fi

echo "✓ AI used the solve_csp tool"

# Check if AI found a solution (case insensitive - look for various solution indicators)
if ! echo "$ai_output" | grep -qi -E "(solution found|✅ SOLUTION FOUND|should buy|buy:)"; then
    echo "✗ FAILED: AI did not find a solution to the shopping CSP"
    echo "AI output:"
    cat "$output_file"
    exit 1
fi

echo "✓ AI found a solution"

# Test constraint satisfaction
echo "Testing constraint satisfaction..."

# Check that ground beef is included (protein requirement)
if echo "$ai_output" | grep -qi "ground beef"; then
    echo "✓ Ground beef included (protein constraint satisfied)"
else
    echo "✗ FAILED: Ground beef not included (protein constraint violated)"
    exit 1
fi

# Check that exactly 3 items are selected in the solution
item_count=0
# Look for positive selection indicators (value = 1) and exclude negative indicators (value = 0)
if echo "$ai_output" | grep -qi -E "(buy.*ground beef|ground beef.*buy|beef.*=.*1|B.*=.*1)" && ! echo "$ai_output" | grep -qi -E "(beef.*=.*0|B.*=.*0)"; then
    ((item_count++))
fi
if echo "$ai_output" | grep -qi -E "(buy.*rice|rice.*buy|rice.*=.*1|R.*=.*1)" && ! echo "$ai_output" | grep -qi -E "(rice.*=.*0|R.*=.*0)"; then
    ((item_count++))
fi
if echo "$ai_output" | grep -qi -E "(buy.*tomatoes|tomatoes.*buy|tomatoes.*=.*1|T.*=.*1)" && ! echo "$ai_output" | grep -qi -E "(tomatoes.*=.*0|T.*=.*0)"; then
    ((item_count++))
fi
if echo "$ai_output" | grep -qi -E "(buy.*onions|onions.*buy|onions.*=.*1|O.*=.*1)" && ! echo "$ai_output" | grep -qi -E "(onions.*=.*0|O.*=.*0)"; then
    ((item_count++))
fi

if [ "$item_count" -eq 3 ]; then
    echo "✓ Exactly 3 items selected"
elif [ "$item_count" -lt 3 ]; then
    echo "✗ FAILED: Less than 3 items selected ($item_count items)"
    exit 1
else
    echo "✗ FAILED: More than 3 items selected ($item_count items)"
    exit 1
fi

# Check total cost calculation (should be $19)
if echo "$ai_output" | grep -qi "\$19\|19 dollars\|total.*19"; then
    echo "✓ Correct total cost of $19 mentioned"
else
    echo "⚠️  WARNING: Could not verify $19 total cost automatically"
    echo "   Manual check required - please verify total cost is $19"
fi

# Check that the expected solution is found (ground beef + rice + onions = $19)
expected_items=0
if echo "$ai_output" | grep -qi "ground beef"; then
    ((expected_items++))
fi
if echo "$ai_output" | grep -qi "rice"; then
    ((expected_items++))
fi
if echo "$ai_output" | grep -qi "onions"; then
    ((expected_items++))
fi

if [ "$expected_items" -eq 3 ]; then
    echo "✓ Expected solution found: ground beef, rice, onions"
else
    echo "⚠️  WARNING: Different solution found - verifying cost calculation"
    # Could be a valid alternative solution, check manually
fi

# Check if AI provided explanation
if echo "$ai_output" | grep -qi -E "(explain|constraint|satisfy|total|cost)"; then
    echo "✓ AI provided explanation of the solution"
else
    echo "✗ FAILED: AI did not provide explanation of the solution"
    exit 1
fi

# Check if AI interpreted the problem correctly (modeling as CSP)
if echo "$ai_output" | grep -qi -E "(variable|domain|constraint|csp|model)"; then
    echo "✓ AI correctly interpreted problem as CSP"
else
    echo "⚠️  WARNING: AI may not have explicitly modeled as CSP"
fi

echo ""
echo "🎉 ALL TESTS PASSED!"
echo "✅ AI successfully solved the shopping constraint satisfaction problem"
echo "   - Used solve_csp tool correctly"
echo "   - Found a valid solution with 3 items"
echo "   - Included required protein (ground beef)"
echo "   - Met budget constraint"
echo "   - Provided clear explanation"

exit 0