import { describe, expect, it, vi } from 'vitest'
import { defineComponent, ref } from 'vue'
import { mount } from '@vue/test-utils'
import CreateAccountModal from '../CreateAccountModal.vue'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
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
      create: vi.fn(),
      checkMixedChannelRisk: vi.fn()
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([])
    }
  }
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN',
  i18n: {
    global: {
      t: (key: string) => key
    }
  }
}))

const buildOAuthMock = () => ({
  authUrl: ref(''),
  sessionId: ref(''),
  loading: ref(false),
  error: ref(''),
  state: ref(''),
  resetState: vi.fn(),
  generateAuthUrl: vi.fn(),
  getCapabilities: vi.fn().mockResolvedValue({
    aiStudioOAuthEnabled: true
  }),
  exchangeAuthCode: vi.fn(),
  buildCredentials: vi.fn(),
  buildExtraInfo: vi.fn(),
  validateRefreshToken: vi.fn(),
  parseSessionKeys: vi.fn().mockReturnValue([]),
  normalizeSessionKeys: vi.fn().mockReturnValue([]),
  buildAuthUrl: vi.fn()
})

vi.mock('@/composables/useAccountOAuth', () => ({
  useAccountOAuth: () => buildOAuthMock()
}))

vi.mock('@/composables/useOpenAIOAuth', () => ({
  useOpenAIOAuth: () => buildOAuthMock()
}))

vi.mock('@/composables/useGeminiOAuth', () => ({
  useGeminiOAuth: () => buildOAuthMock()
}))

vi.mock('@/composables/useAntigravityOAuth', () => ({
  useAntigravityOAuth: () => buildOAuthMock()
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
    }
  },
  emits: ['update:modelValue'],
  template: `
    <select
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `
})

function mountModal() {
  return mount(CreateAccountModal, {
    props: {
      show: true,
      proxies: [],
      groups: []
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        Select: SelectStub,
        Icon: true,
        ProxySelector: true,
        GroupSelector: true,
        ModelWhitelistSelector: true,
        QuotaLimitCard: true,
        OAuthAuthorizationFlow: true
      }
    }
  })
}

describe('CreateAccountModal', () => {
  it('默认并发数应为 5', () => {
    const wrapper = mountModal()

    const numberInputs = wrapper.findAll('input[type="number"]')
    expect(numberInputs.length).toBeGreaterThan(0)
    expect((numberInputs[0].element as HTMLInputElement).value).toBe('5')
  })

  it('OpenAI 新建账号时应展示 ctx_pool WS mode 选项', async () => {
    const wrapper = mountModal()

    const openAIPlatformButton = wrapper.findAll('button').find((button) => button.text().includes('OpenAI'))
    expect(openAIPlatformButton).toBeTruthy()

    await openAIPlatformButton!.trigger('click')

    const wsModeSelect = wrapper.findAll('select').find((select) => select.find('option[value="ctx_pool"]').exists())

    expect(wsModeSelect).toBeTruthy()
    expect(wsModeSelect!.findAll('option').map((option) => option.element.getAttribute('value'))).toEqual([
      'off',
      'ctx_pool',
      'passthrough'
    ])
    expect((wsModeSelect!.element as HTMLSelectElement).value).toBe('ctx_pool')
  })

  it('OpenAI Chat Completions API 类型应隐藏 OpenAI passthrough 与 WS mode', async () => {
    const wrapper = mountModal()

    const openAIPlatformButton = wrapper.findAll('button').find((button) => button.text().includes('OpenAI'))
    expect(openAIPlatformButton).toBeTruthy()
    await openAIPlatformButton!.trigger('click')

    const chatapiButton = wrapper.findAll('button').find((button) =>
      button.text().includes('admin.accounts.types.chatCompletionsApi')
    )
    expect(chatapiButton).toBeTruthy()
    await chatapiButton!.trigger('click')

    expect(wrapper.text()).not.toContain('admin.accounts.openai.oauthPassthrough')
    expect(wrapper.findAll('select').some((select) => select.find('option[value="ctx_pool"]').exists())).toBe(false)
  })
})
