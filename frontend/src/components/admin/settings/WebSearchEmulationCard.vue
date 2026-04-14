<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.settings.webSearchEmulation.title') }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.webSearchEmulation.description') }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          :disabled="loading || saving"
          @click="saveConfig"
        >
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </div>

    <div class="space-y-5 p-6">
      <div v-if="loading" class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
        <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
        {{ t('common.loading') }}
      </div>

      <template v-else>
        <div class="flex items-center justify-between">
          <div>
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.webSearchEmulation.enabled') }}
            </label>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.webSearchEmulation.enabledHint') }}
            </p>
          </div>
          <Toggle v-model="config.enabled" />
        </div>

        <div class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div class="flex items-center justify-between">
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.webSearchEmulation.providers') }}
            </label>
            <button type="button" class="btn btn-secondary btn-sm" @click="addProvider">
              {{ t('admin.settings.webSearchEmulation.addProvider') }}
            </button>
          </div>

          <div
            v-if="config.providers.length === 0"
            class="rounded-lg border border-dashed border-gray-300 p-4 text-center text-sm text-gray-400 dark:border-dark-600"
          >
            {{ t('admin.settings.webSearchEmulation.noProviders') }}
          </div>

          <div
            v-for="(provider, index) in config.providers"
            :key="`${provider.type}-${index}`"
            class="rounded-xl border border-gray-200 dark:border-dark-700"
          >
            <div class="flex items-center justify-between gap-3 px-4 py-3">
              <div class="flex items-center gap-3">
                <button
                  type="button"
                  class="rounded-lg border border-gray-200 p-2 text-gray-500 transition hover:bg-gray-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700"
                  @click="toggleExpanded(index)"
                >
                  <Icon
                    name="chevronDown"
                    size="sm"
                    :class="['transition-transform', expanded[index] && 'rotate-180']"
                  />
                </button>
                <div>
                  <div class="text-sm font-medium text-gray-900 dark:text-white">
                    {{ provider.type.toUpperCase() }}
                  </div>
                  <div class="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                    <span>
                      {{ t('admin.settings.webSearchEmulation.quotaUsage') }}:
                      {{ provider.quota_used ?? 0 }} /
                      {{ provider.quota_limit && provider.quota_limit > 0 ? provider.quota_limit : '∞' }}
                    </span>
                    <span
                      v-if="provider.api_key_configured && !provider.api_key"
                      class="text-green-600 dark:text-green-400"
                    >
                      {{ t('admin.settings.webSearchEmulation.apiKeyConfigured') }}
                    </span>
                  </div>
                </div>
              </div>

              <button
                type="button"
                class="text-xs font-medium text-red-500 transition hover:text-red-700"
                @click="removeProvider(index)"
              >
                {{ t('admin.settings.webSearchEmulation.removeProvider') }}
              </button>
            </div>

            <div
              v-if="expanded[index]"
              class="space-y-4 border-t border-gray-100 px-4 py-4 dark:border-dark-700"
            >
              <div class="grid gap-4 lg:grid-cols-2">
                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.settings.webSearchEmulation.providerType') }}
                  </label>
                  <Select
                    v-model="provider.type"
                    :options="providerTypeOptions(index)"
                  />
                </div>

                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.settings.webSearchEmulation.proxy') }}
                  </label>
                  <ProxySelector v-model="provider.proxy_id" :proxies="proxies" />
                </div>
              </div>

              <div>
                <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                  {{ t('admin.settings.webSearchEmulation.apiKey') }}
                </label>
                <div class="relative">
                  <input
                    v-model="provider.api_key"
                    :type="apiKeyVisible[index] ? 'text' : 'password'"
                    class="input w-full pr-20 font-mono text-sm"
                    :placeholder="
                      provider.api_key_configured && !provider.api_key
                        ? '••••••••'
                        : t('admin.settings.webSearchEmulation.apiKeyPlaceholder')
                    "
                  />
                  <div class="absolute inset-y-0 right-0 flex items-center gap-1 pr-2">
                    <button
                      type="button"
                      class="rounded p-1 text-gray-400 transition hover:text-gray-600 dark:hover:text-gray-300"
                      :title="
                        apiKeyVisible[index]
                          ? t('admin.settings.webSearchEmulation.hideApiKey')
                          : t('admin.settings.webSearchEmulation.showApiKey')
                      "
                      @click="toggleApiKeyVisible(index)"
                    >
                      <Icon :name="apiKeyVisible[index] ? 'eyeOff' : 'eye'" size="sm" />
                    </button>
                    <button
                      type="button"
                      class="rounded p-1 text-gray-400 transition hover:text-gray-600 disabled:cursor-not-allowed disabled:opacity-30 dark:hover:text-gray-300"
                      :title="t('admin.settings.webSearchEmulation.copyApiKey')"
                      :disabled="!provider.api_key"
                      @click="copyApiKey(index)"
                    >
                      <Icon name="copy" size="sm" />
                    </button>
                  </div>
                </div>
              </div>

              <div class="grid gap-4 md:grid-cols-3">
                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.settings.webSearchEmulation.quotaLimit') }}
                  </label>
                  <input
                    v-model.number="provider.quota_limit"
                    type="number"
                    min="1"
                    class="input text-sm"
                    :placeholder="'∞'"
                  />
                  <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                    {{ t('admin.settings.webSearchEmulation.quotaLimitHint') }}
                  </p>
                </div>

                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.settings.webSearchEmulation.subscribedAt') }}
                  </label>
                  <input
                    :value="formatDateInput(provider.subscribed_at)"
                    type="date"
                    class="input text-sm"
                    @input="updateSubscribedAt(provider, $event)"
                  />
                  <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                    {{ t('admin.settings.webSearchEmulation.subscribedAtHint') }}
                  </p>
                </div>

                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ t('admin.settings.webSearchEmulation.expiresAt') }}
                  </label>
                  <input
                    :value="formatDateInput(provider.expires_at)"
                    type="date"
                    class="input text-sm"
                    @input="updateExpiresAt(provider, $event)"
                  />
                </div>
              </div>

              <div class="space-y-2">
                <div class="flex items-center gap-2">
                  <span class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.webSearchEmulation.quotaUsage') }}:
                  </span>
                  <span class="text-xs font-medium text-gray-700 dark:text-gray-200">
                    {{ provider.quota_used ?? 0 }} /
                    {{ provider.quota_limit && provider.quota_limit > 0 ? provider.quota_limit : '∞' }}
                  </span>
                </div>
                <div
                  v-if="provider.quota_limit && provider.quota_limit > 0"
                  class="h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700"
                >
                  <div
                    class="h-full rounded-full bg-primary-500 transition-all"
                    :style="{ width: `${Math.min(quotaPercentage(provider), 100)}%` }"
                  ></div>
                </div>
              </div>

              <div class="flex flex-wrap items-center gap-3">
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="resettingType === provider.type || (provider.quota_used ?? 0) <= 0"
                  @click="resetUsage(provider)"
                >
                  {{
                    resettingType === provider.type
                      ? t('common.processing')
                      : t('admin.settings.webSearchEmulation.resetUsage')
                  }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="testing || !config.enabled"
                  @click="openTestDialog(provider.type)"
                >
                  {{ testing ? t('admin.settings.webSearchEmulation.testing') : t('admin.settings.webSearchEmulation.test') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>

    <div
      v-if="testDialogOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4"
      @click.self="closeTestDialog"
    >
      <div class="w-full max-w-2xl rounded-xl bg-white shadow-xl dark:bg-dark-800">
        <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.settings.webSearchEmulation.testResultTitle') }}
          </h3>
        </div>
        <div class="space-y-4 p-6">
          <div class="flex items-center gap-3">
            <input
              v-model="testQuery"
              type="text"
              class="input flex-1 text-sm"
              :placeholder="t('admin.settings.webSearchEmulation.testDefaultQuery')"
              @keyup.enter="runTest"
            />
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="testing"
              @click="runTest"
            >
              {{ testing ? t('admin.settings.webSearchEmulation.testing') : t('admin.settings.webSearchEmulation.test') }}
            </button>
          </div>

          <div
            v-if="testResult"
            class="max-h-96 space-y-3 overflow-y-auto rounded-lg bg-gray-50 p-4 dark:bg-dark-700"
          >
            <p class="text-sm font-medium text-gray-700 dark:text-gray-200">
              {{ t('admin.settings.webSearchEmulation.testResultProvider') }}:
              {{ testResult.provider }}
            </p>
            <div
              v-if="testResult.results.length === 0"
              class="text-sm text-gray-400 dark:text-gray-500"
            >
              {{ t('admin.settings.webSearchEmulation.testNoResults') }}
            </div>
            <div
              v-for="(result, resultIndex) in testResult.results"
              :key="`${result.url}-${resultIndex}`"
              class="border-t border-gray-200 pt-3 first:border-0 first:pt-0 dark:border-dark-600"
            >
              <a
                :href="result.url"
                target="_blank"
                rel="noreferrer"
                class="text-sm font-medium text-blue-600 hover:underline dark:text-blue-400"
              >
                {{ result.title }}
              </a>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ result.snippet }}
              </p>
              <p class="mt-1 break-all text-xs text-gray-400 dark:text-gray-500">
                {{ result.url }}
              </p>
            </div>
          </div>
        </div>
        <div class="flex justify-end border-t border-gray-100 px-6 py-4 dark:border-dark-700">
          <button type="button" class="btn btn-secondary" @click="closeTestDialog">
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import type {
  WebSearchEmulationConfig,
  WebSearchProviderConfig,
  WebSearchTestResult
} from '@/api/admin/settings'
import type { Proxy } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const DEFAULT_QUOTA_LIMIT = 1000

const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const resettingType = ref<string | null>(null)
const testDialogOpen = ref(false)
const testQuery = ref('')
const testProviderType = ref<string | null>(null)
const testResult = ref<WebSearchTestResult | null>(null)
const proxies = ref<Proxy[]>([])
const expanded = reactive<Record<number, boolean>>({})
const apiKeyVisible = reactive<Record<number, boolean>>({})
const config = reactive<WebSearchEmulationConfig>({
  enabled: false,
  providers: []
})

function createProvider(type: string = 'brave'): WebSearchProviderConfig {
  return {
    type,
    api_key: '',
    api_key_configured: false,
    quota_limit: DEFAULT_QUOTA_LIMIT,
    subscribed_at: null,
    quota_used: 0,
    proxy_id: null,
    expires_at: null
  }
}

function normalizeConfig(value: WebSearchEmulationConfig | null | undefined): WebSearchEmulationConfig {
  return {
    enabled: Boolean(value?.enabled),
    providers: Array.isArray(value?.providers)
      ? value.providers.map((provider) => ({
          type: provider.type || 'brave',
          api_key: provider.api_key || '',
          api_key_configured: Boolean(provider.api_key_configured),
          quota_limit:
            typeof provider.quota_limit === 'number' && provider.quota_limit > 0
              ? provider.quota_limit
              : null,
          subscribed_at: provider.subscribed_at ?? null,
          quota_used: provider.quota_used ?? 0,
          proxy_id: provider.proxy_id ?? null,
          expires_at: provider.expires_at ?? null
        }))
      : []
  }
}

function applyConfig(value: WebSearchEmulationConfig) {
  const normalized = normalizeConfig(value)
  config.enabled = normalized.enabled
  config.providers.splice(0, config.providers.length, ...normalized.providers)
  Object.keys(expanded).forEach((key) => delete expanded[Number(key)])
  Object.keys(apiKeyVisible).forEach((key) => delete apiKeyVisible[Number(key)])
  config.providers.forEach((_, index) => {
    expanded[index] = index === 0
    apiKeyVisible[index] = false
  })
}

function selectedTypes(excludeIndex?: number): Set<string> {
  const types = new Set<string>()
  config.providers.forEach((provider, index) => {
    if (excludeIndex === index) {
      return
    }
    if (provider.type) {
      types.add(provider.type)
    }
  })
  return types
}

function providerTypeOptions(index: number) {
  const used = selectedTypes(index)
  return [
    { value: 'brave', label: 'Brave', disabled: used.has('brave') },
    { value: 'tavily', label: 'Tavily', disabled: used.has('tavily') }
  ]
}

function toggleExpanded(index: number) {
  expanded[index] = !expanded[index]
}

function toggleApiKeyVisible(index: number) {
  apiKeyVisible[index] = !apiKeyVisible[index]
}

function addProvider() {
  const type = selectedTypes().has('brave') ? 'tavily' : 'brave'
  const index = config.providers.length
  config.providers.push(createProvider(type))
  expanded[index] = true
  apiKeyVisible[index] = false
}

function removeProvider(index: number) {
  config.providers.splice(index, 1)
  const nextExpanded: Record<number, boolean> = {}
  const nextVisible: Record<number, boolean> = {}
  config.providers.forEach((_, providerIndex) => {
    nextExpanded[providerIndex] = expanded[providerIndex >= index ? providerIndex + 1 : providerIndex] ?? false
    nextVisible[providerIndex] = apiKeyVisible[providerIndex >= index ? providerIndex + 1 : providerIndex] ?? false
  })
  Object.keys(expanded).forEach((key) => delete expanded[Number(key)])
  Object.assign(expanded, nextExpanded)
  Object.keys(apiKeyVisible).forEach((key) => delete apiKeyVisible[Number(key)])
  Object.assign(apiKeyVisible, nextVisible)
}

function quotaPercentage(provider: WebSearchProviderConfig): number {
  if (!provider.quota_limit || provider.quota_limit <= 0) {
    return 0
  }
  return ((provider.quota_used ?? 0) / provider.quota_limit) * 100
}

function formatDateInput(timestamp?: number | null): string {
  if (!timestamp) {
    return ''
  }
  return new Date(timestamp * 1000).toISOString().slice(0, 10)
}

function parseDateInput(value: string): number | null {
  if (!value) {
    return null
  }
  return Math.floor(new Date(`${value}T00:00:00Z`).getTime() / 1000)
}

function updateSubscribedAt(provider: WebSearchProviderConfig, event: Event) {
  const value = (event.target as HTMLInputElement).value
  provider.subscribed_at = parseDateInput(value)
}

function updateExpiresAt(provider: WebSearchProviderConfig, event: Event) {
  const value = (event.target as HTMLInputElement).value
  provider.expires_at = parseDateInput(value)
}

async function copyApiKey(index: number) {
  const key = config.providers[index]?.api_key
  if (!key) {
    return
  }
  try {
    await navigator.clipboard.writeText(key)
    appStore.showSuccess(t('admin.settings.webSearchEmulation.copied'))
  } catch {
    appStore.showError(t('common.copyFailed'))
  }
}

async function loadData() {
  loading.value = true
  try {
    const [webSearchConfig, proxyList] = await Promise.all([
      adminAPI.settings.getWebSearchEmulationConfig(),
      adminAPI.proxies.getAll().catch(() => [] as Proxy[])
    ])
    applyConfig(webSearchConfig)
    proxies.value = proxyList
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

function buildPayload(): WebSearchEmulationConfig | null {
  for (const provider of config.providers) {
    if (provider.quota_limit != null && Number(provider.quota_limit) < 1) {
      appStore.showError(t('admin.settings.webSearchEmulation.quotaLimitMustBePositive'))
      return null
    }
  }

  return {
    enabled: config.enabled,
    providers: config.providers.map((provider) => ({
      type: provider.type,
      api_key: provider.api_key?.trim() || '',
      api_key_configured: provider.api_key_configured,
      quota_limit:
        provider.quota_limit != null && Number(provider.quota_limit) > 0
          ? Number(provider.quota_limit)
          : null,
      subscribed_at: provider.subscribed_at ?? null,
      quota_used: provider.quota_used ?? 0,
      proxy_id: provider.proxy_id ?? null,
      expires_at: provider.expires_at ?? null
    }))
  }
}

async function saveConfig() {
  const payload = buildPayload()
  if (!payload) {
    return
  }

  saving.value = true
  try {
    const updated = await adminAPI.settings.updateWebSearchEmulationConfig(payload)
    applyConfig(updated)
    appStore.showSuccess(t('admin.settings.settingsSaved'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.failedToSave')))
  } finally {
    saving.value = false
  }
}

async function resetUsage(provider: WebSearchProviderConfig) {
  if (!confirm(t('admin.settings.webSearchEmulation.resetUsageConfirm'))) {
    return
  }

  resettingType.value = provider.type
  try {
    await adminAPI.settings.resetWebSearchUsage({ provider_type: provider.type })
    provider.quota_used = 0
    appStore.showSuccess(t('admin.settings.webSearchEmulation.resetUsageSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    resettingType.value = null
  }
}

function openTestDialog(providerType: string) {
  testProviderType.value = providerType
  testDialogOpen.value = true
  testResult.value = null
}

function closeTestDialog() {
  testDialogOpen.value = false
  testProviderType.value = null
  testResult.value = null
}

async function runTest() {
  testing.value = true
  try {
    const query = testQuery.value.trim() || t('admin.settings.webSearchEmulation.testDefaultQuery')
    const result = await adminAPI.settings.testWebSearchEmulation(query)
    if (testProviderType.value && result.provider !== testProviderType.value) {
      appStore.showSuccess(
        `${t('admin.settings.webSearchEmulation.testResultProvider')}: ${result.provider}`
      )
    }
    testResult.value = result
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    testing.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>
