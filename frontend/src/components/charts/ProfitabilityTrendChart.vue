<template>
  <div class="h-full">
    <div v-if="loading" class="flex h-full items-center justify-center">
      <LoadingSpinner size="md" />
    </div>
    <Bar v-else-if="chartData" :data="chartData" :options="options" />
    <div
      v-else
      class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  type ChartData,
  Filler,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip
} from 'chart.js'
import { Bar } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { ProfitabilityTrendPoint } from '@/types'
import {
  buildProfitabilityChartData,
  type ProfitabilityGranularity
} from '@/views/admin/dashboardProfitability'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  Tooltip,
  Legend,
  Filler
)

const props = defineProps<{
  trendData: ProfitabilityTrendPoint[]
  loading?: boolean
  granularity: ProfitabilityGranularity
  startDate: string
  endDate: string
}>()

const { t } = useI18n()

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

const colors = computed(() => ({
  text: isDarkMode.value ? '#d1d5db' : '#4b5563',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  rate: '#7c3aed'
}))

const chartData = computed<ChartData<'bar'> | null>(() => buildProfitabilityChartData(props.trendData, t, {
  granularity: props.granularity,
  startDate: props.startDate,
  endDate: props.endDate
}) as ChartData<'bar'> | null)

const hasRateDataset = computed(() =>
  Boolean(chartData.value?.datasets.some(dataset => (dataset as { yAxisID?: string }).yAxisID === 'yRate'))
)

const formatCost = (value: number): string => {
  if (value >= 1000) {
    return `${(value / 1000).toFixed(2)}K`
  }
  if (value >= 1) {
    return value.toFixed(2)
  }
  if (value >= 0.01) {
    return value.toFixed(3)
  }
  return value.toFixed(4)
}

const formatCny = (value: number): string => `¥${formatCost(value)}`

const formatSignedCny = (value: number): string => `${value >= 0 ? '+' : '-'}${formatCny(Math.abs(value))}`

const formatRate = (value: number | null | undefined): string => {
  if (value == null || Number.isNaN(value)) {
    return '--'
  }
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
}

const options = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: colors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 14,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          const rawValue = context?.raw
          const numericValue = typeof rawValue === 'number'
            ? rawValue
            : rawValue == null
              ? null
              : Number(context?.parsed?.y ?? rawValue)

          if (context?.dataset?.tooltipValueType === 'rate') {
            return `${context.dataset.label}: ${formatRate(numericValue)}`
          }

          if (context?.dataset?.tooltipValueType === 'signedAmount') {
            return `${context.dataset.label}: ${formatSignedCny(numericValue ?? 0)}`
          }

          return `${context.dataset.label}: ${formatCny(numericValue ?? 0)}`
        },
        footer: (items: any[]) => {
          if (!items.length) {
            return ''
          }

          const totalRevenue = items.reduce((sum, item) => {
            if (item?.dataset?.label === t('admin.dashboard.profitability.balanceRevenue')) {
              return sum + Number(item.raw || 0)
            }
            if (item?.dataset?.label === t('admin.dashboard.profitability.subscriptionRevenue')) {
              return sum + Number(item.raw || 0)
            }
            return sum
          }, 0)

          return `${t('common.total')}: ${formatCny(totalRevenue)}`
        }
      }
    }
  },
  scales: {
    x: {
      stacked: true,
      grid: {
        display: false
      },
      ticks: {
        color: colors.value.text,
        maxRotation: 0,
        autoSkip: true,
        font: {
          size: 10
        }
      }
    },
    yAmount: {
      stacked: true,
      beginAtZero: true,
      grid: {
        color: colors.value.grid,
        borderDash: [4, 4]
      },
      ticks: {
        color: colors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatCny(Number(value))
      }
    },
    yRate: {
      display: hasRateDataset.value,
      position: 'right' as const,
      grid: {
        drawOnChartArea: false
      },
      ticks: {
        color: colors.value.rate,
        font: {
          size: 10
        },
        callback: (value: string | number) => `${Number(value).toFixed(0)}%`
      }
    }
  }
}))
</script>
