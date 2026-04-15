import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { i18n } from '@/i18n'
import {
  formatBalanceAmount,
  formatCNY,
  formatCurrency,
  formatMoney,
  formatUSD,
  formatUsageCost,
} from '../format'

describe('currency formatting', () => {
  const originalLocale = i18n.global.locale.value

  beforeEach(() => {
    i18n.global.locale.value = 'en'
  })

  afterEach(() => {
    i18n.global.locale.value = originalLocale
  })

  it('formats CNY amounts with yuan semantics', () => {
    expect(formatCNY(12.5)).toBe('¥12.50')
    expect(formatBalanceAmount(12.5)).toBe('¥12.50')
    expect(formatMoney(12.5, 'CNY')).toBe('¥12.50')
  })

  it('formats USD amounts with dollar semantics', () => {
    expect(formatUSD(12.5)).toBe('$12.50')
    expect(formatUsageCost(12.5)).toBe('$12.50')
    expect(formatMoney(12.5, 'USD')).toBe('$12.50')
  })

  it('keeps extra precision for very small amounts', () => {
    expect(formatUSD(0.001234)).toBe('$0.001234')
    expect(formatCNY(0.001234)).toBe('¥0.001234')
  })

  it('returns currency-specific nullish fallbacks', () => {
    expect(formatCurrency(undefined, 'USD')).toBe('$0.00')
    expect(formatCurrency(null, 'CNY')).toBe('¥0.00')
  })

  it('keeps explicit currency symbols across locales', () => {
    i18n.global.locale.value = 'zh'

    expect(formatUSD(1234.5)).toBe('$1,234.50')
    expect(formatCNY(1234.5)).toBe('¥1,234.50')
  })
})
