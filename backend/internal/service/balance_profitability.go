package service

import (
	"context"
	"log/slog"
	"time"
)

type standardBalanceChargeResolution struct {
	EstimatedCostCNY float64
	ChargeSnapshot   *UsageChargeSnapshot
	ChargeCostUSD    float64
}

func resolveProfitabilityEstimatedCostCNY(account *Account, totalCostUSD float64) (float64, bool) {
	if totalCostUSD <= 0 || account == nil || !account.HasActualCostPricing() {
		return 0, false
	}

	unitCostCNYPerUSD := account.ActualCostUnitPriceCNYPerUSD()
	if unitCostCNYPerUSD <= 0 {
		return 0, false
	}

	estimatedCostCNY := roundTo(totalCostUSD*unitCostCNYPerUSD, 8)
	if estimatedCostCNY <= 0 {
		return 0, false
	}

	return estimatedCostCNY, true
}

func resolveStandardBalanceCharge(
	ctx context.Context,
	account *Account,
	group *Group,
	now time.Time,
	totalCostUSD float64,
	fxService ExchangeRateService,
) *standardBalanceChargeResolution {
	if group == nil || fxService == nil {
		return nil
	}
	extraProfitRate := group.ResolveExtraProfitRateAt(now)
	if extraProfitRate == nil {
		return nil
	}
	if *extraProfitRate < 0 {
		return nil
	}

	estimatedCostCNY, ok := resolveProfitabilityEstimatedCostCNY(account, totalCostUSD)
	if !ok {
		return nil
	}
	chargedAmountCNY := roundTo(estimatedCostCNY*(1+*extraProfitRate/100), 8)
	if chargedAmountCNY <= 0 {
		return nil
	}

	resolvedRate, err := fxService.ResolveUSDCNYRate(ctx)
	if err != nil {
		slog.Warn("resolve standard balance profitability fx failed", "error", err)
		return nil
	}
	if resolvedRate == nil || resolvedRate.EffectiveRate <= 0 {
		return nil
	}

	chargeCostUSD := roundTo(chargedAmountCNY/resolvedRate.EffectiveRate, 10)
	if chargeCostUSD <= 0 {
		return nil
	}

	return &standardBalanceChargeResolution{
		EstimatedCostCNY: estimatedCostCNY,
		ChargeSnapshot:   BuildUsageChargeSnapshotFromCNY(chargedAmountCNY, resolvedRate),
		ChargeCostUSD:    chargeCostUSD,
	}
}
