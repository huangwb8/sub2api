package service

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type dashboardRecommendationGroupSnapshot struct {
	GroupID               int64
	GroupName             string
	Platform              string
	PlanNames             []string
	ActiveSubscriptions   int
	ActiveUsers30d        int
	TotalActualCost30d    float64
	TotalActualCost7d     float64
	TotalActualCostPrev7d float64
	Capacity              dashboardRecommendationCapacity
}

type dashboardRecommendationPoolMembership struct {
	GroupID     int64
	AccountID   int64
	AccountType string
	Schedulable bool
}

type dashboardCapacityPool struct {
	PoolKey                string
	Platform               string
	RecommendedAccountType string
	GroupIDs               []int64
	GroupNames             []string
	PlanNames              []string
	AccountIDs             []int64
	TotalAccounts          int
	SchedulableAccounts    int
}

type dashboardRecommendationPoolInput struct {
	PoolKey                    string
	Platform                   string
	GroupNames                 []string
	PlanNames                  []string
	ActiveSubscriptions        int
	ActiveUsers30d             int
	AvgDailyCost30d            float64
	AvgDailyCostPerActiveUser  float64
	CurrentTotalAccounts       int
	CurrentSchedulableAccounts int
	RecommendedAccountType     string
	GrowthFactor               float64
	ConcurrencyUtilization     float64
	SessionsUtilization        float64
	RPMUtilization             float64
}

func buildDashboardCapacityPools(
	groups []dashboardRecommendationGroupSnapshot,
	memberships []dashboardRecommendationPoolMembership,
) []dashboardCapacityPool {
	if len(groups) == 0 {
		return nil
	}

	groupByID := make(map[int64]dashboardRecommendationGroupSnapshot, len(groups))
	groupIDs := make([]int64, 0, len(groups))
	for _, grp := range groups {
		groupByID[grp.GroupID] = grp
		groupIDs = append(groupIDs, grp.GroupID)
	}
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })

	groupToAccounts := make(map[int64]map[int64]struct{}, len(groups))
	accountToGroups := map[int64]map[int64]struct{}{}
	accountTypeByID := map[int64]string{}
	schedulableByAccountID := map[int64]bool{}

	for _, membership := range memberships {
		if _, ok := groupByID[membership.GroupID]; !ok {
			continue
		}
		if groupToAccounts[membership.GroupID] == nil {
			groupToAccounts[membership.GroupID] = map[int64]struct{}{}
		}
		groupToAccounts[membership.GroupID][membership.AccountID] = struct{}{}
		if accountToGroups[membership.AccountID] == nil {
			accountToGroups[membership.AccountID] = map[int64]struct{}{}
		}
		accountToGroups[membership.AccountID][membership.GroupID] = struct{}{}
		accountTypeByID[membership.AccountID] = normalizeRecommendedAccountType(membership.AccountType)
		if membership.Schedulable {
			schedulableByAccountID[membership.AccountID] = true
		}
	}

	visited := make(map[int64]bool, len(groups))
	pools := make([]dashboardCapacityPool, 0, len(groups))

	for _, rootGroupID := range groupIDs {
		if visited[rootGroupID] {
			continue
		}

		componentGroupIDs := make([]int64, 0, 4)
		componentAccounts := map[int64]struct{}{}
		stack := []int64{rootGroupID}
		visited[rootGroupID] = true

		for len(stack) > 0 {
			groupID := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			componentGroupIDs = append(componentGroupIDs, groupID)

			for accountID := range groupToAccounts[groupID] {
				componentAccounts[accountID] = struct{}{}
				for linkedGroupID := range accountToGroups[accountID] {
					if visited[linkedGroupID] {
						continue
					}
					visited[linkedGroupID] = true
					stack = append(stack, linkedGroupID)
				}
			}
		}

		sort.Slice(componentGroupIDs, func(i, j int) bool { return componentGroupIDs[i] < componentGroupIDs[j] })
		accountIDs := make([]int64, 0, len(componentAccounts))
		schedulableAccounts := 0
		accountTypeCounts := map[string]int{}
		for accountID := range componentAccounts {
			accountIDs = append(accountIDs, accountID)
			if schedulableByAccountID[accountID] {
				schedulableAccounts++
			}
			accountTypeCounts[accountTypeByID[accountID]]++
		}
		sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })

		groupNames := make([]string, 0, len(componentGroupIDs))
		planNamesSet := map[string]struct{}{}
		platform := ""
		for _, groupID := range componentGroupIDs {
			group := groupByID[groupID]
			groupNames = append(groupNames, group.GroupName)
			if platform == "" {
				platform = group.Platform
			}
			for _, planName := range group.PlanNames {
				trimmed := strings.TrimSpace(planName)
				if trimmed == "" {
					continue
				}
				planNamesSet[trimmed] = struct{}{}
			}
		}
		planNames := make([]string, 0, len(planNamesSet))
		for planName := range planNamesSet {
			planNames = append(planNames, planName)
		}
		sort.Strings(planNames)

		recommendedAccountType := dominantRecommendedAccountType(accountTypeCounts)
		if recommendedAccountType == "" {
			recommendedAccountType = "schedulable"
		}

		pools = append(pools, dashboardCapacityPool{
			PoolKey:                buildDashboardCapacityPoolKey(platform, componentGroupIDs),
			Platform:               platform,
			RecommendedAccountType: recommendedAccountType,
			GroupIDs:               componentGroupIDs,
			GroupNames:             groupNames,
			PlanNames:              planNames,
			AccountIDs:             accountIDs,
			TotalAccounts:          len(accountIDs),
			SchedulableAccounts:    schedulableAccounts,
		})
	}

	sort.Slice(pools, func(i, j int) bool {
		if pools[i].Platform != pools[j].Platform {
			return pools[i].Platform < pools[j].Platform
		}
		return pools[i].PoolKey < pools[j].PoolKey
	})

	return pools
}

func buildDashboardPoolInputs(
	groups []dashboardRecommendationGroupSnapshot,
	pools []dashboardCapacityPool,
) []dashboardRecommendationPoolInput {
	if len(groups) == 0 || len(pools) == 0 {
		return nil
	}

	groupByID := make(map[int64]dashboardRecommendationGroupSnapshot, len(groups))
	for _, grp := range groups {
		groupByID[grp.GroupID] = grp
	}

	inputs := make([]dashboardRecommendationPoolInput, 0, len(pools))
	for _, pool := range pools {
		totalActualCost30d := 0.0
		totalActualCost7d := 0.0
		totalActualCostPrev7d := 0.0
		activeSubscriptions := 0
		activeUsers30d := 0
		concurrencyUtilization := 0.0
		sessionsUtilization := 0.0
		rpmUtilization := 0.0

		for _, groupID := range pool.GroupIDs {
			group := groupByID[groupID]
			activeSubscriptions += group.ActiveSubscriptions
			activeUsers30d += group.ActiveUsers30d
			totalActualCost30d += group.TotalActualCost30d
			totalActualCost7d += group.TotalActualCost7d
			totalActualCostPrev7d += group.TotalActualCostPrev7d
			concurrencyUtilization = maxFloat(concurrencyUtilization, group.Capacity.ConcurrencyUtilization)
			sessionsUtilization = maxFloat(sessionsUtilization, group.Capacity.SessionsUtilization)
			rpmUtilization = maxFloat(rpmUtilization, group.Capacity.RPMUtilization)
		}

		avgDailyCost30d := totalActualCost30d / float64(dashboardRecommendationLookbackDays)
		avgDailyCostPerActiveUser := 0.0
		if activeUsers30d > 0 {
			avgDailyCostPerActiveUser = avgDailyCost30d / float64(activeUsers30d)
		}

		inputs = append(inputs, dashboardRecommendationPoolInput{
			PoolKey:                    pool.PoolKey,
			Platform:                   pool.Platform,
			GroupNames:                 append([]string(nil), pool.GroupNames...),
			PlanNames:                  append([]string(nil), pool.PlanNames...),
			ActiveSubscriptions:        activeSubscriptions,
			ActiveUsers30d:             activeUsers30d,
			AvgDailyCost30d:            avgDailyCost30d,
			AvgDailyCostPerActiveUser:  avgDailyCostPerActiveUser,
			CurrentTotalAccounts:       pool.TotalAccounts,
			CurrentSchedulableAccounts: pool.SchedulableAccounts,
			RecommendedAccountType:     pool.RecommendedAccountType,
			GrowthFactor:               computeGrowthFactor(totalActualCost7d, totalActualCostPrev7d),
			ConcurrencyUtilization:     concurrencyUtilization,
			SessionsUtilization:        sessionsUtilization,
			RPMUtilization:             rpmUtilization,
		})
	}

	return inputs
}

func computeDashboardPoolBaselines(inputs []dashboardRecommendationPoolInput) map[string]DashboardRecommendationBaseline {
	buckets := map[string]*struct {
		platform               string
		activeSubs             int
		activeUsers            int
		schedulableAccounts    int
		avgDailyCost           float64
		weightedActiveUserCost float64
	}{}

	addToBucket := func(key, platform string, input dashboardRecommendationPoolInput) {
		isActiveSample := input.ActiveSubscriptions > 0 ||
			input.ActiveUsers30d > 0 ||
			input.AvgDailyCost30d > 0
		if !isActiveSample || input.CurrentSchedulableAccounts <= 0 {
			return
		}

		bucket := buckets[key]
		if bucket == nil {
			bucket = &struct {
				platform               string
				activeSubs             int
				activeUsers            int
				schedulableAccounts    int
				avgDailyCost           float64
				weightedActiveUserCost float64
			}{platform: platform}
			buckets[key] = bucket
		}

		bucket.activeSubs += input.ActiveSubscriptions
		bucket.activeUsers += input.ActiveUsers30d
		bucket.schedulableAccounts += input.CurrentSchedulableAccounts
		bucket.avgDailyCost += input.AvgDailyCost30d
		bucket.weightedActiveUserCost += input.AvgDailyCostPerActiveUser * float64(input.ActiveUsers30d)
	}

	for _, input := range inputs {
		addToBucket("", "", input)
		addToBucket(input.Platform, input.Platform, input)
	}

	result := make(map[string]DashboardRecommendationBaseline, len(buckets))
	for key, bucket := range buckets {
		activeSubsPerSchedulable := minCountBaselinePerSchedulable
		activeUsersPerSchedulable := minCountBaselinePerSchedulable
		dailyCostPerSchedulable := minCostBaselinePerSchedulable
		activationRate := minActivationRate
		avgDailyCostPerActiveUser := minDailyCostPerActiveUser

		if bucket.schedulableAccounts > 0 {
			activeSubsPerSchedulable = maxFloat(
				float64(bucket.activeSubs)/float64(bucket.schedulableAccounts),
				minCountBaselinePerSchedulable,
			)
			activeUsersPerSchedulable = maxFloat(
				float64(bucket.activeUsers)/float64(bucket.schedulableAccounts),
				minCountBaselinePerSchedulable,
			)
			dailyCostPerSchedulable = maxFloat(
				bucket.avgDailyCost/float64(bucket.schedulableAccounts),
				minCostBaselinePerSchedulable,
			)
		}
		if bucket.activeSubs > 0 {
			activationRate = clampFloat(
				float64(bucket.activeUsers)/float64(bucket.activeSubs),
				minActivationRate,
				maxActivationRate,
			)
		}
		if bucket.activeUsers > 0 {
			avgDailyCostPerActiveUser = maxFloat(
				bucket.weightedActiveUserCost/float64(bucket.activeUsers),
				minDailyCostPerActiveUser,
			)
		}

		result[key] = DashboardRecommendationBaseline{
			Platform:                          bucket.platform,
			ActiveSubscriptionsPerSchedulable: activeSubsPerSchedulable,
			ActiveUsersPerSchedulable:         activeUsersPerSchedulable,
			DailyCostPerSchedulable:           dailyCostPerSchedulable,
			ActivationRate:                    activationRate,
			AvgDailyCostPerActiveUser:         avgDailyCostPerActiveUser,
		}
	}

	return result
}

func computeDashboardCapacityPoolRecommendation(
	input dashboardRecommendationPoolInput,
	platformBaseline DashboardRecommendationBaseline,
	globalBaseline DashboardRecommendationBaseline,
	profile capacityRecommendationPreferenceProfile,
) DashboardCapacityPoolRecommendation {
	recommendedAccountType := normalizeRecommendedAccountType(input.RecommendedAccountType)
	baseline := mergeDashboardRecommendationBaseline(platformBaseline, globalBaseline, input.Platform)
	utilization := maxFloat(input.ConcurrencyUtilization, input.SessionsUtilization, input.RPMUtilization)
	activationRate := ratio(input.ActiveUsers30d, input.ActiveSubscriptions)
	blendedActivationRate := blendMetric(
		activationRate,
		baseline.ActivationRate,
		float64(input.ActiveSubscriptions),
		16,
		minActivationRate,
		maxActivationRate,
	)
	blendedAvgDailyCostPerActiveUser := blendMetric(
		input.AvgDailyCostPerActiveUser,
		baseline.AvgDailyCostPerActiveUser,
		float64(input.ActiveUsers30d),
		12,
		minDailyCostPerActiveUser,
		math.MaxFloat64,
	)

	projectedActiveUsers := maxFloat(float64(input.ActiveSubscriptions)*blendedActivationRate, 0)
	projectedDailyCost := projectedActiveUsers * blendedAvgDailyCostPerActiveUser * input.GrowthFactor
	if projectedDailyCost < input.AvgDailyCost30d {
		projectedDailyCost = input.AvgDailyCost30d
	}

	expectedBySubscriptions := ceilDivFloat(
		float64(input.ActiveSubscriptions),
		baseline.ActiveSubscriptionsPerSchedulable*profile.BaselineScale,
	)
	expectedByActiveUsers := ceilDivFloat(
		projectedActiveUsers,
		baseline.ActiveUsersPerSchedulable*profile.BaselineScale,
	)
	expectedByCost := ceilDivFloat(
		projectedDailyCost,
		baseline.DailyCostPerSchedulable*profile.BaselineScale,
	)

	utilizationBuffer := 0
	if utilization >= profile.ActionUtilizationThreshold {
		utilizationBuffer = 1
	}
	if utilization >= profile.EmergencyUtilizationThreshold {
		utilizationBuffer = 2
	}

	recommendedSchedulable := maxInt(
		input.CurrentSchedulableAccounts,
		expectedBySubscriptions,
		expectedByActiveUsers,
		expectedByCost,
	)
	if input.CurrentSchedulableAccounts > 0 {
		recommendedSchedulable = maxInt(recommendedSchedulable, input.CurrentSchedulableAccounts+utilizationBuffer)
	}
	if input.CurrentSchedulableAccounts == 0 && (len(input.PlanNames) > 0 || input.ActiveSubscriptions > 0) {
		recommendedSchedulable = maxInt(recommendedSchedulable, 1)
	}

	currentUnschedulable := maxInt(0, input.CurrentTotalAccounts-input.CurrentSchedulableAccounts)
	additionalSchedulable := maxInt(0, recommendedSchedulable-input.CurrentSchedulableAccounts)
	recoverableUnschedulable := minInt(additionalSchedulable, currentUnschedulable)
	newAccountsRequired := maxInt(0, additionalSchedulable-recoverableUnschedulable)

	status := "healthy"
	if additionalSchedulable > 0 || utilization >= profile.ActionUtilizationThreshold {
		status = "action"
	} else if utilization >= profile.WatchUtilizationThreshold || input.GrowthFactor >= 1.12 {
		status = "watch"
	}

	confidenceScore := computeDashboardRecommendationConfidence(
		input.ActiveSubscriptions,
		input.ActiveUsers30d,
		input.CurrentTotalAccounts,
	)

	return DashboardCapacityPoolRecommendation{
		PoolKey:                                  input.PoolKey,
		Platform:                                 input.Platform,
		GroupNames:                               append([]string(nil), input.GroupNames...),
		PlanNames:                                append([]string(nil), input.PlanNames...),
		RecommendedAccountType:                   recommendedAccountType,
		Status:                                   status,
		ConfidenceScore:                          confidenceScore,
		CurrentTotalAccounts:                     input.CurrentTotalAccounts,
		CurrentSchedulableAccounts:               input.CurrentSchedulableAccounts,
		CurrentUnschedulableAccounts:             currentUnschedulable,
		RecommendedSchedulableAccounts:           recommendedSchedulable,
		RecommendedAdditionalSchedulableAccounts: additionalSchedulable,
		RecoverableUnschedulableAccounts:         recoverableUnschedulable,
		NewAccountsRequired:                      newAccountsRequired,
		Reason: buildDashboardCapacityPoolReason(
			recommendedAccountType,
			additionalSchedulable,
			recoverableUnschedulable,
			newAccountsRequired,
			utilization,
			input.ActiveSubscriptions,
			projectedDailyCost,
			input.GrowthFactor,
		),
		Metrics: DashboardCapacityPoolRecommendationMetrics{
			ActiveSubscriptions:              input.ActiveSubscriptions,
			ActiveUsers30d:                   input.ActiveUsers30d,
			ActivationRate:                   activationRate,
			BlendedActivationRate:            blendedActivationRate,
			AvgDailyCost30d:                  input.AvgDailyCost30d,
			AvgDailyCostPerActiveUser:        input.AvgDailyCostPerActiveUser,
			BlendedAvgDailyCostPerActiveUser: blendedAvgDailyCostPerActiveUser,
			GrowthFactor:                     input.GrowthFactor,
			ProjectedDailyCost:               projectedDailyCost,
			CapacityUtilization:              utilization,
			ConcurrencyUtilization:           input.ConcurrencyUtilization,
			SessionsUtilization:              input.SessionsUtilization,
			RPMUtilization:                   input.RPMUtilization,
			ExpectedAccountsBySubscriptions:  expectedBySubscriptions,
			ExpectedAccountsByActiveUsers:    expectedByActiveUsers,
			ExpectedAccountsByCost:           expectedByCost,
			PlatformBaseline:                 baseline,
		},
	}
}

func buildDashboardCapacityPoolKey(platform string, groupIDs []int64) string {
	parts := make([]string, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		parts = append(parts, fmt.Sprintf("%d", groupID))
	}
	return fmt.Sprintf("%s:%s", platform, strings.Join(parts, "-"))
}

func dominantRecommendedAccountType(counts map[string]int) string {
	type candidate struct {
		accountType string
		count       int
	}
	var best candidate
	for accountType, count := range counts {
		normalized := normalizeRecommendedAccountType(accountType)
		if count > best.count || (count == best.count && normalized < best.accountType) {
			best = candidate{accountType: normalized, count: count}
		}
	}
	return best.accountType
}

func buildDashboardCapacityPoolReason(
	accountType string,
	additionalSchedulable int,
	recoverableUnschedulable int,
	newAccountsRequired int,
	utilization float64,
	activeSubscriptions int,
	projectedDailyCost float64,
	growthFactor float64,
) string {
	if additionalSchedulable <= 0 {
		return fmt.Sprintf(
			"当前容量池还能承载同类用户画像，容量利用率 %.0f%%，预测日负载约 $%.2f，可继续观察。",
			utilization*100,
			projectedDailyCost,
		)
	}

	if recoverableUnschedulable > 0 && newAccountsRequired <= 0 {
		return fmt.Sprintf(
			"建议优先恢复 %d 个现有不可调度的 %s 账号，使容量池补足 %d 个可调度缺口；当前 %.0f%% 容量利用率、%d 个活跃订阅对应的预测日负载约为 $%.2f，近 7 天增长系数 %.2f。",
			recoverableUnschedulable,
			accountType,
			additionalSchedulable,
			utilization*100,
			activeSubscriptions,
			projectedDailyCost,
			growthFactor,
		)
	}

	if recoverableUnschedulable > 0 {
		return fmt.Sprintf(
			"建议补充 %d 个可调度的 %s 账号，其中 %d 个可优先恢复现有不可调度账号，仍需新增 %d 个；当前 %.0f%% 容量利用率、%d 个活跃订阅对应的预测日负载约为 $%.2f，近 7 天增长系数 %.2f。",
			additionalSchedulable,
			accountType,
			recoverableUnschedulable,
			newAccountsRequired,
			utilization*100,
			activeSubscriptions,
			projectedDailyCost,
			growthFactor,
		)
	}

	return fmt.Sprintf(
		"建议补充 %d 个可调度的 %s 账号；当前 %.0f%% 容量利用率下，%d 个活跃订阅对应的预测日负载约为 $%.2f，近 7 天增长系数 %.2f。",
		additionalSchedulable,
		accountType,
		utilization*100,
		activeSubscriptions,
		projectedDailyCost,
		growthFactor,
	)
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}
