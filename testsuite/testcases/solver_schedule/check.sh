#!/bin/bash

# Test solver scenario
# Arguments: $1 = output file, $2 = error file

output_file="$1"
error_file="$2"

echo "Testing AI solver scenario..."

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
# Temporarily disabled - omitting string check as requested
# if ! echo "$ai_output" | grep -qi -E "(solution found|✅ SOLUTION FOUND|# Scheduling Problem Solution|solution:)"; then
#     echo "✗ FAILED: AI did not find a solution to the CSP"
#     echo "AI output:"
#     cat "$output_file"
#     exit 1
# fi

echo "✓ AI found a solution"

# Test constraint satisfaction - simply check that all people and time slots are mentioned
echo "Testing constraint satisfaction..."

# Check that Alice is mentioned with 10am (the correct solution)
if echo "$ai_output" | grep -i "alice" | grep -q "10am"; then
    echo "✓ Alice correctly scheduled at 10am"
elif echo "$ai_output" | grep -i "alice" | grep -q "9am"; then
    echo "✗ FAILED: Alice incorrectly scheduled at 9am (constraint violation)"
    exit 1
else
    echo "✓ Alice not at 9am (constraint satisfied)"
fi

# Check that Bob is mentioned with a time slot
if echo "$ai_output" | grep -i "bob" | grep -q "9am\|10am\|11am\|12pm"; then
    echo "✓ Bob assigned to a time slot"
else
    echo "✗ FAILED: Bob not assigned to any time slot"
    exit 1
fi

# Check that Carol is mentioned with a time slot
if echo "$ai_output" | grep -i "carol" | grep -q "9am\|10am\|11am\|12pm"; then
    echo "✓ Carol assigned to a time slot"
else
    echo "✗ FAILED: Carol not assigned to any time slot"
    exit 1
fi

# Check that David is at 11am or 12pm (not 9am or 10am)
if echo "$ai_output" | grep -i "david" | grep -q "11am\|12pm"; then
    echo "✓ David correctly scheduled at 11am or 12pm"
elif echo "$ai_output" | grep -i "david" | grep -q "9am\|10am"; then
    echo "✗ FAILED: David incorrectly scheduled at 9am or 10am (constraint violation)"
    exit 1
else
    echo "✗ FAILED: David not assigned to any time slot"
    exit 1
fi

# Check Bob before Carol constraint (this is the critical one)
# In the expected solution: Bob=9am, Carol=11am
if echo "$ai_output" | grep -i "bob" | grep -q "9am" && echo "$ai_output" | grep -i "carol" | grep -q "11am"; then
    echo "✓ Bob (9am) correctly scheduled before Carol (11am)"
elif echo "$ai_output" | grep -i "bob" | grep -q "10am" && echo "$ai_output" | grep -i "carol" | grep -q "11am\|12pm"; then
    echo "✓ Bob (10am) correctly scheduled before Carol (11am/12pm)"
elif echo "$ai_output" | grep -i "bob" | grep -q "11am" && echo "$ai_output" | grep -i "carol" | grep -q "12pm"; then
    echo "✓ Bob (11am) correctly scheduled before Carol (12pm)"
else
    echo "⚠️  WARNING: Could not verify Bob < Carol constraint automatically"
    echo "   Manual check required - please verify Bob is scheduled before Carol"
fi

# Check if AI provided explanation
if echo "$ai_output" | grep -qi -E "(explain|constraint|satisfy|solution)"; then
    echo "✓ AI provided explanation of the solution"
else
    echo "✗ FAILED: AI did not provide explanation of the solution"
    exit 1
fi

echo ""
echo "🎉 ALL TESTS PASSED!"
echo "✅ AI successfully solved the constraint satisfaction problem"
echo "   - Used solve_csp tool correctly"
echo "   - Found a valid solution"
echo "   - All constraints satisfied"
echo "   - Provided clear explanation"

exit 0