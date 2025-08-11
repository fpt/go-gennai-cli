#!/bin/bash

# Test code generation scenario
# Arguments: $1 = output file, $2 = error file

output_file="$1"
error_file="$2"

# Check that response contains Go function code
if grep -q "func.*(" "$output_file" && \
   grep -q "return" "$output_file" && \
   grep -q "int" "$output_file"; then
    echo "✓ Code generation response contains expected Go function elements"
    exit 0
else
    echo "✗ Code generation response missing expected Go function elements"
    echo "Expected: function declaration with 'func', 'return' statement, and 'int' type"
    echo "Output was:"
    cat "$output_file"
    if [ -s "$error_file" ]; then
        echo "Errors:"
        cat "$error_file"
    fi
    exit 1
fi