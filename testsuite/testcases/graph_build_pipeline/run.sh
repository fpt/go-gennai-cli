#!/bin/bash

# Custom run script for solver shopping test case
# This forces the use of the SOLVER scenario instead of automatic scenario selection

# Arguments: $1 = CLI binary, $2 = workdir, $3 = settings file, $4 = prompt file

CLI="$1"
WORKDIR="$2"
SETTINGS="$3"
PROMPT="$4"

# Force SOLVER scenario with -s flag
"$CLI" --workdir "$WORKDIR" --settings "$SETTINGS" -s SOLVER -f "$PROMPT"
