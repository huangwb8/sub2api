import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { createRouter, createMemoryHistory } from 'vue-router'

import { i18n } from '@/i18n'
import AppSidebar from '../AppSidebar.vue'
import { useAppStore } from '@/stores'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useAuthStore } from '@/stores/auth'
import type { PublicSettings, User } from '@/types'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')

const makePublicSettings = (overrides: Partial<PublicSettings> = {}): PublicSettings => ({
  registration_enabled: true,
  email_verify_enabled: false,
  registration_email_suffix_whitelist: [],
  promo_code_enabled: true,
  password_reset_enabled: true,
  invitation_code_enabled: false,
  affiliate_enabled: false,
  turnstile_enabled: false,
  turnstile_site_key: '',
  site_name: 'Sub2API',
  site_logo: '',
  site_subtitle: '',
  api_base_url: '',
  contact_info: '',
  doc_url: '',
  home_content: '',
  terms_of_service_content: '',
  privacy_policy_content: '',
  hide_ccs_import_button: false,
  payment_enabled: false,
  table_default_page_size: 20,
  table_page_size_options: [10, 20, 50, 100],
  custom_menu_items: [],
  custom_endpoints: [],
  linuxdo_oauth_enabled: false,
  oidc_oauth_enabled: false,
  oidc_oauth_provider_name: 'OIDC',
  backend_mode_enabled: false,
  version: 'dev',
  ...overrides
})

const adminUser: User = {
  id: 1,
  username: 'admin',
  email: 'admin@example.com',
  avatar_url: '',
  avatar_type: 'generated',
  avatar_style: 'classic_letter',
  role: 'admin',
  balance: 0,
  concurrency: 0,
  status: 'active',
  allowed_groups: null,
  created_at: '2026-04-26T00:00:00Z',
  updated_at: '2026-04-26T00:00:00Z'
}

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar navigation', () => {
  it('管理员侧边栏将 /admin/dashboard 标记为控制台文案', () => {
    expect(componentSource).toMatch(/path:\s*'\/admin\/dashboard'[\s\S]*label:\s*t\('nav\.adminConsole'\)/)
  })

  it('管理员侧边栏会在代理与兑换码之间显示调度机制入口', () => {
    const proxiesIndex = componentSource.indexOf("path: '/admin/proxies'")
    const schedulingIndex = componentSource.indexOf("path: '/admin/scheduling-mechanisms'")
    const redeemIndex = componentSource.indexOf("path: '/admin/redeem'")

    expect(proxiesIndex).toBeGreaterThan(-1)
    expect(schedulingIndex).toBeGreaterThan(proxiesIndex)
    expect(redeemIndex).toBeGreaterThan(schedulingIndex)
  })

  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: () => ({
        matches: false,
        media: '',
        onchange: null,
        addListener: () => {},
        removeListener: () => {},
        addEventListener: () => {},
        removeEventListener: () => {},
        dispatchEvent: () => false
      })
    })
  })

  it('点击左上角品牌区时跳转到 home 页面', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/dashboard', component: { template: '<div>Dashboard</div>' } },
        { path: '/home', component: { template: '<div>Home</div>' } }
      ]
    })

    const appStore = useAppStore()
    appStore.siteName = 'BenszAPI'
    appStore.siteVersion = '1.0.9'
    appStore.publicSettingsLoaded = true

    await router.push('/dashboard')
    await router.isReady()

    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [router, i18n],
        stubs: {
          VersionBadge: {
            template: '<div class="version-badge-stub" />'
          }
        }
      }
    })

    await wrapper.get('[data-testid="sidebar-home-link"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/home')
  })

  it('邀请返利关闭时隐藏管理员侧边栏入口', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }]
    })

    const appStore = useAppStore()
    appStore.cachedPublicSettings = makePublicSettings({ affiliate_enabled: false })
    appStore.publicSettingsLoaded = true

    const authStore = useAuthStore()
    authStore.user = adminUser
    authStore.token = 'admin-token'

    const adminSettingsStore = useAdminSettingsStore()
    adminSettingsStore.loaded = true

    await router.push('/admin/dashboard')
    await router.isReady()

    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [router, i18n],
        stubs: {
          VersionBadge: {
            template: '<div class="version-badge-stub" />'
          }
        }
      }
    })

    expect(wrapper.find('a[href="/admin/affiliate"]').exists()).toBe(false)
  })

  it('邀请返利开启时显示管理员侧边栏入口', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }]
    })

    const appStore = useAppStore()
    appStore.cachedPublicSettings = makePublicSettings({ affiliate_enabled: true })
    appStore.publicSettingsLoaded = true

    const authStore = useAuthStore()
    authStore.user = adminUser
    authStore.token = 'admin-token'

    const adminSettingsStore = useAdminSettingsStore()
    adminSettingsStore.loaded = true

    await router.push('/admin/dashboard')
    await router.isReady()

    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [router, i18n],
        stubs: {
          VersionBadge: {
            template: '<div class="version-badge-stub" />'
          }
        }
      }
    })

    expect(wrapper.find('a[href="/admin/affiliate"]').exists()).toBe(true)
  })
})
