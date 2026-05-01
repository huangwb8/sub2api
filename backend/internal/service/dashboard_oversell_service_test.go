package service

import (
	"database/sql"
	"math"
	"testing"
)

func TestLoadDashboardOversellDefaults_UsesNonZeroResidentialIPPrice(t *testing.T) {
	svc := &DashboardRecommendationService{}

	defaults, err := svc.loadDashboardOversellDefaults(t.Context())
	if err != nil {
		t.Fatalf("loadDashboardOversellDefaults() error = %v", err)
	}
	if math.Abs(defaults.ResidentialIPPriceUSDPerGBMonth-dashboardOversellDefaultResidentialIPPrice) > 0.000001 {
		t.Fatalf(
			"ResidentialIPPriceUSDPerGBMonth = %v, want %v",
			defaults.ResidentialIPPriceUSDPerGBMonth,
			dashboardOversellDefaultResidentialIPPrice,
		)
	}
}

func TestDashboardOversellPriceMultiplier(t *testing.T) {
	t.Run("markup", func(t *testing.T) {
		got, err := dashboardOversellPriceMultiplier("markup", 20)
		if err != nil {
			t.Fatalf("dashboardOversellPriceMultiplier() error = %v", err)
		}
		if got != 1.2 {
			t.Fatalf("dashboardOversellPriceMultiplier() = %v, want 1.2", got)
		}
	})

	t.Run("net margin", func(t *testing.T) {
		got, err := dashboardOversellPriceMultiplier("net_margin", 20)
		if err != nil {
			t.Fatalf("dashboardOversellPriceMultiplier() error = %v", err)
		}
		if got != 1.25 {
			t.Fatalf("dashboardOversellPriceMultiplier() = %v, want 1.25", got)
		}
	})
}

func TestCalculateDashboardOversellScenario_Feasible(t *testing.T) {
	req := DashboardOversellCalculatorRequest{
		ActualCostCNY:                   168,
		ResidentialIPPriceUSDPerGBMonth: 12,
		CapacityUnitsPerProduct:         3,
		ConfidenceLevel:                 0.95,
		ProfitRatePercent:               20,
		ProfitMode:                      "net_margin",
		TargetProfitTotalCNY:            36,
	}
	estimate := DashboardOversellEstimate{
		EstimatedLightUserRatio:     0.74,
		CurrentCheapestMonthlyPrice: 88,
		ResidentialIPMonthlyCostCNY: 12,
		ResidentialIPActualDays:     7,
		ResidentialIPTotalTrafficGB: 0.03,
		ResidentialIPFXRateUSDCNY:   7.2,
	}
	plans := []dashboardOversellPlanSnapshot{
		{
			PlanID:                 1,
			GroupID:                11,
			GroupName:              "OpenAI",
			PlanName:               "月付",
			PriceCNY:               88,
			ValidityDays:           30,
			ValidityUnit:           "day",
			DurationDaysEquivalent: 30,
			MonthlyPriceCNY:        88,
		},
		{
			PlanID:                 2,
			GroupID:                11,
			GroupName:              "OpenAI",
			PlanName:               "季付",
			PriceCNY:               240,
			ValidityDays:           90,
			ValidityUnit:           "day",
			DurationDaysEquivalent: 90,
			MonthlyPriceCNY:        80,
		},
	}

	result, recommendations := calculateDashboardOversellScenario(req, estimate, plans)
	if !result.Feasible {
		t.Fatalf("Feasible = false, want true; reason = %q", result.Reason)
	}
	if result.MinimumUsers <= 0 {
		t.Fatalf("MinimumUsers = %d, want > 0", result.MinimumUsers)
	}
	if result.RecommendedMonthlyPriceCNY <= 0 {
		t.Fatalf("RecommendedMonthlyPriceCNY = %v, want > 0", result.RecommendedMonthlyPriceCNY)
	}
	if len(recommendations) != 2 {
		t.Fatalf("len(recommendations) = %d, want 2", len(recommendations))
	}
	if recommendations[0].RecommendedPriceCNY <= 0 {
		t.Fatalf("RecommendedPriceCNY = %v, want > 0", recommendations[0].RecommendedPriceCNY)
	}
}

func TestCalculateDashboardOversellScenario_RejectsMissingCost(t *testing.T) {
	req := DashboardOversellCalculatorRequest{
		ActualCostCNY:                   0,
		ResidentialIPPriceUSDPerGBMonth: 0,
		CapacityUnitsPerProduct:         3,
		ConfidenceLevel:                 0.95,
		ProfitRatePercent:               20,
		ProfitMode:                      "net_margin",
	}
	estimate := DashboardOversellEstimate{
		EstimatedLightUserRatio:     0.7,
		CurrentCheapestMonthlyPrice: 79,
	}

	result, recommendations := calculateDashboardOversellScenario(req, estimate, nil)
	if result.Feasible {
		t.Fatalf("Feasible = true, want false")
	}
	if result.Reason == "" {
		t.Fatalf("Reason = empty, want non-empty")
	}
	if len(recommendations) != 0 {
		t.Fatalf("len(recommendations) = %d, want 0", len(recommendations))
	}
}

func TestCalculateDashboardOversellScenario_AcceptsResidentialIPCostOnly(t *testing.T) {
	req := DashboardOversellCalculatorRequest{
		ActualCostCNY:                   0,
		ResidentialIPPriceUSDPerGBMonth: 12,
		CapacityUnitsPerProduct:         3,
		ConfidenceLevel:                 0.95,
		ProfitRatePercent:               20,
		ProfitMode:                      "markup",
	}
	estimate := DashboardOversellEstimate{
		EstimatedLightUserRatio:     0.74,
		CurrentCheapestMonthlyPrice: 88,
		ResidentialIPMonthlyCostCNY: 90,
		ResidentialIPActualDays:     7,
		ResidentialIPTotalTrafficGB: 0.2,
		ResidentialIPFXRateUSDCNY:   7.2,
	}

	result, _ := calculateDashboardOversellScenario(req, estimate, nil)
	if !result.Feasible {
		t.Fatalf("Feasible = false, want true; reason = %q", result.Reason)
	}
	if result.RecommendedMonthlyPriceCNY <= 0 {
		t.Fatalf("RecommendedMonthlyPriceCNY = %v, want > 0", result.RecommendedMonthlyPriceCNY)
	}
}

func TestBuildDashboardOversellPlanRecommendations_ScalesByPlanCapacity(t *testing.T) {
	plans := normalizeDashboardOversellPlanCapacityRatios([]dashboardOversellPlanSnapshot{
		{
			PlanID:                 1,
			GroupID:                11,
			GroupName:              "OpenAI Basic",
			PlanName:               "基础版",
			PriceCNY:               50,
			ValidityDays:           30,
			ValidityUnit:           "day",
			DurationDaysEquivalent: 30,
			MonthlyPriceCNY:        50,
			MonthlyQuotaUSD:        10,
			EffectiveCapacityUnits: 10,
			PricingBasis:           "monthly_limit_usd",
		},
		{
			PlanID:                 2,
			GroupID:                12,
			GroupName:              "OpenAI Pro",
			PlanName:               "高级版",
			PriceCNY:               120,
			ValidityDays:           30,
			ValidityUnit:           "day",
			DurationDaysEquivalent: 30,
			MonthlyPriceCNY:        120,
			MonthlyQuotaUSD:        30,
			EffectiveCapacityUnits: 30,
			PricingBasis:           "monthly_limit_usd",
		},
	})

	recommendations := buildDashboardOversellPlanRecommendations(plans, 40)
	if len(recommendations) != 2 {
		t.Fatalf("len(recommendations) = %d, want 2", len(recommendations))
	}
	if recommendations[0].RecommendedPriceCNY != 40 {
		t.Fatalf("basic RecommendedPriceCNY = %v, want 40", recommendations[0].RecommendedPriceCNY)
	}
	if recommendations[1].RecommendedPriceCNY != 120 {
		t.Fatalf("pro RecommendedPriceCNY = %v, want 120", recommendations[1].RecommendedPriceCNY)
	}
	if recommendations[1].RecommendedPriceCNY == recommendations[0].RecommendedPriceCNY {
		t.Fatalf("same-duration plans with different capacity should not share recommended price")
	}
	if recommendations[1].CapacityRatio != 3 {
		t.Fatalf("pro CapacityRatio = %v, want 3", recommendations[1].CapacityRatio)
	}
}

func TestDashboardOversellPlanCapacity_UsesQuotaAndRateMultiplier(t *testing.T) {
	monthlyQuota, basis := oversellPlanMonthlyQuotaUSD(
		sql.NullFloat64{Float64: 1, Valid: true},
		sql.NullFloat64{Float64: 7, Valid: true},
		sql.NullFloat64{Float64: 30, Valid: true},
	)
	if monthlyQuota != 30 {
		t.Fatalf("monthlyQuota = %v, want 30", monthlyQuota)
	}
	if basis != "monthly_limit_usd" {
		t.Fatalf("basis = %q, want monthly_limit_usd", basis)
	}

	weeklyQuota, weeklyBasis := oversellPlanMonthlyQuotaUSD(
		sql.NullFloat64{},
		sql.NullFloat64{Float64: 7, Valid: true},
		sql.NullFloat64{},
	)
	if weeklyQuota != 30 {
		t.Fatalf("weeklyQuota = %v, want 30", weeklyQuota)
	}
	if weeklyBasis != "weekly_limit_usd" {
		t.Fatalf("weeklyBasis = %q, want weekly_limit_usd", weeklyBasis)
	}

	effectiveCapacity := oversellPlanEffectiveCapacityUnits(30, 0.5)
	if effectiveCapacity != 60 {
		t.Fatalf("effectiveCapacity = %v, want 60", effectiveCapacity)
	}
}

func TestDashboardOversellResidentialIPMonthlyCostCNY(t *testing.T) {
	req := DashboardOversellCalculatorRequest{
		ActualCostCNY:                   120,
		ResidentialIPPriceUSDPerGBMonth: 10,
	}
	estimate := DashboardOversellEstimate{
		ResidentialIPActualDays:     5,
		ResidentialIPTotalTrafficGB: 2,
		ResidentialIPFXRateUSDCNY:   7.5,
	}

	if got := dashboardOversellResidentialIPMonthlyCostCNY(req, estimate); got != 900 {
		t.Fatalf("dashboardOversellResidentialIPMonthlyCostCNY() = %v, want 900", got)
	}
}
