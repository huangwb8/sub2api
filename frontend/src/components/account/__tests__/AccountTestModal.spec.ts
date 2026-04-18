import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AccountTestModal from '../AccountTestModal.vue'

const { getAvailableModels, copyToClipboard } = vi.hoisted(() => ({
  getAvailableModels: vi.fn(),
  copyToClipboard: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getAvailableModels
    }
  }
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.accounts.geminiImagePromptDefault': 'Generate a cute orange cat astronaut sticker on a clean pastel background.'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] || key
    })
  }
})

function mountModal() {
  return mount(AccountTestModal, {
    props: {
      show: false,
      account: {
        id: 42,
        name: 'Gemini Image Test',
        platform: 'gemini',
        type: 'apikey',
        status: 'active'
      }
    } as any,
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Select: { template: '<div class="select-stub"></div>' },
        TextArea: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template: '<textarea class="textarea-stub" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
        },
        Icon: true
      }
    }
  })
}

describe('AccountTestModal user', () => {
  beforeEach(() => {
    getAvailableModels.mockResolvedValue([
      { id: 'gemini-2.5-flash-image', display_name: 'Gemini 2.5 Flash Image' }
    ])
    copyToClipboard.mockReset()
    Object.defineProperty(globalThis, 'localStorage', {
      value: {
        getItem: vi.fn((key: string) => (key === 'auth_token' ? 'test-token' : null)),
        setItem: vi.fn(),
        removeItem: vi.fn(),
        clear: vi.fn()
      },
      configurable: true
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('连接中点击关闭会中断流式请求并发出 close', async () => {
    global.fetch = vi.fn().mockImplementation((_url, init?: RequestInit) => {
      const signal = init?.signal as AbortSignal | undefined
      return new Promise((_resolve, reject) => {
        signal?.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))
      })
    }) as any

    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()

    const buttons = wrapper.findAll('button')
    const startButton = buttons.find((button) => button.text().includes('admin.accounts.startTest'))
    expect(startButton).toBeTruthy()

    await startButton!.trigger('click')
    await flushPromises()

    const closeButton = wrapper.findAll('button').find((button) => button.text().includes('common.close'))
    expect(closeButton).toBeTruthy()
    expect(closeButton!.attributes('disabled')).toBeUndefined()

    await closeButton!.trigger('click')
    await flushPromises()

    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
