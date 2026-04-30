package service

type ResidentialIPTrafficInput struct {
	ProxyID       *int64
	RequestBytes  int64
	ResponseBytes int64
	TotalTokens   int
	Calibration   ResidentialIPCalibration
}

type ResidentialIPTrafficObservation struct {
	UsedResidentialProxy      *bool
	ProxyID                   *int64
	ProxyTrafficInputBytes    *int64
	ProxyTrafficOutputBytes   *int64
	ProxyTrafficOverheadBytes *int64
	EstimateSource            *string
}

func MeterResidentialIPTraffic(input ResidentialIPTrafficInput) ResidentialIPTrafficObservation {
	if input.ProxyID == nil || *input.ProxyID <= 0 {
		used := false
		source := "not_applicable"
		return ResidentialIPTrafficObservation{
			UsedResidentialProxy: &used,
			EstimateSource:       &source,
		}
	}

	used := true
	observation := ResidentialIPTrafficObservation{
		UsedResidentialProxy: &used,
		ProxyID:              input.ProxyID,
	}

	requestBytes := maxInt64(input.RequestBytes, 0)
	responseBytes := maxInt64(input.ResponseBytes, 0)
	observedBytes := requestBytes + responseBytes

	if requestBytes > 0 {
		observation.ProxyTrafficInputBytes = &requestBytes
	}
	if responseBytes > 0 {
		observation.ProxyTrafficOutputBytes = &responseBytes
	}

	estimatedTotalBytes := int64(0)
	if input.TotalTokens > 0 && input.Calibration.EffectiveBytesPerToken > 0 {
		estimatedTotalBytes = int64(float64(input.TotalTokens) * input.Calibration.EffectiveBytesPerToken)
	}

	switch {
	case observedBytes > 0 && estimatedTotalBytes <= observedBytes:
		source := "observed_bytes"
		observation.EstimateSource = &source
		overheadBytes := int64(0)
		observation.ProxyTrafficOverheadBytes = &overheadBytes
	case observedBytes > 0 && estimatedTotalBytes > observedBytes:
		source := "mixed_observed_and_token_estimate"
		observation.EstimateSource = &source
		overheadBytes := estimatedTotalBytes - observedBytes
		observation.ProxyTrafficOverheadBytes = &overheadBytes
	case estimatedTotalBytes > 0:
		source := "token_estimate"
		observation.EstimateSource = &source
		overheadBytes := estimatedTotalBytes
		observation.ProxyTrafficOverheadBytes = &overheadBytes
	default:
		source := "unknown"
		observation.EstimateSource = &source
	}

	return observation
}

func applyResidentialIPTrafficObservation(log *UsageLog, observation ResidentialIPTrafficObservation) {
	if log == nil {
		return
	}
	log.ProxyID = observation.ProxyID
	log.UsedResidentialProxy = observation.UsedResidentialProxy
	log.ProxyTrafficInputBytes = observation.ProxyTrafficInputBytes
	log.ProxyTrafficOutputBytes = observation.ProxyTrafficOutputBytes
	log.ProxyTrafficOverheadBytes = observation.ProxyTrafficOverheadBytes
	log.ProxyTrafficEstimateSource = observation.EstimateSource
}

func totalUsageTokens(log *UsageLog) int {
	if log == nil {
		return 0
	}
	return log.InputTokens + log.OutputTokens + log.CacheCreationTokens + log.CacheReadTokens
}
