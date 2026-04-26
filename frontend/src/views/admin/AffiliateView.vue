<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="card p-6">
        <div class="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">邀请返利专属配置</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              仅展示被设置过专属邀请码或专属返利比例的用户。
            </p>
          </div>
          <div class="flex gap-2">
            <input
              v-model="search"
              type="search"
              class="input w-64"
              placeholder="搜索邮箱或用户名"
              @keyup.enter="loadUsers(1)"
            />
            <button type="button" class="btn btn-secondary" @click="loadUsers(1)">搜索</button>
          </div>
        </div>
      </div>

      <div class="card overflow-hidden">
        <div v-if="loading" class="flex justify-center py-12">
          <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
        </div>
        <div v-else-if="items.length === 0" class="px-6 py-10 text-center text-gray-500 dark:text-gray-400">
          暂无专属用户配置
        </div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-100 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">用户</th>
                <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">邀请码</th>
                <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">返利比例</th>
                <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="item in items" :key="item.user_id">
                <td class="px-6 py-4">
                  <div class="font-medium text-gray-900 dark:text-white">{{ item.username || item.email }}</div>
                  <div class="text-sm text-gray-500 dark:text-gray-400">#{{ item.user_id }} · {{ item.email }}</div>
                </td>
                <td class="px-6 py-4">
                  <input v-model="drafts[item.user_id].aff_code" class="input h-10 font-mono" />
                </td>
                <td class="px-6 py-4">
                  <input
                    v-model.number="drafts[item.user_id].aff_rebate_rate_percent"
                    type="number"
                    min="0"
                    max="100"
                    step="0.01"
                    class="input h-10"
                    placeholder="全局"
                  />
                </td>
                <td class="px-6 py-4 text-right">
                  <div class="flex justify-end gap-2">
                    <button type="button" class="btn btn-secondary btn-sm" @click="clearSettings(item.user_id)">
                      清除
                    </button>
                    <button type="button" class="btn btn-primary btn-sm" @click="saveItem(item.user_id)">
                      保存
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { adminAPI } from '@/api'
import type { AffiliateAdminEntry } from '@/api/admin/affiliate'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const appStore = useAppStore()
const loading = ref(false)
const search = ref('')
const items = ref<AffiliateAdminEntry[]>([])
const drafts = reactive<Record<number, { aff_code: string; aff_rebate_rate_percent: number | null }>>({})

function hydrateDrafts(rows: AffiliateAdminEntry[]) {
  for (const row of rows) {
    drafts[row.user_id] = {
      aff_code: row.aff_code,
      aff_rebate_rate_percent: row.aff_rebate_rate_percent ?? null
    }
  }
}

async function loadUsers(page = 1) {
  loading.value = true
  try {
    const data = await adminAPI.affiliate.listUsers(page, 50, search.value.trim())
    items.value = data.items
    hydrateDrafts(data.items)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载邀请返利用户失败'))
  } finally {
    loading.value = false
  }
}

async function saveItem(userID: number) {
  const draft = drafts[userID]
  if (!draft) return
  try {
    await adminAPI.affiliate.updateUserSettings(userID, {
      aff_code: draft.aff_code,
      aff_rebate_rate_percent:
        draft.aff_rebate_rate_percent === null || draft.aff_rebate_rate_percent === undefined
          ? undefined
          : Number(draft.aff_rebate_rate_percent),
      clear_rebate_rate: draft.aff_rebate_rate_percent === null || draft.aff_rebate_rate_percent === undefined
    })
    appStore.showSuccess('已保存专属配置')
    await loadUsers()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '保存专属配置失败'))
  }
}

async function clearSettings(userID: number) {
  try {
    await adminAPI.affiliate.clearUserSettings(userID)
    appStore.showSuccess('已清除专属配置')
    await loadUsers()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '清除专属配置失败'))
  }
}

onMounted(() => loadUsers())
</script>
