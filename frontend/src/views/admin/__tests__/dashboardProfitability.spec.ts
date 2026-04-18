import { describe, expect, it } from 'vitest'

import type { ProfitabilityTrendPoint } from '@/types'
import {
  buildProfitabilityChartData,
  normalizeProfitabilityTrend,
  summarizeProfitabilityTrend
} from '../dashboardProfitability'

const label = (key: string) => key

describe('dashboardProfitability', () => {
  it('builds a chart from revenue and profit series even when extra profit rate is unavailable', () => {
    const trend: ProfitabilityTrendPoint[] = [
      {
        date: '2025-01-10',
        revenue_balance_cny: 88,
        revenue_subscription_cny: 0,
        estimated_cost_cny: 0,
        profit_cny: 88,
        extra_profit_rate_percent: null
      }
    ]

    const chartData = buildProfitabilityChartData(trend, label, {
      granularity: 'day',
      startDate: '2025-01-10',
      endDate: '2025-01-10'
    })

    expect(chartData).not.toBeNull()
    expect(chartData?.datasets.map(dataset => dataset.label)).toEqual([
      'admin.dashboard.profitability.balanceRevenue',
      'admin.dashboard.profitability.subscriptionRevenue',
      'admin.dashboard.profitability.estimatedCost',
      'admin.dashboard.profitability.profit',
      'admin.dashboard.profitability.extraProfitRate'
    ])
    expect(chartData?.datasets[0].type).toBe('bar')
    expect(chartData?.datasets[3].type).toBe('line')
    expect(chartData?.datasets[0].data).toEqual([88])
    expect(chartData?.datasets[3].data).toEqual([88])
    expect(chartData?.datasets[4].data).toEqual([null])
  })

  it('fills missing profitability buckets inside the selected range with zero-value points', () => {
    const trend: ProfitabilityTrendPoint[] = [
      {
        date: '2025-01-10',
        revenue_balance_cny: 10,
        revenue_subscription_cny: 5,
        estimated_cost_cny: 6,
        profit_cny: 9,
        extra_profit_rate_percent: 150
      },
      {
        date: '2025-01-12',
        revenue_balance_cny: 3,
        revenue_subscription_cny: 2,
        estimated_cost_cny: 1,
        profit_cny: 4,
        extra_profit_rate_percent: 400
      }
    ]

    expect(normalizeProfitabilityTrend(trend, {
      granularity: 'day',
      startDate: '2025-01-10',
      endDate: '2025-01-12'
    })).toEqual([
      trend[0],
      {
        date: '2025-01-11',
        revenue_balance_cny: 0,
        revenue_subscription_cny: 0,
        estimated_cost_cny: 0,
        profit_cny: 0,
        extra_profit_rate_percent: null
      },
      trend[1]
    ])
  })

  it('summarizes the selected range instead of only using the last profitability point', () => {
    const trend: ProfitabilityTrendPoint[] = [
      {
        date: '2025-01-10',
        revenue_balance_cny: 10,
        revenue_subscription_cny: 20,
        estimated_cost_cny: 5,
        profit_cny: 25,
        extra_profit_rate_percent: 500
      },
      {
        date: '2025-01-11',
        revenue_balance_cny: 30,
        revenue_subscription_cny: 40,
        estimated_cost_cny: 20,
        profit_cny: 50,
        extra_profit_rate_percent: 250
      }
    ]

    expect(summarizeProfitabilityTrend(trend)).toEqual({
      revenueBalanceCNY: 40,
      revenueSubscriptionCNY: 60,
      estimatedCostCNY: 25,
      profitCNY: 75,
      extraProfitRatePercent: 300
    })
  })
})
