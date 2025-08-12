#!/bin/bash

# Simple single test runner
# Usage: CLI=path/to/gennai ./runner.sh <testcase> <backend>
# Example: CLI=output/gennai ./runner.sh fibonacci_test ollama_gbnf

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Check if CLI is set
if [ -z "$CLI" ]; then
    echo "Error: CLI environment variable is not set"
    echo "Usage: CLI=path/to/gennai ./testsuite/runner.sh <testcase> <backend>"
    echo "Example: CLI=output/gennai ./testsuite/runner.sh fibonacci_test ollama_gbnf"
    exit 1
fi

# Check if the binary exists
if [ ! -x "$CLI" ]; then
    echo "Error: CLI binary '$CLI' does not exist or is not executable"
    exit 1
fi

# Get script directory
script_dir="$(cd "$(dirname "$0")" && pwd)"

# Parse arguments
if [ $# -eq 0 ]; then
    echo -e "${BLUE}🧪 Available Test Cases:${NC}"
    find "${script_dir}/testcases" -maxdepth 1 -type d -name "*" | grep -v "/testcases$" | sort | while read testcase_dir; do
        testcase_name=$(basename "$testcase_dir")
        echo "  • $testcase_name"
    done
    echo ""
    echo -e "${BLUE}🔧 Available Backends:${NC}"
    find "${script_dir}/backends" -maxdepth 1 -name "*.json" | sort | while read backend_file; do
        backend_name=$(basename "$backend_file" .json)
        echo "  • $backend_name"
    done
    echo ""
    echo "Usage: CLI=path/to/gennai ./runner.sh <testcase> <backend>"
    echo "   or: CLI=path/to/gennai ./runner.sh <testcase>  # runs with ollama"
    exit 0
fi

testcase_name="$1"
backend_name="${2:-ollama}"  # Default to ollama

# Validate testcase
testcase_dir="${script_dir}/testcases/${testcase_name}"
if [ ! -d "$testcase_dir" ]; then
    echo -e "${RED}Error: Testcase '$testcase_name' not found${NC}"
    echo "Available testcases:"
    find "${script_dir}/testcases" -maxdepth 1 -type d -name "*" | grep -v "/testcases$" | sort | while read dir; do
        echo "  • $(basename "$dir")"
    done
    exit 1
fi

# Validate backend
backend_file="${script_dir}/backends/${backend_name}.json"
if [ ! -f "$backend_file" ]; then
    echo -e "${RED}Error: Backend '$backend_name' not found${NC}"
    echo "Available backends:"
    find "${script_dir}/backends" -maxdepth 1 -name "*.json" | sort | while read file; do
        echo "  • $(basename "$file" .json)"
    done
    exit 1
fi

# Check required test files
if [ ! -f "$testcase_dir/prompt.txt" ]; then
    echo -e "${RED}Error: $testcase_name/prompt.txt not found${NC}"
    exit 1
fi

if [ ! -f "$testcase_dir/check.sh" ] || [ ! -x "$testcase_dir/check.sh" ]; then
    echo -e "${RED}Error: $testcase_name/check.sh not found or not executable${NC}"
    exit 1
fi

echo -e "${BLUE}🧪 Running Single Test${NC}"
echo -e "${CYAN}Testcase: $testcase_name${NC}"
echo -e "${CYAN}Backend: $backend_name${NC}"
echo -e "${BLUE}Binary: $CLI${NC}"
echo ""

# Create temporary files  
output_file=$(mktemp)
error_file=$(mktemp)

# Create a temporary directory for the test to run in isolation
temp_test_dir=$(mktemp -d)
echo -e "${YELLOW}🗂️  Created temporary test directory: $temp_test_dir${NC}"

# Copy all test files from the testcase directory to the temp directory
echo -e "${YELLOW}📋 Copying test files to temporary directory...${NC}"
cp -r "$testcase_dir/"* "$temp_test_dir/"

# Use the temporary directory as the working directory (complete isolation)
test_work_dir="$temp_test_dir"
echo "Test working directory: $test_work_dir"

# Note: No need for git clean since we're in a fresh temp directory

# Run the test in the temporary directory using the copied files
prompt_file="$temp_test_dir/prompt.txt"
echo -e "${CYAN}Running: $CLI --workdir $test_work_dir --settings $backend_file -f $prompt_file${NC}"
if "$CLI" --workdir "$test_work_dir" --settings "$backend_file" -f "$prompt_file" > "$output_file" 2> "$error_file"; then
    exit_code=0
else
    exit_code=$?
fi

echo ""
echo -e "${BLUE}📋 Test Output:${NC}"
echo "----------------------------------------"
cat "$output_file"
echo "----------------------------------------"

if [ $exit_code -eq 0 ]; then
    # Run the check script from the test working directory (use copied check.sh)
    echo -e "${YELLOW}🔍 Running validation check in: $test_work_dir${NC}"
    
    if (cd "$test_work_dir" && "$temp_test_dir/check.sh" "$output_file" "$error_file"); then
        echo ""
        echo -e "${GREEN}✅ PASS: $testcase_name × $backend_name${NC}"
        # Clean up temporary directory and files
        echo -e "${YELLOW}🧹 Cleaning up temporary directory: $temp_test_dir${NC}"
        rm -rf "$temp_test_dir"
        rm -f "$output_file" "$error_file"
        exit 0
    else
        echo ""
        echo -e "${RED}❌ FAIL: $testcase_name × $backend_name (check script failed)${NC}"
        echo -e "${YELLOW}Error output:${NC}"
        cat "$error_file"
        echo -e "${YELLOW}Test directory contents:${NC}"
        ls -la "$test_work_dir"
        echo -e "${YELLOW}💾 Temporary directory preserved for debugging: $temp_test_dir${NC}"
        echo -e "${YELLOW}    (Will be cleaned up automatically on next test run)${NC}"
        # Clean up temporary files but preserve temp directory for debugging
        rm -f "$output_file" "$error_file"
        exit 1
    fi
else
    echo -e "${RED}❌ FAIL: $testcase_name × $backend_name (gennai execution failed, exit code: $exit_code)${NC}"
    echo -e "${YELLOW}Error output:${NC}"
    cat "$error_file"
    echo -e "${YELLOW}💾 Temporary directory preserved for debugging: $temp_test_dir${NC}"
    echo -e "${YELLOW}    (Will be cleaned up automatically on next test run)${NC}"
    # Clean up temporary files but preserve temp directory for debugging
    rm -f "$output_file" "$error_file"
    exit 1
fi