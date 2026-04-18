package service

import "testing"

func TestComputePoolBaseline_IgnoreIdleGroupsForCostBaseline(t *testing.T) {
	inputs := []dashboardRecommendationPoolInput{
		{
			PoolKey:                    "openai:1",
			Platform:                   PlatformOpenAI,
			ActiveSubscriptions:        10,
			ActiveUsers30d:             8,
			AvgDailyCost30d:            20,
			AvgDailyCostPerActiveUser:  2.5,
			CurrentSchedulableAccounts: 2,
		},
		{
			PoolKey:                    "openai:2",
			Platform:                   PlatformOpenAI,
			ActiveSubscriptions:        0,
			ActiveUsers30d:             0,
			AvgDailyCost30d:            0,
			AvgDailyCostPerActiveUser:  0,
			CurrentSchedulableAccounts: 10,
		},
	}

	baselines := computeDashboardPoolBaselines(inputs)

	got := baselines[PlatformOpenAI].DailyCostPerSchedulable
	want := 10.0
	if got != want {
		t.Fatalf("daily cost per schedulable = %f, want %f", got, want)
	}
}

func TestBuildCapacityPools_SharedAccountsMergeIntoOnePool(t *testing.T) {
	groups := []dashboardRecommendationGroupSnapshot{
		{GroupID: 1, GroupName: "OpenAI Pro A", Platform: PlatformOpenAI, PlanNames: []string{"OpenAI Pro 30d"}},
		{GroupID: 2, GroupName: "OpenAI Pro B", Platform: PlatformOpenAI, PlanNames: []string{"OpenAI Team 30d"}},
	}
	memberships := []dashboardRecommendationPoolMembership{
		{GroupID: 1, AccountID: 101, AccountType: AccountTypeOAuth, Schedulable: true},
		{GroupID: 1, AccountID: 102, AccountType: AccountTypeOAuth, Schedulable: true},
		{GroupID: 2, AccountID: 102, AccountType: AccountTypeOAuth, Schedulable: true},
		{GroupID: 2, AccountID: 103, AccountType: AccountTypeAPIKey, Schedulable: false},
	}

	pools := buildDashboardCapacityPools(groups, memberships)

	if len(pools) != 1 {
		t.Fatalf("pool count = %d, want 1", len(pools))
	}
	pool := pools[0]
	if pool.TotalAccounts != 3 {
		t.Fatalf("total accounts = %d, want 3", pool.TotalAccounts)
	}
	if pool.SchedulableAccounts != 2 {
		t.Fatalf("schedulable accounts = %d, want 2", pool.SchedulableAccounts)
	}
	if len(pool.GroupIDs) != 2 {
		t.Fatalf("group ids count = %d, want 2", len(pool.GroupIDs))
	}
}

func TestRecommendByPool_DoesNotDuplicateAcrossPlans(t *testing.T) {
	groups := []dashboardRecommendationGroupSnapshot{
		{
			GroupID:               1,
			GroupName:             "OpenAI Pro A",
			Platform:              PlatformOpenAI,
			PlanNames:             []string{"OpenAI Pro 30d"},
			ActiveSubscriptions:   12,
			ActiveUsers30d:        10,
			TotalActualCost30d:    180,
			TotalActualCost7d:     42,
			TotalActualCostPrev7d: 35,
		},
		{
			GroupID:               2,
			GroupName:             "OpenAI Pro B",
			Platform:              PlatformOpenAI,
			PlanNames:             []string{"OpenAI Team 30d"},
			ActiveSubscriptions:   8,
			ActiveUsers30d:        7,
			TotalActualCost30d:    120,
			TotalActualCost7d:     28,
			TotalActualCostPrev7d: 21,
		},
	}
	memberships := []dashboardRecommendationPoolMembership{
		{GroupID: 1, AccountID: 101, AccountType: AccountTypeOAuth, Schedulable: true},
		{GroupID: 1, AccountID: 102, AccountType: AccountTypeOAuth, Schedulable: true},
		{GroupID: 2, AccountID: 102, AccountType: AccountTypeOAuth, Schedulable: true},
	}

	pools := buildDashboardCapacityPools(groups, memberships)
	inputs := buildDashboardPoolInputs(groups, pools)
	if len(inputs) != 1 {
		t.Fatalf("pool inputs count = %d, want 1", len(inputs))
	}

	baselines := computeDashboardPoolBaselines(inputs)
	profile := buildCapacityRecommendationPreferenceProfile(60)
	got := computeDashboardCapacityPoolRecommendation(inputs[0], baselines[PlatformOpenAI], baselines[""], profile)

	if len(got.PlanNames) != 2 {
		t.Fatalf("plan names count = %d, want 2", len(got.PlanNames))
	}
	if got.RecommendedAdditionalSchedulableAccounts < 0 {
		t.Fatalf("recommended additional schedulable accounts = %d, want >= 0", got.RecommendedAdditionalSchedulableAccounts)
	}
}
