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
	"github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/ent/accountgroup"
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
	exchangeRateService  ExchangeRateService
}

type DashboardCapacityRecommendationResponse struct {
	GeneratedAt  time.Time                              `json:"generated_at"`
	LookbackDays int                                    `json:"lookback_days"`
	Summary      DashboardCapacityRecommendationSummary `json:"summary"`
	Pools        []DashboardCapacityPoolRecommendation  `json:"pools"`
}

type DashboardCapacityRecommendationSummary struct {
	PoolCount                                int `json:"pool_count"`
	GroupCount                               int `json:"group_count"`
	CurrentSchedulableAccounts               int `json:"current_schedulable_accounts"`
	RecommendedAdditionalSchedulableAccounts int `json:"recommended_additional_schedulable_accounts"`
	RecoverableUnschedulableAccounts         int `json:"recoverable_unschedulable_accounts"`
	UrgentPoolCount                          int `json:"urgent_pool_count"`
}

type DashboardCapacityPoolRecommendation struct {
	PoolKey                                  string                                     `json:"pool_key"`
	Platform                                 string                                     `json:"platform"`
	GroupNames                               []string                                   `json:"group_names"`
	PlanNames                                []string                                   `json:"plan_names"`
	RecommendedAccountType                   string                                     `json:"recommended_account_type"`
	Status                                   string                                     `json:"status"`
	ConfidenceScore                          float64                                    `json:"confidence_score"`
	CurrentTotalAccounts                     int                                        `json:"current_total_accounts"`
	CurrentSchedulableAccounts               int                                        `json:"current_schedulable_accounts"`
	CurrentUnschedulableAccounts             int                                        `json:"current_unschedulable_accounts"`
	RecommendedSchedulableAccounts           int                                        `json:"recommended_schedulable_accounts"`
	RecommendedAdditionalSchedulableAccounts int                                        `json:"recommended_additional_schedulable_accounts"`
	RecoverableUnschedulableAccounts         int                                        `json:"recoverable_unschedulable_accounts"`
	NewAccountsRequired                      int                                        `json:"new_accounts_required"`
	Reason                                   string                                     `json:"reason"`
	Metrics                                  DashboardCapacityPoolRecommendationMetrics `json:"metrics"`
}

type DashboardCapacityPoolRecommendationMetrics struct {
	ActiveSubscriptions              int                             `json:"active_subscriptions"`
	ActiveUsers30d                   int                             `json:"active_users_30d"`
	ActivationRate                   float64                         `json:"activation_rate"`
	BlendedActivationRate            float64                         `json:"blended_activation_rate"`
	AvgDailyCost30d                  float64                         `json:"avg_daily_cost_30d"`
	AvgDailyCostPerActiveUser        float64                         `json:"avg_daily_cost_per_active_user"`
	BlendedAvgDailyCostPerActiveUser float64                         `json:"blended_avg_daily_cost_per_active_user"`
	GrowthFactor                     float64                         `json:"growth_factor"`
	ProjectedDailyCost               float64                         `json:"projected_daily_cost"`
	CapacityUtilization              float64                         `json:"capacity_utilization"`
	ConcurrencyUtilization           float64                         `json:"concurrency_utilization"`
	SessionsUtilization              float64                         `json:"sessions_utilization"`
	RPMUtilization                   float64                         `json:"rpm_utilization"`
	ExpectedAccountsBySubscriptions  int                             `json:"expected_accounts_by_subscriptions"`
	ExpectedAccountsByActiveUsers    int                             `json:"expected_accounts_by_active_users"`
	ExpectedAccountsByCost           int                             `json:"expected_accounts_by_cost"`
	PlatformBaseline                 DashboardRecommendationBaseline `json:"platform_baseline"`
}

type DashboardRecommendationBaseline struct {
	Platform                          string  `json:"platform"`
	ActiveSubscriptionsPerSchedulable float64 `json:"active_subscriptions_per_schedulable"`
	ActiveUsersPerSchedulable         float64 `json:"active_users_per_schedulable"`
	DailyCostPerSchedulable           float64 `json:"daily_cost_per_schedulable"`
	ActivationRate                    float64 `json:"activation_rate"`
	AvgDailyCostPerActiveUser         float64 `json:"avg_daily_cost_per_active_user"`
}

type dashboardRecommendationAggregateRow struct {
	GroupID                int64
	ActiveSubscriptions    int
	ActiveUsers30d         int
	TotalActualCost30d     float64
	TotalActualCost7d      float64
	TotalActualCostPrev7d  float64
	RecommendedAccountType string
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
	exchangeRateService ExchangeRateService,
) *DashboardRecommendationService {
	return &DashboardRecommendationService{
		db:                   db,
		entClient:            entClient,
		groupCapacityService: groupCapacityService,
		settingRepo:          settingRepo,
		exchangeRateService:  exchangeRateService,
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
			Pools:        []DashboardCapacityPoolRecommendation{},
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
	memberships, err := s.loadPoolMemberships(ctx, groupIDs)
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

	snapshots := make([]dashboardRecommendationGroupSnapshot, 0, len(groups))
	for _, grp := range groups {
		agg := aggregates[grp.ID]
		snapshots = append(snapshots, dashboardRecommendationGroupSnapshot{
			GroupID:               grp.ID,
			GroupName:             grp.Name,
			Platform:              grp.Platform,
			PlanNames:             planNamesByGroup[grp.ID],
			ActiveSubscriptions:   agg.ActiveSubscriptions,
			ActiveUsers30d:        maxInt(agg.ActiveUsers30d, 0),
			TotalActualCost30d:    agg.TotalActualCost30d,
			TotalActualCost7d:     agg.TotalActualCost7d,
			TotalActualCostPrev7d: agg.TotalActualCostPrev7d,
			Capacity:              capacityByGroup[grp.ID],
		})
	}

	pools := buildDashboardCapacityPools(snapshots, memberships)
	inputs := buildDashboardPoolInputs(snapshots, pools)
	baselines := computeDashboardPoolBaselines(inputs)
	recommendations := make([]DashboardCapacityPoolRecommendation, 0, len(inputs))
	summary := DashboardCapacityRecommendationSummary{
		PoolCount:  len(pools),
		GroupCount: len(groups),
	}

	for _, input := range inputs {
		item := computeDashboardCapacityPoolRecommendation(
			input,
			baselines[input.Platform],
			baselines[""],
			preferenceProfile,
		)
		recommendations = append(recommendations, item)
		summary.CurrentSchedulableAccounts += item.CurrentSchedulableAccounts
		summary.RecommendedAdditionalSchedulableAccounts += item.RecommendedAdditionalSchedulableAccounts
		summary.RecoverableUnschedulableAccounts += item.RecoverableUnschedulableAccounts
		if item.Status == "action" {
			summary.UrgentPoolCount++
		}
	}

	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].RecommendedAdditionalSchedulableAccounts != recommendations[j].RecommendedAdditionalSchedulableAccounts {
			return recommendations[i].RecommendedAdditionalSchedulableAccounts > recommendations[j].RecommendedAdditionalSchedulableAccounts
		}
		if recommendations[i].Metrics.CapacityUtilization != recommendations[j].Metrics.CapacityUtilization {
			return recommendations[i].Metrics.CapacityUtilization > recommendations[j].Metrics.CapacityUtilization
		}
		if recommendations[i].Metrics.ActiveSubscriptions != recommendations[j].Metrics.ActiveSubscriptions {
			return recommendations[i].Metrics.ActiveSubscriptions > recommendations[j].Metrics.ActiveSubscriptions
		}
		return recommendations[i].PoolKey < recommendations[j].PoolKey
	})

	return &DashboardCapacityRecommendationResponse{
		GeneratedAt:  time.Now().UTC(),
		LookbackDays: dashboardRecommendationLookbackDays,
		Summary:      summary,
		Pools:        recommendations,
	}, nil
}

func (s *DashboardRecommendationService) loadPoolMemberships(
	ctx context.Context,
	groupIDs []int64,
) ([]dashboardRecommendationPoolMembership, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	entries, err := s.entClient.AccountGroup.Query().
		Where(accountgroup.GroupIDIn(groupIDs...)).
		Where(accountgroup.HasAccountWith(account.DeletedAtIsNil())).
		WithAccount().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query dashboard recommendation pool memberships: %w", err)
	}

	memberships := make([]dashboardRecommendationPoolMembership, 0, len(entries))
	for _, entry := range entries {
		if entry.Edges.Account == nil {
			continue
		}
		acc := entry.Edges.Account
		schedulable := acc.Status == StatusActive &&
			acc.Schedulable &&
			(!acc.AutoPauseOnExpired || acc.ExpiresAt == nil || acc.ExpiresAt.After(now)) &&
			(acc.RateLimitResetAt == nil || !acc.RateLimitResetAt.After(now)) &&
			(acc.OverloadUntil == nil || !acc.OverloadUntil.After(now)) &&
			(acc.TempUnschedulableUntil == nil || !acc.TempUnschedulableUntil.After(now))

		memberships = append(memberships, dashboardRecommendationPoolMembership{
			GroupID:     entry.GroupID,
			AccountID:   entry.AccountID,
			AccountType: acc.Type,
			Schedulable: schedulable,
		})
	}

	return memberships, nil
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
		ul.group_id,
		COUNT(DISTINCT ul.user_id) AS active_users_30d,
		COALESCE(SUM(ul.actual_cost), 0) AS total_actual_cost_30d
	FROM usage_logs ul
	JOIN users u ON u.id = ul.user_id
	WHERE ul.group_id IS NOT NULL
		AND ul.created_at >= $3
		AND ul.created_at < $2
		AND u.status = $6
	GROUP BY ul.group_id
),
usage_7d AS (
	SELECT
		ul.group_id,
		COALESCE(SUM(ul.actual_cost), 0) AS total_actual_cost_7d
	FROM usage_logs ul
	JOIN users u ON u.id = ul.user_id
	WHERE ul.group_id IS NOT NULL
		AND ul.created_at >= $4
		AND ul.created_at < $2
		AND u.status = $6
	GROUP BY ul.group_id
),
usage_prev_7d AS (
	SELECT
		ul.group_id,
		COALESCE(SUM(ul.actual_cost), 0) AS total_actual_cost_prev_7d
	FROM usage_logs ul
	JOIN users u ON u.id = ul.user_id
	WHERE ul.group_id IS NOT NULL
		AND ul.created_at >= $5
		AND ul.created_at < $4
		AND u.status = $6
	GROUP BY ul.group_id
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
		var totalAccounts int
		var schedulableAccounts int
		if err := rows.Scan(
			&row.GroupID,
			&row.ActiveSubscriptions,
			&row.ActiveUsers30d,
			&row.TotalActualCost30d,
			&row.TotalActualCost7d,
			&row.TotalActualCostPrev7d,
			&totalAccounts,
			&schedulableAccounts,
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
