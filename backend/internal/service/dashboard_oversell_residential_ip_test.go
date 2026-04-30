package service

import (
	"database/sql"
	"math"
	"testing"
	"time"
)

func TestResidentialIPReconciliation_FiveDaySupplierSampleGap(t *testing.T) {
	result := defaultResidentialIPReconciliationResult()
	if result == nil {
		t.Fatal("defaultResidentialIPReconciliationResult() = nil")
	}
	if result.WindowStart.Format("2006-01-02") != "2026-04-26" {
		t.Fatalf("WindowStart = %s, want 2026-04-26", result.WindowStart.Format("2006-01-02"))
	}
	if result.WindowEnd.Format("2006-01-02") != "2026-04-30" {
		t.Fatalf("WindowEnd = %s, want 2026-04-30", result.WindowEnd.Format("2006-01-02"))
	}
	if math.Abs(result.SupplierTrafficGB-9.08) > 0.000001 {
		t.Fatalf("SupplierTrafficGB = %v, want 9.08", result.SupplierTrafficGB)
	}
	if math.Abs(result.EstimatedTrafficGB-5.118354) > 0.00001 {
		t.Fatalf("EstimatedTrafficGB = %v, want about 5.118354", result.EstimatedTrafficGB)
	}
	if math.Abs(result.SuggestedCalibration-7.096031857) > 0.000001 {
		t.Fatalf("SuggestedCalibration = %v, want about 7.096031857", result.SuggestedCalibration)
	}
	if math.Abs(result.RelativeErrorRate-(-0.436304)) > 0.00001 {
		t.Fatalf("RelativeErrorRate = %v, want about -0.436304", result.RelativeErrorRate)
	}
}

func TestResidentialIPEstimate_DistinguishesPricingAndSiteScopes(t *testing.T) {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	window := residentialIPUsageWindow{
		earliestUsageAt:      sqlNullTime(time.Date(2026, 4, 26, 8, 0, 0, 0, time.UTC)),
		involvedUsers:        18,
		observedTrafficBytes: 2 * 1024 * 1024 * 1024,
	}
	calibration := defaultResidentialIPCalibration()

	pricing := buildResidentialIPEstimate(ResidentialIPScopePricing, false, window, calibration, 12, 7.2, now)
	site := buildResidentialIPEstimate(ResidentialIPScopeSite, true, window, calibration, 12, 7.2, now)

	if pricing.Scope != ResidentialIPScopePricing || pricing.IncludesAdmin {
		t.Fatalf("pricing scope = %+v, want pricing without admin", pricing)
	}
	if site.Scope != ResidentialIPScopeSite || !site.IncludesAdmin {
		t.Fatalf("site scope = %+v, want site with admin", site)
	}
	if pricing.EstimatedTotalTrafficGB != site.EstimatedTotalTrafficGB {
		t.Fatalf("EstimatedTotalTrafficGB mismatch: pricing=%v site=%v", pricing.EstimatedTotalTrafficGB, site.EstimatedTotalTrafficGB)
	}
}

func TestResidentialIPEstimate_ReportsCalibrationMetadata(t *testing.T) {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	window := residentialIPUsageWindow{
		earliestUsageAt:       sqlNullTime(time.Date(2026, 4, 26, 8, 0, 0, 0, time.UTC)),
		involvedUsers:         18,
		legacyEstimatedTokens: 1373947575,
	}

	estimate := buildResidentialIPEstimate(
		ResidentialIPScopePricing,
		false,
		window,
		defaultResidentialIPCalibration(),
		12,
		7.2,
		now,
	)

	if estimate.CalibrationSource != "supplier_reconciliation" {
		t.Fatalf("CalibrationSource = %q, want supplier_reconciliation", estimate.CalibrationSource)
	}
	if math.Abs(estimate.EffectiveBytesPerToken-7.096031856906913) > 0.000001 {
		t.Fatalf("EffectiveBytesPerToken = %v, want about 7.0960318569", estimate.EffectiveBytesPerToken)
	}
	if estimate.TrafficBasis != "legacy_token_estimate" {
		t.Fatalf("TrafficBasis = %q, want legacy_token_estimate", estimate.TrafficBasis)
	}
	if math.Abs(estimate.EstimatedTotalTrafficGB-9.08) > 0.01 {
		t.Fatalf("EstimatedTotalTrafficGB = %v, want about 9.08", estimate.EstimatedTotalTrafficGB)
	}
}

func TestResidentialIPEstimate_SeparatesObservedAndEstimatedTrafficBytes(t *testing.T) {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	window := residentialIPUsageWindow{
		earliestUsageAt:       sqlNullTime(time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC)),
		involvedUsers:         3,
		observedTrafficBytes:  1024,
		estimatedTrafficBytes: 512,
	}

	estimate := buildResidentialIPEstimate(
		ResidentialIPScopePricing,
		false,
		window,
		defaultResidentialIPCalibration(),
		12,
		7.2,
		now,
	)

	if estimate.ObservedTrafficBytes != 1024 {
		t.Fatalf("ObservedTrafficBytes = %d, want 1024", estimate.ObservedTrafficBytes)
	}
	if estimate.EstimatedTrafficBytes != 512 {
		t.Fatalf("EstimatedTrafficBytes = %d, want 512", estimate.EstimatedTrafficBytes)
	}
	if estimate.TrafficBasis != "usage_log_observed_proxy_bytes_with_token_fallback" {
		t.Fatalf("TrafficBasis = %q, want usage_log_observed_proxy_bytes_with_token_fallback", estimate.TrafficBasis)
	}
}

func TestApplyLegacyResidentialIPEstimate_UsesFXSnapshotMetadata(t *testing.T) {
	var estimate DashboardOversellEstimate
	residentialIP := &ResidentialIPEstimate{
		ActualDays:                      6,
		InvolvedUsers:                   12,
		EstimatedTotalTrafficGB:         1.8,
		EstimatedMonthlyCostUSD:         108,
		EstimatedMonthlyCostCNY:         777.6,
		ResidentialIPPriceUSDPerGBMonth: 12,
		TrafficBasis:                    "usage_log_observed_proxy_bytes_with_token_fallback",
	}

	applyLegacyResidentialIPEstimate(&estimate, residentialIP, residentialIPFXSnapshot{
		rate:   7.2,
		source: "fallback_floor",
	})

	if estimate.ResidentialIPFXRateUSDCNY != 7.2 {
		t.Fatalf("ResidentialIPFXRateUSDCNY = %v, want 7.2", estimate.ResidentialIPFXRateUSDCNY)
	}
	if estimate.ResidentialIPFXRateSource != "fallback_floor" {
		t.Fatalf("ResidentialIPFXRateSource = %q, want fallback_floor", estimate.ResidentialIPFXRateSource)
	}
}

func sqlNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}
