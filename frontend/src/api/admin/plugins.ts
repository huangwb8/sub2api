import { apiClient } from '../client'
import type { Plugin, APIPromptPluginConfig } from '@/types'

export interface CreatePluginRequest {
  name: string
  type: 'api-prompt'
  description?: string
  enabled: boolean
  api_prompt?: APIPromptPluginConfig
}

export interface UpdatePluginRequest {
  description?: string
  enabled?: boolean
  api_prompt?: APIPromptPluginConfig
}

export interface PluginTestResult {
  ok: boolean
  message: string
  checked_at: string
}

export async function listPlugins(): Promise<Plugin[]> {
  const { data } = await apiClient.get<Plugin[]>('/admin/settings/plugins')
  return data
}

export async function createPlugin(payload: CreatePluginRequest): Promise<Plugin> {
  const { data } = await apiClient.post<Plugin>('/admin/settings/plugins', payload)
  return data
}

export async function updatePlugin(name: string, payload: UpdatePluginRequest): Promise<Plugin> {
  const { data } = await apiClient.put<Plugin>(`/admin/settings/plugins/${encodeURIComponent(name)}`, payload)
  return data
}

export async function setPluginEnabled(name: string, enabled: boolean): Promise<Plugin> {
  const { data } = await apiClient.put<Plugin>(`/admin/settings/plugins/${encodeURIComponent(name)}/enabled`, { enabled })
  return data
}

export async function testPlugin(name: string): Promise<PluginTestResult> {
  const { data } = await apiClient.post<PluginTestResult>(`/admin/settings/plugins/${encodeURIComponent(name)}/test`)
  return data
}

export const adminPluginsAPI = {
  list: listPlugins,
  create: createPlugin,
  update: updatePlugin,
  setEnabled: setPluginEnabled,
  test: testPlugin
}
