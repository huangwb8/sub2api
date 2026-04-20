package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	dashboardOversellLightUserThresholdUnits  = 0.3
	dashboardOversellFallbackLightUserRatio   = 0.7
	dashboardOversellDefaultCapacityUnits     = 3.0
	dashboardOversellDefaultConfidenceLevel   = 0.95
	dashboardOversellDefaultProfitRatePercent = 20.0
	dashboardOversellDefaultProfitMode        = "net_margin"
	dashboardOversellDaysPerMonth             = 30.0
	dashboardOversellMaxUsersSearch           = 500
)

type DashboardOversellCalculatorRequest struct {
	ActualCostCNY           float64 `json:"actual_cost_cny"`
	CapacityUnitsPerProduct float64 `json:"capacity_units_per_product"`
	ConfidenceLevel         float64 `json:"confidence_level"`
	ProfitRatePercent       float64 `json:"profit_rate_percent"`
	ProfitMode              string  `json:"profit_mode"`
	TargetProfitTotalCNY    float64 `json:"target_profit_total_cny"`
}

type DashboardOversellEstimate struct {
	LightUserThresholdUnits     float64 `json:"light_user_threshold_units"`
	EstimatedLightUserRatio     float64 `json:"estimated_light_user_ratio"`
	SampledSubscriptionCount    int     `json:"sampled_subscription_count"`
	LightUserCount              int     `json:"light_user_count"`
	EstimatedFromLiveData       bool    `json:"estimated_from_live_data"`
	FallbackApplied             bool    `json:"fallback_applied"`
	Basis                       string  `json:"basis"`
	CurrentCheapestMonthlyPrice float64 `json:"current_cheapest_monthly_price_cny"`
	CurrentCheapestPlanName     string  `json:"current_cheapest_plan_name"`
}

type DashboardOversellCalculationResult struct {
	Feasible                       bool    `json:"feasible"`
	MinimumUsers                   int     `json:"minimum_users"`
	RecommendedMonthlyPriceCNY     float64 `json:"recommended_monthly_price_cny"`
	CurrentCheapestMonthlyPriceCNY float64 `json:"current_cheapest_monthly_price_cny"`
	MonthlyPriceGapCNY             float64 `json:"monthly_price_gap_cny"`
	ExpectedMeanUnits              float64 `json:"expected_mean_units"`
	RiskAdjustedMeanUnits          float64 `json:"risk_adjusted_mean_units"`
	ConfidenceLevel                float64 `json:"confidence_level"`
	PriceMultiplier                float64 `json:"price_multiplier"`
	Reason                         string  `json:"reason"`
}

type DashboardOversellPlanRecommendation struct {
	PlanID                     int64   `json:"plan_id"`
	GroupID                    int64   `json:"group_id"`
	GroupName                  string  `json:"group_name"`
	PlanName                   string  `json:"plan_name"`
	ValidityDays               int     `json:"validity_days"`
	ValidityUnit               string  `json:"validity_unit"`
	DurationDaysEquivalent     float64 `json:"duration_days_equivalent"`
	CurrentPriceCNY            float64 `json:"current_price_cny"`
	CurrentMonthlyPriceCNY     float64 `json:"current_monthly_price_cny"`
	RecommendedPriceCNY        float64 `json:"recommended_price_cny"`
	RecommendedMonthlyPriceCNY float64 `json:"recommended_monthly_price_cny"`
	PriceDeltaCNY              float64 `json:"price_delta_cny"`
}

type DashboardOversellCalculatorResponse struct {
	GeneratedAt time.Time                             `json:"generated_at"`
	Defaults    DashboardOversellCalculatorRequest    `json:"defaults"`
	Input       DashboardOversellCalculatorRequest    `json:"input"`
	Estimate    DashboardOversellEstimate             `json:"estimate"`
	Result      DashboardOversellCalculationResult    `json:"result"`
	Plans       []DashboardOversellPlanRecommendation `json:"plans"`
}

type dashboardOversellPlanSnapshot struct {
	PlanID                 int64
	GroupID                int64
	GroupName              string
	PlanName               string
	PriceCNY               float64
	ValidityDays           int
	ValidityUnit           string
	DurationDaysEquivalent float64
	MonthlyPriceCNY        float64
}

func (s *DashboardRecommendationService) GetOversellCalculator(ctx context.Context) (*DashboardOversellCalculatorResponse, error) {
	defaults, err := s.loadDashboardOversellDefaults(ctx)
	if err != nil {
		return nil, err
	}
	return s.CalculateOversellCalculator(ctx, defaults)
}

func (s *DashboardRecommendationService) CalculateOversellCalculator(
	ctx context.Context,
	req DashboardOversellCalculatorRequest,
) (*DashboardOversellCalculatorResponse, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("dashboard recommendation service is not fully initialized")
	}

	plans, err := s.loadDashboardOversellPlans(ctx)
	if err != nil {
		return nil, err
	}
	estimate, err := s.loadDashboardOversellEstimate(ctx, plans)
	if err != nil {
		return nil, err
	}
	defaults, err := s.loadDashboardOversellDefaults(ctx)
	if err != nil {
		return nil, err
	}

	normalized := normalizeDashboardOversellRequest(req, defaults)
	result, recommendations := calculateDashboardOversellScenario(normalized, estimate, plans)

	return &DashboardOversellCalculatorResponse{
		GeneratedAt: time.Now().UTC(),
		Defaults:    defaults,
		Input:       normalized,
		Estimate:    estimate,
		Result:      result,
		Plans:       recommendations,
	}, nil
}

func (s *DashboardRecommendationService) loadDashboardOversellDefaults(ctx context.Context) (DashboardOversellCalculatorRequest, error) {
	defaults := DashboardOversellCalculatorRequest{
		CapacityUnitsPerProduct: dashboardOversellDefaultCapacityUnits,
		ConfidenceLevel:         dashboardOversellDefaultConfidenceLevel,
		ProfitRatePercent:       dashboardOversellDefaultProfitRatePercent,
		ProfitMode:              dashboardOversellDefaultProfitMode,
		TargetProfitTotalCNY:    0,
	}

	if s == nil || s.db == nil {
		return defaults, nil
	}

	query := `
SELECT COALESCE(AVG(actual_cost_cny), 0)
FROM accounts
WHERE deleted_at IS NULL
  AND actual_cost_cny IS NOT NULL
  AND actual_cost_cny > 0
`
	if err := s.db.QueryRowContext(ctx, query).Scan(&defaults.ActualCostCNY); err != nil {
		return defaults, fmt.Errorf("query dashboard oversell default actual cost: %w", err)
	}

	return defaults, nil
}

func (s *DashboardRecommendationService) loadDashboardOversellPlans(
	ctx context.Context,
) ([]dashboardOversellPlanSnapshot, error) {
	query := `
SELECT
	sp.id,
	sp.group_id,
	COALESCE(g.name, '') AS group_name,
	COALESCE(sp.name, '') AS plan_name,
	COALESCE(sp.price, 0) AS price_cny,
	COALESCE(sp.validity_days, 0) AS validity_days,
	COALESCE(sp.validity_unit, 'day') AS validity_unit
FROM subscription_plans sp
JOIN groups g ON g.id = sp.group_id
WHERE sp.for_sale = TRUE
  AND g.deleted_at IS NULL
  AND g.status = $1
  AND g.subscription_type = $2
ORDER BY sp.sort_order ASC, sp.id ASC
`

	rows, err := s.db.QueryContext(ctx, query, StatusActive, SubscriptionTypeSubscription)
	if err != nil {
		return nil, fmt.Errorf("query dashboard oversell plans: %w", err)
	}
	defer rows.Close()

	plans := make([]dashboardOversellPlanSnapshot, 0)
	for rows.Next() {
		var item dashboardOversellPlanSnapshot
		if err := rows.Scan(
			&item.PlanID,
			&item.GroupID,
			&item.GroupName,
			&item.PlanName,
			&item.PriceCNY,
			&item.ValidityDays,
			&item.ValidityUnit,
		); err != nil {
			return nil, fmt.Errorf("scan dashboard oversell plan: %w", err)
		}

		item.DurationDaysEquivalent = oversellPlanDurationDays(item.ValidityDays, item.ValidityUnit)
		item.MonthlyPriceCNY = oversellMonthlyEquivalentPrice(item.PriceCNY, item.DurationDaysEquivalent)
		plans = append(plans, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dashboard oversell plans: %w", err)
	}

	return plans, nil
}

func (s *DashboardRecommendationService) loadDashboardOversellEstimate(
	ctx context.Context,
	plans []dashboardOversellPlanSnapshot,
) (DashboardOversellEstimate, error) {
	query := `
WITH subscription_ratios AS (
	SELECT
		CASE
			WHEN g.monthly_limit_usd IS NOT NULL AND g.monthly_limit_usd > 0
				THEN COALESCE(us.monthly_usage_usd, 0) / g.monthly_limit_usd
			WHEN g.weekly_limit_usd IS NOT NULL AND g.weekly_limit_usd > 0
				THEN COALESCE(us.weekly_usage_usd, 0) / g.weekly_limit_usd
			WHEN g.daily_limit_usd IS NOT NULL AND g.daily_limit_usd > 0
				THEN COALESCE(us.daily_usage_usd, 0) / g.daily_limit_usd
			ELSE NULL
		END AS usage_ratio
	FROM user_subscriptions us
	JOIN groups g ON g.id = us.group_id
	WHERE us.deleted_at IS NULL
	  AND g.deleted_at IS NULL
	  AND us.status = $1
	  AND us.expires_at > NOW()
	  AND g.status = $2
	  AND g.subscription_type = $3
)
SELECT
	COUNT(*) FILTER (WHERE usage_ratio IS NOT NULL) AS sampled_subscription_count,
	COUNT(*) FILTER (WHERE usage_ratio IS NOT NULL AND usage_ratio <= $4) AS light_user_count
FROM subscription_ratios
`

	estimate := DashboardOversellEstimate{
		LightUserThresholdUnits: dashboardOversellLightUserThresholdUnits,
		Basis:                   "按当前活跃订阅的已用额度 / 当前周期额度估算轻度用户占比",
	}

	if err := s.db.QueryRowContext(
		ctx,
		query,
		SubscriptionStatusActive,
		StatusActive,
		SubscriptionTypeSubscription,
		dashboardOversellLightUserThresholdUnits,
	).Scan(&estimate.SampledSubscriptionCount, &estimate.LightUserCount); err != nil {
		return estimate, fmt.Errorf("query dashboard oversell estimate: %w", err)
	}

	if estimate.SampledSubscriptionCount > 0 {
		estimate.EstimatedLightUserRatio = float64(estimate.LightUserCount) / float64(estimate.SampledSubscriptionCount)
		estimate.EstimatedFromLiveData = true
	} else {
		estimate.EstimatedLightUserRatio = dashboardOversellFallbackLightUserRatio
		estimate.FallbackApplied = true
	}

	for _, plan := range plans {
		if plan.MonthlyPriceCNY <= 0 {
			continue
		}
		if estimate.CurrentCheapestMonthlyPrice == 0 || plan.MonthlyPriceCNY < estimate.CurrentCheapestMonthlyPrice {
			estimate.CurrentCheapestMonthlyPrice = plan.MonthlyPriceCNY
			estimate.CurrentCheapestPlanName = plan.PlanName
		}
	}

	return estimate, nil
}

func normalizeDashboardOversellRequest(
	req DashboardOversellCalculatorRequest,
	defaults DashboardOversellCalculatorRequest,
) DashboardOversellCalculatorRequest {
	normalized := req

	if normalized.ActualCostCNY <= 0 {
		normalized.ActualCostCNY = defaults.ActualCostCNY
	}
	if normalized.CapacityUnitsPerProduct <= 0 {
		normalized.CapacityUnitsPerProduct = defaults.CapacityUnitsPerProduct
	}
	if normalized.ConfidenceLevel <= 0 || normalized.ConfidenceLevel >= 1 {
		normalized.ConfidenceLevel = defaults.ConfidenceLevel
	}
	if normalized.ProfitRatePercent < 0 {
		normalized.ProfitRatePercent = defaults.ProfitRatePercent
	}
	if normalized.ProfitMode == "" {
		normalized.ProfitMode = defaults.ProfitMode
	}
	normalized.ProfitMode = normalizeDashboardOversellProfitMode(normalized.ProfitMode)
	if normalized.TargetProfitTotalCNY < 0 {
		normalized.TargetProfitTotalCNY = 0
	}

	return normalized
}

func normalizeDashboardOversellProfitMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "markup", "cost_plus", "cost-plus":
		return "markup"
	case "net_margin", "net-margin", "margin":
		return "net_margin"
	default:
		return dashboardOversellDefaultProfitMode
	}
}

func calculateDashboardOversellScenario(
	req DashboardOversellCalculatorRequest,
	estimate DashboardOversellEstimate,
	plans []dashboardOversellPlanSnapshot,
) (DashboardOversellCalculationResult, []DashboardOversellPlanRecommendation) {
	result := DashboardOversellCalculationResult{
		ConfidenceLevel:                req.ConfidenceLevel,
		CurrentCheapestMonthlyPriceCNY: estimate.CurrentCheapestMonthlyPrice,
	}

	priceMultiplier, err := dashboardOversellPriceMultiplier(req.ProfitMode, req.ProfitRatePercent)
	if err != nil {
		result.Reason = err.Error()
		return result, buildDashboardOversellPlanRecommendations(plans, 0)
	}
	result.PriceMultiplier = priceMultiplier

	if req.ActualCostCNY <= 0 {
		result.Reason = "缺少可用的实际商品成本，暂时无法给出超售建议"
		return result, buildDashboardOversellPlanRecommendations(plans, 0)
	}
	if req.CapacityUnitsPerProduct <= 0 {
		result.Reason = "实际商品承载能力必须大于 0"
		return result, buildDashboardOversellPlanRecommendations(plans, 0)
	}
	if estimate.CurrentCheapestMonthlyPrice <= 0 {
		result.Reason = "当前没有可用于反推门槛的在售订阅套餐"
		return result, buildDashboardOversellPlanRecommendations(plans, 0)
	}

	expectedMeanUnits := estimate.EstimatedLightUserRatio*dashboardOversellLightUserThresholdUnits +
		(1-estimate.EstimatedLightUserRatio)*req.CapacityUnitsPerProduct
	result.ExpectedMeanUnits = expectedMeanUnits

	minimumUsers := 0
	recommendedMonthlyPrice := 0.0
	riskAdjustedMeanUnits := 0.0

	for users := 1; users <= dashboardOversellMaxUsersSearch; users++ {
		currentRiskAdjusted := dashboardOversellRiskAdjustedMean(
			expectedMeanUnits,
			req.CapacityUnitsPerProduct,
			req.ConfidenceLevel,
			users,
		)
		requiredMonthlyPrice := dashboardOversellRequiredMonthlyPrice(req, currentRiskAdjusted, priceMultiplier, users)
		if estimate.CurrentCheapestMonthlyPrice >= requiredMonthlyPrice {
			minimumUsers = users
			recommendedMonthlyPrice = requiredMonthlyPrice
			riskAdjustedMeanUnits = currentRiskAdjusted
			break
		}
	}

	if minimumUsers == 0 {
		lastRiskAdjusted := dashboardOversellRiskAdjustedMean(
			expectedMeanUnits,
			req.CapacityUnitsPerProduct,
			req.ConfidenceLevel,
			dashboardOversellMaxUsersSearch,
		)
		result.RiskAdjustedMeanUnits = lastRiskAdjusted
		result.Reason = fmt.Sprintf(
			"当前最便宜的月均套餐价 ¥%.2f 仍不足以覆盖 %.0f%% 把握下的保守月均成本，建议先上调定价或降低固定盈利目标",
			estimate.CurrentCheapestMonthlyPrice,
			req.ConfidenceLevel*100,
		)
		return result, buildDashboardOversellPlanRecommendations(plans, 0)
	}

	result.Feasible = true
	result.MinimumUsers = minimumUsers
	result.RecommendedMonthlyPriceCNY = recommendedMonthlyPrice
	result.MonthlyPriceGapCNY = estimate.CurrentCheapestMonthlyPrice - recommendedMonthlyPrice
	result.RiskAdjustedMeanUnits = riskAdjustedMeanUnits
	result.Reason = fmt.Sprintf(
		"按当前最便宜月均套餐价 ¥%.2f 反推，至少需要 %d 个用户，才能在 %.0f%% 把握下覆盖保守人均消耗 %.2f 个，并满足 %.2f%% 利润率与固定盈利 ¥%.2f",
		estimate.CurrentCheapestMonthlyPrice,
		minimumUsers,
		req.ConfidenceLevel*100,
		riskAdjustedMeanUnits,
		req.ProfitRatePercent,
		req.TargetProfitTotalCNY,
	)

	return result, buildDashboardOversellPlanRecommendations(plans, recommendedMonthlyPrice)
}

func dashboardOversellPriceMultiplier(profitMode string, profitRatePercent float64) (float64, error) {
	if profitRatePercent < 0 {
		return 0, fmt.Errorf("盈利率不能小于 0")
	}
	rate := profitRatePercent / 100

	switch normalizeDashboardOversellProfitMode(profitMode) {
	case "markup":
		return 1 + rate, nil
	case "net_margin":
		if rate >= 1 {
			return 0, fmt.Errorf("净利率必须小于 100%%")
		}
		return 1 / (1 - rate), nil
	default:
		return 0, fmt.Errorf("不支持的盈利模式 %q", profitMode)
	}
}

func dashboardOversellRiskAdjustedMean(
	expectedMeanUnits float64,
	capacityUnits float64,
	confidenceLevel float64,
	users int,
) float64 {
	if users <= 0 || capacityUnits <= 0 {
		return expectedMeanUnits
	}
	alpha := 1 - confidenceLevel
	if alpha <= 0 || alpha >= 1 {
		alpha = 1 - dashboardOversellDefaultConfidenceLevel
	}
	buffer := capacityUnits * math.Sqrt(math.Log(1/alpha)/(2*float64(users)))
	return expectedMeanUnits + buffer
}

func dashboardOversellRequiredMonthlyPrice(
	req DashboardOversellCalculatorRequest,
	riskAdjustedMeanUnits float64,
	priceMultiplier float64,
	users int,
) float64 {
	if users <= 0 || req.CapacityUnitsPerProduct <= 0 {
		return 0
	}

	costPerUnit := req.ActualCostCNY / req.CapacityUnitsPerProduct
	basePrice := costPerUnit * riskAdjustedMeanUnits * priceMultiplier
	return basePrice + req.TargetProfitTotalCNY/float64(users)
}

func buildDashboardOversellPlanRecommendations(
	plans []dashboardOversellPlanSnapshot,
	recommendedMonthlyPrice float64,
) []DashboardOversellPlanRecommendation {
	result := make([]DashboardOversellPlanRecommendation, 0, len(plans))
	for _, plan := range plans {
		recommendedPrice := 0.0
		if recommendedMonthlyPrice > 0 {
			recommendedPrice = recommendedMonthlyPrice * plan.DurationDaysEquivalent / dashboardOversellDaysPerMonth
		}

		result = append(result, DashboardOversellPlanRecommendation{
			PlanID:                     plan.PlanID,
			GroupID:                    plan.GroupID,
			GroupName:                  plan.GroupName,
			PlanName:                   plan.PlanName,
			ValidityDays:               plan.ValidityDays,
			ValidityUnit:               plan.ValidityUnit,
			DurationDaysEquivalent:     plan.DurationDaysEquivalent,
			CurrentPriceCNY:            plan.PriceCNY,
			CurrentMonthlyPriceCNY:     plan.MonthlyPriceCNY,
			RecommendedPriceCNY:        recommendedPrice,
			RecommendedMonthlyPriceCNY: recommendedMonthlyPrice,
			PriceDeltaCNY:              recommendedPrice - plan.PriceCNY,
		})
	}
	return result
}

func oversellPlanDurationDays(validityDays int, validityUnit string) float64 {
	if validityDays <= 0 {
		validityDays = 1
	}
	unit, err := normalizePlanValidityUnit(validityUnit)
	if err != nil {
		unit = planValidityUnitDay
	}

	multiplier := 1.0
	switch unit {
	case planValidityUnitWeek:
		multiplier = 7
	case planValidityUnitMonth:
		multiplier = dashboardOversellDaysPerMonth
	case planValidityUnitYear:
		multiplier = 365
	}

	return float64(validityDays) * multiplier
}

func oversellMonthlyEquivalentPrice(priceCNY, durationDays float64) float64 {
	if priceCNY <= 0 || durationDays <= 0 {
		return 0
	}
	return priceCNY * dashboardOversellDaysPerMonth / durationDays
}

var _ = sql.ErrNoRows
