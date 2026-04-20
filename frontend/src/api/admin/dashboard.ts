/**
 * Admin Dashboard API endpoints
 * Provides system-wide statistics and metrics
 */

import { apiClient } from '../client'
import type {
  DashboardStats,
  DashboardOversellCalculatorRequest,
  DashboardOversellCalculatorResponse,
  DashboardRecommendationMetrics,
  DashboardRecommendationsResponse,
  DashboardRecommendationsSummary,
  DashboardCapacityPoolRecommendation,
  TrendDataPoint,
  ProfitabilityTrendPoint,
  ModelStat,
  GroupStat,
  ApiKeyUsageTrendPoint,
  UserUsageTrendPoint,
  UserSpendingRankingResponse,
  UserBreakdownItem,
  UsageRequestType
} from '@/types'

/**
 * Get dashboard statistics
 * @returns Dashboard statistics including users, keys, accounts, and token usage
 */
export async function getStats(): Promise<DashboardStats> {
  const { data } = await apiClient.get<DashboardStats>('/admin/dashboard/stats')
  return data
}

export async function getRecommendations(): Promise<DashboardRecommendationsResponse> {
  const { data } = await apiClient.get<DashboardRecommendationsResponse | LegacyDashboardRecommendationsResponse>('/admin/dashboard/recommendations')
  return normalizeDashboardRecommendations(data)
}

export async function getOversellCalculator(): Promise<DashboardOversellCalculatorResponse> {
  const { data } = await apiClient.get<DashboardOversellCalculatorResponse>('/admin/dashboard/oversell-calculator')
  return data
}

export async function calculateOversellCalculator(
  payload: DashboardOversellCalculatorRequest
): Promise<DashboardOversellCalculatorResponse> {
  const { data } = await apiClient.post<DashboardOversellCalculatorResponse>(
    '/admin/dashboard/oversell-calculator',
    payload
  )
  return data
}

interface LegacyDashboardRecommendationItem {
  group_id: number
  group_name: string
  platform: string
  plan_names: string[]
  recommended_account_type: string
  status: 'healthy' | 'watch' | 'action'
  confidence_score: number
  current_total_accounts: number
  current_schedulable_accounts: number
  recommended_total_accounts: number
  recommended_additional_accounts: number
  subscriber_headroom: number
  reason: string
  metrics: DashboardRecommendationMetrics
}

interface LegacyDashboardRecommendationsResponse {
  generated_at: string
  lookback_days: number
  summary: {
    group_count: number
    current_schedulable_accounts: number
    recommended_additional_accounts: number
    urgent_group_count: number
  }
  items: LegacyDashboardRecommendationItem[]
}

const normalizeDashboardRecommendations = (
  payload: DashboardRecommendationsResponse | LegacyDashboardRecommendationsResponse
): DashboardRecommendationsResponse => {
  if ('pools' in payload && Array.isArray(payload.pools)) {
    return payload
  }

  const legacy = payload as LegacyDashboardRecommendationsResponse
  const summary: DashboardRecommendationsSummary = {
    pool_count: legacy.items.length,
    group_count: legacy.summary.group_count,
    current_schedulable_accounts: legacy.summary.current_schedulable_accounts,
    recommended_additional_schedulable_accounts: legacy.summary.recommended_additional_accounts,
    recoverable_unschedulable_accounts: 0,
    urgent_pool_count: legacy.summary.urgent_group_count
  }

  const pools: DashboardCapacityPoolRecommendation[] = legacy.items.map((item) => {
    const recoverableUnschedulableAccounts = Math.max(
      item.current_total_accounts - item.current_schedulable_accounts,
      0
    )

    return {
      pool_key: `legacy-group-${item.group_id}`,
      platform: item.platform,
      group_names: [item.group_name],
      plan_names: item.plan_names,
      recommended_account_type: item.recommended_account_type,
      status: item.status,
      confidence_score: item.confidence_score,
      current_total_accounts: item.current_total_accounts,
      current_schedulable_accounts: item.current_schedulable_accounts,
      recommended_schedulable_accounts: Math.max(
        item.recommended_total_accounts,
        item.current_schedulable_accounts
      ),
      recommended_additional_schedulable_accounts: item.recommended_additional_accounts,
      recoverable_unschedulable_accounts: Math.min(
        recoverableUnschedulableAccounts,
        item.recommended_additional_accounts
      ),
      reason: item.reason,
      metrics: item.metrics
    }
  })

  return {
    generated_at: legacy.generated_at,
    lookback_days: legacy.lookback_days,
    summary,
    pools
  }
}

/**
 * Get real-time metrics
 * @returns Real-time system metrics
 */
export async function getRealtimeMetrics(): Promise<{
  active_requests: number
  requests_per_minute: number
  average_response_time: number
  error_rate: number
}> {
  const { data } = await apiClient.get<{
    active_requests: number
    requests_per_minute: number
    average_response_time: number
    error_rate: number
  }>('/admin/dashboard/realtime')
  return data
}

export interface TrendParams {
  start_date?: string
  end_date?: string
  granularity?: 'day' | 'hour'
  user_id?: number
  api_key_id?: number
  model?: string
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface TrendResponse {
  trend: TrendDataPoint[]
  start_date: string
  end_date: string
  granularity: string
}

export interface ProfitabilityTrendResponse {
  trend: ProfitabilityTrendPoint[]
  start_date: string
  end_date: string
  granularity: string
}

export interface ProfitabilityBoundsResponse {
  has_data: boolean
  earliest_date?: string
}

/**
 * Get usage trend data
 * @param params - Query parameters for filtering
 * @returns Usage trend data
 */
export async function getUsageTrend(params?: TrendParams): Promise<TrendResponse> {
  const { data } = await apiClient.get<TrendResponse>('/admin/dashboard/trend', { params })
  return data
}

export async function getProfitabilityTrend(
  params?: Pick<TrendParams, 'start_date' | 'end_date' | 'granularity'>
): Promise<ProfitabilityTrendResponse> {
  const { data } = await apiClient.get<ProfitabilityTrendResponse>('/admin/dashboard/profitability', {
    params
  })
  return data
}

export async function getProfitabilityBounds(): Promise<ProfitabilityBoundsResponse> {
  const { data } = await apiClient.get<ProfitabilityBoundsResponse>('/admin/dashboard/profitability/bounds')
  return data
}

export interface ModelStatsParams {
  start_date?: string
  end_date?: string
  user_id?: number
  api_key_id?: number
  model?: string
  model_source?: 'requested' | 'upstream' | 'mapping'
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface ModelStatsResponse {
  models: ModelStat[]
  start_date: string
  end_date: string
}

/**
 * Get model usage statistics
 * @param params - Query parameters for filtering
 * @returns Model usage statistics
 */
export async function getModelStats(params?: ModelStatsParams): Promise<ModelStatsResponse> {
  const { data } = await apiClient.get<ModelStatsResponse>('/admin/dashboard/models', { params })
  return data
}

export interface GroupStatsParams {
  start_date?: string
  end_date?: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface GroupStatsResponse {
  groups: GroupStat[]
  start_date: string
  end_date: string
}

export interface DashboardSnapshotV2Params extends TrendParams {
  include_stats?: boolean
  include_trend?: boolean
  include_model_stats?: boolean
  include_group_stats?: boolean
  include_users_trend?: boolean
  users_trend_limit?: number
}

export interface DashboardSnapshotV2Stats extends DashboardStats {
  uptime: number
}

export interface DashboardSnapshotV2Response {
  generated_at: string
  start_date: string
  end_date: string
  granularity: string
  stats?: DashboardSnapshotV2Stats
  trend?: TrendDataPoint[]
  models?: ModelStat[]
  groups?: GroupStat[]
  users_trend?: UserUsageTrendPoint[]
}

/**
 * Get group usage statistics
 * @param params - Query parameters for filtering
 * @returns Group usage statistics
 */
export async function getGroupStats(params?: GroupStatsParams): Promise<GroupStatsResponse> {
  const { data } = await apiClient.get<GroupStatsResponse>('/admin/dashboard/groups', { params })
  return data
}

export interface UserBreakdownParams {
  start_date?: string
  end_date?: string
  group_id?: number
  model?: string
  model_source?: 'requested' | 'upstream' | 'mapping'
  endpoint?: string
  endpoint_type?: 'inbound' | 'upstream' | 'path'
  limit?: number
  // Additional filter conditions
  user_id?: number
  api_key_id?: number
  account_id?: number
  request_type?: number
  stream?: boolean
  billing_type?: number | null
}

export interface UserBreakdownResponse {
  users: UserBreakdownItem[]
  start_date: string
  end_date: string
}

export async function getUserBreakdown(params: UserBreakdownParams): Promise<UserBreakdownResponse> {
  const { data } = await apiClient.get<UserBreakdownResponse>('/admin/dashboard/user-breakdown', {
    params
  })
  return data
}

/**
 * Get dashboard snapshot v2 (aggregated response for heavy admin pages).
 */
export async function getSnapshotV2(params?: DashboardSnapshotV2Params): Promise<DashboardSnapshotV2Response> {
  const { data } = await apiClient.get<DashboardSnapshotV2Response>('/admin/dashboard/snapshot-v2', {
    params
  })
  return data
}

export interface ApiKeyTrendParams extends TrendParams {
  limit?: number
}

export interface ApiKeyTrendResponse {
  trend: ApiKeyUsageTrendPoint[]
  start_date: string
  end_date: string
  granularity: string
}

/**
 * Get API key usage trend data
 * @param params - Query parameters for filtering
 * @returns API key usage trend data
 */
export async function getApiKeyUsageTrend(
  params?: ApiKeyTrendParams
): Promise<ApiKeyTrendResponse> {
  const { data } = await apiClient.get<ApiKeyTrendResponse>('/admin/dashboard/api-keys-trend', {
    params
  })
  return data
}

export interface UserTrendParams extends TrendParams {
  limit?: number
}

export interface UserTrendResponse {
  trend: UserUsageTrendPoint[]
  start_date: string
  end_date: string
  granularity: string
}

export interface UserSpendingRankingParams
  extends Pick<TrendParams, 'start_date' | 'end_date'> {
  limit?: number
}

/**
 * Get user usage trend data
 * @param params - Query parameters for filtering
 * @returns User usage trend data
 */
export async function getUserUsageTrend(params?: UserTrendParams): Promise<UserTrendResponse> {
  const { data } = await apiClient.get<UserTrendResponse>('/admin/dashboard/users-trend', {
    params
  })
  return data
}

/**
 * Get user spending ranking data
 * @param params - Query parameters for filtering
 * @returns User spending ranking data
 */
export async function getUserSpendingRanking(
  params?: UserSpendingRankingParams
): Promise<UserSpendingRankingResponse> {
  const { data } = await apiClient.get<UserSpendingRankingResponse>('/admin/dashboard/users-ranking', {
    params
  })
  return data
}

export interface BatchUserUsageStats {
  user_id: number
  today_actual_cost: number
  total_actual_cost: number
}

export interface BatchUsersUsageResponse {
  stats: Record<string, BatchUserUsageStats>
}

/**
 * Get batch usage stats for multiple users
 * @param userIds - Array of user IDs
 * @returns Usage stats map keyed by user ID
 */
export async function getBatchUsersUsage(userIds: number[]): Promise<BatchUsersUsageResponse> {
  const { data } = await apiClient.post<BatchUsersUsageResponse>('/admin/dashboard/users-usage', {
    user_ids: userIds
  })
  return data
}

export interface BatchApiKeyUsageStats {
  api_key_id: number
  today_actual_cost: number
  total_actual_cost: number
}

export interface BatchApiKeysUsageResponse {
  stats: Record<string, BatchApiKeyUsageStats>
}

/**
 * Get batch usage stats for multiple API keys
 * @param apiKeyIds - Array of API key IDs
 * @returns Usage stats map keyed by API key ID
 */
export async function getBatchApiKeysUsage(
  apiKeyIds: number[]
): Promise<BatchApiKeysUsageResponse> {
  const { data } = await apiClient.post<BatchApiKeysUsageResponse>(
    '/admin/dashboard/api-keys-usage',
    {
      api_key_ids: apiKeyIds
    }
  )
  return data
}

export const dashboardAPI = {
  getStats,
  getRecommendations,
  getOversellCalculator,
  calculateOversellCalculator,
  getRealtimeMetrics,
  getUsageTrend,
  getProfitabilityBounds,
  getProfitabilityTrend,
  getModelStats,
  getGroupStats,
  getSnapshotV2,
  getApiKeyUsageTrend,
  getUserUsageTrend,
  getUserSpendingRanking,
  getBatchUsersUsage,
  getBatchApiKeysUsage
}

export default dashboardAPI
