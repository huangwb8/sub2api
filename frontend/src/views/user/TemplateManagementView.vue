<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
            {{ t('templateManagement.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ access?.can_create_custom
              ? t('templateManagement.customEnabled', { plan: access.custom_template_plan_name })
              : t('templateManagement.defaultOnly', { plan: access?.custom_template_plan_name || 'G-Ultra' }) }}
          </p>
        </div>
        <button class="btn btn-secondary" :disabled="loading" @click="loadData">
          <Icon name="refresh" size="sm" :class="loading ? 'mr-2 animate-spin' : 'mr-2'" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
        <SearchInput
          v-model="search"
          :placeholder="t('templateManagement.searchPlaceholder')"
          class="w-full sm:w-80"
        />
        <span class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('templateManagement.keyCount', { count: filteredKeys.length }) }}
        </span>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-16 text-gray-500">
        <Icon name="refresh" size="md" class="mr-2 animate-spin" />
        {{ t('common.loading') }}
      </div>

      <EmptyState
        v-else-if="apiKeys.length === 0"
        :title="t('templateManagement.emptyTitle')"
        :description="t('templateManagement.emptyDescription')"
      />

      <div v-else class="space-y-4">
        <article
          v-for="key in filteredKeys"
          :key="key.id"
          class="rounded-lg border border-slate-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900/70"
        >
          <div class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(360px,0.9fr)]">
            <div class="min-w-0 space-y-3">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">
                  {{ key.name }}
                </h2>
                <span class="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-600 dark:bg-dark-700 dark:text-slate-300">
                  {{ key.group?.name || t('keys.noGroup') }}
                </span>
                <span
                  :class="key.status === 'active'
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
                    : 'bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-slate-300'"
                  class="rounded-full px-2.5 py-1 text-xs font-medium"
                >
                  {{ t('keys.status.' + key.status) }}
                </span>
              </div>
              <code class="block truncate rounded bg-slate-50 px-3 py-2 font-mono text-xs text-slate-500 dark:bg-dark-800 dark:text-slate-300">
                {{ maskKey(key.key) }}
              </code>
              <p class="text-sm text-gray-500 dark:text-gray-400">
                {{ describeBinding(key) }}
              </p>
            </div>

            <div class="space-y-4">
              <Select
                :model-value="drafts[key.id]?.bindingValue || ''"
                :options="templateOptions"
                searchable
                :search-placeholder="t('templateManagement.templateSearchPlaceholder')"
                @update:model-value="(value) => updateDraftBinding(key.id, value as string)"
              />

              <div v-if="drafts[key.id]?.custom" class="space-y-3 rounded-lg border border-slate-200 bg-slate-50 p-4 dark:border-dark-700 dark:bg-dark-800/60">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span class="text-sm font-medium text-gray-900 dark:text-white">
                    {{ t('templateManagement.customTemplate') }}
                  </span>
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    :disabled="!access?.can_create_custom"
                    @click="copySelectedTemplate(key.id)"
                  >
                    <Icon name="copy" size="sm" class="mr-1.5" />
                    {{ t('templateManagement.copyTemplate') }}
                  </button>
                </div>
                <input
                  v-model.trim="drafts[key.id].customName"
                  class="input"
                  :disabled="!access?.can_create_custom"
                  :placeholder="t('templateManagement.customNamePlaceholder')"
                />
                <textarea
                  v-model="drafts[key.id].customPrompt"
                  rows="7"
                  class="input min-h-[180px] font-mono text-sm leading-6"
                  :disabled="!access?.can_create_custom"
                  :placeholder="t('templateManagement.customPromptPlaceholder')"
                />
              </div>

              <div class="flex justify-end">
                <button
                  class="btn btn-primary"
                  :disabled="savingId === key.id || !isDraftChanged(key)"
                  @click="saveBinding(key)"
                >
                  <Icon v-if="savingId === key.id" name="refresh" size="sm" class="mr-2 animate-spin" />
                  {{ savingId === key.id ? t('common.saving') : t('common.save') }}
                </button>
              </div>
            </div>
          </div>
        </article>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { keysAPI } from '@/api'
import { pluginsAPI } from '@/api/plugins'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import { useAppStore } from '@/stores/app'
import type { APIKeyPluginSettings, APIPromptTemplateAccess, APIPromptTemplateOption, ApiKey, SelectOption } from '@/types'
import { matchesSearchTerms } from '@/utils/searchMatcher'

interface DraftState {
  bindingValue: string
  custom: boolean
  customName: string
  customPrompt: string
}

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const savingId = ref<number | null>(null)
const search = ref('')
const apiKeys = ref<ApiKey[]>([])
const templates = ref<APIPromptTemplateOption[]>([])
const access = ref<APIPromptTemplateAccess | null>(null)
const drafts = ref<Record<number, DraftState>>({})

const templateOptions = computed<SelectOption[]>(() => {
  const options: SelectOption[] = [
    { value: '', label: t('keys.promptMode.general'), description: t('keys.promptMode.generalDescription') },
    ...templates.value.map((template) => ({
      value: encodeTemplateValue(template),
      label: `${template.name} · ${template.plugin_name}`,
      description: template.description
    }))
  ]
  if (access.value?.can_create_custom) {
    options.push({
      value: '__custom__',
      label: t('templateManagement.newCustomTemplate'),
      description: t('templateManagement.newCustomTemplateHint')
    })
  }
  return options
})

const filteredKeys = computed(() => {
  const q = search.value.trim()
  if (!q) return apiKeys.value
  return apiKeys.value.filter((key) =>
    matchesSearchTerms(q, [
      key.name,
      key.key,
      key.group?.name || '',
      describeBinding(key)
    ])
  )
})

function encodeTemplateValue(template: APIPromptTemplateOption): string {
  return JSON.stringify({
    plugin_name: template.plugin_name,
    template_id: template.template_id
  })
}

function encodeBindingValue(settings?: APIKeyPluginSettings | null): string {
  const binding = settings?.api_prompt
  if (!binding) return ''
  if (binding.custom) return '__custom__'
  return JSON.stringify({
    plugin_name: binding.plugin_name,
    template_id: binding.template_id
  })
}

function parseTemplateValue(value: string): APIKeyPluginSettings | undefined {
  if (!value) return {}
  const parsed = JSON.parse(value) as { plugin_name: string; template_id: string }
  return {
    api_prompt: {
      plugin_name: parsed.plugin_name,
      template_id: parsed.template_id
    }
  }
}

function hydrateDrafts() {
  drafts.value = Object.fromEntries(apiKeys.value.map((key) => {
    const binding = key.plugin_settings?.api_prompt
    return [key.id, {
      bindingValue: encodeBindingValue(key.plugin_settings),
      custom: !!binding?.custom,
      customName: binding?.custom ? binding.name || '' : '',
      customPrompt: binding?.custom ? binding.prompt || '' : ''
    }]
  }))
}

async function loadData() {
  loading.value = true
  try {
    const [keys, promptTemplates, promptAccess] = await Promise.all([
      keysAPI.list(1, 100),
      pluginsAPI.listAPIPromptTemplates(),
      keysAPI.getPromptTemplateAccess()
    ])
    apiKeys.value = keys.items
    templates.value = promptTemplates
    access.value = promptAccess
    hydrateDrafts()
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || t('templateManagement.loadFailed'))
  } finally {
    loading.value = false
  }
}

function updateDraftBinding(keyId: number, value: string) {
  const draft = drafts.value[keyId]
  if (!draft) return
  draft.bindingValue = value
  draft.custom = value === '__custom__'
}

function selectedTemplateForDraft(keyId: number) {
  const value = drafts.value[keyId]?.bindingValue
  if (!value || value === '__custom__') return null
  try {
    const parsed = JSON.parse(value) as { plugin_name: string; template_id: string }
    return templates.value.find((template) =>
      template.plugin_name === parsed.plugin_name && template.template_id === parsed.template_id
    ) || null
  } catch {
    return null
  }
}

function copySelectedTemplate(keyId: number) {
  const draft = drafts.value[keyId]
  if (!draft) return
  const selected = selectedTemplateForDraft(keyId) || templates.value[0]
  if (!selected) return
  draft.custom = true
  draft.bindingValue = '__custom__'
  draft.customName = selected.name
  draft.customPrompt = selected.prompt
}

function buildSettings(keyId: number): APIKeyPluginSettings | undefined {
  const draft = drafts.value[keyId]
  if (!draft) return undefined
  if (draft.custom) {
    return {
      api_prompt: {
        plugin_name: 'api-prompt',
        template_id: `custom-${keyId}`,
        name: draft.customName.trim() || t('templateManagement.customTemplate'),
        prompt: draft.customPrompt.trim(),
        custom: true
      }
    }
  }
  return parseTemplateValue(draft.bindingValue)
}

async function saveBinding(key: ApiKey) {
  const settings = buildSettings(key.id)
  if (drafts.value[key.id]?.custom && !drafts.value[key.id].customPrompt.trim()) {
    appStore.showError(t('templateManagement.customPromptRequired'))
    return
  }
  savingId.value = key.id
  try {
    const updated = await keysAPI.update(key.id, { plugin_settings: settings })
    apiKeys.value = apiKeys.value.map((item) => item.id === key.id ? updated : item)
    hydrateDrafts()
    appStore.showSuccess(t('templateManagement.saveSuccess'))
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || t('templateManagement.saveFailed'))
  } finally {
    savingId.value = null
  }
}

function isDraftChanged(key: ApiKey): boolean {
  const draft = drafts.value[key.id]
  if (!draft) return false
  const currentBinding = key.plugin_settings?.api_prompt
  if (draft.custom) {
    return !currentBinding?.custom ||
      draft.customName !== (currentBinding.name || '') ||
      draft.customPrompt !== (currentBinding.prompt || '')
  }
  return draft.bindingValue !== encodeBindingValue(key.plugin_settings)
}

function describeBinding(key: ApiKey): string {
  const binding = key.plugin_settings?.api_prompt
  if (!binding) return t('keys.promptMode.general')
  if (binding.custom) return binding.name || t('templateManagement.customTemplate')
  const matched = templates.value.find((template) =>
    template.plugin_name === binding.plugin_name && template.template_id === binding.template_id
  )
  return matched?.name || `${binding.plugin_name} / ${binding.template_id}`
}

function maskKey(key: string): string {
  if (key.length <= 12) return key
  return `${key.slice(0, 8)}...${key.slice(-4)}`
}

onMounted(loadData)
</script>
