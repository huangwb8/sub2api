package service

import "testing"

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
		ActualCostCNY:           168,
		CapacityUnitsPerProduct: 3,
		ConfidenceLevel:         0.95,
		ProfitRatePercent:       20,
		ProfitMode:              "net_margin",
		TargetProfitTotalCNY:    36,
	}
	estimate := DashboardOversellEstimate{
		EstimatedLightUserRatio:     0.74,
		CurrentCheapestMonthlyPrice: 88,
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
		ActualCostCNY:           0,
		CapacityUnitsPerProduct: 3,
		ConfidenceLevel:         0.95,
		ProfitRatePercent:       20,
		ProfitMode:              "net_margin",
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
