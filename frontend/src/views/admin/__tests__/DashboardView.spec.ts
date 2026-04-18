import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { DashboardStats } from '@/types'
import DashboardView from '../DashboardView.vue'

const { getSnapshotV2, getUserUsageTrend, getUserSpendingRanking, getProfitabilityTrend, getProfitabilityBounds, getRecommendations } = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  getUserUsageTrend: vi.fn(),
  getUserSpendingRanking: vi.fn(),
  getProfitabilityTrend: vi.fn(),
  getProfitabilityBounds: vi.fn(),
  getRecommendations: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getSnapshotV2,
      getUserUsageTrend,
      getUserSpendingRanking,
      getProfitabilityTrend,
      getProfitabilityBounds,
      getRecommendations
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.dashboard.recommendations.title': '加号推荐',
    'admin.dashboard.recommendations.description': '描述',
    'admin.dashboard.recommendations.pool': '容量池',
    'admin.dashboard.recommendations.contributors': '涉及套餐',
    'admin.dashboard.recommendations.status': '状态',
    'admin.dashboard.recommendations.current': '当前可调度 / 总账号',
    'admin.dashboard.recommendations.recommended': '建议可调度账号',
    'admin.dashboard.recommendations.gap': '缺口',
    'admin.dashboard.recommendations.utilization': '容量利用率',
    'admin.dashboard.recommendations.reason': '推荐理由',
    'admin.dashboard.recommendations.poolsAndGroups': '评估 {pools} 个容量池 / {groups} 个订阅分组',
    'admin.dashboard.recommendations.toAddSchedulable': '全站建议补充 {count} 个可调度账号',
    'admin.dashboard.recommendations.recoverable': '可优先恢复现有不可调度账号 {count} 个',
    'admin.dashboard.recommendations.urgent': '需优先处理 {count} 个容量池',
    'admin.dashboard.recommendations.subscriptions': '{count} 个活跃订阅',
    'admin.dashboard.recommendations.addSchedulableCount': '建议补充 {count} 个可调度账号',
    'admin.dashboard.recommendations.recoverableInline': '可先恢复 {count} 个不可调度账号',
    'admin.dashboard.recommendations.newAccountsInline': '预计新增 {count} 个账号',
    'admin.dashboard.recommendations.noAction': '无需动作',
    'admin.dashboard.recommendations.projectedCost': '预计日负载 ${amount}',
    'admin.dashboard.recommendations.empty': '暂无推荐',
    'admin.dashboard.recommendations.statusMap.healthy': '健康',
    'admin.dashboard.recommendations.statusMap.watch': '观察',
    'admin.dashboard.recommendations.statusMap.action': '行动'
  }

  const interpolate = (template: string, params?: Record<string, unknown>) =>
    template.replace(/\{(\w+)\}/g, (_, key: string) => String(params?.[key] ?? ''))

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        interpolate(messages[key] ?? key, params)
    })
  }
})

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const createDashboardStats = (): DashboardStats => ({
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  hourly_active_users: 0,
  stats_updated_at: '',
  stats_stale: false,
  total_api_keys: 0,
  active_api_keys: 0,
  total_accounts: 0,
  normal_accounts: 0,
  error_accounts: 0,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  average_duration_ms: 0,
  uptime: 0,
  rpm: 0,
  tpm: 0
})

describe('admin DashboardView', () => {
  beforeEach(() => {
    getSnapshotV2.mockReset()
    getUserUsageTrend.mockReset()
    getUserSpendingRanking.mockReset()
    getProfitabilityTrend.mockReset()
    getProfitabilityBounds.mockReset()
    getRecommendations.mockReset()

    getSnapshotV2.mockResolvedValue({
      stats: createDashboardStats(),
      trend: [],
      models: []
    })
    getUserUsageTrend.mockResolvedValue({
      trend: [],
      start_date: '',
      end_date: '',
      granularity: 'hour'
    })
    getUserSpendingRanking.mockResolvedValue({
      ranking: [],
      total_actual_cost: 0,
      total_requests: 0,
      total_tokens: 0,
      start_date: '',
      end_date: ''
    })
    getProfitabilityBounds.mockResolvedValue({
      has_data: true,
      earliest_date: '2025-01-10'
    })
    getProfitabilityTrend.mockResolvedValue({
      trend: [],
      start_date: '',
      end_date: '',
      granularity: 'day'
    })
    getRecommendations.mockResolvedValue({
      generated_at: '',
      lookback_days: 30,
      summary: {
        pool_count: 0,
        group_count: 0,
        current_schedulable_accounts: 0,
        recommended_additional_schedulable_accounts: 0,
        recoverable_unschedulable_accounts: 0,
        urgent_pool_count: 0
      },
      pools: []
    })
  })

  it('uses last 24 hours as default dashboard range', async () => {
    mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          ProfitabilityTrendChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: formatLocalDate(yesterday),
      end_date: formatLocalDate(now),
      granularity: 'hour'
    }))
  })

  it('loads profitability panel with all-time range by default', async () => {
    mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          ProfitabilityTrendChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    expect(getProfitabilityBounds).toHaveBeenCalledTimes(1)
    expect(getProfitabilityTrend).toHaveBeenCalledWith({
      start_date: '2025-01-10',
      end_date: formatLocalDate(new Date()),
      granularity: 'day'
    })
  })

  it('renders recommendations with site + capacity pool semantics', async () => {
    getRecommendations.mockResolvedValue({
      generated_at: '2026-04-18T00:00:00Z',
      lookback_days: 30,
      summary: {
        pool_count: 1,
        group_count: 2,
        current_schedulable_accounts: 5,
        recommended_additional_schedulable_accounts: 3,
        recoverable_unschedulable_accounts: 2,
        urgent_pool_count: 1
      },
      pools: [
        {
          pool_key: 'openai-shared-pool-1',
          platform: 'openai',
          group_names: ['共享池-A', '共享池-B'],
          plan_names: ['GPT-Standard', 'GPT-Pro'],
          recommended_account_type: 'shared',
          status: 'action',
          confidence_score: 0.92,
          current_total_accounts: 7,
          current_schedulable_accounts: 5,
          recommended_schedulable_accounts: 8,
          recommended_additional_schedulable_accounts: 3,
          recoverable_unschedulable_accounts: 2,
          reason: '容量紧张，需要补充可调度账号',
          metrics: {
            active_subscriptions: 11,
            active_users_30d: 88,
            activation_rate: 0.66,
            blended_activation_rate: 0.7,
            avg_daily_cost_30d: 12.8,
            avg_daily_cost_per_active_user: 0.15,
            blended_avg_daily_cost_per_active_user: 0.16,
            growth_factor: 1.12,
            projected_daily_cost: 15.4,
            capacity_utilization: 0.87,
            concurrency_utilization: 0.58,
            sessions_utilization: 0.52,
            rpm_utilization: 0.6,
            expected_accounts_by_subscriptions: 6,
            expected_accounts_by_active_users: 7,
            expected_accounts_by_cost: 8,
            platform_baseline: {
              platform: 'openai',
              active_subscriptions_per_schedulable: 2.1,
              active_users_per_schedulable: 12.5,
              daily_cost_per_schedulable: 1.3,
              activation_rate: 0.6,
              avg_daily_cost_per_active_user: 0.1
            }
          }
        }
      ]
    })

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          ProfitabilityTrendChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('全站建议补充 3 个可调度账号')
    expect(text).toContain('容量池')
    expect(text).not.toContain('评估 2 个订阅分组')
    expect(text).toContain('5 / 7')
    expect(text).toContain('建议补充 3 个可调度账号')
    expect(text).toContain('涉及套餐: GPT-Standard / GPT-Pro')
  })
})
