import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

const {
  updateAccountMock,
  checkMixedChannelRiskMock,
  getSchedulingMechanismSettingsMock,
  showErrorMock
} = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  checkMixedChannelRiskMock: vi.fn(),
  getSchedulingMechanismSettingsMock: vi.fn(),
  showErrorMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isSimpleMode: true
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      update: updateAccountMock,
      checkMixedChannelRisk: checkMixedChannelRiskMock
    },
    settings: {
      getSchedulingMechanismSettings: getSchedulingMechanismSettingsMock
    }
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
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

import EditAccountModal from '../EditAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const SelectStub = defineComponent({
  name: 'UiSelectStub',
  props: {
    modelValue: {
      type: String,
      default: ''
    },
    options: {
      type: Array,
      default: () => []
    },
    disabled: {
      type: Boolean,
      default: false
    }
  },
  emits: ['update:modelValue', 'change'],
  template: `
    <select
      :value="modelValue"
      :disabled="disabled"
      @change="$emit('update:modelValue', $event.target.value); $emit('change', $event.target.value)"
    >
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `
})

const ModelWhitelistSelectorStub = defineComponent({
  name: 'ModelWhitelistSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: `
    <div>
      <button
        type="button"
        data-testid="rewrite-to-snapshot"
        @click="$emit('update:modelValue', ['gpt-5.2-2025-12-11'])"
      >
        rewrite
      </button>
      <span data-testid="model-whitelist-value">
        {{ Array.isArray(modelValue) ? modelValue.join(',') : '' }}
      </span>
    </div>
  `
})

function buildAccount() {
  return {
    id: 1,
    name: 'OpenAI Key',
    notes: '',
    platform: 'openai',
    type: 'apikey',
    credentials: {
      api_key: 'sk-test',
      base_url: 'https://api.openai.com',
      model_mapping: {
        'gpt-5.2': 'gpt-5.2'
      }
    },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function mountModal(account = buildAccount()) {
  return mount(EditAccountModal, {
    props: {
      show: true,
      account,
      proxies: [],
      groups: []
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Select: SelectStub,
        Icon: true,
        ProxySelector: true,
        GroupSelector: true,
        ModelWhitelistSelector: ModelWhitelistSelectorStub
      }
    }
  })
}

describe('EditAccountModal', () => {
  beforeEach(() => {
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSchedulingMechanismSettingsMock.mockReset()
    showErrorMock.mockReset()
    getSchedulingMechanismSettingsMock.mockResolvedValue({ mechanisms: [], proxy_failover: {} })
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
  })

  it('API Key 输入框应禁用密码管理器自动填充', () => {
    const wrapper = mountModal()
    const apiKeyInput = wrapper.find('input[type="password"].font-mono')

    expect(apiKeyInput.exists()).toBe(true)
    expect(apiKeyInput.attributes('autocomplete')).toBe('new-password')
    expect(apiKeyInput.attributes('data-1p-ignore')).toBeDefined()
    expect(apiKeyInput.attributes('data-lpignore')).toBe('true')
    expect(apiKeyInput.attributes('data-bwignore')).toBe('true')
  })

  it('OpenAI 账号编辑时应展示并回显 ctx_pool WS mode', () => {
    const wrapper = mountModal({
      ...buildAccount(),
      extra: {
        openai_apikey_responses_websockets_v2_mode: 'ctx_pool'
      }
    })

    const wsModeSelect = wrapper.findAll('select').find((select) => select.find('option[value="ctx_pool"]').exists())

    expect(wsModeSelect).toBeTruthy()
    expect(wsModeSelect!.findAll('option').map((option) => option.element.getAttribute('value'))).toEqual([
      'off',
      'ctx_pool',
      'passthrough'
    ])
    expect((wsModeSelect!.element as HTMLSelectElement).value).toBe('ctx_pool')
  })

  it('ChatAPI 账号编辑时应保留 API Key 表单，但不展示 OpenAI passthrough 与 WS mode', async () => {
    const wrapper = mountModal({
      ...buildAccount(),
      type: 'chatapi'
    })

    await flushPromises()

    expect(wrapper.find('input[type="password"].font-mono').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('admin.accounts.openai.oauthPassthrough')
    expect(wrapper.findAll('select').some((select) => select.find('option[value="ctx_pool"]').exists())).toBe(false)
  })

  it('reopening the same account rehydrates the OpenAI whitelist from props', async () => {
    const account = buildAccount()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')
    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2-2025-12-11')

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'gpt-5.2': 'gpt-5.2'
    })
  })

  it('没有匹配的调度机制规则时不能启用临时不可调度', async () => {
    const wrapper = mountModal()
    await flushPromises()

    const toggle = wrapper.get('[data-testid="temp-unsched-toggle"]')

    expect((toggle.element as HTMLButtonElement).disabled).toBe(true)
    expect(wrapper.text()).toContain('admin.accounts.tempUnschedulable.noMechanismRules')
  })

  it('临时不可调度只保存从调度机制选择的规则引用', async () => {
    const account = buildAccount()
    updateAccountMock.mockResolvedValue(account)
    getSchedulingMechanismSettingsMock.mockResolvedValue({
      mechanisms: [
        {
          id: 'default',
          name: '默认临时不可调度',
          platform: 'openai',
          account_type: 'apikey',
          enabled: true,
          hidden: false,
          temp_unschedulable_enabled: true,
          temp_unschedulable_rules: [
            {
              id: 'rate-limit',
              error_code: 429,
              keywords: ['rate limit'],
              duration_minutes: 10,
              description: '限流'
            }
          ]
        }
      ],
      proxy_failover: {}
    })

    const wrapper = mountModal(account)
    await flushPromises()

    await wrapper.get('[data-testid="temp-unsched-toggle"]').trigger('click')
    const ruleSelect = wrapper.findAll('select').find((select) =>
      select.find('option[value="default:rate-limit"]').exists()
    )
    expect(ruleSelect).toBeTruthy()

    await ruleSelect!.setValue('default:rate-limit')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.temp_unschedulable_enabled).toBe(true)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.temp_unschedulable_rule_refs).toEqual([
      { mechanism_id: 'default', rule_id: 'rate-limit' }
    ])
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('temp_unschedulable_rules')
  })
})
