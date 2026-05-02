<template>
  <div class="min-w-0">
    <button
      type="button"
      class="group flex w-full max-w-[16rem] flex-col items-start rounded-lg border border-gray-200 bg-white px-3 py-2 text-left transition-colors hover:border-primary-300 hover:bg-primary-50/40 dark:border-dark-700 dark:bg-dark-800/80 dark:hover:border-primary-500/50 dark:hover:bg-dark-700/80"
      data-testid="account-proxy-cell-trigger"
      @click="openDialog"
    >
      <div class="flex w-full items-center gap-2">
        <span
          v-if="currentProxy"
          class="min-w-0 truncate text-sm font-medium text-gray-900 dark:text-white"
          :title="currentProxy.name"
        >
          {{ currentProxy.name }}
        </span>
        <span v-else class="text-sm font-medium text-gray-500 dark:text-gray-300">
          {{ t('admin.accounts.proxySwitchDialog.unassigned') }}
        </span>
        <span
          v-if="availabilityState"
          :class="availabilityBadgeClass"
          class="inline-flex shrink-0 items-center rounded-full px-2 py-0.5 text-[11px] font-medium"
        >
          {{ availabilityLabel }}
        </span>
      </div>
      <div class="mt-1 flex w-full items-center justify-between gap-2 text-xs">
        <span
          v-if="currentProxyMeta"
          class="min-w-0 truncate text-gray-500 dark:text-gray-400"
          :title="currentProxyMeta"
        >
          {{ currentProxyMeta }}
        </span>
        <span v-else class="text-gray-400 dark:text-gray-500">-</span>
        <span class="shrink-0 font-medium text-primary-600 transition-colors group-hover:text-primary-700 dark:text-primary-400 dark:group-hover:text-primary-300">
          {{ t('admin.accounts.proxySwitchDialog.action') }}
        </span>
      </div>
    </button>

    <BaseDialog
      :show="showDialog"
      :title="t('admin.accounts.proxySwitchDialog.title', { name: account.name })"
      width="normal"
      @close="closeDialog"
    >
      <div class="space-y-4">
        <div class="rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-sm font-medium text-gray-900 dark:text-white">
              {{ currentProxy?.name || t('admin.accounts.proxySwitchDialog.unassigned') }}
            </span>
            <span
              v-if="availabilityState"
              :class="availabilityBadgeClass"
              class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium"
            >
              {{ availabilityLabel }}
            </span>
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ currentProxySummary }}
          </p>
        </div>

        <div>
          <label class="input-label">{{ t('admin.accounts.proxySwitchDialog.targetLabel') }}</label>
          <ProxySelector v-model="selectedProxyID" :proxies="selectorProxies" />
          <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.proxySwitchDialog.hint') }}
          </p>
          <p
            v-if="!hasAlternativeAvailableProxy"
            class="mt-2 text-xs text-amber-600 dark:text-amber-300"
          >
            {{ t('admin.accounts.proxySwitchDialog.noAlternative') }}
          </p>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeDialog">
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="switching || !hasProxyChanged"
            data-testid="account-proxy-switch-confirm"
            @click="handleSwitch"
          >
            <Icon v-if="switching" name="refresh" size="sm" class="mr-2 animate-spin" />
            {{ t('admin.accounts.proxySwitchDialog.confirm') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Account, Proxy } from '@/types'
import {
  buildProxyTransferTargetLabel,
  compareProxyTransferTargets,
  formatProxyLocation,
  getProxyAvailabilityState,
  isProxyAvailable
} from '@/utils/proxyAvailability'

interface Props {
  account: Account
  proxies: Proxy[]
}

const props = defineProps<Props>()

const emit = defineEmits<{
  updated: [account: Account]
}>()

const { t } = useI18n()
const appStore = useAppStore()

const showDialog = ref(false)
const switching = ref(false)
const selectedProxyID = ref<number | null>(null)

const currentProxy = computed(() => {
  if (props.account.proxy) return props.account.proxy
  if (props.account.proxy_id == null) return null
  return props.proxies.find((proxy) => proxy.id === props.account.proxy_id) ?? null
})

const availabilityState = computed(() => getProxyAvailabilityState(currentProxy.value))

const availabilityBadgeClass = computed(() =>
  availabilityState.value === 'available'
    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
    : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
)

const availabilityLabel = computed(() =>
  availabilityState.value === 'available'
    ? t('admin.accounts.proxyAvailability.available')
    : t('admin.accounts.proxyAvailability.failed')
)

const currentProxyMeta = computed(() => {
  if (!currentProxy.value) return ''
  const location = formatProxyLocation(currentProxy.value)
  if (location) return location
  return `${currentProxy.value.protocol}://${currentProxy.value.host}:${currentProxy.value.port}`
})

const currentProxySummary = computed(() => {
  if (!currentProxy.value) {
    return t('admin.accounts.proxySwitchDialog.noProxySummary')
  }
  return buildProxyTransferTargetLabel(currentProxy.value)
})

const selectorProxies = computed(() => {
  const next = new Map<number, Proxy>()
  if (currentProxy.value) {
    next.set(currentProxy.value.id, currentProxy.value)
  }
  props.proxies
    .filter((proxy) => isProxyAvailable(proxy))
    .sort(compareProxyTransferTargets)
    .forEach((proxy) => {
      next.set(proxy.id, proxy)
    })
  return [...next.values()]
})

const hasAlternativeAvailableProxy = computed(() =>
  props.proxies.some((proxy) => proxy.id !== props.account.proxy_id && isProxyAvailable(proxy))
)

const hasProxyChanged = computed(() => selectedProxyID.value !== (props.account.proxy_id ?? null))

const syncSelectedProxy = () => {
  selectedProxyID.value = props.account.proxy_id ?? null
}

watch(() => props.account.id, syncSelectedProxy, { immediate: true })
watch(() => props.account.proxy_id, syncSelectedProxy)

const openDialog = () => {
  syncSelectedProxy()
  showDialog.value = true
}

const closeDialog = () => {
  showDialog.value = false
  switching.value = false
  syncSelectedProxy()
}

const handleSwitch = async () => {
  if (switching.value || !hasProxyChanged.value) return
  switching.value = true
  try {
    const updated = await adminAPI.accounts.update(props.account.id, {
      proxy_id: selectedProxyID.value === null ? 0 : selectedProxyID.value
    })
    emit('updated', updated)
    appStore.showSuccess(
      t('admin.accounts.proxySwitchSuccess', {
        account: props.account.name,
        proxy: updated.proxy?.name || t('admin.accounts.noProxy')
      })
    )
    closeDialog()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.accounts.proxySwitchFailed'))
  } finally {
    switching.value = false
  }
}
</script>
