import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface AffiliateAdminEntry {
  user_id: number
  email: string
  username: string
  aff_code: string
  aff_rebate_rate_percent?: number | null
  aff_code_custom: boolean
  updated_at: string
}

export interface AffiliateUserSummary {
  id: number
  email: string
  username: string
}

export async function listUsers(
  page = 1,
  pageSize = 20,
  search = ''
): Promise<PaginatedResponse<AffiliateAdminEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateAdminEntry>>(
    '/admin/affiliate/users',
    {
      params: {
        page,
        page_size: pageSize,
        search: search || undefined
      }
    }
  )
  return data
}

export async function lookupUsers(q: string): Promise<AffiliateUserSummary[]> {
  const { data } = await apiClient.get<AffiliateUserSummary[]>('/admin/affiliate/users/lookup', {
    params: { q }
  })
  return data
}

export async function updateUserSettings(
  userID: number,
  payload: {
    aff_code?: string
    aff_rebate_rate_percent?: number
    clear_rebate_rate?: boolean
  }
): Promise<{ user_id: number }> {
  const { data } = await apiClient.put<{ user_id: number }>(
    `/admin/affiliate/users/${userID}`,
    payload
  )
  return data
}

export async function clearUserSettings(userID: number): Promise<{ user_id: number }> {
  const { data } = await apiClient.delete<{ user_id: number }>(
    `/admin/affiliate/users/${userID}`
  )
  return data
}

export async function batchSetRate(payload: {
  user_ids: number[]
  aff_rebate_rate_percent?: number
  clear?: boolean
}): Promise<{ affected: number }> {
  const { data } = await apiClient.post<{ affected: number }>(
    '/admin/affiliate/users/batch-rate',
    payload
  )
  return data
}

export default {
  listUsers,
  lookupUsers,
  updateUserSettings,
  clearUserSettings,
  batchSetRate
}
