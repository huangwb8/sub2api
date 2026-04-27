import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import TurnstileWidget from '@/components/TurnstileWidget.vue'

interface CapturedTurnstileOptions {
  callback: (token: string) => void
  'expired-callback'?: () => void
  'error-callback'?: () => void
}

const turnstileWindow = window as Window & {
  turnstile?: {
    render: ReturnType<typeof vi.fn>
    reset: ReturnType<typeof vi.fn>
    remove: ReturnType<typeof vi.fn>
  }
  onTurnstileLoad?: () => void
}

describe('TurnstileWidget', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    delete turnstileWindow.turnstile
    delete turnstileWindow.onTurnstileLoad
  })

  it('emits verification events and exposes reset/state controls', async () => {
    let options: CapturedTurnstileOptions | null = null
    const reset = vi.fn()

    turnstileWindow.turnstile = {
      render: vi.fn((_container: HTMLElement, renderOptions: CapturedTurnstileOptions) => {
        options = renderOptions
        return 'widget-1'
      }),
      reset,
      remove: vi.fn()
    }

    const wrapper = mount(TurnstileWidget, {
      props: {
        siteKey: 'site-key'
      }
    })
    await flushPromises()

    expect(turnstileWindow.turnstile!.render).toHaveBeenCalledOnce()
    options?.callback('response-token')
    expect(wrapper.emitted('verify')?.[0]).toEqual(['response-token'])

    const exposed = wrapper.vm as unknown as {
      reset: () => void
      getWidgetState: () => { widgetId: string | null; scriptLoaded: boolean; verifiedAt: number | null }
    }
    expect(exposed.getWidgetState()).toMatchObject({
      widgetId: 'widget-1',
      scriptLoaded: true
    })
    expect(exposed.getWidgetState().verifiedAt).toEqual(expect.any(Number))

    exposed.reset()
    expect(reset).toHaveBeenCalledWith('widget-1')
    expect(exposed.getWidgetState().verifiedAt).toBeNull()
  })

  it('clears verification state when token expires or widget errors', async () => {
    let options: CapturedTurnstileOptions | null = null

    turnstileWindow.turnstile = {
      render: vi.fn((_container: HTMLElement, renderOptions: CapturedTurnstileOptions) => {
        options = renderOptions
        return 'widget-1'
      }),
      reset: vi.fn(),
      remove: vi.fn()
    }

    const wrapper = mount(TurnstileWidget, {
      props: {
        siteKey: 'site-key'
      }
    })
    await flushPromises()

    const exposed = wrapper.vm as unknown as {
      getWidgetState: () => { verifiedAt: number | null }
    }

    options?.callback('response-token')
    expect(exposed.getWidgetState().verifiedAt).toEqual(expect.any(Number))

    options?.['expired-callback']?.()
    expect(wrapper.emitted('expire')).toHaveLength(1)
    expect(exposed.getWidgetState().verifiedAt).toBeNull()

    options?.callback('another-token')
    options?.['error-callback']?.()
    expect(wrapper.emitted('error')).toHaveLength(1)
    expect(exposed.getWidgetState().verifiedAt).toBeNull()
  })
})
