//go:build unit

package service

import "testing"

func TestBuildCapacityRecommendationPreferenceProfile_Monotonic(t *testing.T) {
	low := buildCapacityRecommendationPreferenceProfile(0)
	high := buildCapacityRecommendationPreferenceProfile(100)

	if high.Score != 100 {
		t.Fatalf("high score = %d, want 100", high.Score)
	}
	if low.Score != 0 {
		t.Fatalf("low score = %d, want 0", low.Score)
	}
	if !(high.BaselineScale < low.BaselineScale) {
		t.Fatalf("baseline scale should shrink as tightness increases: low=%f high=%f", low.BaselineScale, high.BaselineScale)
	}
	if !(high.WatchUtilizationThreshold < low.WatchUtilizationThreshold) {
		t.Fatalf("watch threshold should decrease as tightness increases: low=%f high=%f", low.WatchUtilizationThreshold, high.WatchUtilizationThreshold)
	}
	if !(high.ActionUtilizationThreshold < low.ActionUtilizationThreshold) {
		t.Fatalf("action threshold should decrease as tightness increases: low=%f high=%f", low.ActionUtilizationThreshold, high.ActionUtilizationThreshold)
	}
	if !(high.EmergencyUtilizationThreshold < low.EmergencyUtilizationThreshold) {
		t.Fatalf("emergency threshold should decrease as tightness increases: low=%f high=%f", low.EmergencyUtilizationThreshold, high.EmergencyUtilizationThreshold)
	}
	if !(high.EmailTargetMultiplier < low.EmailTargetMultiplier) {
		t.Fatalf("email target multiplier should decrease as tightness increases: low=%f high=%f", low.EmailTargetMultiplier, high.EmailTargetMultiplier)
	}
}
