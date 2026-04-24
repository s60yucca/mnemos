#!/bin/bash

# Test mnemos_context MCP call with debug logging

# Kill any existing mnemos serve processes
pkill -f "mnemos serve" || true
sleep 1

# Start mnemos serve with stderr redirected to a log file
./mnemos serve 2>mnemos_debug.log &
SERVE_PID=$!
echo "Started mnemos serve (PID: $SERVE_PID)"
sleep 2

# Test context call using the MCP protocol
# We'll use a simple Python script to make the MCP call
python3 - <<'EOF'
import json
import subprocess
import sys

# Prepare MCP request
request = {
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
        "name": "mnemos_context",
        "arguments": {
            "query": "payment processing",
            "project_id": "hms",
            "max_tokens": 2000
        }
    }
}

# Send request via stdin to mnemos serve
try:
    result = subprocess.run(
        ["./mnemos", "serve"],
        input=json.dumps(request) + "\n",
        capture_output=True,
        text=True,
        timeout=5
    )
    print("STDOUT:", result.stdout)
    print("STDERR:", result.stderr)
except subprocess.TimeoutExpired:
    print("Request timed out")
except Exception as e:
    print(f"Error: {e}")
EOF

# Wait a bit for the request to process
sleep 2

# Show debug log
echo ""
echo "=== Debug Log ==="
cat mnemos_debug.log

# Cleanup
kill $SERVE_PID 2>/dev/null || true
