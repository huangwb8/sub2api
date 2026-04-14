package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
)

const (
	dashboardRecommendationLookbackDays = 30
	dashboardRecommendationGrowthDays   = 7
	minActivationRate                   = 0.15
	maxActivationRate                   = 1.0
	minDailyCostPerActiveUser           = 0.05
	minCountBaselinePerSchedulable      = 1.0
	minCostBaselinePerSchedulable       = 0.35
)

type DashboardRecommendationService struct {
	db                   *sql.DB
	entClient            *dbent.Client
	groupCapacityService *GroupCapacityService
	settingRepo          SettingRepository
}

type DashboardCapacityRecommendationResponse struct {
	GeneratedAt  time.Time                                `json:"generated_at"`
	LookbackDays int                                      `json:"lookback_days"`
	Summary      DashboardCapacityRecommendationSummary   `json:"summary"`
	Items        []DashboardGroupCapacityRecommendation   `json:"items"`
}

type DashboardCapacityRecommendationSummary struct {
	GroupCount                    int `json:"group_count"`
	CurrentSchedulableAccounts    int `json:"current_schedulable_accounts"`
	RecommendedAdditionalAccounts int `json:"recommended_additional_accounts"`
	UrgentGroupCount              int `json:"urgent_group_count"`
}

type DashboardGroupCapacityRecommendation struct {
	GroupID                      int64                                         `json:"group_id"`
	GroupName                    string                                        `json:"group_name"`
	Platform                     string                                        `json:"platform"`
	PlanNames                    []string                                      `json:"plan_names"`
	RecommendedAccountType       string                                        `json:"recommended_account_type"`
	Status                       string                                        `json:"status"`
	ConfidenceScore              float64                                       `json:"confidence_score"`
	CurrentTotalAccounts         int                                           `json:"current_total_accounts"`
	CurrentSchedulableAccounts   int                                           `json:"current_schedulable_accounts"`
	RecommendedTotalAccounts     int                                           `json:"recommended_total_accounts"`
	RecommendedAdditionalAccounts int                                          `json:"recommended_additional_accounts"`
	SubscriberHeadroom           int                                           `json:"subscriber_headroom"`
	Reason                       string                                        `json:"reason"`
	Metrics                      DashboardGroupCapacityRecommendationMetrics    `json:"metrics"`
}

type DashboardGroupCapacityRecommendationMetrics struct {
	ActiveSubscriptions              int                                `json:"active_subscriptions"`
	ActiveUsers30d                   int                                `json:"active_users_30d"`
	ActivationRate                   float64                            `json:"activation_rate"`
	BlendedActivationRate            float64                            `json:"blended_activation_rate"`
	AvgDailyCost30d                  float64                            `json:"avg_daily_cost_30d"`
	AvgDailyCostPerActiveUser        float64                            `json:"avg_daily_cost_per_active_user"`
	BlendedAvgDailyCostPerActiveUser float64                            `json:"blended_avg_daily_cost_per_active_user"`
	GrowthFactor                     float64                            `json:"growth_factor"`
	ProjectedDailyCost               float64                            `json:"projected_daily_cost"`
	CapacityUtilization              float64                            `json:"capacity_utilization"`
	ConcurrencyUtilization           float64                            `json:"concurrency_utilization"`
	SessionsUtilization              float64                            `json:"sessions_utilization"`
	RPMUtilization                   float64                            `json:"rpm_utilization"`
	ExpectedAccountsBySubscriptions  int                                `json:"expected_accounts_by_subscriptions"`
	ExpectedAccountsByActiveUsers    int                                `json:"expected_accounts_by_active_users"`
	ExpectedAccountsByCost           int                                `json:"expected_accounts_by_cost"`
	PlatformBaseline                 DashboardRecommendationBaseline    `json:"platform_baseline"`
}

type DashboardRecommendationBaseline struct {
	Platform                         string  `json:"platform"`
	ActiveSubscriptionsPerSchedulable float64 `json:"active_subscriptions_per_schedulable"`
	ActiveUsersPerSchedulable        float64 `json:"active_users_per_schedulable"`
	DailyCostPerSchedulable          float64 `json:"daily_cost_per_schedulable"`
	ActivationRate                   float64 `json:"activation_rate"`
	AvgDailyCostPerActiveUser        float64 `json:"avg_daily_cost_per_active_user"`
}

type dashboardRecommendationAggregateRow struct {
	GroupID                 int64
	ActiveSubscriptions     int
	ActiveUsers30d          int
	TotalActualCost30d      float64
	TotalActualCost7d       float64
	TotalActualCostPrev7d   float64
	TotalAccounts           int
	SchedulableAccounts     int
	RecommendedAccountType  string
}

type dashboardRecommendationInput struct {
	GroupID                   int64
	GroupName                 string
	Platform                  string
	PlanNames                 []string
	ActiveSubscriptions       int
	ActiveUsers30d            int
	AvgDailyCost30d           float64
	AvgDailyCostPerActiveUser float64
	CurrentTotalAccounts      int
	CurrentSchedulableAccounts int
	RecommendedAccountType    string
	GrowthFactor              float64
	ConcurrencyUtilization    float64
	SessionsUtilization       float64
	RPMUtilization            float64
}

type dashboardRecommendationCapacity struct {
	ConcurrencyUtilization float64
	SessionsUtilization    float64
	RPMUtilization         float64
}

func NewDashboardRecommendationService(
	db *sql.DB,
	entClient *dbent.Client,
	groupCapacityService *GroupCapacityService,
	settingRepo SettingRepository,
) *DashboardRecommendationService {
	return &DashboardRecommendationService{
		db:                   db,
		entClient:            entClient,
		groupCapacityService: groupCapacityService,
		settingRepo:          settingRepo,
	}
}

func (s *DashboardRecommendationService) GetCapacityRecommendations(ctx context.Context) (*DashboardCapacityRecommendationResponse, error) {
	if s == nil || s.db == nil || s.entClient == nil {
		return nil, fmt.Errorf("dashboard recommendation service is not fully initialized")
	}

	groups, err := s.entClient.Group.Query().
		Where(
			group.StatusEQ(StatusActive),
			group.SubscriptionTypeEQ(SubscriptionTypeSubscription),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query subscription groups: %w", err)
	}
	if len(groups) == 0 {
		return &DashboardCapacityRecommendationResponse{
			GeneratedAt:  time.Now().UTC(),
			LookbackDays: dashboardRecommendationLookbackDays,
			Summary:      DashboardCapacityRecommendationSummary{},
			Items:        []DashboardGroupCapacityRecommendation{},
		}, nil
	}
	preferenceScore := defaultSubscriptionCapacityTightness
	if s.settingRepo != nil {
		if raw, err := s.settingRepo.GetValue(ctx, SettingKeySubscriptionCapacityTightness); err == nil {
			preferenceScore = parseSubscriptionCapacityTightness(raw)
		}
	}
	preferenceProfile := buildCapacityRecommendationPreferenceProfile(preferenceScore)

	groupIDs := make([]int64, 0, len(groups))
	for _, grp := range groups {
		groupIDs = append(groupIDs, grp.ID)
	}

	planNamesByGroup, err := s.loadPlanNamesByGroup(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	aggregates, err := s.loadAggregateRows(ctx)
	if err != nil {
		return nil, err
	}

	capacityByGroup := map[int64]dashboardRecommendationCapacity{}
	if s.groupCapacityService != nil {
		summaries, capErr := s.groupCapacityService.GetAllGroupCapacity(ctx)
		if capErr == nil {
			for _, summary := range summaries {
				capacityByGroup[summary.GroupID] = dashboardRecommendationCapacity{
					ConcurrencyUtilization: ratio(summary.ConcurrencyUsed, summary.ConcurrencyMax),
					SessionsUtilization:    ratio(summary.SessionsUsed, summary.SessionsMax),
					RPMUtilization:         ratio(summary.RPMUsed, summary.RPMMax),
				}
			}
		}
	}

	inputs := make([]dashboardRecommendationInput, 0, len(groups))
	for _, grp := range groups {
		agg := aggregates[grp.ID]
		capacity := capacityByGroup[grp.ID]
		totalActualCost30d := agg.TotalActualCost30d
		avgDailyCost30d := totalActualCost30d / float64(dashboardRecommendationLookbackDays)
		activeUsers := maxInt(agg.ActiveUsers30d, 0)
		avgDailyCostPerActiveUser := 0.0
		if activeUsers > 0 {
			avgDailyCostPerActiveUser = avgDailyCost30d / float64(activeUsers)
		}

		inputs = append(inputs, dashboardRecommendationInput{
			GroupID:                    grp.ID,
			GroupName:                  grp.Name,
			Platform:                   grp.Platform,
			PlanNames:                  planNamesByGroup[grp.ID],
			ActiveSubscriptions:        agg.ActiveSubscriptions,
			ActiveUsers30d:             activeUsers,
			AvgDailyCost30d:            avgDailyCost30d,
			AvgDailyCostPerActiveUser:  avgDailyCostPerActiveUser,
			CurrentTotalAccounts:       agg.TotalAccounts,
			CurrentSchedulableAccounts: agg.SchedulableAccounts,
			RecommendedAccountType:     normalizeRecommendedAccountType(agg.RecommendedAccountType),
			GrowthFactor:               computeGrowthFactor(agg.TotalActualCost7d, agg.TotalActualCostPrev7d),
			ConcurrencyUtilization:     capacity.ConcurrencyUtilization,
			SessionsUtilization:        capacity.SessionsUtilization,
			RPMUtilization:             capacity.RPMUtilization,
		})
	}

	baselines := computeDashboardRecommendationBaselines(inputs)
	items := make([]DashboardGroupCapacityRecommendation, 0, len(inputs))
	summary := DashboardCapacityRecommendationSummary{GroupCount: len(inputs)}

	for _, input := range inputs {
		item := computeDashboardGroupCapacityRecommendation(
			input,
			baselines[input.Platform],
			baselines[""],
			preferenceProfile,
		)
		items = append(items, item)
		summary.CurrentSchedulableAccounts += item.CurrentSchedulableAccounts
		summary.RecommendedAdditionalAccounts += item.RecommendedAdditionalAccounts
		if item.Status == "action" {
			summary.UrgentGroupCount++
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].RecommendedAdditionalAccounts != items[j].RecommendedAdditionalAccounts {
			return items[i].RecommendedAdditionalAccounts > items[j].RecommendedAdditionalAccounts
		}
		if items[i].Metrics.CapacityUtilization != items[j].Metrics.CapacityUtilization {
			return items[i].Metrics.CapacityUtilization > items[j].Metrics.CapacityUtilization
		}
		if items[i].Metrics.ActiveSubscriptions != items[j].Metrics.ActiveSubscriptions {
			return items[i].Metrics.ActiveSubscriptions > items[j].Metrics.ActiveSubscriptions
		}
		return items[i].GroupID < items[j].GroupID
	})

	return &DashboardCapacityRecommendationResponse{
		GeneratedAt:  time.Now().UTC(),
		LookbackDays: dashboardRecommendationLookbackDays,
		Summary:      summary,
		Items:        items,
	}, nil
}

func (s *DashboardRecommendationService) loadPlanNamesByGroup(ctx context.Context, groupIDs []int64) (map[int64][]string, error) {
	plans, err := s.entClient.SubscriptionPlan.Query().
		Where(subscriptionplan.GroupIDIn(groupIDs...)).
		Order(subscriptionplan.BySortOrder()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query subscription plans: %w", err)
	}
	result := make(map[int64][]string, len(groupIDs))
	for _, plan := range plans {
		name := strings.TrimSpace(plan.Name)
		if name == "" {
			continue
		}
		result[plan.GroupID] = append(result[plan.GroupID], name)
	}
	return result, nil
}

func (s *DashboardRecommendationService) loadAggregateRows(ctx context.Context) (map[int64]dashboardRecommendationAggregateRow, error) {
	now := time.Now().UTC()
	start30d := now.AddDate(0, 0, -dashboardRecommendationLookbackDays)
	start7d := now.AddDate(0, 0, -dashboardRecommendationGrowthDays)
	prev7d := now.AddDate(0, 0, -2*dashboardRecommendationGrowthDays)

	query := `
WITH active_subscriptions AS (
	SELECT
		group_id,
		COUNT(*) AS active_subscriptions
	FROM user_subscriptions
	WHERE deleted_at IS NULL
		AND status = $1
		AND expires_at > $2
	GROUP BY group_id
),
usage_30d AS (
	SELECT
		group_id,
		COUNT(DISTINCT user_id) AS active_users_30d,
		COALESCE(SUM(actual_cost), 0) AS total_actual_cost_30d
	FROM usage_logs
	WHERE group_id IS NOT NULL
		AND created_at >= $3
		AND created_at < $2
	GROUP BY group_id
),
usage_7d AS (
	SELECT
		group_id,
		COALESCE(SUM(actual_cost), 0) AS total_actual_cost_7d
	FROM usage_logs
	WHERE group_id IS NOT NULL
		AND created_at >= $4
		AND created_at < $2
	GROUP BY group_id
),
usage_prev_7d AS (
	SELECT
		group_id,
		COALESCE(SUM(actual_cost), 0) AS total_actual_cost_prev_7d
	FROM usage_logs
	WHERE group_id IS NOT NULL
		AND created_at >= $5
		AND created_at < $4
	GROUP BY group_id
),
group_accounts AS (
	SELECT
		ag.group_id,
		COUNT(*) AS total_accounts,
		COUNT(CASE
			WHEN a.deleted_at IS NULL
				AND a.status = $6
				AND a.schedulable = true
				AND (a.auto_pause_on_expired = false OR a.expires_at IS NULL OR a.expires_at > $2)
				AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= $2)
				AND (a.overload_until IS NULL OR a.overload_until <= $2)
				AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= $2)
			THEN 1 END
		) AS schedulable_accounts
	FROM account_groups ag
	JOIN accounts a ON a.id = ag.account_id
	WHERE a.deleted_at IS NULL
	GROUP BY ag.group_id
),
dominant_account_type AS (
	SELECT group_id, type
	FROM (
		SELECT
			ag.group_id,
			a.type,
			ROW_NUMBER() OVER (
				PARTITION BY ag.group_id
				ORDER BY COUNT(*) DESC, a.type ASC
			) AS rank_index
		FROM account_groups ag
		JOIN accounts a ON a.id = ag.account_id
		WHERE a.deleted_at IS NULL
			AND a.status = $6
		GROUP BY ag.group_id, a.type
	) ranked
	WHERE rank_index = 1
)
SELECT
	g.id,
	COALESCE(s.active_subscriptions, 0) AS active_subscriptions,
	COALESCE(u30.active_users_30d, 0) AS active_users_30d,
	COALESCE(u30.total_actual_cost_30d, 0) AS total_actual_cost_30d,
	COALESCE(u7.total_actual_cost_7d, 0) AS total_actual_cost_7d,
	COALESCE(up.total_actual_cost_prev_7d, 0) AS total_actual_cost_prev_7d,
	COALESCE(ga.total_accounts, 0) AS total_accounts,
	COALESCE(ga.schedulable_accounts, 0) AS schedulable_accounts,
	COALESCE(dat.type, '') AS recommended_account_type
FROM groups g
LEFT JOIN active_subscriptions s ON s.group_id = g.id
LEFT JOIN usage_30d u30 ON u30.group_id = g.id
LEFT JOIN usage_7d u7 ON u7.group_id = g.id
LEFT JOIN usage_prev_7d up ON up.group_id = g.id
LEFT JOIN group_accounts ga ON ga.group_id = g.id
LEFT JOIN dominant_account_type dat ON dat.group_id = g.id
WHERE g.deleted_at IS NULL
	AND g.status = $6
	AND g.subscription_type = $7
`

	rows, err := s.db.QueryContext(
		ctx,
		query,
		SubscriptionStatusActive,
		now,
		start30d,
		start7d,
		prev7d,
		StatusActive,
		SubscriptionTypeSubscription,
	)
	if err != nil {
		return nil, fmt.Errorf("query dashboard recommendation aggregates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64]dashboardRecommendationAggregateRow)
	for rows.Next() {
		var row dashboardRecommendationAggregateRow
		if err := rows.Scan(
			&row.GroupID,
			&row.ActiveSubscriptions,
			&row.ActiveUsers30d,
			&row.TotalActualCost30d,
			&row.TotalActualCost7d,
			&row.TotalActualCostPrev7d,
			&row.TotalAccounts,
			&row.SchedulableAccounts,
			&row.RecommendedAccountType,
		); err != nil {
			return nil, fmt.Errorf("scan dashboard recommendation aggregate row: %w", err)
		}
		result[row.GroupID] = row
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dashboard recommendation aggregates: %w", err)
	}
	return result, nil
}

func computeDashboardRecommendationBaselines(inputs []dashboardRecommendationInput) map[string]DashboardRecommendationBaseline {
	buckets := map[string]*struct {
		platform               string
		activeSubs             int
		activeUsers            int
		schedulableAccounts    int
		avgDailyCost           float64
		weightedActiveUserCost float64
	}{}

	addToBucket := func(key, platform string, input dashboardRecommendationInput) {
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
		if input.CurrentSchedulableAccounts > 0 {
			bucket.activeSubs += input.ActiveSubscriptions
			bucket.activeUsers += input.ActiveUsers30d
			bucket.schedulableAccounts += input.CurrentSchedulableAccounts
			bucket.avgDailyCost += input.AvgDailyCost30d
			bucket.weightedActiveUserCost += input.AvgDailyCostPerActiveUser * float64(input.ActiveUsers30d)
		}
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

func computeDashboardGroupCapacityRecommendation(
	input dashboardRecommendationInput,
	platformBaseline DashboardRecommendationBaseline,
	globalBaseline DashboardRecommendationBaseline,
	profile capacityRecommendationPreferenceProfile,
) DashboardGroupCapacityRecommendation {
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

	expectedAccountsBySubscriptions := ceilDivFloat(
		float64(input.ActiveSubscriptions),
		baseline.ActiveSubscriptionsPerSchedulable*profile.BaselineScale,
	)
	expectedAccountsByActiveUsers := ceilDivFloat(
		projectedActiveUsers,
		baseline.ActiveUsersPerSchedulable*profile.BaselineScale,
	)
	expectedAccountsByCost := ceilDivFloat(
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

	recommendedTotalAccounts := maxInt(
		input.CurrentSchedulableAccounts,
		expectedAccountsBySubscriptions,
		expectedAccountsByActiveUsers,
		expectedAccountsByCost,
	)
	if input.CurrentSchedulableAccounts > 0 {
		recommendedTotalAccounts = maxInt(recommendedTotalAccounts, input.CurrentSchedulableAccounts+utilizationBuffer)
	}
	if input.CurrentSchedulableAccounts == 0 && (len(input.PlanNames) > 0 || input.ActiveSubscriptions > 0) {
		recommendedTotalAccounts = maxInt(recommendedTotalAccounts, 1)
	}

	recommendedAdditional := maxInt(0, recommendedTotalAccounts-input.CurrentSchedulableAccounts)
	subscriberHeadroom := 0
	if baseline.ActiveSubscriptionsPerSchedulable > 0 {
		subscriberHeadroom = maxInt(
			0,
			int(math.Floor(float64(recommendedTotalAccounts)*baseline.ActiveSubscriptionsPerSchedulable))-input.ActiveSubscriptions,
		)
	}

	status := "healthy"
	if recommendedAdditional > 0 || utilization >= profile.ActionUtilizationThreshold {
		status = "action"
	} else if utilization >= profile.WatchUtilizationThreshold || input.GrowthFactor >= 1.12 {
		status = "watch"
	}

	confidenceScore := computeDashboardRecommendationConfidence(input.ActiveSubscriptions, input.ActiveUsers30d, input.CurrentSchedulableAccounts)

	return DashboardGroupCapacityRecommendation{
		GroupID:                       input.GroupID,
		GroupName:                     input.GroupName,
		Platform:                      input.Platform,
		PlanNames:                     input.PlanNames,
		RecommendedAccountType:        input.RecommendedAccountType,
		Status:                        status,
		ConfidenceScore:               confidenceScore,
		CurrentTotalAccounts:          input.CurrentTotalAccounts,
		CurrentSchedulableAccounts:    input.CurrentSchedulableAccounts,
		RecommendedTotalAccounts:      recommendedTotalAccounts,
		RecommendedAdditionalAccounts: recommendedAdditional,
		SubscriberHeadroom:            subscriberHeadroom,
		Reason: buildDashboardRecommendationReason(
			input.RecommendedAccountType,
			recommendedAdditional,
			utilization,
			input.ActiveSubscriptions,
			projectedDailyCost,
			input.GrowthFactor,
		),
		Metrics: DashboardGroupCapacityRecommendationMetrics{
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
			ExpectedAccountsBySubscriptions:  expectedAccountsBySubscriptions,
			ExpectedAccountsByActiveUsers:    expectedAccountsByActiveUsers,
			ExpectedAccountsByCost:           expectedAccountsByCost,
			PlatformBaseline:                 baseline,
		},
	}
}

func mergeDashboardRecommendationBaseline(
	platformBaseline DashboardRecommendationBaseline,
	globalBaseline DashboardRecommendationBaseline,
	platform string,
) DashboardRecommendationBaseline {
	baseline := globalBaseline
	if platformBaseline.ActiveSubscriptionsPerSchedulable > 0 {
		baseline = platformBaseline
	}
	if baseline.Platform == "" {
		baseline.Platform = platform
	}
	baseline.ActiveSubscriptionsPerSchedulable = maxFloat(
		baseline.ActiveSubscriptionsPerSchedulable,
		minCountBaselinePerSchedulable,
	)
	baseline.ActiveUsersPerSchedulable = maxFloat(
		baseline.ActiveUsersPerSchedulable,
		minCountBaselinePerSchedulable,
	)
	baseline.DailyCostPerSchedulable = maxFloat(
		baseline.DailyCostPerSchedulable,
		minCostBaselinePerSchedulable,
	)
	if baseline.ActivationRate <= 0 {
		baseline.ActivationRate = minActivationRate
	}
	if baseline.AvgDailyCostPerActiveUser <= 0 {
		baseline.AvgDailyCostPerActiveUser = minDailyCostPerActiveUser
	}
	return baseline
}

func normalizeRecommendedAccountType(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "schedulable"
	}
	return value
}

func buildDashboardRecommendationReason(
	accountType string,
	additional int,
	utilization float64,
	activeSubscriptions int,
	projectedDailyCost float64,
	growthFactor float64,
) string {
	if additional > 0 {
		return fmt.Sprintf(
			"建议补充 %d 个同类 %s 账号：当前 %.0f%% 容量利用率下，%d 个活跃订阅对应的预测日负载约为 $%.2f，近 7 天增长系数 %.2f。",
			additional,
			accountType,
			utilization*100,
			activeSubscriptions,
			projectedDailyCost,
			growthFactor,
		)
	}
	return fmt.Sprintf(
		"当前账号池还能承载同类用户画像，容量利用率 %.0f%%，预测日负载约 $%.2f，可继续观察。",
		utilization*100,
		projectedDailyCost,
	)
}

func computeGrowthFactor(last7d, prev7d float64) float64 {
	if last7d <= 0 && prev7d <= 0 {
		return 1
	}
	if prev7d <= 0 {
		return 1.15
	}
	return clampFloat(last7d/prev7d, 0.85, 1.35)
}

func computeDashboardRecommendationConfidence(activeSubscriptions, activeUsers, schedulableAccounts int) float64 {
	score := 0.35
	score += math.Min(float64(activeSubscriptions)/20, 0.3)
	score += math.Min(float64(activeUsers)/15, 0.2)
	if schedulableAccounts > 0 {
		score += 0.15
	}
	return clampFloat(score, 0.25, 0.98)
}

func blendMetric(observed, prior, sample, priorWeight, minValue, maxValue float64) float64 {
	if priorWeight <= 0 {
		return clampFloat(observed, minValue, maxValue)
	}
	blended := ((observed * sample) + (prior * priorWeight)) / (sample + priorWeight)
	return clampFloat(blended, minValue, maxValue)
}

func ratio(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func ceilDivFloat(numerator, denominator float64) int {
	if denominator <= 0 {
		return 0
	}
	if numerator <= 0 {
		return 0
	}
	return int(math.Ceil(numerator / denominator))
}

func maxFloat(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	result := values[0]
	for _, value := range values[1:] {
		if value > result {
			result = value
		}
	}
	return result
}

func maxInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	result := values[0]
	for _, value := range values[1:] {
		if value > result {
			result = value
		}
	}
	return result
}
