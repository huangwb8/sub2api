<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <template v-else>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-4">
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-gray-400">可转余额</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatBalanceAmount(detail?.aff_quota || 0) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-gray-400">冻结中</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatBalanceAmount(detail?.aff_frozen_quota || 0) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-gray-400">累计返利</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatBalanceAmount(detail?.aff_history_quota || 0) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-gray-400">邀请人数</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ detail?.aff_count || 0 }}
            </p>
          </div>
        </div>

        <div class="card p-6">
          <div class="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
            <div class="min-w-0 flex-1">
              <label class="input-label">专属邀请码</label>
              <div class="mt-2 flex flex-col gap-2 sm:flex-row">
                <input :value="detail?.aff_code || ''" readonly class="input font-mono" />
                <button type="button" class="btn btn-secondary shrink-0" @click="copyInviteCode">
                  复制
                </button>
              </div>
              <p class="mt-2 break-all text-sm text-gray-500 dark:text-gray-400">
                {{ inviteUrl }}
              </p>
            </div>
            <button
              type="button"
              class="btn btn-primary"
              :disabled="transferring || !detail || detail.aff_quota <= 0"
              @click="transferQuota"
            >
              {{ transferring ? '转入中...' : '转入余额' }}
            </button>
          </div>
          <div class="mt-4 rounded-lg bg-gray-50 px-4 py-3 text-sm text-gray-600 dark:bg-dark-800 dark:text-gray-300">
            当前有效返利比例：{{ (detail?.effective_rebate_rate_percent || 0).toFixed(2) }}%
          </div>
        </div>

        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">邀请记录</h2>
          </div>
          <div v-if="!detail?.invitees?.length" class="px-6 py-10 text-center text-gray-500 dark:text-gray-400">
            暂无邀请记录
          </div>
          <div v-else class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-100 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">用户</th>
                  <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">绑定时间</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="invitee in detail.invitees" :key="invitee.user_id">
                  <td class="px-6 py-4">
                    <div class="font-medium text-gray-900 dark:text-white">{{ invitee.username || invitee.email }}</div>
                    <div class="text-sm text-gray-500 dark:text-gray-400">{{ invitee.email }}</div>
                  </td>
                  <td class="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {{ formatDate(invitee.bound_at) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { userAPI, type AffiliateDetail } from '@/api/user'
import { useAppStore, useAuthStore } from '@/stores'
import { formatBalanceAmount, formatDate } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const appStore = useAppStore()
const authStore = useAuthStore()
const loading = ref(true)
const transferring = ref(false)
const detail = ref<AffiliateDetail | null>(null)

const inviteUrl = computed(() => {
  const code = detail.value?.aff_code
  if (!code || typeof window === 'undefined') return ''
  const url = new URL('/register', window.location.origin)
  url.searchParams.set('aff_code', code)
  return url.toString()
})

async function loadAffiliate() {
  loading.value = true
  try {
    detail.value = await userAPI.getAffiliate()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载邀请返利信息失败'))
  } finally {
    loading.value = false
  }
}

async function copyInviteCode() {
  const text = inviteUrl.value || detail.value?.aff_code || ''
  if (!text) return
  await navigator.clipboard.writeText(text)
  appStore.showSuccess('已复制邀请链接')
}

async function transferQuota() {
  transferring.value = true
  try {
    const result = await userAPI.transferAffiliateQuota()
    if (authStore.user) {
      authStore.user.balance = result.balance
    }
    appStore.showSuccess(`已转入 ${formatBalanceAmount(result.transferred)}`)
    await loadAffiliate()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '转入余额失败'))
  } finally {
    transferring.value = false
  }
}

onMounted(loadAffiliate)
</script>
