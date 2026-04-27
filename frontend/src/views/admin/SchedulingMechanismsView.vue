<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-2xl font-semibold text-gray-900 dark:text-white">
              {{ t('admin.schedulingMechanisms.title') }}
            </h2>
            <p class="mt-2 text-sm text-gray-600 dark:text-gray-300">
              {{ t('admin.schedulingMechanisms.description') }}
            </p>
            <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.schedulingMechanisms.importHint') }}
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button class="btn btn-secondary" @click="triggerImport">
              {{ t('admin.schedulingMechanisms.importButton') }}
            </button>
            <button class="btn btn-primary" @click="openCreateDialog">
              <Icon name="plus" size="sm" class="mr-2" />
              {{ t('admin.schedulingMechanisms.createButton') }}
            </button>
          </div>
        </div>

        <input
          ref="importInputRef"
          type="file"
          accept="application/json,.json"
          class="hidden"
          @change="handleImportFile"
        />

        <div class="mt-6 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div class="relative w-full lg:max-w-sm">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.schedulingMechanisms.searchPlaceholder')"
              class="input pl-10"
            />
          </div>
          <label class="inline-flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
            <input v-model="showHidden" type="checkbox" class="rounded border-gray-300" />
            {{ t('admin.schedulingMechanisms.showHidden') }}
          </label>
        </div>

        <div class="mt-6">
          <DataTable :columns="columns" :data="filteredMechanisms" :loading="loading">
            <template #cell-name="{ row }">
              <div class="space-y-1">
                <div class="font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
                <div v-if="row.description" class="text-xs text-gray-500 dark:text-gray-400">
                  {{ row.description }}
                </div>
              </div>
            </template>

            <template #cell-scope="{ row }">
              <div class="flex flex-wrap gap-2">
                <span class="badge badge-gray">{{ platformLabel(row.platform) }}</span>
                <span class="badge badge-gray">{{ accountTypeLabel(row.account_type) }}</span>
              </div>
            </template>

            <template #cell-status="{ row }">
              <div class="flex flex-wrap gap-2">
                <span :class="['badge', row.enabled ? 'badge-success' : 'badge-gray']">
                  {{ row.enabled ? t('admin.schedulingMechanisms.enabled') : t('admin.schedulingMechanisms.disabled') }}
                </span>
                <span :class="['badge', row.hidden ? 'badge-warning' : 'badge-primary']">
                  {{ row.hidden ? t('admin.schedulingMechanisms.hidden') : t('admin.schedulingMechanisms.visible') }}
                </span>
              </div>
            </template>

            <template #cell-rules="{ row }">
              <div class="space-y-1 text-sm text-gray-600 dark:text-gray-300">
                <div>
                  {{ t('admin.schedulingMechanisms.ruleCount', { count: row.temp_unschedulable_rules.length }) }}
                </div>
                <div v-if="row.temp_unschedulable_rules[0]" class="text-xs text-gray-500 dark:text-gray-400">
                  HTTP {{ row.temp_unschedulable_rules[0].error_code }} · {{ row.temp_unschedulable_rules[0].duration_minutes }}m
                </div>
              </div>
            </template>

            <template #cell-updated_at="{ row }">
              <span class="text-sm text-gray-600 dark:text-gray-300">{{ formatUpdatedAt(row.updated_at_unix) }}</span>
            </template>

            <template #cell-actions="{ row }">
              <div class="flex flex-wrap gap-2">
                <button class="btn btn-secondary btn-sm" @click="openEditDialog(row)">
                  {{ t('common.edit') }}
                </button>
                <button class="btn btn-secondary btn-sm" @click="toggleHidden(row)">
                  {{ row.hidden ? t('admin.schedulingMechanisms.showAction') : t('admin.schedulingMechanisms.hideAction') }}
                </button>
                <button class="btn btn-danger btn-sm" @click="removeMechanism(row.id)">
                  {{ t('common.delete') }}
                </button>
              </div>
            </template>

            <template #empty>
              <EmptyState
                :title="t('admin.schedulingMechanisms.emptyTitle')"
                :description="t('admin.schedulingMechanisms.emptyDescription')"
                :action-text="t('admin.schedulingMechanisms.createButton')"
                @action="openCreateDialog"
              />
            </template>
          </DataTable>
        </div>
      </section>
    </div>

    <BaseDialog
      :show="showDialog"
      :title="dialogMode === 'create' ? t('admin.schedulingMechanisms.createDialogTitle') : t('admin.schedulingMechanisms.editDialogTitle')"
      width="wide"
      @close="closeDialog"
    >
      <form id="mechanism-form" class="space-y-5" @submit.prevent="submitMechanism">
        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.schedulingMechanisms.form.name') }}</label>
            <input v-model="form.name" type="text" class="input" required />
          </div>
          <div>
            <label class="input-label">{{ t('admin.schedulingMechanisms.form.platform') }}</label>
            <Select v-model="form.platform" :options="platformOptions" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.schedulingMechanisms.form.accountType') }}</label>
            <Select v-model="form.account_type" :options="accountTypeOptions" />
          </div>
          <div class="grid grid-cols-2 gap-3">
            <label class="space-y-2 rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
              <span class="text-sm font-medium text-gray-800 dark:text-gray-200">
                {{ t('admin.schedulingMechanisms.form.enabled') }}
              </span>
              <input v-model="form.enabled" type="checkbox" class="toggle" />
            </label>
            <label class="space-y-2 rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
              <span class="text-sm font-medium text-gray-800 dark:text-gray-200">
                {{ t('admin.schedulingMechanisms.form.hidden') }}
              </span>
              <input v-model="form.hidden" type="checkbox" class="toggle" />
            </label>
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.schedulingMechanisms.form.description') }}</label>
          <textarea v-model="form.description" rows="3" class="input"></textarea>
        </div>

        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <div>
              <label class="input-label">{{ t('admin.schedulingMechanisms.form.rules') }}</label>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.schedulingMechanisms.form.rulesHint') }}
              </p>
            </div>
            <button type="button" class="btn btn-secondary" @click="addRule">
              {{ t('admin.schedulingMechanisms.addRule') }}
            </button>
          </div>

          <div
            v-for="(rule, index) in form.rules"
            :key="`rule-${index}`"
            class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700"
          >
            <div class="grid gap-4 md:grid-cols-[120px_140px_minmax(0,1fr)_auto]">
              <div>
                <label class="input-label">{{ t('admin.schedulingMechanisms.form.errorCode') }}</label>
                <input v-model.number="rule.error_code" type="number" min="100" max="599" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.schedulingMechanisms.form.durationMinutes') }}</label>
                <input v-model.number="rule.duration_minutes" type="number" min="1" max="240" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.schedulingMechanisms.form.keywords') }}</label>
                <input v-model="rule.keywordsText" type="text" class="input" :placeholder="t('admin.schedulingMechanisms.form.keywordsPlaceholder')" />
              </div>
              <div class="flex items-end">
                <button type="button" class="btn btn-danger" @click="removeRule(index)">
                  {{ t('common.delete') }}
                </button>
              </div>
            </div>
            <div class="mt-4">
              <label class="input-label">{{ t('admin.schedulingMechanisms.form.ruleDescription') }}</label>
              <input v-model="rule.description" type="text" class="input" />
            </div>
          </div>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="closeDialog">
            {{ t('common.cancel') }}
          </button>
          <button class="btn btn-primary" form="mechanism-form" type="submit" :disabled="saving">
            {{ t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  SchedulingMechanism,
  SchedulingMechanismSettings
} from '@/api/admin/settings'
import type { TempUnschedulableRule } from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import { useAppStore } from '@/stores/app'

type EditableRule = TempUnschedulableRule & { keywordsText: string }

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const searchQuery = ref('')
const showHidden = ref(false)
const showDialog = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingMechanismID = ref<string>('')
const importInputRef = ref<HTMLInputElement | null>(null)

const settings = reactive<SchedulingMechanismSettings>({
  mechanisms: [],
  proxy_failover: {
    enabled: true,
    auto_test_enabled: true,
    probe_interval_minutes: 5,
    failure_threshold: 3,
    failure_window_minutes: 10,
    cooldown_minutes: 15,
    max_accounts_per_proxy: 6,
    max_migrations_per_cycle: 12,
    prefer_same_country: true,
    only_openai_oauth: true,
    temp_unsched_minutes: 10
  }
})

const form = reactive({
  id: '',
  name: '',
  platform: 'all',
  account_type: 'all',
  enabled: true,
  hidden: false,
  description: '',
  rules: [] as EditableRule[]
})

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.schedulingMechanisms.columns.name') },
  { key: 'scope', label: t('admin.schedulingMechanisms.columns.scope') },
  { key: 'status', label: t('admin.schedulingMechanisms.columns.status') },
  { key: 'rules', label: t('admin.schedulingMechanisms.columns.rules') },
  { key: 'updated_at', label: t('admin.schedulingMechanisms.columns.updatedAt') },
  { key: 'actions', label: t('admin.schedulingMechanisms.columns.actions') }
])

const platformOptions = computed(() => [
  { value: 'all', label: t('admin.schedulingMechanisms.scope.platformAll') },
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' }
])

const accountTypeOptions = computed(() => [
  { value: 'all', label: t('admin.schedulingMechanisms.scope.accountTypeAll') },
  { value: 'oauth', label: 'OAuth' },
  { value: 'apikey', label: 'API Key' },
  { value: 'setuptoken', label: 'Setup Token' },
  { value: 'upstream', label: 'Upstream' },
  { value: 'bedrock', label: 'Bedrock' }
])

const filteredMechanisms = computed(() => {
  const keyword = searchQuery.value.trim().toLowerCase()
  return settings.mechanisms
    .filter((item) => (showHidden.value ? true : !item.hidden))
    .filter((item) => {
      if (!keyword) return true
      return [item.name, item.description, item.platform, item.account_type]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(keyword))
    })
    .slice()
    .sort((left, right) => {
      if (left.hidden !== right.hidden) return left.hidden ? 1 : -1
      return (right.updated_at_unix || 0) - (left.updated_at_unix || 0)
    })
})

const cloneSettings = (value: SchedulingMechanismSettings): SchedulingMechanismSettings =>
  JSON.parse(JSON.stringify(value))

const normalizeImportedMechanism = (item: Partial<SchedulingMechanism>): SchedulingMechanism => ({
  id: String(item.id || generateMechanismID()),
  name: String(item.name || '').trim(),
  platform: String(item.platform || 'all'),
  account_type: String(item.account_type || 'all'),
  enabled: item.enabled !== false,
  hidden: item.hidden === true,
  description: String(item.description || '').trim(),
  temp_unschedulable_enabled: item.temp_unschedulable_enabled !== false,
  temp_unschedulable_rules: Array.isArray(item.temp_unschedulable_rules)
    ? item.temp_unschedulable_rules
    : [],
  updated_at_unix: Number(item.updated_at_unix || Math.floor(Date.now() / 1000))
})

const platformLabel = (value: string) =>
  platformOptions.value.find((item) => item.value === value)?.label || value

const accountTypeLabel = (value: string) =>
  accountTypeOptions.value.find((item) => item.value === value)?.label || value

const formatUpdatedAt = (value?: number) =>
  value ? new Date(value * 1000).toLocaleString() : '-'

const buildMechanismPayload = (): SchedulingMechanism => ({
  id: form.id || generateMechanismID(),
  name: form.name.trim(),
  platform: form.platform,
  account_type: form.account_type,
  enabled: form.enabled,
  hidden: form.hidden,
  description: form.description.trim(),
  temp_unschedulable_enabled: true,
  temp_unschedulable_rules: form.rules
    .map((rule) => ({
      id: rule.id,
      error_code: Number(rule.error_code),
      keywords: rule.keywordsText
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean),
      duration_minutes: Number(rule.duration_minutes),
      description: rule.description.trim()
    }))
    .filter((rule) => rule.error_code >= 100 && rule.error_code <= 599 && rule.duration_minutes > 0 && rule.keywords.length > 0),
  updated_at_unix: Math.floor(Date.now() / 1000)
})

async function loadSettings() {
  loading.value = true
  try {
    const payload = await adminAPI.settings.getSchedulingMechanismSettings()
    settings.mechanisms = payload.mechanisms || []
    settings.proxy_failover = payload.proxy_failover || settings.proxy_failover
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('common.error'))
  } finally {
    loading.value = false
  }
}

async function saveAllSettings(successMessageKey: string) {
  saving.value = true
  try {
    const payload = cloneSettings(settings)
    const updated = await adminAPI.settings.updateSchedulingMechanismSettings(payload)
    settings.mechanisms = updated.mechanisms || []
    settings.proxy_failover = updated.proxy_failover || settings.proxy_failover
    appStore.showSuccess(t(successMessageKey))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.schedulingMechanisms.saveFailed'))
  } finally {
    saving.value = false
  }
}

function resetForm() {
  form.id = ''
  form.name = ''
  form.platform = 'all'
  form.account_type = 'all'
  form.enabled = true
  form.hidden = false
  form.description = ''
  form.rules = [
    {
      error_code: 502,
      keywords: ['bad gateway'],
      keywordsText: 'bad gateway, upstream',
      duration_minutes: 3,
      description: ''
    }
  ]
}

function openCreateDialog() {
  dialogMode.value = 'create'
  editingMechanismID.value = ''
  resetForm()
  showDialog.value = true
}

function openEditDialog(mechanism: SchedulingMechanism) {
  dialogMode.value = 'edit'
  editingMechanismID.value = mechanism.id
  form.id = mechanism.id
  form.name = mechanism.name
  form.platform = mechanism.platform
  form.account_type = mechanism.account_type
  form.enabled = mechanism.enabled
  form.hidden = mechanism.hidden
  form.description = mechanism.description || ''
  form.rules = (mechanism.temp_unschedulable_rules || []).map((rule) => ({
    ...rule,
    keywordsText: rule.keywords.join(', ')
  }))
  if (form.rules.length === 0) {
    addRule()
  }
  showDialog.value = true
}

function closeDialog() {
  showDialog.value = false
}

function addRule() {
  form.rules.push({
    error_code: 502,
    keywords: [],
    keywordsText: '',
    duration_minutes: 3,
    description: ''
  })
}

function removeRule(index: number) {
  form.rules.splice(index, 1)
}

async function submitMechanism() {
  const mechanism = buildMechanismPayload()
  if (!mechanism.name || mechanism.temp_unschedulable_rules.length === 0) {
    appStore.showError(t('admin.schedulingMechanisms.form.validationFailed'))
    return
  }

  const nextSettings = cloneSettings(settings)
  const index = nextSettings.mechanisms.findIndex((item) => item.id === editingMechanismID.value)
  if (index >= 0) {
    nextSettings.mechanisms[index] = mechanism
  } else {
    nextSettings.mechanisms.unshift(mechanism)
  }

  saving.value = true
  try {
    const updated = await adminAPI.settings.updateSchedulingMechanismSettings(nextSettings)
    settings.mechanisms = updated.mechanisms || []
    settings.proxy_failover = updated.proxy_failover || settings.proxy_failover
    appStore.showSuccess(t('admin.schedulingMechanisms.mechanismSaved'))
    closeDialog()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.schedulingMechanisms.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function toggleHidden(mechanism: SchedulingMechanism) {
  const nextSettings = cloneSettings(settings)
  const index = nextSettings.mechanisms.findIndex((item) => item.id === mechanism.id)
  if (index < 0) return
  nextSettings.mechanisms[index].hidden = !nextSettings.mechanisms[index].hidden
  nextSettings.mechanisms[index].updated_at_unix = Math.floor(Date.now() / 1000)
  settings.mechanisms = nextSettings.mechanisms
  await saveAllSettings(
    mechanism.hidden ? 'admin.schedulingMechanisms.mechanismShown' : 'admin.schedulingMechanisms.mechanismHidden'
  )
}

async function removeMechanism(id: string) {
  const nextSettings = cloneSettings(settings)
  nextSettings.mechanisms = nextSettings.mechanisms.filter((item) => item.id !== id)
  settings.mechanisms = nextSettings.mechanisms
  await saveAllSettings('admin.schedulingMechanisms.mechanismDeleted')
}

function triggerImport() {
  importInputRef.value?.click()
}

async function handleImportFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  try {
    const text = await file.text()
    const payload = JSON.parse(text) as
      | SchedulingMechanismSettings
      | { mechanisms?: SchedulingMechanism[] }
      | SchedulingMechanism[]
    const current = cloneSettings(settings)
    const importedMechanisms = Array.isArray(payload)
      ? payload
      : Array.isArray(payload.mechanisms)
        ? payload.mechanisms
        : []

    const mechanismMap = new Map<string, SchedulingMechanism>()
    for (const existing of current.mechanisms) {
      mechanismMap.set(existing.id, existing)
    }
    for (const imported of importedMechanisms) {
      const normalized = normalizeImportedMechanism(imported)
      if (!normalized.name) continue
      mechanismMap.set(normalized.id, normalized)
    }

    current.mechanisms = Array.from(mechanismMap.values())

    saving.value = true
    const updated = await adminAPI.settings.updateSchedulingMechanismSettings(current)
    settings.mechanisms = updated.mechanisms || []
    settings.proxy_failover = updated.proxy_failover || settings.proxy_failover
    appStore.showSuccess(t('admin.schedulingMechanisms.importSuccess'))
  } catch (error: any) {
    console.error('Failed to import scheduling mechanisms:', error)
    appStore.showError(error?.message || t('admin.schedulingMechanisms.importFailed'))
  } finally {
    saving.value = false
    input.value = ''
  }
}

function generateMechanismID() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `mechanism-${Date.now()}-${Math.random().toString(16).slice(2, 10)}`
}

onMounted(() => {
  loadSettings()
})
</script>
