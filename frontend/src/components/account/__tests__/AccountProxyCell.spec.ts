import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import type { Account, Proxy } from '@/types'

const {
  updateAccountMock,
  showSuccessMock,
  showErrorMock
} = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  showSuccessMock: vi.fn(),
  showErrorMock: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      update: updateAccountMock
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: showSuccessMock,
    showError: showErrorMock
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'admin.accounts.proxyAvailability.available') return '正常'
        if (key === 'admin.accounts.proxyAvailability.failed') return '链接失败'
        if (key === 'admin.accounts.proxySwitchSuccess') {
          return `已为 ${params?.account} 切换到 ${params?.proxy}`
        }
        if (key === 'admin.accounts.proxySwitchDialog.confirm') return '切换代理'
        if (key === 'admin.accounts.proxySwitchDialog.title') return `切换代理：${params?.name}`
        return key
      }
    })
  }
})

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: {
      type: Boolean,
      default: false
    },
    title: {
      type: String,
      default: ''
    }
  },
  setup(props, { slots }) {
    return () =>
      props.show
        ? h('div', { class: 'base-dialog-stub' }, [
            h('div', { class: 'dialog-title' }, props.title),
            slots.default?.(),
            slots.footer?.()
          ])
        : null
  }
})

const ProxySelectorStub = defineComponent({
  name: 'ProxySelector',
  props: {
    modelValue: {
      type: [Number, null],
      default: null
    },
    proxies: {
      type: Array,
      default: () => []
    },
    dropdownSize: {
      type: String,
      default: 'normal'
    }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () =>
      h(
        'select',
        {
          class: 'proxy-selector-stub',
          value: props.modelValue == null ? '' : String(props.modelValue),
          onChange: (event: Event) => {
            const value = (event.target as HTMLSelectElement).value
            emit('update:modelValue', value === '' ? null : Number(value))
          }
        },
        [
          h('option', { value: '' }, 'none'),
          ...(props.proxies as Proxy[]).map((proxy) =>
            h('option', { key: proxy.id, value: String(proxy.id) }, proxy.name)
          )
        ]
      )
  }
})

import AccountProxyCell from '../AccountProxyCell.vue'

const buildProxy = (overrides: Partial<Proxy> = {}): Proxy => ({
  id: 1,
  name: 'Tokyo-A',
  protocol: 'http',
  host: '1.1.1.1',
  port: 8080,
  username: null,
  status: 'active',
  created_at: '2026-05-02T00:00:00Z',
  updated_at: '2026-05-02T00:00:00Z',
  ...overrides
})

const buildAccount = (overrides: Partial<Account> = {}): Account => {
  const proxy = overrides.proxy ?? buildProxy()
  return {
    id: 101,
    name: 'Account-A',
    platform: 'anthropic',
    type: 'oauth',
    proxy_id: proxy?.id ?? null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '2026-05-02T00:00:00Z',
    updated_at: '2026-05-02T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    proxy,
    ...overrides
  }
}

describe('AccountProxyCell', () => {
  beforeEach(() => {
    updateAccountMock.mockReset()
    showSuccessMock.mockReset()
    showErrorMock.mockReset()
  })

  it('shows available and failed proxy availability badges', async () => {
    const availableWrapper = mount(AccountProxyCell, {
      props: {
        account: buildAccount({
          proxy: buildProxy({
            latency_status: 'success',
            quality_status: 'healthy'
          })
        }),
        proxies: [buildProxy()]
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ProxySelector: ProxySelectorStub,
          Icon: true
        }
      }
    })

    expect(availableWrapper.text()).toContain('正常')

    const failedWrapper = mount(AccountProxyCell, {
      props: {
        account: buildAccount({
          proxy: buildProxy({
            latency_status: 'failed'
          })
        }),
        proxies: [buildProxy({ latency_status: 'failed' })]
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ProxySelector: ProxySelectorStub,
          Icon: true
        }
      }
    })

    expect(failedWrapper.text()).toContain('链接失败')
  })

  it('prefers enriched proxy list health over account proxy snapshot', () => {
    const accountProxy = buildProxy({
      id: 1,
      status: 'active'
    })
    const enrichedProxy = buildProxy({
      id: 1,
      status: 'active',
      latency_status: 'failed',
      latency_message: 'connect timeout'
    })

    const wrapper = mount(AccountProxyCell, {
      props: {
        account: buildAccount({
          proxy_id: 1,
          proxy: accountProxy
        }),
        proxies: [enrichedProxy]
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ProxySelector: ProxySelectorStub,
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('链接失败')
  })

  it('switches account proxy from the quick dialog and emits the updated account', async () => {
    const currentProxy = buildProxy({ id: 1, name: 'Tokyo-A' })
    const targetProxy = buildProxy({ id: 2, name: 'Singapore-B', host: '2.2.2.2' })
    const updatedAccount = buildAccount({
      proxy_id: targetProxy.id,
      proxy: targetProxy
    })
    updateAccountMock.mockResolvedValue(updatedAccount)

    const wrapper = mount(AccountProxyCell, {
      props: {
        account: buildAccount({
          proxy: currentProxy
        }),
        proxies: [currentProxy, targetProxy]
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ProxySelector: ProxySelectorStub,
          Icon: true
        }
      }
    })

    await wrapper.find('button').trigger('click')
    await wrapper.find('select.proxy-selector-stub').setValue(String(targetProxy.id))
    await wrapper.find('[data-testid="account-proxy-switch-confirm"]').trigger('click')
    await flushPromises()

    expect(updateAccountMock).toHaveBeenCalledWith(101, { proxy_id: 2 })
    expect(wrapper.emitted('updated')?.[0]).toEqual([updatedAccount])
    expect(showSuccessMock).toHaveBeenCalledWith('已为 Account-A 切换到 Singapore-B')
  })
})
