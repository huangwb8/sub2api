import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountStatsModal from '../AccountStatsModal.vue'

const {
  getStatsSummary,
  getStatsDetails,
} = vi.hoisted(() => ({
  getStatsSummary: vi.fn(),
  getStatsDetails: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getStatsSummary,
      getStatsDetails,
    },
  },
}))

vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data'],
    template: '<div data-test="trend-line">{{ data?.labels?.length ?? 0 }}</div>',
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const BaseDialogStub = {
  props: ['show', 'title', 'width'],
  template: `
    <div v-if="show" data-test="dialog">
      <div data-test="title">{{ title }}</div>
      <slot />
      <slot name="footer" />
    </div>
  `,
}

const LoadingSpinnerStub = {
  template: '<div data-test="loading-spinner">loading</div>',
}

const ModelDistributionChartStub = {
  props: ['modelStats', 'loading'],
  template: '<div data-test="model-chart">{{ loading ? "loading" : `models:${modelStats?.length ?? 0}` }}</div>',
}

const EndpointDistributionChartStub = {
  props: ['endpointStats', 'loading', 'title'],
  template: '<div data-test="endpoint-chart">{{ title }}|{{ loading ? "loading" : `endpoints:${endpointStats?.length ?? 0}` }}</div>',
}

const account = {
  id: 101,
  name: 'Account A',
  status: 'active',
} as const

const summaryResponse = {
  summary: {
    days: 30,
    actual_days_used: 2,
    total_cost: 12.34,
    total_user_cost: 10.11,
    total_standard_cost: 15.67,
    total_requests: 123,
    total_tokens: 4567,
    avg_daily_cost: 6.17,
    avg_daily_user_cost: 5.05,
    avg_daily_requests: 61.5,
    avg_daily_tokens: 2283.5,
    avg_duration_ms: 987,
    today: {
      date: '2026-04-28',
      cost: 1.23,
      user_cost: 1.11,
      requests: 12,
      tokens: 345,
    },
    highest_cost_day: {
      date: '2026-04-27',
      label: '04/27',
      cost: 9.87,
      user_cost: 8.76,
      requests: 50,
    },
    highest_request_day: {
      date: '2026-04-27',
      label: '04/27',
      requests: 50,
      cost: 9.87,
      user_cost: 8.76,
    },
  },
}

const detailsResponse = {
  history: [
    {
      date: '2026-04-27',
      label: '04/27',
      requests: 50,
      tokens: 2000,
      cost: 10,
      actual_cost: 9.87,
      user_cost: 8.76,
    },
  ],
  models: [
    {
      model: 'gpt-4.1',
      requests: 50,
      input_tokens: 1000,
      output_tokens: 1000,
      cache_creation_tokens: 0,
      cache_read_tokens: 0,
      total_tokens: 2000,
      cost: 10,
      actual_cost: 9.87,
    },
  ],
  endpoints: [
    {
      endpoint: '/v1/chat/completions',
      requests: 50,
      total_tokens: 2000,
      cost: 10,
      actual_cost: 9.87,
    },
  ],
  upstream_endpoints: [
    {
      endpoint: 'https://api.openai.com/v1/chat/completions',
      requests: 50,
      total_tokens: 2000,
      cost: 10,
      actual_cost: 9.87,
    },
  ],
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function mountModal(accountOverrides?: Partial<typeof account>) {
  return mount(AccountStatsModal, {
    props: {
      show: true,
      account: {
        ...account,
        ...accountOverrides,
      },
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        LoadingSpinner: LoadingSpinnerStub,
        ModelDistributionChart: ModelDistributionChartStub,
        EndpointDistributionChart: EndpointDistributionChartStub,
        Icon: true,
      },
    },
  })
}

describe('AccountStatsModal', () => {
  beforeEach(() => {
    getStatsSummary.mockReset()
    getStatsDetails.mockReset()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('先显示摘要，再异步加载图表明细', async () => {
    const summaryTask = deferred<typeof summaryResponse>()
    const detailsTask = deferred<typeof detailsResponse>()
    getStatsSummary.mockReturnValue(summaryTask.promise)
    getStatsDetails.mockReturnValue(detailsTask.promise)

    const wrapper = mountModal()
    await flushPromises()

    expect(getStatsSummary).toHaveBeenCalledTimes(1)
    expect(getStatsDetails).not.toHaveBeenCalled()
    expect(wrapper.findAll('[data-test="loading-spinner"]').length).toBeGreaterThan(0)

    summaryTask.resolve(summaryResponse)
    await flushPromises()

    expect(getStatsDetails).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('$12.34')
    expect(wrapper.find('[data-test="model-chart"]').text()).toContain('loading')

    detailsTask.resolve(detailsResponse)
    await flushPromises()

    expect(wrapper.find('[data-test="trend-line"]').text()).toBe('1')
    expect(wrapper.find('[data-test="model-chart"]').text()).toContain('models:1')
  })

  it('同账号短时间重复打开时命中前端缓存', async () => {
    getStatsSummary.mockResolvedValue(summaryResponse)
    getStatsDetails.mockResolvedValue(detailsResponse)

    const wrapper = mountModal({ id: 202 })
    await flushPromises()

    expect(getStatsSummary).toHaveBeenCalledTimes(1)
    expect(getStatsDetails).toHaveBeenCalledTimes(1)

    await wrapper.setProps({ show: false })
    await flushPromises()

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(getStatsSummary).toHaveBeenCalledTimes(1)
    expect(getStatsDetails).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('$12.34')
  })
})
