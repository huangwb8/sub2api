import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { pollOrderStatus, cancelOrder, verifyOrder, showError } = vi.hoisted(() => ({
  pollOrderStatus: vi.fn(),
  cancelOrder: vi.fn(),
  verifyOrder: vi.fn(),
  showError: vi.fn(),
}))

const localStorageMock = {
  getItem: vi.fn(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
}

Object.defineProperty(globalThis, 'localStorage', {
  value: localStorageMock,
  configurable: true,
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores/payment', () => ({
  usePaymentStore: () => ({
    pollOrderStatus,
  }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
  }),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    cancelOrder,
    verifyOrder,
  },
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: () => 'error',
  extractI18nErrorMessage: () => 'error',
}))

vi.mock('@/utils/format', () => ({
  formatPaymentAmount: (amount: number) => `¥${Number(amount).toFixed(2)}`,
}))

vi.mock('qrcode', () => ({
  default: {
    toCanvas: vi.fn().mockResolvedValue(undefined),
  },
}))

import PaymentStatusPanel from '../PaymentStatusPanel.vue'

describe('PaymentStatusPanel', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('pending order with out_trade_no triggers verify recovery and emits success', async () => {
    pollOrderStatus.mockResolvedValue({
      id: 101,
      out_trade_no: 'sub2_test_pending',
      pay_amount: 19.9,
      status: 'PENDING',
    })
    verifyOrder.mockResolvedValue({
      data: {
        id: 101,
        out_trade_no: 'sub2_test_pending',
        pay_amount: 19.9,
        status: 'COMPLETED',
      },
    })

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 101,
        qrCode: '',
        expiresAt: new Date(Date.now() + 30 * 60 * 1000).toISOString(),
        paymentType: 'alipay',
        payUrl: 'https://pay.example.com',
        orderType: 'subscription',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    vi.advanceTimersByTime(3000)
    await flushPromises()

    expect(pollOrderStatus).toHaveBeenCalledWith(101)
    expect(verifyOrder).toHaveBeenCalledWith('sub2_test_pending')
    expect(wrapper.emitted('success')).toBeTruthy()
    expect(wrapper.text()).toContain('payment.result.subscriptionSuccess')
    expect(wrapper.text()).toContain('¥19.90')
  })
})
