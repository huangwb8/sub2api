package service

import (
	"context"
	"log/slog"
)

type standardBalanceChargeResolution struct {
	EstimatedCostCNY float64
	ChargeSnapshot   *UsageChargeSnapshot
	ChargeCostUSD    float64
}

func resolveStandardBalanceCharge(
	ctx context.Context,
	account *Account,
	group *Group,
	totalCostUSD float64,
	fxService ExchangeRateService,
) *standardBalanceChargeResolution {
	if totalCostUSD <= 0 || account == nil || group == nil || fxService == nil {
		return nil
	}
	if !group.HasExtraProfitRateConfigured() || !account.HasActualCostPricing() {
		return nil
	}

	extraProfitRate := *group.ExtraProfitRatePercent
	if extraProfitRate < 0 {
		return nil
	}

	unitCostCNYPerUSD := account.ActualCostUnitPriceCNYPerUSD()
	if unitCostCNYPerUSD <= 0 {
		return nil
	}

	estimatedCostCNY := roundTo(totalCostUSD*unitCostCNYPerUSD, 8)
	if estimatedCostCNY <= 0 {
		return nil
	}
	chargedAmountCNY := roundTo(estimatedCostCNY*(1+extraProfitRate/100), 8)
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
