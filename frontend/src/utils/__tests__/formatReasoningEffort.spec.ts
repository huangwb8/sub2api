import { describe, expect, it, vi } from 'vitest'

vi.mock('@/i18n', () => ({
  i18n: {
    global: {
      t: () => '-'
    }
  },
  getLocale: () => 'en'
}))

import { formatReasoningEffort } from '../format'

describe('formatReasoningEffort', () => {
  it('formats known effort levels', () => {
    expect(formatReasoningEffort('low')).toBe('Low')
    expect(formatReasoningEffort('medium')).toBe('Medium')
    expect(formatReasoningEffort('high')).toBe('High')
    expect(formatReasoningEffort('max')).toBe('Max')
    expect(formatReasoningEffort('xhigh')).toBe('XHigh')
    expect(formatReasoningEffort('extra-high')).toBe('XHigh')
  })

  it('returns dash for empty or minimal effort', () => {
    expect(formatReasoningEffort('')).toBe('-')
    expect(formatReasoningEffort('   ')).toBe('-')
    expect(formatReasoningEffort('minimal')).toBe('-')
    expect(formatReasoningEffort('none')).toBe('-')
  })
})
