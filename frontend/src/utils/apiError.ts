/**
 * Centralized API error message extraction
 *
 * The API client interceptor rejects with a plain object: { status, code, message, error }
 * This utility extracts the user-facing message from any error shape.
 */

interface ApiErrorLike {
  status?: number
  code?: number | string
  message?: string
  error?: string
  reason?: string
  metadata?: Record<string, unknown>
  response?: {
    data?: {
      detail?: string
      message?: string
      code?: number | string
    }
  }
}

/**
 * Extract the error code from an API error object.
 */
export function extractApiErrorCode(err: unknown): string | undefined {
  if (!err || typeof err !== 'object') return undefined
  const e = err as ApiErrorLike
  const code = e.reason ?? e.code ?? e.response?.data?.code
  return code != null ? String(code) : undefined
}

/**
 * Extract metadata (interpolation params) from an API error object.
 */
export function extractApiErrorMetadata(err: unknown): Record<string, unknown> | undefined {
  if (!err || typeof err !== 'object') return undefined
  const e = err as ApiErrorLike
  return e.metadata
}

type TranslateFn = (key: string, params?: Record<string, unknown>) => string
type TranslateWithExistsFn = TranslateFn & { te?: (key: string) => boolean }

function tryTranslate(t: TranslateFn, key: string, fallback: string): string {
  const translated = t(key)
  if (translated === key) return fallback
  const te = (t as TranslateWithExistsFn).te
  if (te && !te(key)) return fallback
  return translated
}

function localizeMetadata(metadata: Record<string, unknown>, t: TranslateFn): Record<string, unknown> {
  const localized: Record<string, unknown> = { ...metadata }
  if (typeof localized.key === 'string') {
    localized.key = tryTranslate(t, `admin.settings.payment.field_${localized.key}`, localized.key)
  }
  if (typeof localized.keys === 'string') {
    localized.keys = localized.keys
      .split('/')
      .map(key => tryTranslate(t, `admin.settings.payment.field_${key}`, key))
      .join(' / ')
  }
  return localized
}

export function extractI18nErrorMessage(
  err: unknown,
  t: TranslateFn,
  namespace: string,
  fallback: string,
): string {
  const code = extractApiErrorCode(err)
  if (code) {
    const key = `${namespace}.${code}`
    const metadata = localizeMetadata(extractApiErrorMetadata(err) ?? {}, t)
    const translated = t(key, metadata)
    if (translated !== key) return translated
    const te = (t as TranslateWithExistsFn).te
    if (te && te(key)) return translated
  }
  return extractApiErrorMessage(err, fallback)
}

/**
 * Extract a displayable error message from an API error.
 *
 * @param err - The caught error (unknown type)
 * @param fallback - Fallback message if none can be extracted (use t('common.error') or similar)
 * @param i18nMap - Optional map of error codes to i18n translated strings
 */
export function extractApiErrorMessage(
  err: unknown,
  fallback = 'Unknown error',
  i18nMap?: Record<string, string>,
): string {
  if (!err) return fallback

  // Try i18n mapping by error code first
  if (i18nMap) {
    const code = extractApiErrorCode(err)
    if (code && i18nMap[code]) return i18nMap[code]
  }

  // Plain object from API client interceptor (most common case)
  if (typeof err === 'object' && err !== null) {
    const e = err as ApiErrorLike
    // Interceptor shape: { message, error }
    if (e.message) return e.message
    if (e.error) return e.error
    // Legacy axios shape: { response.data.detail }
    if (e.response?.data?.detail) return e.response.data.detail
    if (e.response?.data?.message) return e.response.data.message
  }

  // Standard Error
  if (err instanceof Error) return err.message

  // Last resort
  const str = String(err)
  return str === '[object Object]' ? fallback : str
}
