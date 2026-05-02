import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import SettingsPluginsTab from '../SettingsPluginsTab.vue'

const pluginFixtures = [
  {
    name: 'api-prompt',
    type: 'api-prompt',
    description: 'Remote prompt plugin',
    base_url: 'https://plugin.example.com',
    enabled: true,
    api_key_configured: true,
    created_at: '2026-05-02T00:00:00Z',
    updated_at: '2026-05-02T00:00:00Z',
    api_prompt: {
      source: 'cache',
      last_synced_at: '2026-05-02T01:00:00Z',
      last_sync_error: 'HTTP 503',
      remote_template_count: 1,
      templates: [
        {
          id: 'remote-focus',
          name: 'Remote Focus',
          description: 'Remote synced template',
          prompt: '',
          enabled: true,
          builtin: false,
          sort_order: 10,
        },
      ],
    },
  },
]

const apiMocks = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  setEnabled: vi.fn(),
  test: vi.fn(),
}))

vi.mock('@/api/admin/plugins', () => ({
  adminPluginsAPI: apiMocks,
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

describe('SettingsPluginsTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.list.mockResolvedValue(pluginFixtures)
    apiMocks.update.mockImplementation(async (_name, payload) => ({
      ...pluginFixtures[0],
      description: payload.description,
      base_url: payload.base_url,
      enabled: payload.enabled,
    }))
  })

  it('renders remote api-prompt instances as cached read-only catalogs', async () => {
    const wrapper = mount(SettingsPluginsTab, {
      global: {
        stubs: {
          Icon: true,
          Toggle: {
            props: ['modelValue'],
            template: '<button class="toggle" :disabled="$attrs.disabled" />',
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('admin.settings.plugins.labels.remoteMode')
    expect(wrapper.text()).toContain('admin.settings.plugins.templates.statusCache')
    expect(wrapper.text()).toContain('HTTP 503')
    expect(wrapper.findAll('button').some((button) => button.text() === 'admin.settings.plugins.templates.add')).toBe(false)

    const save = wrapper.findAll('button').find((button) => button.text().includes('common.save'))
    expect(save).toBeTruthy()
    await save!.trigger('click')
    await flushPromises()

    expect(apiMocks.update).toHaveBeenCalledWith(
      'api-prompt',
      expect.objectContaining({
        api_prompt: undefined,
      })
    )
  })
})
