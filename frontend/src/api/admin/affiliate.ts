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

export interface AffiliateInviteRecord {
  inviter_id: number
  inviter_email: string
  inviter_username: string
  invitee_id: number
  invitee_email: string
  invitee_username: string
  aff_code: string
  total_rebate: number
  created_at: string
}

export interface AffiliateRebateRecord {
  order_id: number
  out_trade_no: string
  inviter_id: number
  inviter_email: string
  inviter_username: string
  invitee_id: number
  invitee_email: string
  invitee_username: string
  order_amount: number
  pay_amount: number
  rebate_amount: number
  payment_type: string
  order_status: string
  created_at: string
}

export interface AffiliateTransferRecord {
  ledger_id: number
  user_id: number
  user_email: string
  username: string
  amount: number
  balance_after?: number
  available_quota_after?: number
  frozen_quota_after?: number
  history_quota_after?: number
  snapshot_available: boolean
  created_at: string
}

export interface AffiliateRecordQuery {
  page?: number
  page_size?: number
  search?: string
  start_at?: string
  end_at?: string
  timezone?: string
}

export async function listUsers(
  page = 1,
  pageSize = 20,
  search = ''
): Promise<PaginatedResponse<AffiliateAdminEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateAdminEntry>>(
    '/admin/affiliates/users',
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
  const { data } = await apiClient.get<AffiliateUserSummary[]>('/admin/affiliates/users/lookup', {
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
    `/admin/affiliates/users/${userID}`,
    payload
  )
  return data
}

export async function clearUserSettings(userID: number): Promise<{ user_id: number }> {
  const { data } = await apiClient.delete<{ user_id: number }>(
    `/admin/affiliates/users/${userID}`
  )
  return data
}

export async function batchSetRate(payload: {
  user_ids: number[]
  aff_rebate_rate_percent?: number
  clear?: boolean
}): Promise<{ affected: number }> {
  const { data } = await apiClient.post<{ affected: number }>(
    '/admin/affiliates/users/batch-rate',
    payload
  )
  return data
}

export async function listInviteRecords(
  params: AffiliateRecordQuery = {}
): Promise<PaginatedResponse<AffiliateInviteRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateInviteRecord>>(
    '/admin/affiliates/invites',
    { params }
  )
  return data
}

export async function listRebateRecords(
  params: AffiliateRecordQuery = {}
): Promise<PaginatedResponse<AffiliateRebateRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateRebateRecord>>(
    '/admin/affiliates/rebates',
    { params }
  )
  return data
}

export async function listTransferRecords(
  params: AffiliateRecordQuery = {}
): Promise<PaginatedResponse<AffiliateTransferRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateTransferRecord>>(
    '/admin/affiliates/transfers',
    { params }
  )
  return data
}

export default {
  listUsers,
  lookupUsers,
  updateUserSettings,
  clearUserSettings,
  batchSetRate,
  listInviteRecords,
  listRebateRecords,
  listTransferRecords
}
