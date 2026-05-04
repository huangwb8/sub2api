const reloadAttemptKey = 'sub2api_asset_reload_attempted_at'
const reloadWindowMs = 10000
const cacheBustParam = '_sub2api_reload'

export function isChunkLoadError(error: unknown): boolean {
  const candidate = error as { message?: string; name?: string } | null
  const message = candidate?.message || ''
  return (
    message.includes('Failed to fetch dynamically imported module') ||
    message.includes('Loading chunk') ||
    message.includes('Loading CSS chunk') ||
    candidate?.name === 'ChunkLoadError'
  )
}

export function buildCacheBustedURL(now = Date.now(), currentHref = window.location.href): string {
  const url = new URL(currentHref)
  url.searchParams.set(cacheBustParam, String(now))
  return url.toString()
}

export function recoverFromChunkLoadError(error: unknown, now = Date.now()): boolean {
  if (!isChunkLoadError(error)) {
    return false
  }

  const lastReload = Number(sessionStorage.getItem(reloadAttemptKey) || '0')
  if (lastReload > 0 && now - lastReload <= reloadWindowMs) {
    console.error('Chunk load error persists after cache-busted reload.', error)
    return true
  }

  sessionStorage.setItem(reloadAttemptKey, String(now))
  window.location.replace(buildCacheBustedURL(now))
  return true
}
