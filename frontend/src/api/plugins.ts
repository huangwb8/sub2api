import { apiClient } from './client'
import type { APIPromptTemplateOption } from '@/types'

export async function listAPIPromptTemplates(): Promise<APIPromptTemplateOption[]> {
  const { data } = await apiClient.get<APIPromptTemplateOption[]>('/plugins/api-prompt/templates')
  return data
}

export const pluginsAPI = {
  listAPIPromptTemplates
}
