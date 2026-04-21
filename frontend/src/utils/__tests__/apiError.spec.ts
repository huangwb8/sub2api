import { describe, expect, it } from 'vitest'

import {
  extractApiErrorCode,
  extractApiErrorMetadata,
  extractI18nErrorMessage,
} from '@/utils/apiError'

describe('apiError helpers', () => {
  it('prefers reason over numeric code', () => {
    expect(extractApiErrorCode({
      code: 400,
      reason: 'WXPAY_CONFIG_MISSING_KEY',
    })).toBe('WXPAY_CONFIG_MISSING_KEY')
  })

  it('returns metadata for structured errors', () => {
    expect(extractApiErrorMetadata({
      metadata: { key: 'publicKeyId' },
    })).toEqual({ key: 'publicKeyId' })
  })

  it('localizes metadata field labels before interpolation', () => {
    const messages: Record<string, string> = {
      'payment.errors.WXPAY_CONFIG_MISSING_KEY': '缺少必填项：{key}',
      'admin.settings.payment.field_publicKeyId': '公钥 ID',
    }

    const t = ((key: string, params?: Record<string, unknown>) => {
      const template = messages[key]
      if (!template) return key
      return template.replace(/\{(\w+)\}/g, (_, name) => String(params?.[name] ?? ''))
    }) as ((key: string, params?: Record<string, unknown>) => string) & { te: (key: string) => boolean }
    t.te = (key: string) => key in messages

    expect(extractI18nErrorMessage({
      reason: 'WXPAY_CONFIG_MISSING_KEY',
      metadata: { key: 'publicKeyId' },
      message: 'raw fallback',
    }, t, 'payment.errors', 'fallback')).toBe('缺少必填项：公钥 ID')
  })

  it('falls back to plain message when no i18n key exists', () => {
    const t = ((key: string) => key) as ((key: string, params?: Record<string, unknown>) => string) & { te: (key: string) => boolean }
    t.te = () => false

    expect(extractI18nErrorMessage({
      reason: 'UNKNOWN_REASON',
      message: 'original message',
    }, t, 'payment.errors', 'fallback')).toBe('original message')
  })
})
