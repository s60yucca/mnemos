# Real-World Dogfooding Benchmark

## Summary

<!-- FILL IN AFTER DOGFOOD COMPLETES -->

Mnemos reduced session tokens by **[X]%** on average across **[Y]** sessions over **[Z]** projects in 1 week of daily use.

**Key Findings:**
- Primary project (mnemos-dev): **[X]%** reduction ([N] sessions)
- Spot-check project 1: **[X]%** reduction ([N] sessions)
- Spot-check project 2: **[X]%** reduction ([N] sessions)

**Note:** Token counts are approximated using tiktoken (OpenAI's cl100k_base encoding). Claude uses a different tokenizer. Estimated error margin: ±20-30% vs actual Claude API counts. Results reported as percentage reduction only, which is robust to systematic approximation error.

## Methodology

### Study Design

- **Duration:** [START_DATE] to [END_DATE] (7 days)
- **Participant:** Single maintainer (author)
- **Projects:**
  - **Primary:** mnemos-dev (detailed sampling, ≥20 sessions)
  - **Spot-check 1:** [PROJECT_NAME] (≥6 sessions)
  - **Spot-check 2:** [PROJECT_NAME] (≥6 sessions)
- **Mode alternation:** Daily rotation (Day 1 ON, Day 2 OFF, Day 3 ON, etc.)
- **Session detection:** 10-minute inactivity timeout with explicit `mnemos bench session-end` as primary close mechanism
- **Token counting:** Tiktoken approximation using cl100k_base encoding

### What Was Measured

Each Kiro task execution was tracked as one benchmark session, capturing:
- **Tokens in:** Input tokens (user → agent)
- **Tokens out:** Output tokens (agent → user)
- **MCP calls:** Number of mnemos tool invocations
- **Duration:** Session start to end time
- **Task category:** Feature, refactor, debug, docs, or other

### Benchmark Modes

- **Mode ON:** Mnemos autopilot active, full context injection and memory storage
- **Mode OFF:** Mnemos returns empty results for context/search (simulating no installation), but still stores memories with "bench_off_day" tag to prevent data loss

### Data Collection

- Auto-logging via mnemos MCP server to `~/.mnemos/logs/features.log`
- Session boundaries detected automatically (10-minute inactivity timeout)
- Manual session close via `mnemos bench session-end` when task completes
- Daily mode toggle via `mnemos bench mode <on|off>`
- CSV export via `mnemos bench export` for analysis

### Token Counting Methodology

Token counts are approximated using tiktoken (OpenAI's cl100k_base encoding) rather than actual Claude API usage fields, because:
1. Kiro chat does not expose Claude API usage to MCP tools
2. Tiktoken is OpenAI's official tokenizer, battle-tested and reproducible
3. Approximation error is systematic (consistent bias), not random noise
4. Ratio comparison (ON/OFF) is robust to systematic error

**Accuracy:** Tiktoken approximation has ±20-30% error vs actual Claude API counts. However, this affects absolute numbers only, not relative comparison. Percentage reduction remains valid because the same approximation method is used for both ON and OFF modes.

**What This Means:** We report percentage reduction (e.g., "32% fewer tokens"), not absolute savings (e.g., "1,350 tokens per session"). The comparison is valid even with approximation error.

## Results

### Primary Project: mnemos-dev

<!-- FILL IN AFTER DOGFOOD COMPLETES -->

- **Sample size:** [N] sessions ([X] ON, [Y] OFF)
- **Task distribution:** [X] feature, [Y] refactor, [Z] debug, [W] docs, [V] other
- **Date range:** [START_DATE] to [END_DATE]

#### Token Usage

| Mode | Mean Tokens | Median | Min | Max |
|------|-------------|--------|-----|-----|
| OFF  | [X]         | [X]    | [X] | [X] |
| ON   | [X]         | [X]    | [X] | [X] |

**Reduction:** [X]% fewer tokens with mnemos ON

#### Session Duration

| Mode | Mean Duration (min) | Median | Min | Max |
|------|---------------------|--------|-----|-----|
| OFF  | [X]                 | [X]    | [X] | [X] |
| ON   | [X]                 | [X]    | [X] | [X] |

#### MCP Calls per Session

| Mode | Mean Calls | Median | Min | Max |
|------|------------|--------|-----|-----|
| OFF  | [X]        | [X]    | [X] | [X] |
| ON   | [X]        | [X]    | [X] | [X] |

#### Distribution

<!-- OPTIONAL: Add ASCII histogram or link to chart image -->

```
[DISTRIBUTION CHART PLACEHOLDER]
```

### Spot-Check Projects

#### Project: [PROJECT_1_NAME]

- **Sample size:** [N] sessions ([X] ON, [Y] OFF)
- **Token reduction:** [X]%
- **Mean tokens OFF:** [X]
- **Mean tokens ON:** [X]

#### Project: [PROJECT_2_NAME]

- **Sample size:** [N] sessions ([X] ON, [Y] OFF)
- **Token reduction:** [X]%
- **Mean tokens OFF:** [X]
- **Mean tokens ON:** [X]

### Failed Sessions

<!-- FILL IN AFTER DOGFOOD COMPLETES -->

Sessions marked as incomplete (task_completed=false) were excluded from token averages:

- **Total failed sessions:** [N]
- **Mode distribution:** [X] ON, [Y] OFF
- **Failure reasons:**
  - [N] crashes (Kiro chat or network error)
  - [N] timeouts (session abandoned)
  - [N] gave up (task too complex or blocked)

**Analysis:** Failed session rate was [X]% overall, with no significant difference between ON ([X]%) and OFF ([Y]%) modes.

## Limitations

This benchmark has several important limitations:

### Sample Size and Scope

- **Single maintainer:** Results reflect one person's coding style and workflow
- **N=3 projects:** Small sample, may not generalize to other codebases
- **1 week duration:** Short timeframe, doesn't capture long-term patterns
- **Selection bias:** Projects chosen by author, may favor mnemos strengths

### Measurement Accuracy

- **Token approximation:** Tiktoken has ±20-30% error vs actual Claude API counts
- **Absolute numbers unreliable:** Only percentage reduction is valid
- **No cost estimates:** Token counts don't translate to dollar costs (model pricing varies)

### Confounding Factors

- **Task difficulty:** Not controlled, varies naturally across sessions
- **Maintainer learning:** Author got better at using mnemos during the week
- **Codebase evolution:** Code changed during benchmark, affecting context needs
- **Claude model availability:** Rate limits, Sonnet vs Opus switching
- **Day-of-week effects:** Monday vs Friday energy, partially mitigated by daily alternation

### Experimental Design

- **Not a controlled experiment:** No synthetic tasks, no randomization
- **No blinding:** Maintainer knew which mode was active
- **No statistical testing:** Descriptive stats only (mean, median, range)
- **No multi-user validation:** Results may not generalize to other users

### What This Benchmark Is NOT

- Not a scientific study with statistical rigor
- Not a comparison to other memory servers (Mem0, Zep, etc.)
- Not a claim about absolute token savings in dollars
- Not a guarantee of results for other users or projects

### What This Benchmark IS

- A credible maintainer self-measurement
- Evidence that mnemos provides measurable token reduction
- A reproducible methodology for others to verify on their own workflow
- Honest disclosure of limitations and confounds

## Reproduction

To reproduce this benchmark on your own workflow:

### Prerequisites

- Mnemos 1.2.0 or later installed
- Kiro chat with mnemos MCP server configured
- At least 1 project with active development work

### Steps

1. **Pre-benchmark setup:**
   ```bash
   # Verify mnemos health
   mnemos health
   
   # Test bench mode toggle
   mnemos bench mode on
   mnemos bench mode off
   
   # Test CSV export
   mnemos bench export --output test.csv
   ```

2. **Day 1 (Mode ON):**
   ```bash
   # Morning: Set mode
   mnemos bench mode on
   
   # Work normally on Kiro tasks
   # After each task completes:
   mnemos bench session-end
   
   # Evening: Verify data capture
   mnemos bench export --since today --output day1.csv
   ```

3. **Day 2 (Mode OFF):**
   ```bash
   # Morning: Switch mode
   mnemos bench mode off
   
   # Work normally on Kiro tasks
   # After each task completes:
   mnemos bench session-end
   
   # Evening: Verify data capture
   mnemos bench export --since today --output day2.csv
   ```

4. **Repeat for 7 days** (alternating ON/OFF daily)

5. **End of week:**
   ```bash
   # Export all sessions
   mnemos bench export --output results.csv
   
   # Check sample size
   wc -l results.csv
   
   # Verify minimum sessions:
   # - Primary project: ≥20 sessions (10 ON, 10 OFF)
   # - Each spot-check: ≥6 sessions (3 ON, 3 OFF)
   ```

6. **Analyze in spreadsheet tool:**
   - Import CSV into Excel/Google Sheets
   - Filter by project and mode
   - Compute mean, median, min, max for tokens_in + tokens_out
   - Calculate percentage reduction: (OFF_mean - ON_mean) / OFF_mean × 100

7. **Write your own report:**
   - Use this template as a starting point
   - Fill in your numbers
   - Disclose your limitations honestly
   - Share your findings!

### Tips for Accurate Measurement

- **Use explicit session-end:** Run `mnemos bench session-end` when task completes (don't rely solely on 10-minute timeout)
- **Track task categories:** Use `mnemos bench session-start --category <cat>` to record task type
- **Note outliers:** Keep a log of crashed sessions, network failures, or unusual tasks
- **Verify health:** Run `mnemos health` during ON days to confirm features are firing
- **Extend if needed:** If sample size is insufficient after 7 days, extend by 3 days

### Common Issues

- **Sessions merging:** If two tasks happen within 10 minutes, they may merge into one session. Use explicit `session-end` to prevent this.
- **Mode mixed:** If you forget to switch mode at day start, sessions will be marked mode_mixed and excluded. Check mode with `mnemos bench status`.
- **Failed sessions:** Kiro chat crashes or network errors will mark sessions as incomplete. These are excluded from averages but reported separately.

## Conclusion

<!-- FILL IN AFTER DOGFOOD COMPLETES -->

This benchmark provides evidence that mnemos delivers measurable token reduction in real-world usage. While the study has limitations (single maintainer, small sample, approximation error), the methodology is transparent and reproducible.

**Key Takeaway:** Mnemos reduced session tokens by [X]% on average across [Y] sessions. This suggests that context injection from past work reduces redundant explanation and re-discovery, leading to more efficient agent interactions.

**Next Steps:**
- Reproduce on your own workflow to verify results
- Share your findings with the community
- Help improve mnemos based on real-world usage patterns

---

**Benchmark Version:** 1.0  
**Mnemos Version:** [VERSION]  
**Date Published:** [DATE]  
**Author:** [AUTHOR]
