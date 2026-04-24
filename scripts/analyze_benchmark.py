#!/usr/bin/env python3
"""
Benchmark Analysis Script

Reads CSV from `mnemos bench export` and computes:
- Mean, median, min, max for ON vs OFF
- Percentage reduction
- ASCII histogram distribution
- Markdown snippet for report
"""

import csv
import sys
import statistics
from collections import defaultdict
from typing import List, Dict, Tuple


def read_csv(filepath: str) -> List[Dict]:
    """Read CSV file and return list of session records."""
    sessions = []
    with open(filepath, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            # Convert numeric fields
            row['duration_ms'] = int(row['duration_ms'])
            row['tokens_in'] = int(row['tokens_in'])
            row['tokens_out'] = int(row['tokens_out'])
            row['mcp_calls_count'] = int(row['mcp_calls_count'])
            row['task_completed'] = row['task_completed'].lower() == 'true'
            sessions.append(row)
    return sessions


def filter_sessions(sessions: List[Dict]) -> List[Dict]:
    """Filter to only completed sessions."""
    return [s for s in sessions if s['task_completed']]


def compute_stats(values: List[int]) -> Dict:
    """Compute mean, median, min, max for a list of values."""
    if not values:
        return {'mean': 0, 'median': 0, 'min': 0, 'max': 0, 'count': 0}
    
    return {
        'mean': int(statistics.mean(values)),
        'median': int(statistics.median(values)),
        'min': min(values),
        'max': max(values),
        'count': len(values)
    }


def compute_total_tokens(session: Dict) -> int:
    """Compute total tokens (in + out) for a session."""
    return session['tokens_in'] + session['tokens_out']


def generate_histogram(values: List[int], bins: int = 10, width: int = 40) -> str:
    """Generate ASCII histogram."""
    if not values:
        return "No data"
    
    min_val = min(values)
    max_val = max(values)
    bin_width = (max_val - min_val) / bins if max_val > min_val else 1
    
    # Create bins
    bin_counts = [0] * bins
    for val in values:
        bin_idx = min(int((val - min_val) / bin_width), bins - 1)
        bin_counts[bin_idx] += 1
    
    # Generate histogram
    max_count = max(bin_counts) if bin_counts else 1
    lines = []
    
    for i, count in enumerate(bin_counts):
        bin_start = int(min_val + i * bin_width)
        bin_end = int(min_val + (i + 1) * bin_width)
        bar_len = int((count / max_count) * width) if max_count > 0 else 0
        bar = '█' * bar_len
        lines.append(f"{bin_start:5d}-{bin_end:5d} | {bar} ({count})")
    
    return '\n'.join(lines)


def analyze_by_mode(sessions: List[Dict]) -> Tuple[Dict, Dict]:
    """Split sessions by mode and compute stats."""
    on_sessions = [s for s in sessions if s['mode'] == 'on']
    off_sessions = [s for s in sessions if s['mode'] == 'off']
    
    on_tokens = [compute_total_tokens(s) for s in on_sessions]
    off_tokens = [compute_total_tokens(s) for s in off_sessions]
    
    on_stats = compute_stats(on_tokens)
    off_stats = compute_stats(off_tokens)
    
    return on_stats, off_stats


def compute_reduction(on_mean: int, off_mean: int) -> float:
    """Compute percentage reduction."""
    if off_mean == 0:
        return 0.0
    return ((off_mean - on_mean) / off_mean) * 100


def analyze_by_project(sessions: List[Dict]) -> Dict[str, Tuple[Dict, Dict]]:
    """Group sessions by project and analyze each."""
    by_project = defaultdict(list)
    for s in sessions:
        by_project[s['project_id']].append(s)
    
    results = {}
    for project_id, project_sessions in by_project.items():
        on_stats, off_stats = analyze_by_mode(project_sessions)
        results[project_id] = (on_stats, off_stats)
    
    return results


def generate_markdown(sessions: List[Dict]) -> str:
    """Generate markdown snippet for benchmark report."""
    md = []
    
    # Overall stats
    on_stats, off_stats = analyze_by_mode(sessions)
    reduction = compute_reduction(on_stats['mean'], off_stats['mean'])
    
    md.append("## Benchmark Results\n")
    md.append(f"**Total Sessions:** {len(sessions)} ({on_stats['count']} ON, {off_stats['count']} OFF)\n")
    md.append(f"**Overall Reduction:** {reduction:.1f}%\n")
    
    # Summary table
    md.append("\n### Overall Statistics\n")
    md.append("| Mode | Mean Tokens | Median | Min | Max | Sessions |")
    md.append("|------|-------------|--------|-----|-----|----------|")
    md.append(f"| OFF  | {off_stats['mean']:,} | {off_stats['median']:,} | {off_stats['min']:,} | {off_stats['max']:,} | {off_stats['count']} |")
    md.append(f"| ON   | {on_stats['mean']:,} | {on_stats['median']:,} | {on_stats['min']:,} | {on_stats['max']:,} | {on_stats['count']} |")
    
    # Per-project breakdown
    by_project = analyze_by_project(sessions)
    if len(by_project) > 1:
        md.append("\n### Per-Project Results\n")
        for project_id, (on_stats, off_stats) in sorted(by_project.items()):
            reduction = compute_reduction(on_stats['mean'], off_stats['mean'])
            md.append(f"\n**{project_id}:** {reduction:.1f}% reduction ({on_stats['count']} ON, {off_stats['count']} OFF sessions)")
    
    # Distribution
    md.append("\n### Token Distribution\n")
    md.append("\n**Mode OFF:**")
    md.append("```")
    off_tokens = [compute_total_tokens(s) for s in sessions if s['mode'] == 'off']
    md.append(generate_histogram(off_tokens))
    md.append("```")
    
    md.append("\n**Mode ON:**")
    md.append("```")
    on_tokens = [compute_total_tokens(s) for s in sessions if s['mode'] == 'on']
    md.append(generate_histogram(on_tokens))
    md.append("```")
    
    return '\n'.join(md)


def main():
    if len(sys.argv) < 2:
        print("Benchmark Analysis Script")
        print("=" * 60)
        print("\nReads CSV from 'mnemos bench export' and computes:")
        print("  - Mean, median, min, max for ON vs OFF")
        print("  - Percentage reduction")
        print("  - ASCII histogram distribution")
        print("  - Markdown snippet for report")
        print("\nUsage: analyze_benchmark.py <csv_file>")
        print("\nExample:")
        print("  mnemos bench export --output benchmark.csv")
        print("  python3 scripts/analyze_benchmark.py benchmark.csv")
        print("\nFor quick stats without histograms:")
        print("  bash scripts/analyze_benchmark.sh benchmark.csv")
        sys.exit(1)
    
    csv_file = sys.argv[1]
    
    try:
        # Read and filter sessions
        sessions = read_csv(csv_file)
        sessions = filter_sessions(sessions)
        
        if not sessions:
            print("No completed sessions found in CSV.")
            sys.exit(1)
        
        # Compute stats
        on_stats, off_stats = analyze_by_mode(sessions)
        reduction = compute_reduction(on_stats['mean'], off_stats['mean'])
        
        # Print summary
        print("=" * 60)
        print("BENCHMARK ANALYSIS")
        print("=" * 60)
        print(f"\nTotal Sessions: {len(sessions)} ({on_stats['count']} ON, {off_stats['count']} OFF)")
        print(f"\nMode OFF: mean={off_stats['mean']:,} median={off_stats['median']:,} min={off_stats['min']:,} max={off_stats['max']:,}")
        print(f"Mode ON:  mean={on_stats['mean']:,} median={on_stats['median']:,} min={on_stats['min']:,} max={on_stats['max']:,}")
        print(f"\nReduction: {reduction:.1f}%")
        
        # Generate markdown
        print("\n" + "=" * 60)
        print("MARKDOWN SNIPPET (copy to report)")
        print("=" * 60)
        print()
        print(generate_markdown(sessions))
        
    except FileNotFoundError:
        print(f"Error: File '{csv_file}' not found.")
        sys.exit(1)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)


if __name__ == '__main__':
    main()
