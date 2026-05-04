import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  buildCacheBustedURL,
  isChunkLoadError,
  recoverFromChunkLoadError
} from '../assetRefresh'

describe('assetRefresh', () => {
  const originalLocation = window.location

  beforeEach(() => {
    sessionStorage.clear()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: {
        href: 'https://example.test/admin/settings?tab=plugin',
        replace: vi.fn()
      }
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    sessionStorage.clear()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: originalLocation
    })
  })

  it('detects common dynamic asset load failures', () => {
    expect(isChunkLoadError(new Error('Failed to fetch dynamically imported module'))).toBe(true)
    expect(isChunkLoadError(new Error('Loading CSS chunk failed'))).toBe(true)

    const namedError = new Error('network')
    namedError.name = 'ChunkLoadError'
    expect(isChunkLoadError(namedError)).toBe(true)

    expect(isChunkLoadError(new Error('ordinary error'))).toBe(false)
  })

  it('builds a cache-busted URL while preserving the current route query', () => {
    expect(buildCacheBustedURL(123, 'https://example.test/admin/settings?tab=plugin')).toBe(
      'https://example.test/admin/settings?tab=plugin&_sub2api_reload=123'
    )
  })

  it('redirects once with cache busting for chunk load errors', () => {
    const error = new Error('Loading chunk failed')
    error.name = 'ChunkLoadError'

    expect(recoverFromChunkLoadError(error, 1000)).toBe(true)
    expect(window.location.replace).toHaveBeenCalledWith(
      'https://example.test/admin/settings?tab=plugin&_sub2api_reload=1000'
    )
  })

  it('does not loop when the cache-busted reload just happened', () => {
    sessionStorage.setItem('sub2api_asset_reload_attempted_at', '1000')

    const error = new Error('Loading chunk failed')
    error.name = 'ChunkLoadError'

    expect(recoverFromChunkLoadError(error, 5000)).toBe(true)
    expect(window.location.replace).not.toHaveBeenCalled()
  })
})
