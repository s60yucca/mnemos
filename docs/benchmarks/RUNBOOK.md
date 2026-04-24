# Dogfooding Benchmark Runbook

This runbook guides you through executing the 1-week dogfooding benchmark to measure mnemos's real-world token impact. Follow these procedures to collect credible, reproducible data.

## Overview

**Goal:** Measure token usage across 20+ sessions on primary project (mnemos-dev) and 6+ sessions on each spot-check project, comparing mnemos ON vs OFF modes.

**Duration:** 7 days (extend by 3 days if sample size insufficient)

**Mode Alternation:** Daily (Day 1 ON, Day 2 OFF, Day 3 ON, etc.)

**Primary Close Mechanism:** Explicit `mnemos bench session-end` command after each task

**Fallback:** 10-minute inactivity timeout auto-closes forgotten sessions

## Pre-Dogfood Checklist

Before starting the benchmark week, verify all systems are working:

### 1. Verify Mnemos Health

```bash
mnemos health
```

**Expected:** All features show `FIRING NORMALLY` status. If any feature shows `NOT FIRING` or `ERROR`, investigate before starting benchmark.

### 2. Test Bench Mode Toggle

```bash
# Set mode to ON
mnemos bench mode on
# Verify
cat ~/.mnemos/bench_mode
# Should show: on

# Set mode to OFF
mnemos bench mode off
# Verify
cat ~/.mnemos/bench_mode
# Should show: off

# Reset to ON for Day 1
mnemos bench mode on
```

### 3. Test Export Command

```bash
mnemos bench export --output test.csv
```

**Expected:** CSV file created with headers. May be empty if no sessions yet.

### 4. Verify Token Counting

Start a test Kiro task and make an MCP call (e.g., ask agent to search mnemos context). Then check:

```bash
tail -20 ~/.mnemos/logs/features.log | grep bench_session
```

**Expected:** You should see `bench_session_start` event with session_id, project_id, mode, and timestamp.

### 5. Test Session End

```bash
mnemos bench session-end
tail -20 ~/.mnemos/logs/features.log | grep bench_session_end
```

**Expected:** You should see `bench_session_end` event with session_id, duration_ms, tokens_in, tokens_out, mcp_calls_count, and task_completed=true.

## Daily Procedure

### Morning Routine

1. **Check current day number** (Day 1, 2, 3, etc.)

2. **Set benchmark mode** (alternate daily):
   ```bash
   # Odd days (1, 3, 5, 7): ON
   mnemos bench mode on
   
   # Even days (2, 4, 6): OFF
   mnemos bench mode off
   ```

3. **Verify mode set correctly**:
   ```bash
   mnemos bench status
   ```

4. **Note the mode in your daily log** (optional but helpful):
   ```
   Day 1 (Mon): ON
   Day 2 (Tue): OFF
   Day 3 (Wed): ON
   ...
   ```

### During Work

**Work normally on Kiro tasks.** The system automatically tracks sessions.

For each task:

1. **Start Kiro task** in your IDE

2. **(Optional) Start session explicitly with category**:
   ```bash
   mnemos bench session-start --category feature
   # Categories: refactor, feature, debug, docs, other
   ```

3. **Confirm session started** (optional verification):
   ```bash
   tail -f ~/.mnemos/logs/features.log | grep bench_session_start
   ```
   Press Ctrl+C to stop tailing.

4. **Work on the task normally** — let Kiro agent use mnemos as usual

5. **When task completes, close the session explicitly** (RECOMMENDED):
   ```bash
   mnemos bench session-end
   ```
   
   This is the primary close mechanism for accurate task boundaries.

6. **Note:** If you forget to run `session-end`, the 10-minute inactivity timeout will auto-close the session.

### Evening Routine

1. **Export today's sessions**:
   ```bash
   mnemos bench export --since today --output daily_$(date +%Y%m%d).csv
   ```

2. **Count sessions**:
   ```bash
   # Total sessions today (subtract 1 for header row)
   wc -l daily_$(date +%Y%m%d).csv
   ```

3. **Verify session count** — aim for ≥2 sessions per day

4. **Review for outliers**:
   - Very short sessions (<1 minute) — may indicate false start
   - Very long sessions (>30 minutes) — may indicate forgotten session-end
   - Crashed sessions (task_completed=false) — note in log

5. **Note any issues** in a daily log file (optional):
   ```
   Day 1 (Mon, ON):
   - 3 sessions completed
   - 1 session crashed (Kiro chat froze during refactor)
   - Tasks: feature work on autopilot, debug session tracker
   ```

## Per-Session Actions

### Starting a Session

**Automatic:** First MCP call (mnemos_context, mnemos_store, etc.) automatically starts a session.

**Manual (optional):** For better categorization:
```bash
mnemos bench session-start --category <refactor|feature|debug|docs|other>
```

**Verification:**
```bash
# Check session started
tail -5 ~/.mnemos/logs/features.log | grep bench_session_start

# Or check current session status
mnemos bench status
```

### During a Session

**Just work normally.** The system tracks:
- Token counts (input + output) for all MCP calls
- MCP call count
- Session duration

**No manual bookkeeping required.**

### Ending a Session

**Primary mechanism (RECOMMENDED):**
```bash
mnemos bench session-end
```

Run this command when you complete a task. This provides accurate task boundaries.

**Fallback mechanism:**
- 10-minute inactivity timeout will auto-close if you forget
- Useful for sessions where you step away without explicitly closing

**Verification:**
```bash
tail -5 ~/.mnemos/logs/features.log | grep bench_session_end
```

## End-of-Day Actions

### 1. Export Sessions

```bash
mnemos bench export --since today --output daily_$(date +%Y%m%d).csv
```

### 2. Verify Session Counts

```bash
# Count today's sessions (subtract 1 for header)
wc -l daily_$(date +%Y%m%d).csv

# View sessions
cat daily_$(date +%Y%m%d).csv
```

**Target:** ≥2 sessions per day

### 3. Note Outliers

Review the CSV for:

- **Very short sessions** (<1 minute, <100 tokens):
  - Likely false starts or test calls
  - Note in daily log, may exclude from analysis

- **Very long sessions** (>30 minutes, >10k tokens):
  - May indicate forgotten session-end
  - Verify this was actually one continuous task

- **Crashed sessions** (task_completed=false):
  - Note the cause (Kiro chat crash, network error, etc.)
  - These will be excluded from main analysis but reported separately

- **Mixed mode sessions** (mode changed mid-session):
  - Should be rare if following daily procedure
  - These will be excluded from analysis

### 4. Check Mnemos Health (ON days only)

```bash
mnemos health
```

**Expected:** All features FIRING NORMALLY during ON days. If features show NOT FIRING, investigate — this may indicate mnemos isn't being used by the agent.

## End-of-Week Actions

### 1. Total Session Count Check

```bash
# Export all sessions from the week
mnemos bench export --output week_results.csv

# Count total sessions (subtract 1 for header)
wc -l week_results.csv

# Count by mode
grep ",on," week_results.csv | wc -l
grep ",off," week_results.csv | wc -l

# Count by project
grep "mnemos-dev" week_results.csv | wc -l
```

### 2. Verify Sample Size Requirements

**Primary project (mnemos-dev):**
- Minimum: 20 valid sessions (10 ON + 10 OFF)
- Valid = task_completed=true AND mode not mixed

**Spot-check projects (if applicable):**
- Minimum: 6 valid sessions each (3 ON + 3 OFF)

### 3. Extend if Needed

**If sample size insufficient:**
- Extend by 3 days
- Continue daily alternation from where you left off
- Re-check at end of extension

**If still insufficient after extension:**
- Proceed to report generation
- Disclose sample size limitation in Summary section

### 4. Final Health Check

```bash
mnemos health
```

Verify all features were FIRING NORMALLY during ON sessions throughout the week.

### 5. Generate Final Export

```bash
# Export all sessions with complete data
mnemos bench export --output final_benchmark_$(date +%Y%m%d).csv

# Optionally filter by project
mnemos bench export --project mnemos-dev --output primary_project.csv
```

## Outlier Handling

### Crashed Session

**Symptom:** Session ends with task_completed=false, or Kiro chat crashes mid-task.

**Action:**
1. Note in daily log: "Session X crashed due to [reason]"
2. Check features.log for session_end event
3. If no session_end event, session will auto-close on next MCP call
4. These sessions are excluded from main analysis but counted separately

**Investigation trigger:** If >20% of sessions crash, investigate before publishing.

### Network Failure

**Symptom:** MCP calls fail, Kiro chat shows connection errors.

**Action:**
1. Note in daily log: "Session X had network failure"
2. Session may have incomplete token counts
3. Mark as outlier, exclude from analysis
4. Restart session after network recovers

### Mixed Mode Accidentally

**Symptom:** Changed mode mid-session (e.g., forgot to wait for session to end before toggling mode).

**Action:**
1. Note in daily log: "Session X mixed mode (changed from ON to OFF mid-task)"
2. Session will be marked mode_mixed in features.log
3. These sessions are automatically excluded from export by default
4. To see them: `mnemos bench export --include-mixed`

**Prevention:** Always run `mnemos bench session-end` before changing mode.

### Forgot to Run session-end

**Symptom:** Session stayed open for hours, merged multiple tasks.

**Action:**
1. Note in daily log: "Session X merged multiple tasks (forgot session-end)"
2. 10-minute timeout should have closed it, but if tasks were <10min apart, they merged
3. Review session duration and token counts — if reasonable, keep it
4. If clearly wrong (e.g., 2-hour session with 50k tokens), mark as outlier

**Prevention:** Run `mnemos bench session-end` after each task completes.

### Very Short Session

**Symptom:** Session <1 minute, <100 tokens.

**Possible causes:**
- False start (opened Kiro chat, closed immediately)
- Test call during verification
- Quick question to agent (not a real task)

**Action:**
1. Review session in CSV
2. If clearly not a real task, note as outlier
3. May exclude from analysis (disclose in report)

### Very Long Session

**Symptom:** Session >30 minutes, >10k tokens.

**Possible causes:**
- Complex refactor (legitimate)
- Forgot to run session-end (merged multiple tasks)
- Agent got stuck in loop (rare)

**Action:**
1. Review session in CSV
2. Check your memory — was this actually one continuous task?
3. If legitimate, keep it
4. If merged tasks, note as outlier

## Verification Commands

### Check Current Status

```bash
mnemos bench status
```

Shows:
- Current bench mode (ON or OFF)
- Current session info (if active)
- Session count in last 7 days

### View Recent Sessions

```bash
mnemos bench export --since today
```

### View Features Log

```bash
tail -50 ~/.mnemos/logs/features.log | grep bench
```

Shows recent bench-related events:
- bench_session_start
- bench_session_end
- bench_mode_change

### Check Mnemos Health

```bash
mnemos health
```

Verify features are FIRING NORMALLY during ON sessions.

## Troubleshooting

### No session_start event appearing

**Check:**
1. Is bench mode set? `cat ~/.mnemos/bench_mode`
2. Is MCP server running? Check Kiro chat connection
3. Are you making MCP calls? (mnemos_context, mnemos_store, etc.)

**Fix:**
- Restart MCP server
- Verify mnemos is configured in Kiro chat
- Make a test MCP call (ask agent to search mnemos context)

### Token counts are zero

**Check:**
1. Are MCP calls actually happening? Check features.log
2. Is token counter initialized? Check MCP server logs

**Fix:**
- Restart MCP server
- Verify tiktoken library is installed: `go list -m github.com/pkoukk/tiktoken-go`

### Mode OFF but agent still references past context

**Check:**
1. Verify mode is actually OFF: `cat ~/.mnemos/bench_mode`
2. Check MCP server read the mode file (restart server if needed)
3. Verify mnemos_context returns empty: check features.log

**Fix:**
- Restart MCP server to reload bench mode
- Clear Kiro chat context (start new chat session)

### Session never ends

**Check:**
1. Is inactivity timeout working? Wait 10 minutes with no MCP calls
2. Check features.log for session_end event

**Fix:**
- Manually run: `mnemos bench session-end`
- Restart MCP server if timeout goroutine is stuck

## Daily Alternation Schedule

| Day | Date | Mode | Notes |
|-----|------|------|-------|
| 1   | Mon  | ON   | Start of benchmark week |
| 2   | Tue  | OFF  | |
| 3   | Wed  | ON   | |
| 4   | Thu  | OFF  | |
| 5   | Fri  | ON   | |
| 6   | Sat  | OFF  | (or skip if not working) |
| 7   | Sun  | ON   | (or skip if not working) |

**If you skip a day** (weekend, illness, etc.):
- Resume alternation from where you stopped
- Example: Skip Sat/Sun → Mon is Day 6 (OFF), Tue is Day 7 (ON)

**Natural work gaps:**
- Don't force work on weekends just for benchmark
- Extend by 3 days if sample size insufficient

## Success Criteria

At end of week, you should have:

- ✅ Primary project: ≥20 valid sessions (10 ON + 10 OFF)
- ✅ Spot-check projects: ≥6 valid sessions each (3 ON + 3 OFF)
- ✅ At least one full day in each mode per project
- ✅ `mnemos health` shows all features FIRING NORMALLY during ON sessions
- ✅ CSV export contains complete data (no missing token counts)
- ✅ Outliers documented in daily log

If criteria met → proceed to benchmark report generation.

If criteria not met → extend by 3 days OR publish with disclosed limitations.

## Next Steps

After completing the dogfood week:

1. **Generate final export:**
   ```bash
   mnemos bench export --output final_results.csv
   ```

2. **Analyze data** (use spreadsheet tool or analysis script)

3. **Fill in benchmark report:** docs/benchmarks/real-world.md

4. **Update README.md** with headline numbers

5. **Review publication gate:** docs/benchmarks/PUBLICATION_GATE.md

6. **Publish results**

## Notes

- **Token counts are approximated** using tiktoken (±20-30% error vs actual Claude API). This is disclosed in the benchmark report. Percentage reduction is robust to approximation error.

- **Mode OFF preserves data:** Memories are still stored (with "bench_off_day" tag) even in OFF mode. No data loss during benchmark week.

- **Primary close mechanism:** Explicit `mnemos bench session-end` command provides accurate task boundaries. Use this after each task.

- **Fallback close mechanism:** 10-minute inactivity timeout auto-closes forgotten sessions.

- **Failed sessions excluded:** Sessions with task_completed=false are excluded from main analysis but reported separately.

- **Mixed mode sessions excluded:** Sessions where mode changed mid-task are excluded from analysis.

- **This is a self-measurement:** Single maintainer, natural workflow, acknowledged confounds. Not a controlled scientific study.
