import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ProxySelector from '../ProxySelector.vue'
import type { Proxy } from '@/types'

vi.mock('@/api/admin', () => ({
  adminAPI: {
    proxies: {
      testProxy: vi.fn()
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const buildProxy = (overrides: Partial<Proxy> = {}): Proxy => ({
  id: 1,
  name: 'Decodo-JP-帐号2',
  protocol: 'http',
  host: '127.0.0.1',
  port: 8080,
  username: null,
  status: 'active',
  created_at: '2026-04-27T00:00:00Z',
  updated_at: '2026-04-27T00:00:00Z',
  ...overrides
})

describe('ProxySelector', () => {
  it('shows proxy account count in the selected label and option name', async () => {
    const wrapper = mount(ProxySelector, {
      props: {
        modelValue: 1,
        proxies: [buildProxy({ account_count: 2 })]
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.find('.select-value').text()).toContain('Decodo-JP-帐号2（2）')

    await wrapper.find('button.select-trigger').trigger('click')

    expect(wrapper.find('.select-options').text()).toContain('Decodo-JP-帐号2（2）')
  })

  it('keeps the proxy name unchanged when account count is unavailable', async () => {
    const wrapper = mount(ProxySelector, {
      props: {
        modelValue: 1,
        proxies: [buildProxy()]
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.find('.select-value').text()).toContain('Decodo-JP-帐号2')
    expect(wrapper.find('.select-value').text()).not.toContain('Decodo-JP-帐号2（')
  })

  it('supports multi-term hit search across proxy fields', async () => {
    const wrapper = mount(ProxySelector, {
      props: {
        modelValue: null,
        proxies: [
          buildProxy({
            id: 1,
            name: 'abdkdkdidddy',
            host: '10.0.0.1'
          }),
          buildProxy({
            id: 2,
            name: 'only-ab-match',
            host: '10.0.0.2'
          })
        ]
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    await wrapper.find('button.select-trigger').trigger('click')
    await wrapper.find('input.select-search-input').setValue('ab dy')

    const optionsText = wrapper.find('.select-options').text()
    expect(optionsText).toContain('abdkdkdidddy')
    expect(optionsText).not.toContain('only-ab-match')
  })

  it('orders proxy options by quality score, latency, then id and shows quality badges', async () => {
    const wrapper = mount(ProxySelector, {
      props: {
        modelValue: null,
        proxies: [
          buildProxy({ id: 4, name: 'unchecked' }),
          buildProxy({ id: 3, name: 'mid', quality_score: 75, quality_grade: 'B', latency_ms: 90 }),
          buildProxy({ id: 2, name: 'slow-high', quality_score: 95, quality_grade: 'A', latency_ms: 300 }),
          buildProxy({ id: 1, name: 'fast-high', quality_score: 95, quality_grade: 'A', latency_ms: 120 })
        ]
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    await wrapper.find('button.select-trigger').trigger('click')

    const optionTexts = wrapper
      .findAll('.select-option')
      .slice(1)
      .map((option) => option.text())

    expect(optionTexts[0]).toContain('fast-high')
    expect(optionTexts[1]).toContain('slow-high')
    expect(optionTexts[2]).toContain('mid')
    expect(optionTexts[3]).toContain('unchecked')

    const badges = wrapper.findAll('.proxy-quality-badge').map((badge) => badge.text())
    expect(badges).toEqual(['A', 'A', 'B', '—'])
  })

  it('supports a taller option list for dense proxy picking dialogs', async () => {
    const wrapper = mount(ProxySelector, {
      props: {
        modelValue: null,
        proxies: [buildProxy()],
        dropdownSize: 'tall'
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    await wrapper.find('button.select-trigger').trigger('click')

    expect(wrapper.find('.select-options').classes()).toContain('select-options-tall')
  })
})
