import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import SubscriptionPlanCard from '../SubscriptionPlanCard.vue'
import type { SubscriptionPlan } from '@/types/payment'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'payment.planCard.idleBilling') return '闲时动态计费'
        if (key === 'payment.planCard.idleWindow') return `${params?.start}-${params?.end} 北京时间`
        if (key === 'payment.planCard.idleRate') return '闲时倍率'
        if (key === 'payment.planCard.idleExtraProfit') return '闲时盈利率'
        return key
      },
    }),
  }
})

function makePlan(overrides: Partial<SubscriptionPlan> = {}): SubscriptionPlan {
  return {
    id: 1,
    group_id: 10,
    group_platform: 'openai',
    group_name: 'GPT Standard',
    rate_multiplier: 1,
    name: 'GPT Standard',
    description: 'High frequency plan',
    price: 99,
    validity_days: 30,
    validity_unit: 'month',
    features: [],
    for_sale: true,
    sort_order: 10,
    ...overrides,
  }
}

describe('SubscriptionPlanCard', () => {
  it('renders idle dynamic billing details when a plan group supports it', () => {
    const wrapper = mount(SubscriptionPlanCard, {
      props: {
        plan: makePlan({
          idle_rate_multiplier: 0.6,
          idle_extra_profit_rate_percent: 8,
          idle_start_time: '00:00:00',
          idle_end_time: '07:00:00',
        }),
      },
    })

    expect(wrapper.text()).toContain('闲时动态计费')
    expect(wrapper.text()).toContain('00:00-07:00 北京时间')
    expect(wrapper.text()).toContain('闲时倍率 ×0.6')
    expect(wrapper.text()).toContain('闲时盈利率 8%')
  })

  it('does not render idle dynamic billing without a complete window', () => {
    const wrapper = mount(SubscriptionPlanCard, {
      props: {
        plan: makePlan({
          idle_rate_multiplier: 0.6,
          idle_start_time: '00:00:00',
        }),
      },
    })

    expect(wrapper.text()).not.toContain('闲时动态计费')
  })
})
