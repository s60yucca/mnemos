package observe

import (
	"testing"
)

func TestBaselines_AllFeaturesHaveBaselines(t *testing.T) {
	expectedFeatures := []string{
		"quality_gate",
		"dedup",
		"summarize",
		"file_link",
		"mmr",
		"autopilot",
		"compile",
		"decay",
		"topic_shift",
	}

	for _, feature := range expectedFeatures {
		if _, ok := Baselines[feature]; !ok {
			t.Errorf("missing baseline for feature: %s", feature)
		}
	}

	if len(Baselines) != len(expectedFeatures) {
		t.Errorf("expected %d baselines, got %d", len(expectedFeatures), len(Baselines))
	}
}

func TestBaselines_EachFeatureHasAtLeastOneSignal(t *testing.T) {
	for feature, baseline := range Baselines {
		if baseline.MinDaily == 0 && baseline.RatioVsMCPCalls == 0 {
			t.Errorf("feature %s has no signal: both MinDaily and RatioVsMCPCalls are 0", feature)
		}
	}
}

func TestBaselines_RatioInValidRange(t *testing.T) {
	for feature, baseline := range Baselines {
		if baseline.RatioVsMCPCalls < 0 || baseline.RatioVsMCPCalls > 1 {
			t.Errorf("feature %s has invalid RatioVsMCPCalls: %f (must be in [0, 1])", feature, baseline.RatioVsMCPCalls)
		}
	}
}

func TestBaselines_ActiveDayThresholdPositive(t *testing.T) {
	if ActiveDayThreshold <= 0 {
		t.Errorf("ActiveDayThreshold must be > 0, got %d", ActiveDayThreshold)
	}
}

func TestBaselines_ExpectedFieldNonEmpty(t *testing.T) {
	for feature, baseline := range Baselines {
		if baseline.Expected == "" {
			t.Errorf("feature %s has empty Expected field", feature)
		}
	}
}

func TestBaselines_MinDailyNonNegative(t *testing.T) {
	for feature, baseline := range Baselines {
		if baseline.MinDaily < 0 {
			t.Errorf("feature %s has negative MinDaily: %d", feature, baseline.MinDaily)
		}
	}
}
