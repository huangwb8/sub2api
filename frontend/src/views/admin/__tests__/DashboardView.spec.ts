import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { DashboardStats } from '@/types'
import DashboardView from '../DashboardView.vue'

const {
  getSnapshotV2,
  getUserUsageTrend,
  getUserSpendingRanking,
  getProfitabilityTrend,
  getProfitabilityBounds,
  getRecommendations,
  getOversellCalculator
} = vi.hoisted(() => {
  vi.stubGlobal('localStorage', {
    getItem: vi.fn(() => null),
    setItem: vi.fn(),
    removeItem: vi.fn()
  })

  return {
    getSnapshotV2: vi.fn(),
    getUserUsageTrend: vi.fn(),
    getUserSpendingRanking: vi.fn(),
    getProfitabilityTrend: vi.fn(),
    getProfitabilityBounds: vi.fn(),
    getRecommendations: vi.fn(),
    getOversellCalculator: vi.fn()
  }
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getSnapshotV2,
      getUserUsageTrend,
      getUserSpendingRanking,
      getProfitabilityTrend,
      getProfitabilityBounds,
      getRecommendations,
      getOversellCalculator
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
    'admin.dashboard.recommendations.statusMap.action': '行动',
    'admin.dashboard.oversell.title': '套餐定价测算',
    'admin.dashboard.oversell.description': '基于成本、用户消耗分布、目标盈利率和风险把握度，统一测算套餐达标售价与预测月利润。',
    'admin.dashboard.oversell.estimateTitle': '系统估算条件',
    'admin.dashboard.oversell.estimateDescription': '最近 {days} 天样本中，{share} 的用户月消耗不超过 {threshold} 个理论商品。',
    'admin.dashboard.oversell.sampleUsers': '样本用户 {count}',
    'admin.dashboard.oversell.updatedAt': '更新于 {time}',
    'admin.dashboard.oversell.costBadge': '采购 ¥{cost}/个 · 容量 {capacity}个/商品',
    'admin.dashboard.oversell.noEstimate': '暂无足够样本，先等待后端完成估算。',
    'admin.dashboard.oversell.sections.parameters': '测算参数',
    'admin.dashboard.oversell.sections.cost': '成本参数',
    'admin.dashboard.oversell.sections.users': '用户参数',
    'admin.dashboard.oversell.sections.profitRisk': '利润与风险',
    'admin.dashboard.oversell.sections.results': '关键结果',
    'admin.dashboard.oversell.form.userCount': '测算用户数',
    'admin.dashboard.oversell.form.plannedPrice': '计划套餐售价',
    'admin.dashboard.oversell.form.procurementCost': '单个实际商品采购成本',
    'admin.dashboard.oversell.form.capacity': '单个实际商品承载理论商品数',
    'admin.dashboard.oversell.form.profitRate': '目标盈利率',
    'admin.dashboard.oversell.form.profitMode': '盈利口径',
    'admin.dashboard.oversell.form.heavyUsage': '重度用户月消耗上限',
    'admin.dashboard.oversell.form.confidence': '把握度',
    'admin.dashboard.oversell.form.costPlus': '成本加成',
    'admin.dashboard.oversell.form.netMargin': '净利率',
    'admin.dashboard.oversell.form.cnyPerMonth': '元 / 月',
    'admin.dashboard.oversell.form.cnyPerItem': '元 / 个',
    'admin.dashboard.oversell.form.units': '个',
    'admin.dashboard.oversell.form.percent': '%',
    'admin.dashboard.oversell.form.confidence95': '95%',
    'admin.dashboard.oversell.form.confidence99': '99%',
    'admin.dashboard.oversell.form.users': '人',
    'admin.dashboard.oversell.tooltips.userCount': '超售测算用户数说明',
    'admin.dashboard.oversell.tooltips.plannedPrice': '超售计划售价说明',
    'admin.dashboard.oversell.tooltips.procurementCost': '超售采购成本说明',
    'admin.dashboard.oversell.tooltips.capacity': '超售承载能力说明',
    'admin.dashboard.oversell.tooltips.heavyUsage': '超售重度用户上限说明',
    'admin.dashboard.oversell.tooltips.profitRate': '超售目标盈利率说明',
    'admin.dashboard.oversell.tooltips.profitMode': '超售盈利口径说明',
    'admin.dashboard.oversell.tooltips.confidence': '超售把握度说明',
    'admin.dashboard.oversell.metrics.meanUpperBound': '保守人均消耗上界',
    'admin.dashboard.oversell.metrics.unitCost': '理论商品单位成本',
    'admin.dashboard.oversell.metrics.floorPrice': '保守保本价',
    'admin.dashboard.oversell.result.recommendedPrice': '达标套餐价格',
    'admin.dashboard.oversell.result.minimumUsers': '测算用户数',
    'admin.dashboard.oversell.result.plannedProfit': '预测月利润',
    'admin.dashboard.oversell.result.conservativeCost': '保守月成本',
    'admin.dashboard.oversell.result.riskDrivenUsers': '按计划售价推导',
    'admin.dashboard.oversell.result.lossRisk': '亏损风险上限 {risk}',
    'admin.dashboard.oversell.result.infiniteUsers': '当前计划售价无法形成稳定超售池，请先提高售价或放宽盈利目标。',
    'admin.dashboard.oversell.result.buffer': '安全垫 {value}',
    'admin.dashboard.oversell.result.helper': '建议价取“保底套餐价”和“目标盈利推导价”中的较高值。',
    'admin.dashboard.oversell.result.floorPriceHint': '保守保本价 ¥{floor}',
    'admin.dashboard.oversell.result.priceGapHint': '与达标单价差额 {gap}',
    'admin.dashboard.oversell.result.revenueHint': '月收入 ¥{value}',
    'admin.dashboard.oversell.result.costHint': '保守月成本 ¥{value}',
    'admin.dashboard.oversell.result.note': 'Hoeffding 上界用于估算在给定把握度下，用户池人均消耗超出可承受阈值的风险。',
    'admin.dashboard.oversell.result.users': '{count} 人',
    'admin.dashboard.oversell.table.title': '套餐价格换算',
    'admin.dashboard.oversell.table.plan': '套餐',
    'admin.dashboard.oversell.table.duration': '时长',
    'admin.dashboard.oversell.table.currentMonthlyEquivalent': '当前月费等价',
    'admin.dashboard.oversell.table.currentPrice': '当前单价',
    'admin.dashboard.oversell.table.recommendedPrice': '达标售价',
    'admin.dashboard.oversell.table.delta': '调价幅度'
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
    getOversellCalculator.mockReset()

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
    getOversellCalculator.mockResolvedValue({
      generated_at: '2026-04-20T09:30:00Z',
      defaults: {
        actual_cost_cny: 50,
        capacity_units_per_product: 3,
        confidence_level: 99,
        profit_rate_percent: 20,
        profit_mode: 'markup',
        target_profit_total_cny: 120
      },
      input: {
        actual_cost_cny: 50,
        capacity_units_per_product: 3,
        confidence_level: 99,
        profit_rate_percent: 20,
        profit_mode: 'markup',
        target_profit_total_cny: 120
      },
      estimate: {
        light_user_threshold_units: 0.3,
        estimated_light_user_ratio: 0.73,
        sampled_subscription_count: 126,
        light_user_count: 92,
        estimated_from_live_data: true,
        fallback_applied: false,
        basis: 'last_30_days',
        current_cheapest_monthly_price_cny: 50,
        current_cheapest_plan_name: '月付基础版'
      },
      result: {
        feasible: true,
        minimum_users: 10,
        recommended_monthly_price_cny: 29.15,
        current_cheapest_monthly_price_cny: 50,
        monthly_price_gap_cny: -20.85,
        expected_mean_units: 1.029,
        risk_adjusted_mean_units: 2.5,
        confidence_level: 99,
        price_multiplier: 0.583,
        reason: 'test'
      },
      plans: [
        {
          plan_id: 1,
          group_id: 10,
          group_name: 'OpenAI 月包',
          plan_name: '月付基础版',
          validity_days: 30,
          validity_unit: 'day',
          duration_days_equivalent: 30,
          current_price_cny: 50,
          current_monthly_price_cny: 50,
          recommended_price_cny: 0,
          recommended_monthly_price_cny: 0,
          price_delta_cny: 0
        }
      ]
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

  it('renders unified package pricing calculator with estimate, inputs, results, and plan conversion', async () => {
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

    expect(getOversellCalculator).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('套餐定价测算')
    expect(wrapper.text()).toContain('73%')
    expect(wrapper.text()).toContain('采购 ¥50.00/个')
    expect(wrapper.text()).toContain('成本参数')
    expect(wrapper.text()).toContain('用户参数')
    expect(wrapper.text()).toContain('利润与风险')
    expect(wrapper.text()).toContain('关键结果')
    expect(wrapper.text()).toContain('预测月利润')
    expect(wrapper.text()).toContain('套餐价格换算')
    expect(wrapper.text()).toContain('当前月费等价')
    expect(wrapper.text()).toContain('月付基础版')

    const initialRequiredPrice = wrapper.get('[data-testid="oversell-recommended-price"]').text()
    const userCountInput = wrapper.get('[data-testid="oversell-user-count"]')
    await userCountInput.setValue('20')
    await flushPromises()

    expect(wrapper.get('[data-testid="oversell-recommended-price"]').text()).toMatch(/¥/)
    expect(wrapper.get('[data-testid="oversell-recommended-price"]').text()).not.toEqual(initialRequiredPrice)
  })

  it('renders help tooltip triggers for unified package pricing parameters', async () => {
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

    const helpTestIds = [
      'oversell-user-count-help',
      'oversell-planned-price-help',
      'oversell-procurement-cost-help',
      'oversell-capacity-help',
      'oversell-heavy-usage-help',
      'oversell-profit-rate-help',
      'oversell-profit-mode-help',
      'oversell-confidence-help'
    ]

    helpTestIds.forEach((testId) => {
      expect(wrapper.find(`[data-testid="${testId}"]`).exists()).toBe(true)
    })
  })
})
