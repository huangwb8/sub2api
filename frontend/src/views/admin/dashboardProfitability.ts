import type { ProfitabilityTrendPoint } from '@/types'

type Translate = (key: string) => string

export interface ProfitabilitySummary {
  revenueBalanceCNY: number
  revenueSubscriptionCNY: number
  estimatedCostCNY: number
  profitCNY: number
  extraProfitRatePercent: number | null
}

const roundTo = (value: number, digits: number): number => {
  const factor = 10 ** digits
  return Math.round(value * factor) / factor
}

export function summarizeProfitabilityTrend(
  trend: ProfitabilityTrendPoint[]
): ProfitabilitySummary {
  const summary = trend.reduce<ProfitabilitySummary>(
    (acc, point) => {
      acc.revenueBalanceCNY += point.revenue_balance_cny || 0
      acc.revenueSubscriptionCNY += point.revenue_subscription_cny || 0
      acc.estimatedCostCNY += point.estimated_cost_cny || 0
      acc.profitCNY += point.profit_cny || 0
      return acc
    },
    {
      revenueBalanceCNY: 0,
      revenueSubscriptionCNY: 0,
      estimatedCostCNY: 0,
      profitCNY: 0,
      extraProfitRatePercent: null
    }
  )

  summary.revenueBalanceCNY = roundTo(summary.revenueBalanceCNY, 8)
  summary.revenueSubscriptionCNY = roundTo(summary.revenueSubscriptionCNY, 8)
  summary.estimatedCostCNY = roundTo(summary.estimatedCostCNY, 8)
  summary.profitCNY = roundTo(summary.profitCNY, 8)

  if (summary.estimatedCostCNY > 0) {
    summary.extraProfitRatePercent = roundTo((summary.profitCNY / summary.estimatedCostCNY) * 100, 4)
  }

  return summary
}

export function buildProfitabilityChartData(
  trend: ProfitabilityTrendPoint[],
  t: Translate
) {
  if (!trend.length) {
    return null
  }

  return {
    labels: trend.map(point => point.date),
    datasets: [
      {
        label: t('admin.dashboard.profitability.balanceRevenue'),
        data: trend.map(point => point.revenue_balance_cny),
        borderColor: '#0f766e',
        backgroundColor: 'rgba(15, 118, 110, 0.12)',
        tooltipValueType: 'amount',
        yAxisID: 'yAmount',
        tension: 0.25,
        pointRadius: 1.5,
        pointHoverRadius: 4
      },
      {
        label: t('admin.dashboard.profitability.subscriptionRevenue'),
        data: trend.map(point => point.revenue_subscription_cny),
        borderColor: '#d97706',
        backgroundColor: 'rgba(217, 119, 6, 0.12)',
        tooltipValueType: 'amount',
        yAxisID: 'yAmount',
        tension: 0.25,
        pointRadius: 1.5,
        pointHoverRadius: 4
      },
      {
        label: t('admin.dashboard.profitability.estimatedCost'),
        data: trend.map(point => point.estimated_cost_cny),
        borderColor: '#dc2626',
        backgroundColor: 'rgba(220, 38, 38, 0.12)',
        tooltipValueType: 'amount',
        yAxisID: 'yAmount',
        tension: 0.25,
        pointRadius: 1.5,
        pointHoverRadius: 4
      },
      {
        label: t('admin.dashboard.profitability.profit'),
        data: trend.map(point => point.profit_cny),
        borderColor: '#2563eb',
        backgroundColor: 'rgba(37, 99, 235, 0.10)',
        tooltipValueType: 'signedAmount',
        yAxisID: 'yAmount',
        tension: 0.25,
        borderWidth: 2.5,
        pointRadius: 1.5,
        pointHoverRadius: 4
      },
      {
        label: t('admin.dashboard.profitability.extraProfitRate'),
        data: trend.map(point => point.extra_profit_rate_percent ?? null),
        borderColor: '#7c3aed',
        backgroundColor: 'rgba(124, 58, 237, 0.10)',
        tooltipValueType: 'rate',
        yAxisID: 'yRate',
        tension: 0.25,
        borderDash: [6, 4],
        pointRadius: 1.5,
        pointHoverRadius: 4,
        spanGaps: true
      }
    ]
  }
}
