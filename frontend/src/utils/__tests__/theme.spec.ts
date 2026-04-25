import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  applyThemePreferences,
  readThemePreferences,
  resolveDarkTheme,
  saveThemePreferences,
  setExplicitThemeMode,
  stopThemeRuntime
} from '../theme'

function mockSystemTheme(matches: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockReturnValue({
      matches,
      media: '(prefers-color-scheme: dark)',
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn()
    })
  })
}

describe('theme preferences', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    mockSystemTheme(false)
  })

  afterEach(() => {
    stopThemeRuntime()
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    vi.restoreAllMocks()
  })

  it('defaults to following the system appearance', () => {
    mockSystemTheme(true)

    expect(readThemePreferences().mode).toBe('system')
    expect(applyThemePreferences().isDark).toBe(true)
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('migrates legacy explicit theme values', () => {
    localStorage.setItem('theme', 'dark')

    expect(readThemePreferences().mode).toBe('dark')
    expect(applyThemePreferences().isDark).toBe(true)
  })

  it('saves explicit light and dark modes', () => {
    setExplicitThemeMode('dark')
    expect(localStorage.getItem('theme-mode')).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)

    setExplicitThemeMode('light')
    expect(localStorage.getItem('theme-mode')).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('supports scheduled dark windows across midnight', () => {
    const preferences = {
      mode: 'schedule' as const,
      darkStart: '18:00',
      darkEnd: '06:00'
    }

    expect(resolveDarkTheme(preferences, new Date('2026-04-25T19:30:00'))).toBe(true)
    expect(resolveDarkTheme(preferences, new Date('2026-04-25T03:30:00'))).toBe(true)
    expect(resolveDarkTheme(preferences, new Date('2026-04-25T12:00:00'))).toBe(false)
  })

  it('persists scheduled times and applies the current result', () => {
    const result = saveThemePreferences({
      mode: 'schedule',
      darkStart: '00:00',
      darkEnd: '23:59'
    })

    expect(result.isDark).toBe(true)
    expect(localStorage.getItem('theme-mode')).toBe('schedule')
    expect(localStorage.getItem('theme-dark-start')).toBe('00:00')
    expect(localStorage.getItem('theme-dark-end')).toBe('23:59')
  })
})
