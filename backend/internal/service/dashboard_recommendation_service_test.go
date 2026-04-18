package service

import "testing"

func TestComputeDashboardCapacityPoolRecommendation_Action(t *testing.T) {
	input := dashboardRecommendationPoolInput{
		PoolKey:                    "openai:1-2",
		Platform:                   PlatformOpenAI,
		GroupNames:                 []string{"OpenAI Pro A", "OpenAI Pro B"},
		PlanNames:                  []string{"OpenAI Pro 30d", "OpenAI Team 30d"},
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

	got := computeDashboardCapacityPoolRecommendation(input, baseline, baseline, profile)

	if got.Status != "action" {
		t.Fatalf("status = %q, want action", got.Status)
	}
	if got.RecommendedAdditionalSchedulableAccounts < 2 {
		t.Fatalf("recommended additional schedulable accounts = %d, want >= 2", got.RecommendedAdditionalSchedulableAccounts)
	}
	if got.RecommendedAccountType != "oauth" {
		t.Fatalf("recommended account type = %q, want oauth", got.RecommendedAccountType)
	}
}

func TestComputeDashboardCapacityPoolRecommendation_SeedEmptySellablePool(t *testing.T) {
	input := dashboardRecommendationPoolInput{
		PoolKey:                "gemini:2",
		Platform:               PlatformGemini,
		GroupNames:             []string{"Gemini Starter"},
		PlanNames:              []string{"Gemini Starter 30d"},
		RecommendedAccountType: "",
		GrowthFactor:           1,
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

	got := computeDashboardCapacityPoolRecommendation(input, baseline, baseline, profile)

	if got.RecommendedSchedulableAccounts != 1 {
		t.Fatalf("recommended schedulable accounts = %d, want 1", got.RecommendedSchedulableAccounts)
	}
	if got.RecommendedAdditionalSchedulableAccounts != 1 {
		t.Fatalf("recommended additional schedulable accounts = %d, want 1", got.RecommendedAdditionalSchedulableAccounts)
	}
	if got.RecommendedAccountType != "schedulable" {
		t.Fatalf("recommended account type = %q, want schedulable", got.RecommendedAccountType)
	}
}

func TestComputeDashboardCapacityPoolRecommendation_Healthy(t *testing.T) {
	input := dashboardRecommendationPoolInput{
		PoolKey:                    "anthropic:3",
		Platform:                   PlatformAnthropic,
		GroupNames:                 []string{"Claude Lite"},
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

	got := computeDashboardCapacityPoolRecommendation(input, baseline, baseline, profile)

	if got.Status != "healthy" {
		t.Fatalf("status = %q, want healthy", got.Status)
	}
	if got.RecommendedAdditionalSchedulableAccounts != 0 {
		t.Fatalf("recommended additional schedulable accounts = %d, want 0", got.RecommendedAdditionalSchedulableAccounts)
	}
}

func TestComputeDashboardCapacityPoolRecommendation_PrioritizesRecoveryBeforeNewAccounts(t *testing.T) {
	input := dashboardRecommendationPoolInput{
		PoolKey:                    "openai:4",
		Platform:                   PlatformOpenAI,
		GroupNames:                 []string{"GPT-Standard"},
		PlanNames:                  []string{"GPT-Standard"},
		ActiveSubscriptions:        20,
		ActiveUsers30d:             18,
		AvgDailyCost30d:            36,
		AvgDailyCostPerActiveUser:  2,
		CurrentTotalAccounts:       5,
		CurrentSchedulableAccounts: 3,
		RecommendedAccountType:     "oauth",
		GrowthFactor:               1.1,
		ConcurrencyUtilization:     0.82,
		SessionsUtilization:        0.76,
		RPMUtilization:             0.7,
	}
	baseline := DashboardRecommendationBaseline{
		Platform:                          PlatformOpenAI,
		ActiveSubscriptionsPerSchedulable: 4,
		ActiveUsersPerSchedulable:         3,
		DailyCostPerSchedulable:           7,
		ActivationRate:                    0.8,
		AvgDailyCostPerActiveUser:         1.8,
	}
	profile := buildCapacityRecommendationPreferenceProfile(60)

	got := computeDashboardCapacityPoolRecommendation(input, baseline, baseline, profile)

	if got.RecommendedAdditionalSchedulableAccounts == 0 {
		t.Fatalf("recommended additional schedulable accounts = %d, want > 0", got.RecommendedAdditionalSchedulableAccounts)
	}
	if got.RecoverableUnschedulableAccounts == 0 {
		t.Fatalf("recoverable unschedulable accounts = %d, want > 0", got.RecoverableUnschedulableAccounts)
	}
	if got.NewAccountsRequired >= got.RecommendedAdditionalSchedulableAccounts {
		t.Fatalf("new accounts required = %d, want smaller than total gap %d", got.NewAccountsRequired, got.RecommendedAdditionalSchedulableAccounts)
	}
}

func TestComputeDashboardCapacityPoolRecommendation_SmallPoolConvergesAfterScaling(t *testing.T) {
	input := dashboardRecommendationPoolInput{
		PoolKey:                    "openai:5",
		Platform:                   PlatformOpenAI,
		GroupNames:                 []string{"GPT-Standard"},
		PlanNames:                  []string{"GPT-Standard"},
		ActiveSubscriptions:        2,
		ActiveUsers30d:             2,
		AvgDailyCost30d:            0.66,
		AvgDailyCostPerActiveUser:  0.33,
		CurrentTotalAccounts:       6,
		CurrentSchedulableAccounts: 6,
		RecommendedAccountType:     "oauth",
		GrowthFactor:               1.15,
		ConcurrencyUtilization:     0,
		SessionsUtilization:        0,
		RPMUtilization:             0,
	}
	lowBaseline := DashboardRecommendationBaseline{
		Platform:                          PlatformOpenAI,
		ActiveSubscriptionsPerSchedulable: 0.35,
		ActiveUsersPerSchedulable:         0.35,
		DailyCostPerSchedulable:           0.35,
		ActivationRate:                    1,
		AvgDailyCostPerActiveUser:         0.33,
	}
	profile := buildCapacityRecommendationPreferenceProfile(70)

	got := computeDashboardCapacityPoolRecommendation(input, lowBaseline, lowBaseline, profile)

	if got.Status != "watch" && got.Status != "healthy" {
		t.Fatalf("status = %q, want watch or healthy", got.Status)
	}
	if got.RecommendedAdditionalSchedulableAccounts != 0 {
		t.Fatalf("recommended additional schedulable accounts = %d, want 0", got.RecommendedAdditionalSchedulableAccounts)
	}
	if got.RecommendedSchedulableAccounts != input.CurrentSchedulableAccounts {
		t.Fatalf("recommended schedulable accounts = %d, want %d", got.RecommendedSchedulableAccounts, input.CurrentSchedulableAccounts)
	}
}
