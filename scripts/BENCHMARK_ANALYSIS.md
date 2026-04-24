# Benchmark Analysis Scripts

Helper scripts to analyze benchmark data exported from `mnemos bench export`.

## Scripts

### analyze_benchmark.py (Recommended)

Full-featured Python script that computes statistics and generates markdown output.

**Features:**
- Mean, median, min, max for ON vs OFF modes
- Percentage reduction calculation
- ASCII histogram distribution charts
- Per-project breakdown
- Markdown snippet ready for benchmark report

**Usage:**
```bash
# Export benchmark data
mnemos bench export --output benchmark.csv

# Analyze
python3 scripts/analyze_benchmark.py benchmark.csv
```

**Requirements:** Python 3.6+

### analyze_benchmark.sh

Lightweight shell script for quick statistics without Python dependency.

**Features:**
- Mean, median, min, max for ON vs OFF modes
- Percentage reduction calculation
- No external dependencies (uses awk, sort)

**Usage:**
```bash
# Export benchmark data
mnemos bench export --output benchmark.csv

# Analyze
bash scripts/analyze_benchmark.sh benchmark.csv
```

**Requirements:** bash, awk, sort (standard Unix tools)

## Output Format

Both scripts output:
1. Summary statistics (total sessions, counts by mode)
2. Mode OFF statistics (mean, median, min, max)
3. Mode ON statistics (mean, median, min, max)
4. Percentage reduction

The Python script additionally provides:
- Markdown-formatted tables
- ASCII histograms showing token distribution
- Per-project breakdown (if multiple projects)

## Example

```bash
$ mnemos bench export --since 2026-04-25 --output week1.csv
Exported 26 sessions to week1.csv

$ python3 scripts/analyze_benchmark.py week1.csv
============================================================
BENCHMARK ANALYSIS
============================================================

Total Sessions: 26 (13 ON, 13 OFF)

Mode OFF: mean=6,450 median=6,200 min=4,100 max=9,800
Mode ON:  mean=4,180 median=4,050 min=2,900 max=6,200

Reduction: 35.2%

============================================================
MARKDOWN SNIPPET (copy to report)
============================================================

## Benchmark Results

**Total Sessions:** 26 (13 ON, 13 OFF)

**Overall Reduction:** 35.2%

[... markdown tables and histograms ...]
```

## Integration with Benchmark Report

The markdown output from `analyze_benchmark.py` can be copied directly into `docs/benchmarks/real-world.md` in the Results section.

## Notes

- Both scripts filter to only completed sessions (`task_completed=true`)
- Mixed-mode sessions should be excluded via `mnemos bench export` (default behavior)
- Token counts are total tokens (tokens_in + tokens_out) per session
- Percentage reduction formula: `(OFF_mean - ON_mean) / OFF_mean × 100`
