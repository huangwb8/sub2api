import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import PublicPricingView from '../PublicPricingView.vue'

const { getPublicPlans, routerPush } = vi.hoisted(() => ({
  getPublicPlans: vi.fn(),
  routerPush: vi.fn(),
}))

const authState = {
  isAuthenticated: false,
  isAdmin: false,
}

const appState = {
  publicSettingsLoaded: true,
  cachedPublicSettings: {
    payment_enabled: true,
    site_name: 'Sub2API',
    site_logo: '',
    doc_url: '',
  },
  siteName: 'Sub2API',
  siteLogo: '',
  docUrl: '',
  fetchPublicSettings: vi.fn(),
}

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    getPublicPlans,
  },
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => authState,
  useAppStore: () => appState,
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => ({ path: '/pricing', query: {} }),
    useRouter: () => ({ push: routerPush }),
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'pricing.availableTitle') {
          return `${params?.count} plans are currently available`
        }
        return key
      },
    }),
  }
})

describe('PublicPricingView', () => {
  beforeEach(() => {
    getPublicPlans.mockReset()
    routerPush.mockReset()
    authState.isAuthenticated = false
    authState.isAdmin = false
    appState.publicSettingsLoaded = true
    appState.cachedPublicSettings.payment_enabled = true
    appState.fetchPublicSettings.mockReset()
  })

  it('renders public plans returned by the backend in sort order', async () => {
    getPublicPlans.mockResolvedValue({
      data: [
        { id: 2, name: 'Pro', price: 99, validity_unit: 'month', validity_days: 30, features: [], for_sale: true, sort_order: 20 },
        { id: 1, name: 'Starter', price: 19, validity_unit: 'month', validity_days: 30, features: [], for_sale: true, sort_order: 10 },
      ],
    })

    const wrapper = mount(PublicPricingView, {
      global: {
        stubs: {
          Icon: true,
          LocaleSwitcher: true,
          'router-link': { template: '<a><slot /></a>' },
          SubscriptionPlanCard: {
            props: ['plan'],
            template: '<button class="plan-card" @click="$emit(\'select\', plan)">{{ plan.name }}</button>',
          },
        },
      },
    })

    await flushPromises()

    const cards = wrapper.findAll('.plan-card')
    expect(cards).toHaveLength(2)
    expect(cards[0].text()).toContain('Starter')
    expect(cards[1].text()).toContain('Pro')
    expect(wrapper.text()).toContain('2 plans are currently available')
  })

  it('redirects guests to login with purchase redirect when selecting a plan', async () => {
    getPublicPlans.mockResolvedValue({
      data: [
        { id: 1, name: 'Starter', price: 19, validity_unit: 'month', validity_days: 30, features: [], for_sale: true, sort_order: 10 },
      ],
    })

    const wrapper = mount(PublicPricingView, {
      global: {
        stubs: {
          Icon: true,
          LocaleSwitcher: true,
          'router-link': { template: '<a><slot /></a>' },
          SubscriptionPlanCard: {
            props: ['plan'],
            template: '<button class="plan-card" @click="$emit(\'select\', plan)">{{ plan.name }}</button>',
          },
        },
      },
    })

    await flushPromises()
    await wrapper.get('.plan-card').trigger('click')

    expect(routerPush).toHaveBeenCalledWith({
      path: '/login',
      query: { redirect: '/purchase' },
    })
  })
})
