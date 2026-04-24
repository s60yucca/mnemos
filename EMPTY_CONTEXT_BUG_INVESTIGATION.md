# Empty Context Bug Investigation

**Date**: 2026-04-24  
**Status**: IN PROGRESS - Debug logging added, awaiting Codex test  
**Priority**: CRITICAL

## Problem Statement

Codex (Kiro agent using Homebrew mnemos) calls `mnemos_context` for project "hms" but receives empty response:
```json
{"memories": null, "relations": null, "total_tokens": 0}
```

Despite 5 memories existing for the hms project.

## Evidence Collected

### 1. Memories Exist
```bash
$ mnemos list --project hms --limit 10
# Returns 5 memories including payment-related content
```

### 2. Search Works
```bash
$ mnemos search "payment" --project hms
# Returns 2 results successfully
```

### 3. Feature Logs Show the Problem
```bash
$ grep -B 1 -A 1 "context_call.*hms" ~/.mnemos/logs/features.log | grep -E "(context_call|mmr)"
# Shows context_call events for hms but NO mmr events following them
# This proves: HybridSearch returns 0 candidates
```

For comparison, context_call for mnemos-dev project DOES have mmr events following it.

## Root Cause Hypothesis

**Most Likely**: Query parameter is empty or invalid when Codex calls mnemos_context

**Evidence**:
- Code shows: `if query == "" { return mcpError("query is required") }`
- But error may not propagate properly through MCP protocol
- AssembleContext returns empty `ContextResult{}` when HybridSearch finds 0 candidates
- No error is logged, suggesting silent failure

**Alternative Hypotheses**:
1. HybridSearch fails for hms project specifically (but search CLI works fine)
2. Embedding provider issue (but semantic search works via CLI)
3. Project ID mismatch (but logs show project=hms consistently)

## Changes Made (2026-04-24)

### 1. Debug Logging in handleContext
**File**: `internal/transport/mcp/tools.go`

Added logging to capture actual MCP request parameters:
```go
// DEBUG: Log incoming parameters to diagnose empty context bug
fmt.Fprintf(os.Stderr, "[DEBUG] handleContext: query=%q project_id=%q max_tokens=%d include_relations=%v\n",
    query, projectID, maxTokens, includeRelations)
```

Added logging for results:
```go
// DEBUG: Log result to diagnose empty context bug
fmt.Fprintf(os.Stderr, "[DEBUG] handleContext result: query=%q project_id=%q memory_count=%d total_tokens=%d\n",
    query, projectID, memCount, result.TotalTokens)
```

### 2. Debug Logging in AssembleContext
**File**: `internal/core/search/engine.go`

Added logging before and after HybridSearch:
```go
// DEBUG: Log incoming parameters
e.logger.Info("AssembleContext called",
    "query", query,
    "project_id", projectID,
    "max_tokens", maxTokens)

// ... HybridSearch call ...

// DEBUG: Log candidate count
e.logger.Info("HybridSearch completed",
    "query", query,
    "project_id", projectID,
    "candidate_count", len(candidates))
```

### 3. Cold Start UX Improvement
**File**: `internal/core/search/engine.go`

Added `Message` field to `ContextResult`:
```go
type ContextResult struct {
    Memories    []*domain.Memory         `json:"memories"`
    Relations   []*domain.MemoryRelation `json:"relations"`
    TotalTokens int                      `json:"total_tokens"`
    Message     string                   `json:"message,omitempty"` // Optional helpful message
}
```

**File**: `internal/transport/mcp/tools.go`

Added helpful message when no memories found:
```go
// Cold start UX: Add helpful message when no memories found
if result != nil && len(result.Memories) == 0 {
    projectLabel := "this workspace"
    if projectID != "" {
        projectLabel = fmt.Sprintf("project '%s'", projectID)
    }
    result.Message = fmt.Sprintf("No memories found for %s. Memories will be available in future sessions after you store them during this session.", projectLabel)
}
```

## Next Steps

### Immediate (Requires Codex Test)
1. **Test with Codex** to capture actual MCP request in debug logs
2. **Check stderr output** from mnemos serve to see:
   - What query parameter Codex is passing
   - How many candidates HybridSearch returns
   - Where exactly the pipeline fails

### If Query is Empty
1. Fix error handling to return informative error instead of silent failure
2. Ensure MCP error propagates correctly to client
3. Add validation logging before the empty check

### If Query is Valid but Search Fails
1. Investigate why HybridSearch returns 0 candidates for hms project
2. Check if there's a project ID mismatch in the database
3. Verify embedding vectors exist for hms memories
4. Test HybridSearch directly with same parameters

### Release
1. Build new version with debug logging
2. Test locally to confirm logs appear
3. Release to Homebrew so Codex gets the fix
4. Monitor Codex usage to capture debug output

## Impact

**CRITICAL** - Core value prop failure:
- Agent calls mnemos_context at session start (autopilot working!)
- Receives empty response
- Learns that mnemos is useless
- Never calls it again

This blocks the entire dogfooding workflow and makes mnemos appear broken to real users.

## Files Modified

- `internal/transport/mcp/tools.go` (handleContext function)
- `internal/core/search/engine.go` (AssembleContext function, ContextResult struct)

## Build Status

✅ Build successful: `go build -o mnemos cmd/mnemos/*.go`

## Testing Instructions

### Local Testing
```bash
# Start mnemos serve with debug logging
./mnemos serve 2>mnemos_debug.log &

# In another terminal, trigger a context call via MCP
# (Use Codex or another MCP client)

# Check debug log
cat mnemos_debug.log
```

### Expected Debug Output
```
[DEBUG] handleContext: query="payment processing" project_id="hms" max_tokens=2000 include_relations=false
[DEBUG] handleContext result: query="payment processing" project_id="hms" memory_count=0 total_tokens=0
```

This will tell us:
1. Is query parameter being passed?
2. Is it empty or valid?
3. How many memories are being returned?

## Related Issues

- Bench mode features not in Homebrew release yet (separate issue)
- Codex using old Homebrew version without benchmark support (separate issue)
- Need to release new version to Homebrew for Codex to get fixes

## References

- Previous investigation: `.kiro/specs/mnemos-dogfooding-benchmark/` (Task 3 context)
- Feature logs: `~/.mnemos/logs/features.log`
- Health analysis: Showed 40% test noise vs 9% real work
