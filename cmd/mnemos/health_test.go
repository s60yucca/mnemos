package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/observe"
)

func TestParseLog_ValidLines(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	// Write test log
	content := `2026-04-15T14:32:10Z	quality_gate	score=0.85 action=accept project=myproject
2026-04-15T14:32:11Z	dedup	tier=fuzzy hit=true similarity=0.87 project=myproject
2026-04-15T14:32:12Z	mmr	candidates=20 selected=8 lambda=0.7
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse log
	since := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	events, err := parseLog(logPath, since, "")
	if err != nil {
		t.Fatalf("parseLog failed: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Verify first event
	if events[0].Feature != "quality_gate" {
		t.Errorf("expected feature 'quality_gate', got %q", events[0].Feature)
	}
	if events[0].Attrs["score"] != "0.85" {
		t.Errorf("expected score '0.85', got %q", events[0].Attrs["score"])
	}
	if events[0].Attrs["action"] != "accept" {
		t.Errorf("expected action 'accept', got %q", events[0].Attrs["action"])
	}
	if events[0].Attrs["project"] != "myproject" {
		t.Errorf("expected project 'myproject', got %q", events[0].Attrs["project"])
	}
}

func TestParseLog_MalformedLines(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	// Write test log with malformed lines
	content := `2026-04-15T14:32:10Z	quality_gate	score=0.85
invalid line without tabs
2026-04-15T14:32:11Z	dedup
2026-04-15T14:32:12Z	mmr	candidates=20
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse log (should skip malformed lines)
	since := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	events, err := parseLog(logPath, since, "")
	if err != nil {
		t.Fatalf("parseLog failed: %v", err)
	}

	// Should only parse the 2 valid lines
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestParseLog_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	// Create empty file
	if err := os.WriteFile(logPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	since := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	events, err := parseLog(logPath, since, "")
	if err != nil {
		t.Fatalf("parseLog failed: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestParseLog_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "nonexistent.log")

	since := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	events, err := parseLog(logPath, since, "")
	if err != nil {
		t.Fatalf("parseLog should not error on missing file, got: %v", err)
	}

	if events != nil {
		t.Fatalf("expected nil events for missing file, got %d events", len(events))
	}
}

func TestParseLog_TimeWindowFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	// Write test log with events across multiple days
	content := `2026-04-10T14:32:10Z	feature1	id=1
2026-04-15T14:32:11Z	feature2	id=2
2026-04-20T14:32:12Z	feature3	id=3
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse with time window starting at 2026-04-15
	since := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	events, err := parseLog(logPath, since, "")
	if err != nil {
		t.Fatalf("parseLog failed: %v", err)
	}

	// Should only get events from 2026-04-15 onwards
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Attrs["id"] != "2" {
		t.Errorf("expected first event id=2, got %q", events[0].Attrs["id"])
	}
	if events[1].Attrs["id"] != "3" {
		t.Errorf("expected second event id=3, got %q", events[1].Attrs["id"])
	}
}

func TestParseLog_ProjectFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	// Write test log with multiple projects
	content := `2026-04-15T14:32:10Z	feature1	project=proj1 id=1
2026-04-15T14:32:11Z	feature2	project=proj2 id=2
2026-04-15T14:32:12Z	feature3	project=proj1 id=3
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse with project filter
	since := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	events, err := parseLog(logPath, since, "proj1")
	if err != nil {
		t.Fatalf("parseLog failed: %v", err)
	}

	// Should only get events for proj1
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	for _, event := range events {
		if event.Attrs["project"] != "proj1" {
			t.Errorf("expected project=proj1, got %q", event.Attrs["project"])
		}
	}
}

func TestParseLog_InvalidTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	// Write test log with invalid timestamp
	content := `invalid-timestamp	feature1	id=1
2026-04-15T14:32:11Z	feature2	id=2
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse log (should skip line with invalid timestamp)
	since := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	events, err := parseLog(logPath, since, "")
	if err != nil {
		t.Fatalf("parseLog failed: %v", err)
	}

	// Should only parse the valid line
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestParseLog_AttributesParsing(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	// Write test log with various attribute formats
	content := `2026-04-15T14:32:10Z	feature1	key1=value1 key2=value2 key3=value3
2026-04-15T14:32:11Z	feature2	single=value
2026-04-15T14:32:12Z	feature3	
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	since := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	events, err := parseLog(logPath, since, "")
	if err != nil {
		t.Fatalf("parseLog failed: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Verify first event has all attributes
	if len(events[0].Attrs) != 3 {
		t.Errorf("expected 3 attributes, got %d", len(events[0].Attrs))
	}

	// Verify second event has one attribute
	if len(events[1].Attrs) != 1 {
		t.Errorf("expected 1 attribute, got %d", len(events[1].Attrs))
	}

	// Verify third event has no attributes
	if len(events[2].Attrs) != 0 {
		t.Errorf("expected 0 attributes, got %d", len(events[2].Attrs))
	}
}

func TestClassifyFeature_FiringNormally(t *testing.T) {
	// Create events for a ratio-based feature (quality_gate)
	baseline := observe.Baseline{
		MinDaily:        0,
		RatioVsMCPCalls: 0.95,
		Expected:        "Every store operation",
	}

	// Create 7 active days with sufficient events
	var events []Event
	baseTime := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	for day := 0; day < 7; day++ {
		dayTime := baseTime.Add(time.Duration(day) * 24 * time.Hour)
		// Add 20 MCP calls per day
		for i := 0; i < 20; i++ {
			events = append(events, Event{
				Timestamp: dayTime.Add(time.Duration(i) * time.Hour),
				Feature:   "store_call",
				Attrs:     map[string]string{"project": "test"},
			})
		}
		// Add 19 quality_gate events (95% ratio)
		for i := 0; i < 19; i++ {
			events = append(events, Event{
				Timestamp: dayTime.Add(time.Duration(i) * time.Hour),
				Feature:   "quality_gate",
				Attrs:     map[string]string{"action": "accept"},
			})
		}
	}

	activeDays := detectActiveDays(events)

	// Filter events to only quality_gate
	var qgEvents []Event
	for _, e := range events {
		if e.Feature == "quality_gate" {
			qgEvents = append(qgEvents, e)
		}
	}

	status := classifyFeature(qgEvents, events, baseline, activeDays)
	if status != StatusFiring {
		t.Errorf("expected StatusFiring, got %v", status)
	}
}

func TestClassifyFeature_LowActivity(t *testing.T) {
	// Create events for a ratio-based feature with low activity
	baseline := observe.Baseline{
		MinDaily:        0,
		RatioVsMCPCalls: 0.95,
		Expected:        "Every store operation",
	}

	var events []Event
	baseTime := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	for day := 0; day < 7; day++ {
		dayTime := baseTime.Add(time.Duration(day) * 24 * time.Hour)
		// Add 20 MCP calls per day
		for i := 0; i < 20; i++ {
			events = append(events, Event{
				Timestamp: dayTime.Add(time.Duration(i) * time.Hour),
				Feature:   "store_call",
				Attrs:     map[string]string{"project": "test"},
			})
		}
		// Add only 10 quality_gate events (50% ratio - below threshold)
		for i := 0; i < 10; i++ {
			events = append(events, Event{
				Timestamp: dayTime.Add(time.Duration(i) * time.Hour),
				Feature:   "quality_gate",
				Attrs:     map[string]string{"action": "accept"},
			})
		}
	}

	activeDays := detectActiveDays(events)

	var qgEvents []Event
	for _, e := range events {
		if e.Feature == "quality_gate" {
			qgEvents = append(qgEvents, e)
		}
	}

	status := classifyFeature(qgEvents, events, baseline, activeDays)
	if status != StatusLow {
		t.Errorf("expected StatusLow, got %v", status)
	}
}

func TestClassifyFeature_SparseWindowIsUnknown(t *testing.T) {
	now := time.Now().UTC()
	events := []Event{
		{Timestamp: now, Feature: "store_call"},
		{Timestamp: now, Feature: "quality_gate"},
	}
	baseline := observe.Baseline{RatioVsMCPCalls: 0.95}

	status := classifyFeatureNamed("quality_gate", events[1:], events, baseline, detectActiveDays(events))
	if status != StatusUnknown {
		t.Errorf("expected StatusUnknown for insufficient activity, got %v", status)
	}

	missing := classifyFeatureNamed("dedup", nil, events, baseline, detectActiveDays(events))
	if missing != StatusUnknown {
		t.Errorf("expected missing feature to remain unknown without an active day, got %v", missing)
	}
}

func TestClassifyFeature_ObservedBelowBaselineIsLowActivity(t *testing.T) {
	// Create events with 3 consecutive days of no activity, but with the feature
	// still observed in the window. This should be LOW ACTIVITY, not not observed.
	baseline := observe.Baseline{
		MinDaily:        2,
		RatioVsMCPCalls: 0.0,
		Expected:        "Daily feature",
	}

	var events []Event
	baseTime := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)

	// Create 7 active days (with MCP calls)
	for day := 0; day < 7; day++ {
		dayTime := baseTime.Add(time.Duration(day) * 24 * time.Hour)
		for i := 0; i < 20; i++ {
			events = append(events, Event{
				Timestamp: dayTime.Add(time.Duration(i) * time.Hour),
				Feature:   "store_call",
				Attrs:     map[string]string{"project": "test"},
			})
		}
	}

	// Add feature events only on days 0 and 6 (days 1-5 have zero - 5 consecutive)
	for _, day := range []int{0, 6} {
		dayTime := baseTime.Add(time.Duration(day) * 24 * time.Hour)
		for i := 0; i < 3; i++ {
			events = append(events, Event{
				Timestamp: dayTime.Add(time.Duration(i) * time.Hour),
				Feature:   "test_feature",
				Attrs:     map[string]string{},
			})
		}
	}

	activeDays := detectActiveDays(events)

	var featureEvents []Event
	for _, e := range events {
		if e.Feature == "test_feature" {
			featureEvents = append(featureEvents, e)
		}
	}

	status := classifyFeature(featureEvents, events, baseline, activeDays)
	if status != StatusLow {
		t.Errorf("expected StatusLow, got %v", status)
	}
}

func TestClassifyFeature_NotObserved(t *testing.T) {
	baseline := observe.Baseline{
		MinDaily:        2,
		RatioVsMCPCalls: 0.0,
		Expected:        "Daily feature",
	}

	var events []Event
	baseTime := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		events = append(events, Event{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Feature:   "store_call",
			Attrs:     map[string]string{"project": "test"},
		})
	}

	status := classifyFeature(nil, events, baseline, detectActiveDays(events))
	if status != StatusNotFiring {
		t.Errorf("expected StatusNotFiring, got %v", status)
	}
	if status.String() != "NOT OBSERVED" {
		t.Errorf("expected NOT OBSERVED label, got %q", status.String())
	}
}

func TestRenderReport_LowActivityShowsLastSeen(t *testing.T) {
	lastSeen := time.Date(2026, 4, 17, 14, 30, 0, 0, time.UTC)
	events := []Event{
		{Timestamp: lastSeen, Feature: "quality_gate", Attrs: map[string]string{"action": "accept"}},
	}
	classifications := map[string]Status{
		"quality_gate": StatusLow,
		"compile":      StatusNotFiring,
	}

	out := renderReport(classifications, events, 7)
	if !strings.Contains(out, "LOW ACTIVITY (1)") {
		t.Fatalf("expected low activity section, got:\n%s", out)
	}
	if !strings.Contains(out, "quality_gate") || !strings.Contains(out, "Last seen:") {
		t.Fatalf("expected low activity feature with last seen, got:\n%s", out)
	}
	if strings.Contains(out, "NOT FIRING") {
		t.Fatalf("did not expect old NOT FIRING label, got:\n%s", out)
	}
	if !strings.Contains(out, "NOT OBSERVED (1)") {
		t.Fatalf("expected not observed section, got:\n%s", out)
	}
}

func TestDetectActiveDays(t *testing.T) {
	var events []Event
	baseTime := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)

	// Day 0: 20 MCP calls (active)
	for i := 0; i < 20; i++ {
		events = append(events, Event{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Feature:   "store_call",
			Attrs:     map[string]string{},
		})
	}

	// Day 1: 5 MCP calls (not active, below threshold of 10)
	for i := 0; i < 5; i++ {
		events = append(events, Event{
			Timestamp: baseTime.Add(24*time.Hour + time.Duration(i)*time.Hour),
			Feature:   "context_call",
			Attrs:     map[string]string{},
		})
	}

	// Day 2: 15 MCP calls (active)
	for i := 0; i < 15; i++ {
		events = append(events, Event{
			Timestamp: baseTime.Add(48*time.Hour + time.Duration(i)*time.Hour),
			Feature:   "search_call",
			Attrs:     map[string]string{},
		})
	}

	activeDays := detectActiveDays(events)

	// Should have 2 active days (day 0 and day 2)
	if len(activeDays) != 2 {
		t.Errorf("expected 2 active days, got %d", len(activeDays))
	}
}

func TestClassifyFeature_MinDailyBaseline(t *testing.T) {
	// Test MinDaily-based classification
	baseline := observe.Baseline{
		MinDaily:        5,
		RatioVsMCPCalls: 0.0,
		Expected:        "At least 5 events per day",
	}

	var events []Event
	baseTime := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)

	// Create 7 active days
	for day := 0; day < 7; day++ {
		dayTime := baseTime.Add(time.Duration(day) * 24 * time.Hour)
		// Add MCP calls to make it an active day
		for i := 0; i < 20; i++ {
			events = append(events, Event{
				Timestamp: dayTime.Add(time.Duration(i) * time.Hour),
				Feature:   "store_call",
				Attrs:     map[string]string{},
			})
		}
		// Add feature events: 6 per day (above threshold)
		for i := 0; i < 6; i++ {
			events = append(events, Event{
				Timestamp: dayTime.Add(time.Duration(i) * time.Hour),
				Feature:   "test_feature",
				Attrs:     map[string]string{},
			})
		}
	}

	activeDays := detectActiveDays(events)

	var featureEvents []Event
	for _, e := range events {
		if e.Feature == "test_feature" {
			featureEvents = append(featureEvents, e)
		}
	}

	status := classifyFeature(featureEvents, events, baseline, activeDays)
	if status != StatusFiring {
		t.Errorf("expected StatusFiring for MinDaily baseline, got %v", status)
	}
}

func TestClassifyFeature_DecayRecentEventIsHealthy(t *testing.T) {
	now := time.Now().UTC()
	var events []Event
	for day := 0; day < 7; day++ {
		dayTime := now.Add(time.Duration(-day) * 24 * time.Hour)
		for i := 0; i < observe.ActiveDayThreshold; i++ {
			events = append(events, Event{
				Timestamp: dayTime.Add(time.Duration(i) * time.Minute),
				Feature:   "store_call",
				Attrs:     map[string]string{},
			})
		}
	}
	decayEvents := []Event{{
		Timestamp: now.Add(-time.Hour),
		Feature:   "decay",
		Attrs:     map[string]string{"outcome": "ok"},
	}}
	events = append(events, decayEvents...)

	status := classifyFeatureNamed("decay", decayEvents, events, observe.Baselines["decay"], detectActiveDays(events))
	if status != StatusFiring {
		t.Errorf("expected recent decay event to be firing, got %v", status)
	}
}
