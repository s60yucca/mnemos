#!/bin/bash
# Wrapper script to capture mnemos serve debug output

LOG_FILE="$HOME/.mnemos/mcp-debug.log"

# Ensure log directory exists
mkdir -p "$(dirname "$LOG_FILE")"

# Log startup
echo "=== mnemos serve started at $(date) ===" >> "$LOG_FILE"

# Run mnemos serve and redirect stderr to log file
/opt/homebrew/bin/mnemos serve 2>> "$LOG_FILE"
