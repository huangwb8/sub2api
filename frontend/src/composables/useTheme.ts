import { onMounted, onUnmounted, ref } from 'vue'
import {
  isDarkThemeActive,
  onThemeChange,
  toggleExplicitThemeMode,
  type ThemeApplyResult
} from '@/utils/theme'

export function useTheme() {
  const isDark = ref(isDarkThemeActive())
  let cleanup: (() => void) | null = null

  function sync(result?: ThemeApplyResult) {
    isDark.value = result?.isDark ?? isDarkThemeActive()
  }

  function toggleTheme() {
    sync(toggleExplicitThemeMode())
  }

  onMounted(() => {
    sync()
    cleanup = onThemeChange(sync)
  })

  onUnmounted(() => {
    cleanup?.()
    cleanup = null
  })

  return {
    isDark,
    toggleTheme
  }
}
