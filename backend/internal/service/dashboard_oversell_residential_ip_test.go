package service

import (
	"database/sql"
	"math"
	"testing"
	"time"
)

func TestResidentialIPCalibration_UsesObservedUsageSample(t *testing.T) {
	lastObservedAt := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	calibration := buildResidentialIPCalibration(residentialIPCalibrationSample{
		lastObservedAt: sqlNullTime(lastObservedAt),
		observedBytes:  480000,
		observedTokens: 60000,
	})

	if calibration.Source != "usage_log_observed_proxy_bytes" {
		t.Fatalf("Source = %q, want usage_log_observed_proxy_bytes", calibration.Source)
	}
	if math.Abs(calibration.EffectiveBytesPerToken-8.0) > 0.000001 {
		t.Fatalf("EffectiveBytesPerToken = %v, want 8.0", calibration.EffectiveBytesPerToken)
	}
	if calibration.LastCalibratedAt == nil || !calibration.LastCalibratedAt.Equal(lastObservedAt) {
		t.Fatalf("LastCalibratedAt = %v, want %v", calibration.LastCalibratedAt, lastObservedAt)
	}
}

func TestResidentialIPCalibration_ClampsObservedSampleBelowDefault(t *testing.T) {
	calibration := buildResidentialIPCalibration(residentialIPCalibrationSample{
		observedBytes:  420000,
		observedTokens: 60000,
	})

	if calibration.Source != "usage_log_observed_proxy_bytes" {
		t.Fatalf("Source = %q, want usage_log_observed_proxy_bytes", calibration.Source)
	}
	if math.Abs(calibration.EffectiveBytesPerToken-dashboardOversellDefaultEffectiveBytesPerToken) > 0.000001 {
		t.Fatalf(
			"EffectiveBytesPerToken = %v, want default floor %v",
			calibration.EffectiveBytesPerToken,
			dashboardOversellDefaultEffectiveBytesPerToken,
		)
	}
}

func TestResidentialIPCalibration_FallsBackToHistoricalDefaultWhenObservedSampleIsInsufficient(t *testing.T) {
	calibration := buildResidentialIPCalibration(residentialIPCalibrationSample{
		observedBytes:  12000,
		observedTokens: 9999,
	})

	if calibration.Source != "static_default" {
		t.Fatalf("Source = %q, want static_default", calibration.Source)
	}
	if math.Abs(calibration.EffectiveBytesPerToken-dashboardOversellDefaultEffectiveBytesPerToken) > 0.000001 {
		t.Fatalf(
			"EffectiveBytesPerToken = %v, want %v",
			calibration.EffectiveBytesPerToken,
			dashboardOversellDefaultEffectiveBytesPerToken,
		)
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
		ResidentialIPCalibration{
			EffectiveBytesPerToken: 6.5,
			Source:                 "usage_log_observed_proxy_bytes",
		},
		12,
		7.2,
		now,
	)

	if estimate.CalibrationSource != "usage_log_observed_proxy_bytes" {
		t.Fatalf("CalibrationSource = %q, want usage_log_observed_proxy_bytes", estimate.CalibrationSource)
	}
	if math.Abs(estimate.EffectiveBytesPerToken-6.5) > 0.000001 {
		t.Fatalf("EffectiveBytesPerToken = %v, want 6.5", estimate.EffectiveBytesPerToken)
	}
	if estimate.TrafficBasis != "legacy_token_estimate" {
		t.Fatalf("TrafficBasis = %q, want legacy_token_estimate", estimate.TrafficBasis)
	}
	expectedGB := bytesToGB(int64(float64(window.legacyEstimatedTokens) * 6.5))
	if math.Abs(estimate.EstimatedTotalTrafficGB-expectedGB) > 0.01 {
		t.Fatalf("EstimatedTotalTrafficGB = %v, want about %v", estimate.EstimatedTotalTrafficGB, expectedGB)
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
