import type { ProfitabilityTrendPoint } from '@/types'

type Translate = (key: string) => string
export type ProfitabilityGranularity = 'day' | 'hour'

export interface ProfitabilityChartRange {
  granularity: ProfitabilityGranularity
  startDate: string
  endDate: string
}

export interface ProfitabilitySummary {
  revenueBalanceCNY: number
  revenueSubscriptionCNY: number
  estimatedCostCNY: number
  profitCNY: number
  extraProfitRatePercent: number | null
}

interface ProfitabilityChartDataset {
  type: 'bar' | 'line'
  label: string
  data: Array<number | null>
  borderColor: string
  backgroundColor: string
  tooltipValueType: 'amount' | 'signedAmount' | 'rate'
  yAxisID: 'yAmount' | 'yRate'
  stack?: 'revenue' | 'cost'
  order: number
  borderRadius?: number
  barPercentage?: number
  categoryPercentage?: number
  maxBarThickness?: number
  tension?: number
  fill?: boolean
  borderWidth?: number
  pointRadius?: number
  pointHoverRadius?: number
  borderDash?: number[]
  spanGaps?: boolean
}

const roundTo = (value: number, digits: number): number => {
  const factor = 10 ** digits
  return Math.round(value * factor) / factor
}

const createEmptyPoint = (date: string): ProfitabilityTrendPoint => ({
  date,
  revenue_balance_cny: 0,
  revenue_subscription_cny: 0,
  estimated_cost_cny: 0,
  profit_cny: 0,
  extra_profit_rate_percent: null
})

const parseBucketToUtc = (bucket: string, granularity: ProfitabilityGranularity): Date => {
  if (granularity === 'hour') {
    const [datePart = '', hourPart = '0'] = bucket.split(' ')
    const [year, month, day] = datePart.split('-').map(Number)
    const hour = Number(hourPart.split(':')[0] || 0)
    return new Date(Date.UTC(year, (month || 1) - 1, day || 1, hour))
  }

  const [year, month, day] = bucket.split('-').map(Number)
  return new Date(Date.UTC(year, (month || 1) - 1, day || 1))
}

const formatBucketFromUtc = (date: Date, granularity: ProfitabilityGranularity): string => {
  const year = date.getUTCFullYear()
  const month = String(date.getUTCMonth() + 1).padStart(2, '0')
  const day = String(date.getUTCDate()).padStart(2, '0')

  if (granularity === 'hour') {
    return `${year}-${month}-${day} ${String(date.getUTCHours()).padStart(2, '0')}:00`
  }

  return `${year}-${month}-${day}`
}

const addBucket = (date: Date, granularity: ProfitabilityGranularity): Date => {
  const next = new Date(date)
  if (granularity === 'hour') {
    next.setUTCHours(next.getUTCHours() + 1)
    return next
  }

  next.setUTCDate(next.getUTCDate() + 1)
  return next
}

export function normalizeProfitabilityTrend(
  trend: ProfitabilityTrendPoint[],
  range: ProfitabilityChartRange
): ProfitabilityTrendPoint[] {
  if (!trend.length) {
    return []
  }

  const pointMap = new Map(
    trend.map((point) => [
      point.date,
      {
        ...point,
        revenue_balance_cny: point.revenue_balance_cny || 0,
        revenue_subscription_cny: point.revenue_subscription_cny || 0,
        estimated_cost_cny: point.estimated_cost_cny || 0,
        profit_cny: point.profit_cny || 0,
        extra_profit_rate_percent: point.extra_profit_rate_percent ?? null
      }
    ])
  )

  const start = parseBucketToUtc(range.startDate, 'day')
  const end = parseBucketToUtc(range.endDate, 'day')
  if (range.granularity === 'hour') {
    end.setUTCHours(23, 0, 0, 0)
  }

  const normalized: ProfitabilityTrendPoint[] = []
  for (let cursor = new Date(start); cursor <= end; cursor = addBucket(cursor, range.granularity)) {
    const key = formatBucketFromUtc(cursor, range.granularity)
    normalized.push(pointMap.get(key) ?? createEmptyPoint(key))
  }

  return normalized
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
  t: Translate,
  range: ProfitabilityChartRange
) {
  const normalizedTrend = normalizeProfitabilityTrend(trend, range)
  if (!normalizedTrend.length) {
    return null
  }

  const pointRadius = normalizedTrend.length <= 1 ? 5 : normalizedTrend.length <= 7 ? 3 : 2
  const hasRateData = normalizedTrend.some(point => point.extra_profit_rate_percent != null)

  const datasets: ProfitabilityChartDataset[] = [
    {
      type: 'bar',
      label: t('admin.dashboard.profitability.balanceRevenue'),
      data: normalizedTrend.map(point => point.revenue_balance_cny),
      borderColor: '#059669',
      backgroundColor: 'rgba(5, 150, 105, 0.72)',
      tooltipValueType: 'amount',
      yAxisID: 'yAmount',
      stack: 'revenue',
      order: 3,
      borderRadius: 4,
      barPercentage: 0.72,
      categoryPercentage: 0.7,
      maxBarThickness: 18
    },
    {
      type: 'bar',
      label: t('admin.dashboard.profitability.subscriptionRevenue'),
      data: normalizedTrend.map(point => point.revenue_subscription_cny),
      borderColor: '#d97706',
      backgroundColor: 'rgba(245, 158, 11, 0.72)',
      tooltipValueType: 'amount',
      yAxisID: 'yAmount',
      stack: 'revenue',
      order: 3,
      borderRadius: 4,
      barPercentage: 0.72,
      categoryPercentage: 0.7,
      maxBarThickness: 18
    },
    {
      type: 'bar',
      label: t('admin.dashboard.profitability.estimatedCost'),
      data: normalizedTrend.map(point => point.estimated_cost_cny),
      borderColor: '#dc2626',
      backgroundColor: 'rgba(248, 113, 113, 0.7)',
      tooltipValueType: 'amount',
      yAxisID: 'yAmount',
      stack: 'cost',
      order: 2,
      borderRadius: 4,
      barPercentage: 0.72,
      categoryPercentage: 0.7,
      maxBarThickness: 18
    },
    {
      type: 'line',
      label: t('admin.dashboard.profitability.profit'),
      data: normalizedTrend.map(point => point.profit_cny),
      borderColor: '#2563eb',
      backgroundColor: 'rgba(37, 99, 235, 0.16)',
      tooltipValueType: 'signedAmount',
      yAxisID: 'yAmount',
      tension: 0.28,
      fill: false,
      borderWidth: 3,
      order: 0,
      pointRadius,
      pointHoverRadius: pointRadius + 2
    }
  ]

  if (hasRateData) {
    datasets.push({
      type: 'line',
      label: t('admin.dashboard.profitability.extraProfitRate'),
      data: normalizedTrend.map(point => point.extra_profit_rate_percent ?? null),
      borderColor: '#7c3aed',
      backgroundColor: 'rgba(124, 58, 237, 0.10)',
      tooltipValueType: 'rate',
      yAxisID: 'yRate',
      tension: 0.28,
      borderDash: [6, 4],
      borderWidth: 2,
      order: 1,
      pointRadius,
      pointHoverRadius: pointRadius + 2,
      spanGaps: true
    })
  }

  return {
    labels: normalizedTrend.map(point => point.date),
    datasets
  }
}
