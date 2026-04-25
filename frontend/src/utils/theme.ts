export type ThemeMode = 'light' | 'dark' | 'system' | 'schedule'

export interface ThemePreferences {
  mode: ThemeMode
  darkStart: string
  darkEnd: string
}

export interface ThemeApplyResult {
  isDark: boolean
  preferences: ThemePreferences
}

const LEGACY_THEME_KEY = 'theme'
const THEME_MODE_KEY = 'theme-mode'
const THEME_DARK_START_KEY = 'theme-dark-start'
const THEME_DARK_END_KEY = 'theme-dark-end'
const DEFAULT_DARK_START = '18:00'
const DEFAULT_DARK_END = '06:00'
const THEME_CHANGE_EVENT = 'sub2api-theme-change'
const VALID_THEME_MODES = new Set<ThemeMode>(['light', 'dark', 'system', 'schedule'])
const TIME_PATTERN = /^([01]\d|2[0-3]):[0-5]\d$/

let runtimeStarted = false
let mediaCleanup: (() => void) | null = null
let storageCleanup: (() => void) | null = null
let scheduleTimer: ReturnType<typeof setInterval> | null = null

function canUseBrowserStorage(): boolean {
  return typeof window !== 'undefined' && typeof window.localStorage !== 'undefined'
}

function safeGetItem(key: string): string | null {
  if (!canUseBrowserStorage()) return null
  try {
    return window.localStorage.getItem(key)
  } catch {
    return null
  }
}

function safeSetItem(key: string, value: string): void {
  if (!canUseBrowserStorage()) return
  try {
    window.localStorage.setItem(key, value)
  } catch {
    // Ignore storage failures so theme rendering still works in restricted browsers.
  }
}

function normalizeTime(value: string | null, fallback: string): string {
  if (value && TIME_PATTERN.test(value)) return value
  return fallback
}

function getSystemPrefersDark(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function timeToMinutes(value: string): number {
  const [hour, minute] = value.split(':').map(Number)
  return hour * 60 + minute
}

function isInDarkWindow(start: string, end: string, date: Date): boolean {
  const startMinutes = timeToMinutes(start)
  const endMinutes = timeToMinutes(end)
  const currentMinutes = date.getHours() * 60 + date.getMinutes()

  if (startMinutes === endMinutes) return true
  if (startMinutes < endMinutes) {
    return currentMinutes >= startMinutes && currentMinutes < endMinutes
  }
  return currentMinutes >= startMinutes || currentMinutes < endMinutes
}

function dispatchThemeChange(result: ThemeApplyResult): void {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent(THEME_CHANGE_EVENT, { detail: result }))
}

export function readThemePreferences(): ThemePreferences {
  const storedMode = safeGetItem(THEME_MODE_KEY)
  const legacyTheme = safeGetItem(LEGACY_THEME_KEY)
  const mode = VALID_THEME_MODES.has(storedMode as ThemeMode)
    ? (storedMode as ThemeMode)
    : legacyTheme === 'dark' || legacyTheme === 'light'
      ? legacyTheme
      : 'system'

  return {
    mode,
    darkStart: normalizeTime(safeGetItem(THEME_DARK_START_KEY), DEFAULT_DARK_START),
    darkEnd: normalizeTime(safeGetItem(THEME_DARK_END_KEY), DEFAULT_DARK_END)
  }
}

export function resolveDarkTheme(preferences = readThemePreferences(), date = new Date()): boolean {
  switch (preferences.mode) {
    case 'dark':
      return true
    case 'light':
      return false
    case 'schedule':
      return isInDarkWindow(preferences.darkStart, preferences.darkEnd, date)
    case 'system':
    default:
      return getSystemPrefersDark()
  }
}

export function applyThemePreferences(
  preferences = readThemePreferences(),
  date = new Date()
): ThemeApplyResult {
  const isDark = resolveDarkTheme(preferences, date)
  if (typeof document !== 'undefined') {
    document.documentElement.classList.toggle('dark', isDark)
  }
  safeSetItem(LEGACY_THEME_KEY, isDark ? 'dark' : 'light')
  dispatchThemeChange({ isDark, preferences })
  return { isDark, preferences }
}

export function saveThemePreferences(preferences: ThemePreferences): ThemeApplyResult {
  const normalized: ThemePreferences = {
    mode: VALID_THEME_MODES.has(preferences.mode) ? preferences.mode : 'system',
    darkStart: normalizeTime(preferences.darkStart, DEFAULT_DARK_START),
    darkEnd: normalizeTime(preferences.darkEnd, DEFAULT_DARK_END)
  }

  safeSetItem(THEME_MODE_KEY, normalized.mode)
  safeSetItem(THEME_DARK_START_KEY, normalized.darkStart)
  safeSetItem(THEME_DARK_END_KEY, normalized.darkEnd)
  return applyThemePreferences(normalized)
}

export function setExplicitThemeMode(mode: 'light' | 'dark'): ThemeApplyResult {
  const current = readThemePreferences()
  return saveThemePreferences({ ...current, mode })
}

export function toggleExplicitThemeMode(): ThemeApplyResult {
  return setExplicitThemeMode(isDarkThemeActive() ? 'light' : 'dark')
}

export function isDarkThemeActive(): boolean {
  if (typeof document === 'undefined') return resolveDarkTheme()
  return document.documentElement.classList.contains('dark')
}

export function onThemeChange(callback: (result: ThemeApplyResult) => void): () => void {
  if (typeof window === 'undefined') return () => {}
  const handler = (event: Event) => {
    const detail = (event as CustomEvent<ThemeApplyResult>).detail
    callback(detail ?? { isDark: isDarkThemeActive(), preferences: readThemePreferences() })
  }
  window.addEventListener(THEME_CHANGE_EVENT, handler)
  return () => window.removeEventListener(THEME_CHANGE_EVENT, handler)
}

export function startThemeRuntime(): void {
  if (runtimeStarted || typeof window === 'undefined') {
    applyThemePreferences()
    return
  }
  runtimeStarted = true
  applyThemePreferences()

  if (window.matchMedia) {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => {
      if (readThemePreferences().mode === 'system') {
        applyThemePreferences()
      }
    }
    media.addEventListener('change', handler)
    mediaCleanup = () => media.removeEventListener('change', handler)
  }

  const storageHandler = (event: StorageEvent) => {
    if (
      event.key === LEGACY_THEME_KEY ||
      event.key === THEME_MODE_KEY ||
      event.key === THEME_DARK_START_KEY ||
      event.key === THEME_DARK_END_KEY
    ) {
      applyThemePreferences()
    }
  }
  window.addEventListener('storage', storageHandler)
  storageCleanup = () => window.removeEventListener('storage', storageHandler)

  scheduleTimer = setInterval(() => {
    if (readThemePreferences().mode === 'schedule') {
      applyThemePreferences()
    }
  }, 60_000)
}

export function stopThemeRuntime(): void {
  mediaCleanup?.()
  storageCleanup?.()
  if (scheduleTimer) clearInterval(scheduleTimer)
  mediaCleanup = null
  storageCleanup = null
  scheduleTimer = null
  runtimeStarted = false
}

export const themeStorageKeys = {
  legacyTheme: LEGACY_THEME_KEY,
  mode: THEME_MODE_KEY,
  darkStart: THEME_DARK_START_KEY,
  darkEnd: THEME_DARK_END_KEY
} as const
