#!/bin/bash

# Generate enough MCP calls to create an active day
for i in {1..15}; do
  /tmp/mnemos store "Test memory $i about authentication and JWT tokens" --project test-health --tags "test,auth" > /dev/null 2>&1
done

# Make some search calls
for i in {1..5}; do
  /tmp/mnemos search "authentication" --project test-health > /dev/null 2>&1
done

# Make some context calls
for i in {1..3}; do
  /tmp/mnemos context "JWT tokens" --project test-health > /dev/null 2>&1
done

echo "Generated test events. Running health check..."
/tmp/mnemos health --days 1 --project test-health
