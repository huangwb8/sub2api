import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import PricingView from '../PricingView.vue'

const appState = {
  publicSettingsLoaded: true,
  cachedPublicSettings: {
    home_content: '',
  },
  fetchPublicSettings: vi.fn(),
}

const routeState = {
  query: {} as Record<string, string>,
}

vi.mock('@/stores', () => ({
  useAppStore: () => appState,
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeState,
  }
})

vi.mock('../HomeView.vue', () => ({
  default: {
    name: 'HomeView',
    template: '<div data-test="custom-home-pricing">custom home pricing</div>',
  },
}))

vi.mock('../PublicPricingView.vue', () => ({
  default: {
    name: 'PublicPricingView',
    template: '<div data-test="default-pricing">default pricing</div>',
  },
}))

describe('PricingView', () => {
  beforeEach(() => {
    appState.publicSettingsLoaded = true
    appState.cachedPublicSettings.home_content = ''
    appState.fetchPublicSettings.mockReset()
    routeState.query = {}
  })

  it('uses custom home content as the /pricing page when inline home_content is configured', async () => {
    appState.cachedPublicSettings.home_content = '<section>custom</section>'

    const wrapper = mount(PricingView)
    await flushPromises()

    expect(wrapper.find('[data-test="custom-home-pricing"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="default-pricing"]').exists()).toBe(false)
  })

  it('falls back to the default public pricing page without inline home_content', async () => {
    const wrapper = mount(PricingView)
    await flushPromises()

    expect(wrapper.find('[data-test="default-pricing"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="custom-home-pricing"]').exists()).toBe(false)
  })

  it('keeps the default pricing page for embedded pricing views', async () => {
    appState.cachedPublicSettings.home_content = '<section>custom</section>'
    routeState.query = { ui_mode: 'embedded' }

    const wrapper = mount(PricingView)
    await flushPromises()

    expect(wrapper.find('[data-test="default-pricing"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="custom-home-pricing"]').exists()).toBe(false)
  })
})
