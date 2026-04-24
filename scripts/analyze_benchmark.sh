#!/bin/bash
# Benchmark Analysis Script (Shell version)
# Reads CSV from `mnemos bench export` and computes basic statistics

set -e

if [ $# -lt 1 ]; then
    echo "Usage: analyze_benchmark.sh <csv_file>"
    echo ""
    echo "Example:"
    echo "  mnemos bench export --output benchmark.csv"
    echo "  bash scripts/analyze_benchmark.sh benchmark.csv"
    exit 1
fi

CSV_FILE="$1"

if [ ! -f "$CSV_FILE" ]; then
    echo "Error: File '$CSV_FILE' not found."
    exit 1
fi

# Skip header and filter completed sessions
SESSIONS=$(tail -n +2 "$CSV_FILE" | awk -F',' '$10 == "true"')

if [ -z "$SESSIONS" ]; then
    echo "No completed sessions found in CSV."
    exit 1
fi

# Compute total tokens (tokens_in + tokens_out) for each session
# CSV columns: session_id,timestamp_start,timestamp_end,project_id,mode,duration_ms,tokens_in,tokens_out,mcp_calls_count,task_completed,task_category
ON_TOKENS=$(echo "$SESSIONS" | awk -F',' '$5 == "on" {print $7 + $8}')
OFF_TOKENS=$(echo "$SESSIONS" | awk -F',' '$5 == "off" {print $7 + $8}')

# Count sessions
ON_COUNT=$(echo "$ON_TOKENS" | grep -c . || echo 0)
OFF_COUNT=$(echo "$OFF_TOKENS" | grep -c . || echo 0)
TOTAL_COUNT=$((ON_COUNT + OFF_COUNT))

# Compute stats using awk and sort
compute_stats() {
    local tokens="$1"
    local count=$(echo "$tokens" | wc -l | tr -d ' ')
    local sum=$(echo "$tokens" | awk '{sum += $1} END {print sum}')
    local mean=$((sum / count))
    local min=$(echo "$tokens" | sort -n | head -1)
    local max=$(echo "$tokens" | sort -n | tail -1)
    local median=$(echo "$tokens" | sort -n | awk -v count="$count" '
        {values[NR] = $1}
        END {
            if (count % 2 == 1) {
                print values[(count+1)/2]
            } else {
                print int((values[count/2] + values[count/2+1]) / 2)
            }
        }')
    
    echo "mean=$mean median=$median min=$min max=$max count=$count"
}

echo "============================================================"
echo "BENCHMARK ANALYSIS"
echo "============================================================"
echo ""
echo "Total Sessions: $TOTAL_COUNT ($ON_COUNT ON, $OFF_COUNT OFF)"
echo ""

if [ -n "$OFF_TOKENS" ]; then
    OFF_STATS=$(compute_stats "$OFF_TOKENS")
    echo "Mode OFF: $OFF_STATS"
else
    echo "Mode OFF: No sessions"
fi

if [ -n "$ON_TOKENS" ]; then
    ON_STATS=$(compute_stats "$ON_TOKENS")
    echo "Mode ON:  $ON_STATS"
else
    echo "Mode ON: No sessions"
fi

# Compute reduction
if [ -n "$ON_TOKENS" ] && [ -n "$OFF_TOKENS" ]; then
    ON_MEAN=$(echo "$ON_STATS" | sed -n 's/.*mean=\([0-9]*\).*/\1/p')
    OFF_MEAN=$(echo "$OFF_STATS" | sed -n 's/.*mean=\([0-9]*\).*/\1/p')
    
    if [ "$OFF_MEAN" -gt 0 ]; then
        REDUCTION=$(awk "BEGIN {printf \"%.1f\", (($OFF_MEAN - $ON_MEAN) / $OFF_MEAN) * 100}")
        echo ""
        echo "Reduction: ${REDUCTION}%"
    fi
fi

echo ""
echo "============================================================"
echo "For detailed analysis with histograms and markdown output:"
echo "  python3 scripts/analyze_benchmark.py $CSV_FILE"
echo "============================================================"
