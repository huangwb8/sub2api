import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import HomeView from '../HomeView.vue'

const authState = {
  isAuthenticated: false,
  isAdmin: false,
  user: null as null | { email?: string },
  checkAuth: vi.fn(),
}

const appState = {
  publicSettingsLoaded: true,
  cachedPublicSettings: {
    home_content: '',
    payment_enabled: true,
    site_name: 'BenszAPI',
    site_logo: '',
    site_subtitle: '',
    doc_url: '',
    terms_of_service_content: '',
    privacy_policy_content: '',
  },
  siteName: 'BenszAPI',
  siteLogo: '',
  siteSubtitle: '',
  docUrl: '',
  fetchPublicSettings: vi.fn(),
}

vi.mock('@/stores', () => ({
  useAuthStore: () => authState,
  useAppStore: () => appState,
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => ({ path: '/home', query: {} }),
  }
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

describe('HomeView custom home content CSP handling', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation(() => ({
        matches: false,
        media: '',
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    })
    authState.isAuthenticated = false
    authState.isAdmin = false
    authState.user = null
    authState.checkAuth.mockReset()
    appState.publicSettingsLoaded = true
    appState.fetchPublicSettings.mockReset()
    appState.cachedPublicSettings.home_content = ''
    appState.cachedPublicSettings.terms_of_service_content = ''
    appState.cachedPublicSettings.privacy_policy_content = ''
    delete (window as Window & typeof globalThis & { __HOME_CONTENT_NONCE_TEST__?: boolean }).__HOME_CONTENT_NONCE_TEST__
    document.head.querySelectorAll('[data-test-csp-nonce]').forEach(node => node.remove())
  })

  it('applies the current CSP nonce to dynamically executed home_content scripts', async () => {
    const nonceScript = document.createElement('script')
    nonceScript.dataset.testCspNonce = 'true'
    nonceScript.nonce = 'test-csp-nonce'
    document.head.appendChild(nonceScript)

    appState.cachedPublicSettings.home_content = `
      <section class="demo-home">hello</section>
      <script>window.__HOME_CONTENT_NONCE_TEST__ = true;</script>
    `

    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          Icon: true,
          LocaleSwitcher: true,
          'router-link': {
            props: ['to'],
            template: '<a :href="typeof to === \'string\' ? to : to?.path"><slot /></a>'
          },
        },
      },
    })

    await flushPromises()

    const inlineScript = wrapper.element.querySelector('script')
    expect(inlineScript).not.toBeNull()
    expect((inlineScript as HTMLScriptElement).nonce).toBe('test-csp-nonce')
  })

  it('shows legal links in the footer when public legal content is configured', async () => {
    appState.cachedPublicSettings.terms_of_service_content = '# Terms'
    appState.cachedPublicSettings.privacy_policy_content = '# Privacy'

    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          Icon: true,
          LocaleSwitcher: true,
          'router-link': {
            props: ['to'],
            template: '<a :href="typeof to === \'string\' ? to : to?.path"><slot /></a>'
          },
        },
      },
    })

    await flushPromises()

    const links = Array.from(wrapper.findAll('a')).map((node) => ({
      href: node.attributes('href'),
      text: node.text()
    }))
    expect(links).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ href: '/legal/terms', text: 'legal.terms.shortTitle' }),
        expect.objectContaining({ href: '/legal/privacy', text: 'legal.privacy.shortTitle' })
      ])
    )
  })
})
