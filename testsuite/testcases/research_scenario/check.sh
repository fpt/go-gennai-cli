#!/bin/bash

# Test research scenario - knowledge-based response
# Arguments: $1 = output file, $2 = error file

output_file="$1"
error_file="$2"

# Check that response contains clean architecture concepts
if grep -iq "SOLID\|dependency\|abstraction\|separation\|clean\|architecture\|principle" "$output_file"; then
    echo "✓ Research scenario provided relevant clean architecture information"
    exit 0
else
    echo "✗ Research scenario response missing expected clean architecture concepts"
    echo "Expected: mentions of SOLID, dependency, abstraction, separation, etc."
    echo "Output was:"
    cat "$output_file"
    if [ -s "$error_file" ]; then
        echo "Errors:"
        cat "$error_file"
    fi
    exit 1
fi