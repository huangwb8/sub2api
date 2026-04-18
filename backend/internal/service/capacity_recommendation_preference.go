package service

import (
	"strconv"
	"strings"
)

const (
	defaultSubscriptionCapacityTightness = 50
	minSubscriptionCapacityTightness     = 0
	maxSubscriptionCapacityTightness     = 100
)

type capacityRecommendationPreferenceProfile struct {
	Score                         int
	BaselineScale                 float64
	WatchUtilizationThreshold     float64
	ActionUtilizationThreshold    float64
	EmergencyUtilizationThreshold float64
	EmailTargetMultiplier         float64
}

func parseSubscriptionCapacityTightness(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return defaultSubscriptionCapacityTightness
	}
	return clampInt(value, minSubscriptionCapacityTightness, maxSubscriptionCapacityTightness)
}

func buildCapacityRecommendationPreferenceProfile(score int) capacityRecommendationPreferenceProfile {
	normalized := float64(clampInt(score, minSubscriptionCapacityTightness, maxSubscriptionCapacityTightness)) / 100
	return capacityRecommendationPreferenceProfile{
		Score:                         clampInt(score, minSubscriptionCapacityTightness, maxSubscriptionCapacityTightness),
		BaselineScale:                 lerpFloat(1.2, 0.9, normalized),
		WatchUtilizationThreshold:     lerpFloat(0.76, 0.60, normalized),
		ActionUtilizationThreshold:    lerpFloat(0.92, 0.72, normalized),
		EmergencyUtilizationThreshold: lerpFloat(0.99, 0.87, normalized),
		EmailTargetMultiplier:         lerpFloat(1.25, 0.75, normalized),
	}
}

func lerpFloat(start, end, t float64) float64 {
	return start + (end-start)*t
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
