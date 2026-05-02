import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import SettingsPluginsTab from '../SettingsPluginsTab.vue'

const pluginFixtures = [
  {
    name: 'api-prompt',
    type: 'api-prompt',
    description: 'Local prompt plugin',
    enabled: true,
    created_at: '2026-05-02T00:00:00Z',
    updated_at: '2026-05-02T00:00:00Z',
    api_prompt: {
      source: 'local',
      templates: [
        {
          id: 'local-focus',
          name: 'Local Focus',
          description: 'Local template',
          prompt: 'Use local config.',
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
      enabled: payload.enabled,
      api_prompt: payload.api_prompt,
    }))
  })

  it('renders local api-prompt instances as editable catalogs', async () => {
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

    expect(wrapper.text()).toContain('admin.settings.plugins.labels.localMode')
    expect(wrapper.text()).toContain('admin.settings.plugins.templates.statusLocal')
    expect(wrapper.findAll('input').some((input) => (input.element as HTMLInputElement).value === 'Local Focus')).toBe(true)
    expect(wrapper.findAll('button').some((button) => button.text() === 'admin.settings.plugins.templates.add')).toBe(true)

    const save = wrapper.findAll('button').find((button) => button.text().includes('common.save'))
    expect(save).toBeTruthy()
    await save!.trigger('click')
    await flushPromises()

    expect(apiMocks.update).toHaveBeenCalledWith(
      'api-prompt',
      expect.objectContaining({
        api_prompt: expect.objectContaining({
          source: 'local',
          templates: expect.arrayContaining([
            expect.objectContaining({ id: 'local-focus', prompt: 'Use local config.' }),
          ]),
        }),
      })
    )
  })
})
