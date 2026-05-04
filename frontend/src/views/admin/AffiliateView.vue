<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6">
      <div class="card p-6">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">邀请返利管理</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              管理专属用户配置，并审计邀请、返利入账与转余额记录。
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="tab in tabs"
              :key="tab.key"
              type="button"
              class="rounded-md px-3 py-2 text-sm font-medium transition"
              :class="
                activeTab === tab.key
                  ? 'bg-primary-600 text-white shadow-sm'
                  : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'
              "
              @click="activateTab(tab.key)"
            >
              {{ tab.label }}
            </button>
          </div>
        </div>
      </div>

      <template v-if="activeTab === 'settings'">
        <div class="card p-6">
          <div class="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">专属配置</h3>
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
                    <div class="font-medium text-gray-900 dark:text-white">{{ userName(item.username, item.email) }}</div>
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
      </template>

      <template v-else>
        <div class="card p-6">
          <div class="grid gap-3 md:grid-cols-[minmax(0,1fr)_160px_160px_auto] md:items-end">
            <label class="space-y-1">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">搜索</span>
              <input
                v-model="recordSearch"
                type="search"
                class="input"
                placeholder="邮箱、用户名、订单号或用户 ID"
                @keyup.enter="loadRecords(1)"
              />
            </label>
            <label class="space-y-1">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">开始日期</span>
              <input v-model="recordStartDate" type="date" class="input" />
            </label>
            <label class="space-y-1">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">结束日期</span>
              <input v-model="recordEndDate" type="date" class="input" />
            </label>
            <button type="button" class="btn btn-primary" @click="loadRecords(1)">刷新</button>
          </div>
        </div>

        <div class="card overflow-hidden">
          <div v-if="recordsLoading" class="flex justify-center py-12">
            <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
          </div>

          <div
            v-else-if="activeTab === 'invites'"
            class="overflow-x-auto"
          >
            <table v-if="inviteRecords.length > 0" class="min-w-full divide-y divide-gray-100 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="record-th">邀请人</th>
                  <th class="record-th">被邀请人</th>
                  <th class="record-th">邀请码</th>
                  <th class="record-th text-right">累计返利</th>
                  <th class="record-th">绑定时间</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="record in inviteRecords" :key="`${record.inviter_id}-${record.invitee_id}`">
                  <td class="record-td">
                    <div class="font-medium text-gray-900 dark:text-white">{{ userName(record.inviter_username, record.inviter_email) }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">#{{ record.inviter_id }} · {{ record.inviter_email }}</div>
                  </td>
                  <td class="record-td">
                    <div class="font-medium text-gray-900 dark:text-white">{{ userName(record.invitee_username, record.invitee_email) }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">#{{ record.invitee_id }} · {{ record.invitee_email }}</div>
                  </td>
                  <td class="record-td font-mono">{{ record.aff_code || '-' }}</td>
                  <td class="record-td text-right tabular-nums">{{ formatAmount(record.total_rebate) }}</td>
                  <td class="record-td whitespace-nowrap">{{ formatDateTime(record.created_at) }}</td>
                </tr>
              </tbody>
            </table>
            <EmptyRecords v-else />
          </div>

          <div
            v-else-if="activeTab === 'rebates'"
            class="overflow-x-auto"
          >
            <table v-if="rebateRecords.length > 0" class="min-w-full divide-y divide-gray-100 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="record-th">订单</th>
                  <th class="record-th">邀请人</th>
                  <th class="record-th">被邀请人</th>
                  <th class="record-th text-right">支付金额</th>
                  <th class="record-th text-right">返利</th>
                  <th class="record-th">状态</th>
                  <th class="record-th">入账时间</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="record in rebateRecords" :key="record.order_id">
                  <td class="record-td">
                    <div class="font-medium text-gray-900 dark:text-white">#{{ record.order_id }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">{{ record.out_trade_no || '-' }}</div>
                  </td>
                  <td class="record-td">
                    <div class="font-medium text-gray-900 dark:text-white">{{ userName(record.inviter_username, record.inviter_email) }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">#{{ record.inviter_id }}</div>
                  </td>
                  <td class="record-td">
                    <div class="font-medium text-gray-900 dark:text-white">{{ userName(record.invitee_username, record.invitee_email) }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">#{{ record.invitee_id }}</div>
                  </td>
                  <td class="record-td text-right tabular-nums">{{ formatAmount(record.pay_amount || record.order_amount) }}</td>
                  <td class="record-td text-right tabular-nums">{{ formatAmount(record.rebate_amount) }}</td>
                  <td class="record-td">
                    <span class="rounded bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-dark-700 dark:text-gray-300">
                      {{ record.order_status || '-' }}
                    </span>
                  </td>
                  <td class="record-td whitespace-nowrap">{{ formatDateTime(record.created_at) }}</td>
                </tr>
              </tbody>
            </table>
            <EmptyRecords v-else />
          </div>

          <div
            v-else
            class="overflow-x-auto"
          >
            <table v-if="transferRecords.length > 0" class="min-w-full divide-y divide-gray-100 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="record-th">用户</th>
                  <th class="record-th text-right">转入余额</th>
                  <th class="record-th text-right">余额快照</th>
                  <th class="record-th text-right">可用返利快照</th>
                  <th class="record-th text-right">冻结返利快照</th>
                  <th class="record-th">转入时间</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="record in transferRecords" :key="record.ledger_id">
                  <td class="record-td">
                    <div class="font-medium text-gray-900 dark:text-white">{{ userName(record.username, record.user_email) }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">#{{ record.user_id }} · {{ record.user_email }}</div>
                  </td>
                  <td class="record-td text-right tabular-nums">{{ formatAmount(record.amount) }}</td>
                  <td class="record-td text-right tabular-nums">{{ formatOptionalAmount(record.balance_after) }}</td>
                  <td class="record-td text-right tabular-nums">{{ formatOptionalAmount(record.available_quota_after) }}</td>
                  <td class="record-td text-right tabular-nums">{{ formatOptionalAmount(record.frozen_quota_after) }}</td>
                  <td class="record-td whitespace-nowrap">{{ formatDateTime(record.created_at) }}</td>
                </tr>
              </tbody>
            </table>
            <EmptyRecords v-else />
          </div>

          <div class="flex items-center justify-between border-t border-gray-100 px-6 py-4 text-sm dark:border-dark-700">
            <span class="text-gray-500 dark:text-gray-400">共 {{ recordTotal }} 条</span>
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="recordPage <= 1"
                @click="loadRecords(recordPage - 1)"
              >
                上一页
              </button>
              <span class="text-gray-600 dark:text-gray-300">{{ recordPage }} / {{ recordPages }}</span>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="recordPage >= recordPages"
                @click="loadRecords(recordPage + 1)"
              >
                下一页
              </button>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { defineComponent, h, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { adminAPI } from '@/api'
import type {
  AffiliateAdminEntry,
  AffiliateInviteRecord,
  AffiliateRebateRecord,
  AffiliateTransferRecord
} from '@/api/admin/affiliate'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatBalanceAmount, formatDateTime } from '@/utils/format'

type ActiveTab = 'settings' | 'invites' | 'rebates' | 'transfers'

const EmptyRecords = defineComponent({
  name: 'EmptyRecords',
  setup() {
    return () => h('div', { class: 'px-6 py-10 text-center text-gray-500 dark:text-gray-400' }, '暂无记录')
  }
})

const tabs: Array<{ key: ActiveTab; label: string }> = [
  { key: 'settings', label: '专属配置' },
  { key: 'invites', label: '邀请记录' },
  { key: 'rebates', label: '返利入账' },
  { key: 'transfers', label: '转余额记录' }
]

const appStore = useAppStore()
const activeTab = ref<ActiveTab>('settings')
const loading = ref(false)
const search = ref('')
const items = ref<AffiliateAdminEntry[]>([])
const drafts = reactive<Record<number, { aff_code: string; aff_rebate_rate_percent: number | null }>>({})

const recordsLoading = ref(false)
const recordSearch = ref('')
const recordStartDate = ref('')
const recordEndDate = ref('')
const recordPage = ref(1)
const recordPages = ref(1)
const recordTotal = ref(0)
const recordPageSize = 20
const inviteRecords = ref<AffiliateInviteRecord[]>([])
const rebateRecords = ref<AffiliateRebateRecord[]>([])
const transferRecords = ref<AffiliateTransferRecord[]>([])
const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone

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

async function activateTab(tab: ActiveTab) {
  activeTab.value = tab
  if (tab === 'settings') {
    await loadUsers()
    return
  }
  await loadRecords(1)
}

function buildRecordParams(page: number) {
  return {
    page,
    page_size: recordPageSize,
    search: recordSearch.value.trim() || undefined,
    start_at: recordStartDate.value || undefined,
    end_at: recordEndDate.value || undefined,
    timezone
  }
}

async function loadRecords(page = recordPage.value) {
  if (activeTab.value === 'settings') return
  recordsLoading.value = true
  try {
    const params = buildRecordParams(page)
    const data =
      activeTab.value === 'invites'
        ? await adminAPI.affiliate.listInviteRecords(params)
        : activeTab.value === 'rebates'
          ? await adminAPI.affiliate.listRebateRecords(params)
          : await adminAPI.affiliate.listTransferRecords(params)

    recordPage.value = data.page
    recordPages.value = Math.max(1, data.pages)
    recordTotal.value = data.total
    if (activeTab.value === 'invites') {
      inviteRecords.value = data.items as AffiliateInviteRecord[]
    } else if (activeTab.value === 'rebates') {
      rebateRecords.value = data.items as AffiliateRebateRecord[]
    } else {
      transferRecords.value = data.items as AffiliateTransferRecord[]
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载邀请返利记录失败'))
  } finally {
    recordsLoading.value = false
  }
}

function userName(username: string, email: string): string {
  return username || email || '-'
}

function formatAmount(value: number | null | undefined): string {
  return formatBalanceAmount(value)
}

function formatOptionalAmount(value: number | null | undefined): string {
  return value === null || value === undefined ? '-' : formatAmount(value)
}

onMounted(() => loadUsers())
</script>

<style scoped>
.record-th {
  @apply px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500;
}

.record-td {
  @apply px-6 py-4 text-sm text-gray-700 dark:text-gray-300;
}
</style>
