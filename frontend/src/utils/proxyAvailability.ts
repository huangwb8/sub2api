import type { Proxy } from '@/types'

export type ProxyAvailabilityState = 'available' | 'failed'

export const isProxyAvailable = (proxy: Proxy | null | undefined): boolean => {
  if (!proxy) return false
  if (proxy.status !== 'active') return false
  if (proxy.latency_status === 'failed') return false
  if (proxy.quality_status === 'failed' || proxy.quality_status === 'challenge') return false
  return true
}

export const getProxyAvailabilityState = (
  proxy: Proxy | null | undefined
): ProxyAvailabilityState | null => {
  if (!proxy) return null
  return isProxyAvailable(proxy) ? 'available' : 'failed'
}

export const formatProxyLocation = (proxy: Proxy): string => {
  const parts = [proxy.country, proxy.city].filter(Boolean) as string[]
  return parts.join(' · ')
}

const transferTargetQualityRank = (status?: Proxy['quality_status']) => {
  switch (status) {
    case 'healthy':
      return 0
    case 'warn':
      return 1
    default:
      return 2
  }
}

const transferTargetLatencyRank = (proxy: Proxy) => {
  if (proxy.latency_status === 'success' && typeof proxy.latency_ms === 'number') {
    return proxy.latency_ms
  }
  return Number.MAX_SAFE_INTEGER
}

export const compareProxyTransferTargets = (left: Proxy, right: Proxy) => {
  const qualityRankDiff = transferTargetQualityRank(left.quality_status) - transferTargetQualityRank(right.quality_status)
  if (qualityRankDiff !== 0) return qualityRankDiff

  const latencyDiff = transferTargetLatencyRank(left) - transferTargetLatencyRank(right)
  if (latencyDiff !== 0) return latencyDiff

  const qualityScoreDiff = (right.quality_score ?? -1) - (left.quality_score ?? -1)
  if (qualityScoreDiff !== 0) return qualityScoreDiff

  const accountCountDiff = (left.account_count ?? 0) - (right.account_count ?? 0)
  if (accountCountDiff !== 0) return accountCountDiff

  return left.name.localeCompare(right.name, 'zh-CN')
}

export const buildProxyTransferTargetLabel = (proxy: Proxy) => {
  const parts = [proxy.name, `${proxy.host}:${proxy.port}`]
  const location = formatProxyLocation(proxy)
  if (location) {
    parts.push(location)
  }
  if (typeof proxy.latency_ms === 'number' && proxy.latency_status === 'success') {
    parts.push(`${proxy.latency_ms}ms`)
  }
  if (proxy.quality_grade) {
    parts.push(`Q${proxy.quality_grade}`)
  }
  return parts.join(' · ')
}
