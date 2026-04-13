package service

import "testing"

func TestComputeDashboardGroupCapacityRecommendation_Action(t *testing.T) {
	input := dashboardRecommendationInput{
		GroupID:                    1,
		GroupName:                  "OpenAI Pro",
		Platform:                   PlatformOpenAI,
		PlanNames:                  []string{"OpenAI Pro 30d"},
		ActiveSubscriptions:        36,
		ActiveUsers30d:             30,
		AvgDailyCost30d:            72,
		AvgDailyCostPerActiveUser:  2.4,
		CurrentTotalAccounts:       3,
		CurrentSchedulableAccounts: 2,
		RecommendedAccountType:     "oauth",
		GrowthFactor:               1.18,
		ConcurrencyUtilization:     0.88,
		SessionsUtilization:        0.74,
		RPMUtilization:             0.81,
	}
	baseline := DashboardRecommendationBaseline{
		Platform:                          PlatformOpenAI,
		ActiveSubscriptionsPerSchedulable: 10,
		ActiveUsersPerSchedulable:         7,
		DailyCostPerSchedulable:           18,
		ActivationRate:                    0.72,
		AvgDailyCostPerActiveUser:         1.8,
	}
	profile := buildCapacityRecommendationPreferenceProfile(70)

	got := computeDashboardGroupCapacityRecommendation(input, baseline, baseline, profile)

	if got.Status != "action" {
		t.Fatalf("status = %q, want action", got.Status)
	}
	if got.RecommendedAdditionalAccounts < 2 {
		t.Fatalf("recommended additional accounts = %d, want >= 2", got.RecommendedAdditionalAccounts)
	}
	if got.RecommendedAccountType != "oauth" {
		t.Fatalf("recommended account type = %q, want oauth", got.RecommendedAccountType)
	}
}

func TestComputeDashboardGroupCapacityRecommendation_SeedEmptySellableGroup(t *testing.T) {
	input := dashboardRecommendationInput{
		GroupID:                 2,
		GroupName:               "Gemini Starter",
		Platform:                PlatformGemini,
		PlanNames:               []string{"Gemini Starter 30d"},
		RecommendedAccountType:  "",
		GrowthFactor:            1,
	}
	baseline := DashboardRecommendationBaseline{
		Platform:                          PlatformGemini,
		ActiveSubscriptionsPerSchedulable: 8,
		ActiveUsersPerSchedulable:         5,
		DailyCostPerSchedulable:           10,
		ActivationRate:                    0.6,
		AvgDailyCostPerActiveUser:         1.2,
	}
	profile := buildCapacityRecommendationPreferenceProfile(50)

	got := computeDashboardGroupCapacityRecommendation(input, baseline, baseline, profile)

	if got.RecommendedTotalAccounts != 1 {
		t.Fatalf("recommended total accounts = %d, want 1", got.RecommendedTotalAccounts)
	}
	if got.RecommendedAdditionalAccounts != 1 {
		t.Fatalf("recommended additional accounts = %d, want 1", got.RecommendedAdditionalAccounts)
	}
	if got.RecommendedAccountType != "schedulable" {
		t.Fatalf("recommended account type = %q, want schedulable", got.RecommendedAccountType)
	}
}

func TestComputeDashboardGroupCapacityRecommendation_Healthy(t *testing.T) {
	input := dashboardRecommendationInput{
		GroupID:                    3,
		GroupName:                  "Claude Lite",
		Platform:                   PlatformAnthropic,
		PlanNames:                  []string{"Claude Lite 30d"},
		ActiveSubscriptions:        9,
		ActiveUsers30d:             6,
		AvgDailyCost30d:            9,
		AvgDailyCostPerActiveUser:  1.5,
		CurrentTotalAccounts:       3,
		CurrentSchedulableAccounts: 3,
		RecommendedAccountType:     "cookie",
		GrowthFactor:               1.02,
		ConcurrencyUtilization:     0.41,
		SessionsUtilization:        0.38,
		RPMUtilization:             0.27,
	}
	baseline := DashboardRecommendationBaseline{
		Platform:                          PlatformAnthropic,
		ActiveSubscriptionsPerSchedulable: 5,
		ActiveUsersPerSchedulable:         3.5,
		DailyCostPerSchedulable:           6,
		ActivationRate:                    0.65,
		AvgDailyCostPerActiveUser:         1.4,
	}
	profile := buildCapacityRecommendationPreferenceProfile(35)

	got := computeDashboardGroupCapacityRecommendation(input, baseline, baseline, profile)

	if got.Status != "healthy" {
		t.Fatalf("status = %q, want healthy", got.Status)
	}
	if got.RecommendedAdditionalAccounts != 0 {
		t.Fatalf("recommended additional accounts = %d, want 0", got.RecommendedAdditionalAccounts)
	}
}
