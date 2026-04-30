package service

import "testing"

func TestResidentialIPTrafficMeter_StreamingResponseCountsOutputBytes(t *testing.T) {
	proxyID := int64(99)
	observation := MeterResidentialIPTraffic(ResidentialIPTrafficInput{
		ProxyID:       &proxyID,
		RequestBytes:  120,
		ResponseBytes: 880,
		TotalTokens:   0,
		Calibration:   defaultResidentialIPCalibration(),
	})

	if observation.EstimateSource == nil || *observation.EstimateSource != "observed_bytes" {
		t.Fatalf("EstimateSource = %v, want observed_bytes", observation.EstimateSource)
	}
	if observation.ProxyTrafficOutputBytes == nil || *observation.ProxyTrafficOutputBytes != 880 {
		t.Fatalf("ProxyTrafficOutputBytes = %v, want 880", observation.ProxyTrafficOutputBytes)
	}
}

func TestResidentialIPTrafficMeter_FallsBackToTokenBasedEstimate(t *testing.T) {
	proxyID := int64(99)
	observation := MeterResidentialIPTraffic(ResidentialIPTrafficInput{
		ProxyID:     &proxyID,
		TotalTokens: 100,
		Calibration: defaultResidentialIPCalibration(),
	})

	if observation.EstimateSource == nil || *observation.EstimateSource != "token_estimate" {
		t.Fatalf("EstimateSource = %v, want token_estimate", observation.EstimateSource)
	}
	if observation.ProxyTrafficOverheadBytes == nil || *observation.ProxyTrafficOverheadBytes <= 0 {
		t.Fatalf("ProxyTrafficOverheadBytes = %v, want > 0", observation.ProxyTrafficOverheadBytes)
	}
}

func TestResidentialIPTrafficMeter_UnknownDoesNotSilentlyBecomeZero(t *testing.T) {
	proxyID := int64(99)
	observation := MeterResidentialIPTraffic(ResidentialIPTrafficInput{
		ProxyID: &proxyID,
	})

	if observation.EstimateSource == nil || *observation.EstimateSource != "unknown" {
		t.Fatalf("EstimateSource = %v, want unknown", observation.EstimateSource)
	}
	if observation.ProxyTrafficInputBytes != nil || observation.ProxyTrafficOutputBytes != nil || observation.ProxyTrafficOverheadBytes != nil {
		t.Fatalf("expected nil traffic bytes for unknown observation, got %+v", observation)
	}
}
