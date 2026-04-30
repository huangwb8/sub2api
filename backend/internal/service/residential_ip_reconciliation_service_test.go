package service

import (
	"math"
	"testing"
)

func TestResidentialIPReconciliationService_DefaultResult(t *testing.T) {
	svc := NewResidentialIPReconciliationService()
	result := svc.BuildDefaultResult()
	if result == nil {
		t.Fatal("BuildDefaultResult() = nil")
	}
	if result.CalibrationSource != "supplier_reconciliation" {
		t.Fatalf("CalibrationSource = %q, want supplier_reconciliation", result.CalibrationSource)
	}
	if math.Abs(result.SupplierTrafficGB-9.08) > 0.000001 {
		t.Fatalf("SupplierTrafficGB = %v, want 9.08", result.SupplierTrafficGB)
	}
	if math.Abs(result.SuggestedCalibration-7.096031857) > 0.000001 {
		t.Fatalf("SuggestedCalibration = %v, want about 7.096031857", result.SuggestedCalibration)
	}
}
