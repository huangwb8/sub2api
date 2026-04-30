package service

import (
	"math"
	"time"
)

type ResidentialIPReconciliationResult struct {
	WindowStart          time.Time `json:"window_start"`
	WindowEnd            time.Time `json:"window_end"`
	SupplierTrafficGB    float64   `json:"supplier_traffic_gb"`
	EstimatedTrafficGB   float64   `json:"estimated_traffic_gb"`
	RelativeErrorRate    float64   `json:"relative_error_rate"`
	SuggestedCalibration float64   `json:"suggested_calibration"`
	CalibrationSource    string    `json:"calibration_source"`
}

type ResidentialIPReconciliationService struct{}

func NewResidentialIPReconciliationService() *ResidentialIPReconciliationService {
	return &ResidentialIPReconciliationService{}
}

func (s *ResidentialIPReconciliationService) BuildDefaultResult() *ResidentialIPReconciliationResult {
	return defaultResidentialIPReconciliationResult()
}

func defaultResidentialIPReconciliationResult() *ResidentialIPReconciliationResult {
	windowStart := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC)
	supplierTrafficGB := 9.08
	estimatedTrafficGB := 1373947575 * dashboardOversellLegacyEstimatedBytesPerToken / (1024 * 1024 * 1024)
	relativeErrorRate := 0.0
	if supplierTrafficGB > 0 {
		relativeErrorRate = (estimatedTrafficGB - supplierTrafficGB) / supplierTrafficGB
	}

	suggestedCalibration := dashboardOversellDefaultEffectiveBytesPerToken
	if 1373947575 > 0 {
		suggestedCalibration = supplierTrafficGB * (1024 * 1024 * 1024) / 1373947575
	}

	return &ResidentialIPReconciliationResult{
		WindowStart:          windowStart,
		WindowEnd:            windowEnd,
		SupplierTrafficGB:    supplierTrafficGB,
		EstimatedTrafficGB:   roundFloat64(estimatedTrafficGB, 6),
		RelativeErrorRate:    roundFloat64(relativeErrorRate, 6),
		SuggestedCalibration: roundFloat64(suggestedCalibration, 9),
		CalibrationSource:    "supplier_reconciliation",
	}
}

func roundFloat64(value float64, places int) float64 {
	if places < 0 {
		return value
	}
	pow := math.Pow10(places)
	return math.Round(value*pow) / pow
}
