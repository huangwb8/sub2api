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
	dashboardOversellDefaultEffectiveBytesPerToken = 7.096031856906913
	dashboardOversellLegacyEstimatedBytesPerToken  = 4.0
	dashboardOversellCalibrationLookbackDays       = 30
	dashboardOversellCalibrationMinObservedTokens  = 10000
)

type ResidentialIPScope string

const (
	ResidentialIPScopePricing ResidentialIPScope = "pricing"
	ResidentialIPScopeSite    ResidentialIPScope = "site"
)

type ResidentialIPCalibration struct {
	EffectiveBytesPerToken float64    `json:"effective_bytes_per_token"`
	Source                 string     `json:"source"`
	LastCalibratedAt       *time.Time `json:"last_calibrated_at,omitempty"`
}

type ResidentialIPEstimate struct {
	Scope                           ResidentialIPScope `json:"scope"`
	IncludesAdmin                   bool               `json:"includes_admin"`
	IncludesFailedRequests          bool               `json:"includes_failed_requests"`
	IncludesProbeTraffic            bool               `json:"includes_probe_traffic"`
	ActualDays                      int                `json:"actual_days"`
	InvolvedUsers                   int                `json:"involved_users"`
	EstimatedTotalTrafficGB         float64            `json:"estimated_total_traffic_gb"`
	EstimatedMonthlyTrafficGB       float64            `json:"estimated_monthly_traffic_gb"`
	EstimatedMonthlyCostUSD         float64            `json:"estimated_monthly_cost_usd"`
	EstimatedMonthlyCostCNY         float64            `json:"estimated_monthly_cost_cny"`
	ResidentialIPPriceUSDPerGBMonth float64            `json:"residential_ip_price_usd_per_gb_month"`
	EffectiveBytesPerToken          float64            `json:"effective_bytes_per_token"`
	CalibrationSource               string             `json:"calibration_source"`
	TrafficBasis                    string             `json:"traffic_basis"`
	ObservedTrafficBytes            int64              `json:"observed_traffic_bytes"`
	EstimatedTrafficBytes           int64              `json:"estimated_traffic_bytes"`
}

type residentialIPFXSnapshot struct {
	rate   float64
	source string
}

type residentialIPUsageWindow struct {
	earliestUsageAt       sql.NullTime
	involvedUsers         int
	observedTrafficBytes  int64
	estimatedTrafficBytes int64
	legacyEstimatedTokens int64
}

type residentialIPCalibrationSample struct {
	lastObservedAt sql.NullTime
	observedBytes  int64
	observedTokens int64
}

func (s *DashboardRecommendationService) estimateResidentialIPScopes(
	ctx context.Context,
	residentialIPPriceUSDPerGBMonth float64,
) ([]ResidentialIPEstimate, *ResidentialIPReconciliationResult, residentialIPFXSnapshot, error) {
	fxSnapshot := residentialIPFXSnapshot{
		rate:   dashboardOversellFallbackUSDCNYRate,
		source: "fallback_floor",
	}
	if s != nil && s.exchangeRateService != nil {
		if resolved, err := s.exchangeRateService.ResolveUSDCNYRate(ctx); err == nil && resolved != nil && resolved.EffectiveRate > 0 {
			fxSnapshot.rate = resolved.EffectiveRate
			if strings.TrimSpace(resolved.Source) != "" {
				fxSnapshot.source = resolved.Source
			}
		}
	}

	now := time.Now().UTC()
	windowEnd := now
	windowStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).
		AddDate(0, 0, -(dashboardOversellResidentialIPLookbackDays - 1))
	calibrationWindowStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).
		AddDate(0, 0, -(dashboardOversellCalibrationLookbackDays - 1))
	calibration, err := s.loadResidentialIPCalibration(ctx, calibrationWindowStart, windowEnd)
	if err != nil {
		return nil, nil, residentialIPFXSnapshot{}, err
	}

	scopes := []struct {
		scope         ResidentialIPScope
		includesAdmin bool
	}{
		{scope: ResidentialIPScopePricing, includesAdmin: false},
		{scope: ResidentialIPScopeSite, includesAdmin: true},
	}

	estimates := make([]ResidentialIPEstimate, 0, len(scopes))
	for _, item := range scopes {
		usageWindow, err := s.loadResidentialIPUsageWindow(ctx, windowStart, windowEnd, item.includesAdmin)
		if err != nil {
			return nil, nil, residentialIPFXSnapshot{}, err
		}
		estimate := buildResidentialIPEstimate(
			item.scope,
			item.includesAdmin,
			usageWindow,
			calibration,
			residentialIPPriceUSDPerGBMonth,
			fxSnapshot.rate,
			now,
		)
		estimates = append(estimates, estimate)
	}

	return estimates, nil, fxSnapshot, nil
}

func (s *DashboardRecommendationService) loadResidentialIPCalibration(
	ctx context.Context,
	windowStart time.Time,
	windowEnd time.Time,
) (ResidentialIPCalibration, error) {
	calibration := defaultResidentialIPCalibration()
	if s == nil || s.db == nil {
		return calibration, nil
	}

	sample := residentialIPCalibrationSample{}
	query := `
SELECT
	MAX(ul.created_at) AS last_observed_at,
	COALESCE(SUM(
		CASE
			WHEN (
				COALESCE(ul.input_tokens, 0) +
				COALESCE(ul.output_tokens, 0) +
				COALESCE(ul.cache_creation_tokens, 0) +
				COALESCE(ul.cache_read_tokens, 0)
			) > 0
			AND (
				ul.proxy_traffic_input_bytes IS NOT NULL
				OR ul.proxy_traffic_output_bytes IS NOT NULL
			)
			THEN
				COALESCE(ul.proxy_traffic_input_bytes, 0) +
				COALESCE(ul.proxy_traffic_output_bytes, 0)
			ELSE 0
		END
	), 0) AS observed_bytes,
	COALESCE(SUM(
		CASE
			WHEN (
				COALESCE(ul.input_tokens, 0) +
				COALESCE(ul.output_tokens, 0) +
				COALESCE(ul.cache_creation_tokens, 0) +
				COALESCE(ul.cache_read_tokens, 0)
			) > 0
			AND (
				ul.proxy_traffic_input_bytes IS NOT NULL
				OR ul.proxy_traffic_output_bytes IS NOT NULL
			)
			THEN
				COALESCE(ul.input_tokens, 0) +
				COALESCE(ul.output_tokens, 0) +
				COALESCE(ul.cache_creation_tokens, 0) +
				COALESCE(ul.cache_read_tokens, 0)
			ELSE 0
		END
	), 0) AS observed_tokens
FROM usage_logs ul
LEFT JOIN accounts a ON a.id = ul.account_id
WHERE ul.created_at >= $1
  AND ul.created_at < $2
  AND (
    CASE
      WHEN ul.used_residential_proxy IS NOT NULL THEN ul.used_residential_proxy
      WHEN a.deleted_at IS NULL AND a.proxy_id IS NOT NULL THEN TRUE
      ELSE FALSE
    END
  )
`

	if err := s.db.QueryRowContext(ctx, query, windowStart, windowEnd).Scan(
		&sample.lastObservedAt,
		&sample.observedBytes,
		&sample.observedTokens,
	); err != nil {
		return calibration, fmt.Errorf("query residential ip calibration: %w", err)
	}

	return buildResidentialIPCalibration(sample), nil
}

func (s *DashboardRecommendationService) loadResidentialIPUsageWindow(
	ctx context.Context,
	windowStart time.Time,
	windowEnd time.Time,
	includesAdmin bool,
) (residentialIPUsageWindow, error) {
	window := residentialIPUsageWindow{}
	if s == nil || s.db == nil {
		return window, nil
	}

	query := `
SELECT
	MIN(ul.created_at) AS earliest_usage_at,
	COUNT(DISTINCT ul.user_id) AS involved_users,
	COALESCE(SUM(
		COALESCE(ul.proxy_traffic_input_bytes, 0) +
		COALESCE(ul.proxy_traffic_output_bytes, 0)
	), 0) AS observed_traffic_bytes,
	COALESCE(SUM(
		CASE
			WHEN ul.proxy_traffic_estimate_source = 'token_estimate'
			  OR ul.proxy_traffic_estimate_source = 'mixed_observed_and_token_estimate'
			THEN COALESCE(ul.proxy_traffic_overhead_bytes, 0)
			ELSE 0
		END
	), 0) AS estimated_traffic_bytes,
	COALESCE(SUM(
		CASE
			WHEN ul.proxy_traffic_estimate_source IS NULL
			 AND ul.proxy_traffic_input_bytes IS NULL
			 AND ul.proxy_traffic_output_bytes IS NULL
			 AND ul.proxy_traffic_overhead_bytes IS NULL
			THEN
				COALESCE(ul.input_tokens, 0) +
				COALESCE(ul.output_tokens, 0) +
				COALESCE(ul.cache_creation_tokens, 0) +
				COALESCE(ul.cache_read_tokens, 0)
			ELSE 0
		END
	), 0) AS legacy_estimated_tokens
FROM usage_logs ul
JOIN users u ON u.id = ul.user_id
LEFT JOIN accounts a ON a.id = ul.account_id
WHERE ul.created_at >= $1
  AND ul.created_at < $2
  AND (
    CASE
      WHEN ul.used_residential_proxy IS NOT NULL THEN ul.used_residential_proxy
      WHEN a.deleted_at IS NULL AND a.proxy_id IS NOT NULL THEN TRUE
      ELSE FALSE
    END
  )
  AND ($3 OR COALESCE(u.role, '') <> 'admin')
`

	if err := s.db.QueryRowContext(ctx, query, windowStart, windowEnd, includesAdmin).Scan(
		&window.earliestUsageAt,
		&window.involvedUsers,
		&window.observedTrafficBytes,
		&window.estimatedTrafficBytes,
		&window.legacyEstimatedTokens,
	); err != nil {
		return window, fmt.Errorf("query residential ip usage window: %w", err)
	}

	return window, nil
}

func buildResidentialIPEstimate(
	scope ResidentialIPScope,
	includesAdmin bool,
	window residentialIPUsageWindow,
	calibration ResidentialIPCalibration,
	priceUSDPerGBMonth float64,
	fxRateUSDCNY float64,
	now time.Time,
) ResidentialIPEstimate {
	estimate := ResidentialIPEstimate{
		Scope:                           scope,
		IncludesAdmin:                   includesAdmin,
		IncludesFailedRequests:          false,
		IncludesProbeTraffic:            false,
		ResidentialIPPriceUSDPerGBMonth: maxFloat64(priceUSDPerGBMonth, 0),
		EffectiveBytesPerToken:          maxFloat64(calibration.EffectiveBytesPerToken, 0),
		CalibrationSource:               strings.TrimSpace(calibration.Source),
		ObservedTrafficBytes:            maxInt64(window.observedTrafficBytes, 0),
	}
	if estimate.CalibrationSource == "" {
		estimate.CalibrationSource = "static_default"
	}

	if window.involvedUsers <= 0 || !window.earliestUsageAt.Valid {
		estimate.TrafficBasis = "unknown"
		return estimate
	}

	estimate.InvolvedUsers = window.involvedUsers
	startDay := time.Date(
		window.earliestUsageAt.Time.UTC().Year(),
		window.earliestUsageAt.Time.UTC().Month(),
		window.earliestUsageAt.Time.UTC().Day(),
		0, 0, 0, 0,
		time.UTC,
	)
	windowStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).
		AddDate(0, 0, -(dashboardOversellResidentialIPLookbackDays - 1))
	if startDay.Before(windowStart) {
		startDay = windowStart
	}
	endDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	actualDays := int(endDay.Sub(startDay).Hours()/24) + 1
	if actualDays < 1 {
		actualDays = 1
	}
	if actualDays > dashboardOversellResidentialIPLookbackDays {
		actualDays = dashboardOversellResidentialIPLookbackDays
	}
	estimate.ActualDays = actualDays

	usageLogEstimatedTrafficBytes := maxInt64(window.estimatedTrafficBytes, 0)
	legacyEstimatedTrafficBytes := int64(0)
	if maxInt64(window.legacyEstimatedTokens, 0) > 0 && estimate.EffectiveBytesPerToken > 0 {
		legacyEstimatedTrafficBytes = int64(float64(window.legacyEstimatedTokens) * estimate.EffectiveBytesPerToken)
	}
	estimate.EstimatedTrafficBytes = usageLogEstimatedTrafficBytes + legacyEstimatedTrafficBytes

	totalTrafficBytes := estimate.ObservedTrafficBytes + estimate.EstimatedTrafficBytes
	if totalTrafficBytes <= 0 {
		estimate.TrafficBasis = "unknown"
		return estimate
	}

	switch {
	case estimate.ObservedTrafficBytes > 0 && usageLogEstimatedTrafficBytes > 0 && legacyEstimatedTrafficBytes > 0:
		estimate.TrafficBasis = "usage_log_observed_proxy_bytes_with_token_and_legacy_fallback"
	case estimate.ObservedTrafficBytes > 0 && usageLogEstimatedTrafficBytes > 0:
		estimate.TrafficBasis = "usage_log_observed_proxy_bytes_with_token_fallback"
	case estimate.ObservedTrafficBytes > 0 && legacyEstimatedTrafficBytes > 0:
		estimate.TrafficBasis = "usage_log_observed_proxy_bytes_with_legacy_token_fallback"
	case estimate.ObservedTrafficBytes > 0:
		estimate.TrafficBasis = "usage_log_observed_proxy_bytes"
	case usageLogEstimatedTrafficBytes > 0 && legacyEstimatedTrafficBytes > 0:
		estimate.TrafficBasis = "usage_log_token_estimate_with_legacy_fallback"
	case usageLogEstimatedTrafficBytes > 0:
		estimate.TrafficBasis = "usage_log_token_estimate"
	default:
		estimate.TrafficBasis = "legacy_token_estimate"
	}

	estimate.EstimatedTotalTrafficGB = bytesToGB(totalTrafficBytes)
	if estimate.ActualDays > 0 {
		estimate.EstimatedMonthlyTrafficGB = estimate.EstimatedTotalTrafficGB / float64(estimate.ActualDays) * dashboardOversellDaysPerMonth
	}
	if estimate.ResidentialIPPriceUSDPerGBMonth > 0 && estimate.EstimatedMonthlyTrafficGB > 0 {
		estimate.EstimatedMonthlyCostUSD = estimate.EstimatedMonthlyTrafficGB * estimate.ResidentialIPPriceUSDPerGBMonth
		estimate.EstimatedMonthlyCostCNY = estimate.EstimatedMonthlyCostUSD * maxFloat64(fxRateUSDCNY, dashboardOversellFallbackUSDCNYRate)
	}

	return estimate
}

func defaultResidentialIPCalibration() ResidentialIPCalibration {
	return ResidentialIPCalibration{
		EffectiveBytesPerToken: dashboardOversellDefaultEffectiveBytesPerToken,
		Source:                 "static_default",
	}
}

func buildResidentialIPCalibration(sample residentialIPCalibrationSample) ResidentialIPCalibration {
	calibration := defaultResidentialIPCalibration()
	if sample.observedTokens < dashboardOversellCalibrationMinObservedTokens || sample.observedBytes <= 0 {
		return calibration
	}

	value := float64(sample.observedBytes) / float64(sample.observedTokens)
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return calibration
	}
	if value < dashboardOversellDefaultEffectiveBytesPerToken {
		value = dashboardOversellDefaultEffectiveBytesPerToken
	}
	if value > 64 {
		value = 64
	}

	calibration.EffectiveBytesPerToken = value
	calibration.Source = "usage_log_observed_proxy_bytes"
	if sample.lastObservedAt.Valid {
		last := sample.lastObservedAt.Time.UTC()
		calibration.LastCalibratedAt = &last
	}
	return calibration
}

func bytesToGB(totalBytes int64) float64 {
	if totalBytes <= 0 {
		return 0
	}
	return float64(totalBytes) / (1024 * 1024 * 1024)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
