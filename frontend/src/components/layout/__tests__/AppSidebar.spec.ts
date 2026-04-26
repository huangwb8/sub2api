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

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')

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
})
