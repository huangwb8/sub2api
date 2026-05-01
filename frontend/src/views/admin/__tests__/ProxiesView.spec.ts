import { beforeEach, describe, expect, it, vi } from 'vitest'
import { computed, defineComponent, h, ref } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

const {
  listProxiesMock,
  getProxyAccountsMock,
  getAllWithCountMock,
  updateAccountMock,
  getProxyFailoverSettingsMock,
  showSuccessMock,
  showErrorMock,
  showInfoMock
} = vi.hoisted(() => ({
  listProxiesMock: vi.fn(),
  getProxyAccountsMock: vi.fn(),
  getAllWithCountMock: vi.fn(),
  updateAccountMock: vi.fn(),
  getProxyFailoverSettingsMock: vi.fn(),
  showSuccessMock: vi.fn(),
  showErrorMock: vi.fn(),
  showInfoMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: showSuccessMock,
    showError: showErrorMock,
    showInfo: showInfoMock
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    proxies: {
      list: listProxiesMock,
      getProxyAccounts: getProxyAccountsMock,
      getAllWithCount: getAllWithCountMock,
      batchCreate: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      batchDelete: vi.fn(),
      exportData: vi.fn(),
      importData: vi.fn(),
      testProxy: vi.fn(),
      checkProxyQuality: vi.fn()
    },
    accounts: {
      update: updateAccountMock
    },
    settings: {
      getProxyFailoverSettings: getProxyFailoverSettingsMock,
      updateProxyFailoverSettings: vi.fn()
    }
  }
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn()
  })
}))

vi.mock('@/composables/useSwipeSelect', () => ({
  useSwipeSelect: vi.fn()
}))

vi.mock('@/composables/useTableSelection', () => ({
  useTableSelection: () => {
    const selectedSet = ref(new Set<number>())
    return {
      selectedSet,
      selectedCount: computed(() => selectedSet.value.size),
      allVisibleSelected: computed(() => false),
      isSelected: () => false,
      select: (id: number) => selectedSet.value.add(id),
      deselect: (id: number) => selectedSet.value.delete(id),
      clear: () => selectedSet.value.clear(),
      removeMany: (ids: number[]) => {
        ids.forEach((id) => selectedSet.value.delete(id))
      },
      toggleVisible: vi.fn()
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (typeof params?.count !== 'undefined') {
          return `${key}:${params.count}`
        }
        if (typeof params?.name === 'string') {
          return `${key}:${params.name}`
        }
        return key
      }
    })
  }
})

import ProxiesView from '../ProxiesView.vue'

const AppLayoutStub = defineComponent({
  name: 'AppLayout',
  template: '<div><slot /></div>'
})

const TablePageLayoutStub = defineComponent({
  name: 'TablePageLayout',
  template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
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
  template: `
    <div v-if="show">
      <div class="dialog-title">{{ title }}</div>
      <slot />
      <slot name="footer" />
    </div>
  `
})

const SelectStub = defineComponent({
  name: 'UiSelectStub',
  props: {
    modelValue: {
      type: [String, Number, null],
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
  setup(props, { emit, attrs }) {
    const normalizeValue = (value: string) => {
      const numericOption = (props.options as Array<{ value: unknown }>).find(
        (option) => String(option.value) === value && typeof option.value === 'number'
      )
      return numericOption ? numericOption.value : value
    }

    return () =>
      h(
        'select',
        {
          ...attrs,
          disabled: props.disabled,
          value: props.modelValue == null ? '' : String(props.modelValue),
          onChange: (event: Event) => {
            const value = normalizeValue((event.target as HTMLSelectElement).value)
            emit('update:modelValue', value)
            emit('change', value)
          }
        },
        (props.options as Array<{ value: string | number; label: string; disabled?: boolean }>).map((option) =>
          h(
            'option',
            {
              key: String(option.value),
              value: String(option.value),
              disabled: option.disabled
            },
            option.label
          )
        )
      )
  }
})

const DataTableStub = defineComponent({
  name: 'DataTable',
  inheritAttrs: false,
  props: {
    columns: {
      type: Array,
      default: () => []
    },
    data: {
      type: Array,
      default: () => []
    },
    loading: {
      type: Boolean,
      default: false
    }
  },
  emits: ['sort'],
  setup(props, { slots, attrs }) {
    return () => {
      if (!props.loading && (props.data as Array<unknown>).length === 0) {
        return slots.empty?.()
      }

      return h(
        'div',
        attrs,
        (props.data as Array<Record<string, unknown>>).map((row) =>
          h(
            'div',
            { key: String(row.id), 'data-testid': `proxy-row-${row.id}` },
            (props.columns as Array<{ key: string }>).map((column) =>
              h(
                'div',
                { key: column.key, 'data-testid': `cell-${row.id}-${column.key}` },
                slots[`cell-${column.key}`]
                  ? slots[`cell-${column.key}`]!({
                      row,
                      value: row[column.key]
                    })
                  : String(row[column.key] ?? '')
              )
            )
          )
        )
      )
    }
  }
})

function buildProxy(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    name: '当前代理',
    protocol: 'http',
    host: '1.1.1.1',
    port: 8080,
    username: null,
    password: null,
    status: 'active',
    account_count: 1,
    latency_status: 'success',
    latency_ms: 120,
    quality_status: 'healthy',
    quality_score: 98,
    quality_grade: 'A',
    created_at: '2026-05-01T00:00:00Z',
    updated_at: '2026-05-01T00:00:00Z',
    ...overrides
  } as any
}

function mountView() {
  return mount(ProxiesView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        ImportDataModal: true,
        Select: SelectStub,
        Icon: true,
        PlatformTypeBadge: true
      }
    }
  })
}

describe('ProxiesView', () => {
  beforeEach(() => {
    listProxiesMock.mockReset()
    getProxyAccountsMock.mockReset()
    getAllWithCountMock.mockReset()
    updateAccountMock.mockReset()
    getProxyFailoverSettingsMock.mockReset()
    showSuccessMock.mockReset()
    showErrorMock.mockReset()
    showInfoMock.mockReset()

    listProxiesMock.mockResolvedValue({
      items: [buildProxy()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getProxyFailoverSettingsMock.mockResolvedValue({
      enabled: true,
      auto_test_enabled: true,
      probe_interval_minutes: 5,
      failure_threshold: 3,
      failure_window_minutes: 10,
      cooldown_minutes: 15,
      half_open_probe_accounts: 2,
      cooldown_backoff_factor: 2,
      max_cooldown_minutes: 120,
      max_accounts_per_proxy: 6,
      max_migrations_per_cycle: 12,
      prefer_same_country: true,
      only_openai_oauth: false,
      temp_unsched_minutes: 10
    })
  })

  it('在代理账号弹窗中只允许切换到可用代理，并在成功后移除当前账号', async () => {
    getProxyAccountsMock.mockResolvedValue([
      {
        id: 101,
        name: 'OpenAI-1',
        platform: 'openai',
        type: 'oauth',
        notes: '主账号'
      }
    ])
    getAllWithCountMock.mockResolvedValue([
      buildProxy(),
      buildProxy({
        id: 2,
        name: '健康代理',
        host: '2.2.2.2',
        account_count: 0,
        latency_status: 'success',
        quality_status: 'healthy'
      }),
      buildProxy({
        id: 3,
        name: '异常代理',
        host: '3.3.3.3',
        account_count: 0,
        latency_status: 'failed',
        quality_status: 'failed'
      })
    ])
    updateAccountMock.mockResolvedValue({
      id: 101,
      proxy_id: 2
    })

    const wrapper = mountView()
    await flushPromises()

    const accountCountButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('admin.groups.accountsCount:1'))

    expect(accountCountButton).toBeTruthy()

    await accountCountButton!.trigger('click')
    await flushPromises()

    const targetSelect = wrapper.get('[data-testid="proxy-transfer-select-101"]')
    const optionValues = targetSelect.findAll('option').map((option) => option.attributes('value'))

    expect(optionValues).toContain('2')
    expect(optionValues).not.toContain('3')

    await targetSelect.setValue('2')
    await wrapper.get('[data-testid="proxy-transfer-submit-101"]').trigger('click')
    await flushPromises()

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock).toHaveBeenCalledWith(101, { proxy_id: 2 })
    expect(wrapper.text()).not.toContain('OpenAI-1')
    expect(showSuccessMock).toHaveBeenCalled()
  })
})
