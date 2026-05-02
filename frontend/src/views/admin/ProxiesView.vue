<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-4">
          <div class="rounded-xl border border-gray-200 bg-white/80 p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900/70">
            <div class="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
              <div class="max-w-3xl">
                <div class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.proxies.failoverTitle') }}
                </div>
                <p v-if="isProxyFailoverPanelExpanded" class="mt-1 text-sm leading-6 text-gray-600 dark:text-gray-300">
                  {{ t('admin.proxies.failoverDescription') }}
                </p>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  class="btn btn-secondary"
                  :aria-expanded="isProxyFailoverPanelExpanded"
                  aria-controls="proxy-failover-panel"
                  @click="toggleProxyFailoverPanel"
                >
                  <Icon
                    :name="isProxyFailoverPanelExpanded ? 'chevronDown' : 'chevronRight'"
                    size="sm"
                    class="mr-2"
                  />
                  {{ isProxyFailoverPanelExpanded ? t('nav.collapse') : t('nav.expand') }}
                </button>
                <button v-if="isProxyFailoverPanelExpanded" class="btn btn-primary" :disabled="loadingFailoverSettings || savingFailoverSettings" @click="saveProxyFailoverSettings">
                  <Icon v-if="savingFailoverSettings" name="refresh" size="sm" class="mr-2 animate-spin" />
                  {{ t('admin.proxies.saveFailoverSettings') }}
                </button>
              </div>
            </div>

            <div v-if="isProxyFailoverPanelExpanded" id="proxy-failover-panel" class="mt-3 grid gap-2 md:grid-cols-2 xl:grid-cols-4">
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.enabled') }}
                </span>
                <input v-model="proxyFailoverSettings.enabled" type="checkbox" class="toggle" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.autoTestEnabled') }}
                </span>
                <input v-model="proxyFailoverSettings.auto_test_enabled" type="checkbox" class="toggle" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.probeIntervalMinutes') }}
                </span>
                <input v-model.number="proxyFailoverSettings.probe_interval_minutes" type="number" min="1" max="120" class="input h-8 w-20 shrink-0 px-2 py-1 text-right text-sm" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.failureThreshold') }}
                </span>
                <input v-model.number="proxyFailoverSettings.failure_threshold" type="number" min="1" max="10" class="input h-8 w-20 shrink-0 px-2 py-1 text-right text-sm" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.failureWindowMinutes') }}
                </span>
                <input v-model.number="proxyFailoverSettings.failure_window_minutes" type="number" min="1" max="120" class="input h-8 w-20 shrink-0 px-2 py-1 text-right text-sm" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.cooldownMinutes') }}
                </span>
                <input v-model.number="proxyFailoverSettings.cooldown_minutes" type="number" min="1" max="240" class="input h-8 w-20 shrink-0 px-2 py-1 text-right text-sm" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.halfOpenProbeAccounts') }}
                </span>
                <input v-model.number="proxyFailoverSettings.half_open_probe_accounts" type="number" min="1" max="10" class="input h-8 w-20 shrink-0 px-2 py-1 text-right text-sm" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.cooldownBackoffFactor') }}
                </span>
                <input v-model.number="proxyFailoverSettings.cooldown_backoff_factor" type="number" min="1" max="4" class="input h-8 w-20 shrink-0 px-2 py-1 text-right text-sm" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.maxCooldownMinutes') }}
                </span>
                <input v-model.number="proxyFailoverSettings.max_cooldown_minutes" type="number" min="1" max="240" class="input h-8 w-20 shrink-0 px-2 py-1 text-right text-sm" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.tempUnschedMinutes') }}
                </span>
                <input v-model.number="proxyFailoverSettings.temp_unsched_minutes" type="number" min="1" max="240" class="input h-8 w-20 shrink-0 px-2 py-1 text-right text-sm" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.maxAccountsPerProxy') }}
                </span>
                <input v-model.number="proxyFailoverSettings.max_accounts_per_proxy" type="number" min="1" max="100" class="input h-8 w-20 shrink-0 px-2 py-1 text-right text-sm" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.maxMigrationsPerCycle') }}
                </span>
                <input v-model.number="proxyFailoverSettings.max_migrations_per_cycle" type="number" min="1" max="200" class="input h-8 w-20 shrink-0 px-2 py-1 text-right text-sm" />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.preferSameCountry') }}
                </span>
                <input :checked="true" type="checkbox" class="toggle" disabled />
              </label>
              <label class="flex min-h-10 items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                <span class="text-sm font-medium leading-5 text-gray-800 dark:text-gray-200">
                  {{ t('admin.proxies.failoverFields.onlyOpenAIOAuth') }}
                </span>
                <input v-model="proxyFailoverSettings.only_openai_oauth" type="checkbox" class="toggle" />
              </label>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-3">
          <!-- Left: Search + Filters -->
          <div class="relative w-full sm:w-64">
            <Icon
              name="search"
              size="md"
              class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
            />
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.proxies.searchProxies')"
              class="input pl-10"
              @input="handleSearch"
            />
          </div>

          <div class="w-full sm:w-40">
            <Select
              v-model="filters.protocol"
              :options="protocolOptions"
              :placeholder="t('admin.proxies.allProtocols')"
              @change="loadProxies"
            />
          </div>
          <div class="w-full sm:w-36">
            <Select
              v-model="filters.status"
              :options="statusOptions"
              :placeholder="t('admin.proxies.allStatus')"
              @change="loadProxies"
            />
          </div>

          <!-- Right: All action buttons -->
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              @click="loadProxies"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button
              @click="handleBatchTest"
              :disabled="batchTesting || loading"
              class="btn btn-secondary"
              :title="t('admin.proxies.testConnection')"
            >
              <Icon name="play" size="md" class="mr-2" />
              {{ t('admin.proxies.testConnection') }}
            </button>
            <button
              @click="handleBatchQualityCheck"
              :disabled="batchQualityChecking || loading"
              class="btn btn-secondary"
              :title="t('admin.proxies.batchQualityCheck')"
            >
              <Icon name="shield" size="md" class="mr-2" :class="batchQualityChecking ? 'animate-pulse' : ''" />
              {{ t('admin.proxies.batchQualityCheck') }}
            </button>
            <button
              @click="openBatchDelete"
              :disabled="selectedCount === 0"
              class="btn btn-danger"
              :title="t('admin.proxies.batchDeleteAction')"
            >
              <Icon name="trash" size="md" class="mr-2" />
              {{ t('admin.proxies.batchDeleteAction') }}
            </button>
            <button @click="showImportData = true" class="btn btn-secondary">
              {{ t('admin.proxies.dataImport') }}
            </button>
            <button @click="showExportDataDialog = true" class="btn btn-secondary">
              {{ selectedCount > 0 ? t('admin.proxies.dataExportSelected') : t('admin.proxies.dataExport') }}
            </button>
            <button @click="showCreateModal = true" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-2" />
              {{ t('admin.proxies.createProxy') }}
            </button>
          </div>
        </div>
        </div>
      </template>

      <template #table>
        <div ref="proxyTableRef" class="flex min-h-0 flex-1 flex-col overflow-hidden">
        <DataTable
          :columns="columns"
          :data="proxies"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="account_count"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #header-select>
            <input
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="allVisibleSelected"
              @click.stop
              @change="toggleSelectAllVisible($event)"
            />
          </template>

          <template #cell-select="{ row }">
            <input
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="selectedProxyIds.has(row.id)"
              @click.stop
              @change="toggleSelectRow(row.id, $event)"
            />
          </template>

          <template #cell-name="{ value }">
            <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
          </template>

          <template #cell-protocol="{ value }">
            <span
              v-if="value"
              :class="['badge', value.startsWith('socks5') ? 'badge-primary' : 'badge-gray']"
            >
              {{ value.toUpperCase() }}
            </span>
            <span v-else class="text-sm text-gray-400">-</span>
          </template>

          <template #cell-address="{ row }">
            <div class="flex items-center gap-1.5">
              <code class="code text-xs">{{ row.host }}:{{ row.port }}</code>
              <div class="relative">
                <button
                  type="button"
                  class="rounded p-0.5 text-gray-400 hover:text-primary-600 dark:hover:text-primary-400"
                  :title="t('admin.proxies.copyProxyUrl')"
                  @click.stop="copyProxyUrl(row)"
                  @contextmenu.prevent="toggleCopyMenu(row.id)"
                >
                  <Icon name="copy" size="sm" />
                </button>
                <!-- 右键展开格式选择菜单 -->
                <div
                  v-if="copyMenuProxyId === row.id"
                  class="absolute left-0 top-full z-50 mt-1 w-auto min-w-[180px] rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-500 dark:bg-dark-700"
                >
                  <button
                    v-for="fmt in getCopyFormats(row)"
                    :key="fmt.label"
                    class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs hover:bg-gray-100 dark:hover:bg-dark-600"
                    @click.stop="copyFormat(fmt.value)"
                  >
                    <span class="truncate font-mono text-gray-600 dark:text-gray-300">{{ fmt.label }}</span>
                  </button>
                </div>
              </div>
            </div>
          </template>

          <template #cell-auth="{ row }">
            <div v-if="row.username || row.password" class="flex items-center gap-1.5">
              <div class="flex flex-col text-xs">
                <span v-if="row.username" class="text-gray-700 dark:text-gray-200">{{ row.username }}</span>
                <span v-if="row.password" class="font-mono text-gray-500 dark:text-gray-400">
                  {{ visiblePasswordIds.has(row.id) ? row.password : '••••••' }}
                </span>
              </div>
              <button
                v-if="row.password"
                type="button"
                class="ml-1 rounded p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                @click.stop="visiblePasswordIds.has(row.id) ? visiblePasswordIds.delete(row.id) : visiblePasswordIds.add(row.id)"
              >
                <Icon :name="visiblePasswordIds.has(row.id) ? 'eyeOff' : 'eye'" size="sm" />
              </button>
            </div>
            <span v-else class="text-sm text-gray-400">-</span>
          </template>

          <template #cell-location="{ row }">
            <div class="flex items-center gap-2">
              <img
                v-if="row.country_code"
                :src="flagUrl(row.country_code)"
                :alt="row.country || row.country_code"
                class="h-4 w-6 rounded-sm"
              />
              <span v-if="formatLocation(row)" class="text-sm text-gray-700 dark:text-gray-200">
                {{ formatLocation(row) }}
              </span>
              <span v-else class="text-sm text-gray-400">-</span>
            </div>
          </template>

          <template #cell-account_count="{ row, value }">
            <button
              v-if="(value || 0) > 0"
              type="button"
              class="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-primary-700 hover:bg-gray-200 dark:bg-dark-600 dark:text-primary-300 dark:hover:bg-dark-500"
              @click="openAccountsModal(row)"
            >
              {{ t('admin.groups.accountsCount', { count: value || 0 }) }}
            </button>
            <span
              v-else
              class="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-800 dark:bg-dark-600 dark:text-gray-300"
            >
              {{ t('admin.groups.accountsCount', { count: 0 }) }}
            </span>
          </template>

          <template #cell-latency="{ row }">
            <div class="flex flex-col gap-1">
              <span
                v-if="row.latency_status === 'failed'"
                class="badge badge-danger"
                :title="row.latency_message || undefined"
              >
                {{ t('admin.proxies.latencyFailed') }}
              </span>
              <span
                v-else-if="typeof row.latency_ms === 'number'"
                :class="['badge', row.latency_ms < 200 ? 'badge-success' : 'badge-warning']"
              >
                {{ row.latency_ms }}ms
              </span>
              <span v-else class="text-sm text-gray-400">-</span>
              <div
                v-if="typeof row.quality_checked === 'number'"
                class="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400"
                :title="row.quality_summary || undefined"
              >
                <span>{{ t('admin.proxies.qualityInline', { grade: row.quality_grade || '-', score: row.quality_score ?? '-' }) }}</span>
                <span class="badge" :class="qualityOverallClass(row.quality_status)">
                  {{ qualityOverallLabel(row.quality_status) }}
                </span>
              </div>
            </div>
          </template>

          <template #cell-status="{ value }">
            <span :class="['badge', value === 'active' ? 'badge-success' : 'badge-danger']">
              {{ t('admin.accounts.status.' + value) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                @click="handleTestConnection(row)"
                :disabled="testingProxyIds.has(row.id)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-emerald-50 hover:text-emerald-600 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-emerald-900/20 dark:hover:text-emerald-400"
              >
                <svg
                  v-if="testingProxyIds.has(row.id)"
                  class="h-4 w-4 animate-spin"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                <Icon v-else name="checkCircle" size="sm" />
                <span class="text-xs">{{ t('admin.proxies.testConnection') }}</span>
              </button>
              <button
                @click="handleQualityCheck(row)"
                :disabled="qualityCheckingProxyIds.has(row.id)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
              >
                <svg
                  v-if="qualityCheckingProxyIds.has(row.id)"
                  class="h-4 w-4 animate-spin"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                <Icon v-else name="shield" size="sm" />
                <span class="text-xs">{{ t('admin.proxies.qualityCheck') }}</span>
              </button>
              <button
                @click="handleEdit(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
              >
                <Icon name="edit" size="sm" />
                <span class="text-xs">{{ t('common.edit') }}</span>
              </button>
              <button
                @click="handleDelete(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
              >
                <Icon name="trash" size="sm" />
                <span class="text-xs">{{ t('common.delete') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.proxies.noProxiesYet')"
              :description="t('admin.proxies.createFirstProxy')"
              :action-text="t('admin.proxies.createProxy')"
              @action="showCreateModal = true"
            />
          </template>
        </DataTable>
        </div>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Create Proxy Modal -->
    <BaseDialog
      :show="showCreateModal"
      :title="t('admin.proxies.createProxy')"
      width="normal"
      @close="closeCreateModal"
    >
      <!-- Tab Switch -->
      <div class="mb-6 flex border-b border-gray-200 dark:border-dark-600">
        <button
          type="button"
          @click="createMode = 'standard'"
          :class="[
            '-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors',
            createMode === 'standard'
              ? 'border-primary-500 text-primary-600 dark:text-primary-400'
              : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
          ]"
        >
          <Icon name="plus" size="sm" class="mr-1.5 inline" />
          {{ t('admin.proxies.standardAdd') }}
        </button>
        <button
          type="button"
          @click="createMode = 'batch'"
          :class="[
            '-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors',
            createMode === 'batch'
              ? 'border-primary-500 text-primary-600 dark:text-primary-400'
              : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
          ]"
        >
          <svg
            class="mr-1.5 inline h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z"
            />
          </svg>
          {{ t('admin.proxies.batchAdd') }}
        </button>
      </div>

      <!-- Standard Add Form -->
      <form
        v-if="createMode === 'standard'"
        id="create-proxy-form"
        @submit.prevent="handleCreateProxy"
        class="space-y-5"
      >
        <div>
          <label class="input-label">{{ t('admin.proxies.name') }}</label>
          <input
            v-model="createForm.name"
            type="text"
            required
            class="input"
            :placeholder="t('admin.proxies.enterProxyName')"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.protocol') }}</label>
          <Select v-model="createForm.protocol" :options="protocolSelectOptions" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="input-label">{{ t('admin.proxies.host') }}</label>
            <input
              v-model="createForm.host"
              type="text"
              required
              :placeholder="t('admin.proxies.form.hostPlaceholder')"
              class="input"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.proxies.port') }}</label>
            <input
              v-model.number="createForm.port"
              type="number"
              required
              min="1"
              max="65535"
              :placeholder="t('admin.proxies.form.portPlaceholder')"
              class="input"
            />
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.username') }}</label>
          <input
            v-model="createForm.username"
            type="text"
            class="input"
            :placeholder="t('admin.proxies.optionalAuth')"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.password') }}</label>
          <div class="relative">
            <input
              v-model="createForm.password"
              :type="createPasswordVisible ? 'text' : 'password'"
              class="input pr-10"
              :placeholder="t('admin.proxies.optionalAuth')"
            />
            <button
              type="button"
              class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              @click="createPasswordVisible = !createPasswordVisible"
            >
              <Icon :name="createPasswordVisible ? 'eyeOff' : 'eye'" size="md" />
            </button>
          </div>
        </div>

      </form>

      <!-- Batch Add Form -->
      <div v-else class="space-y-5">
        <div>
          <label class="input-label">{{ t('admin.proxies.batchInput') }}</label>
          <textarea
            v-model="batchInput"
            rows="10"
            class="input font-mono text-sm"
            :placeholder="t('admin.proxies.batchInputPlaceholder')"
            @input="parseBatchInput"
          ></textarea>
          <p class="input-hint mt-2">
            {{ t('admin.proxies.batchInputHint') }}
          </p>
        </div>

        <!-- Parse Result -->
        <div v-if="batchParseResult.total > 0" class="rounded-lg bg-gray-50 p-4 dark:bg-dark-700">
            <div class="flex items-center gap-4 text-sm">
              <div class="flex items-center gap-1.5">
              <Icon name="checkCircle" size="sm" :stroke-width="2" class="text-primary-500" />
              <span class="text-gray-700 dark:text-gray-300">
                {{ t('admin.proxies.parsedCount', { count: batchParseResult.valid }) }}
              </span>
            </div>
            <div v-if="batchParseResult.invalid > 0" class="flex items-center gap-1.5">
              <Icon
                name="exclamationCircle"
                size="sm"
                :stroke-width="2"
                class="text-amber-500"
              />
              <span class="text-amber-600 dark:text-amber-400">
                {{ t('admin.proxies.invalidCount', { count: batchParseResult.invalid }) }}
              </span>
            </div>
            <div v-if="batchParseResult.duplicate > 0" class="flex items-center gap-1.5">
              <svg
                class="h-4 w-4 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 01-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 011.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 00-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 01-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 00-3.375-3.375h-1.5a1.125 1.125 0 01-1.125-1.125v-1.5a3.375 3.375 0 00-3.375-3.375H9.75"
                />
              </svg>
              <span class="text-gray-500 dark:text-gray-400">
                {{ t('admin.proxies.duplicateCount', { count: batchParseResult.duplicate }) }}
              </span>
            </div>
          </div>
        </div>

      </div>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeCreateModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            v-if="createMode === 'standard'"
            type="submit"
            form="create-proxy-form"
            :disabled="submitting"
            class="btn btn-primary"
          >
            <svg
              v-if="submitting"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{ submitting ? t('admin.proxies.creating') : t('common.create') }}
          </button>
          <button
            v-else
            @click="handleBatchCreate"
            type="button"
            :disabled="submitting || batchParseResult.valid === 0"
            class="btn btn-primary"
          >
            <svg
              v-if="submitting"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{
              submitting
                ? t('admin.proxies.importing')
                : t('admin.proxies.importProxies', { count: batchParseResult.valid })
            }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Edit Proxy Modal -->
    <BaseDialog
      :show="showEditModal"
      :title="t('admin.proxies.editProxy')"
      width="normal"
      @close="closeEditModal"
    >
      <form
        v-if="editingProxy"
        id="edit-proxy-form"
        @submit.prevent="handleUpdateProxy"
        class="space-y-5"
      >
        <div>
          <label class="input-label">{{ t('admin.proxies.name') }}</label>
          <input v-model="editForm.name" type="text" required class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.protocol') }}</label>
          <Select v-model="editForm.protocol" :options="protocolSelectOptions" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="input-label">{{ t('admin.proxies.host') }}</label>
            <input v-model="editForm.host" type="text" required class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.proxies.port') }}</label>
            <input
              v-model.number="editForm.port"
              type="number"
              required
              min="1"
              max="65535"
              class="input"
            />
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.username') }}</label>
          <input v-model="editForm.username" type="text" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.password') }}</label>
          <div class="relative">
            <input
              v-model="editForm.password"
              :type="editPasswordVisible ? 'text' : 'password'"
              :placeholder="t('admin.proxies.leaveEmptyToKeep')"
              class="input pr-10"
              @input="editPasswordDirty = true"
            />
            <button
              type="button"
              class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              @click="editPasswordVisible = !editPasswordVisible"
            >
              <Icon :name="editPasswordVisible ? 'eyeOff' : 'eye'" size="md" />
            </button>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.status') }}</label>
          <Select v-model="editForm.status" :options="editStatusOptions" />
        </div>

      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeEditModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            v-if="editingProxy"
            type="submit"
            form="edit-proxy-form"
            :disabled="submitting"
            class="btn btn-primary"
          >
            <svg
              v-if="submitting"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{ submitting ? t('admin.proxies.updating') : t('common.update') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.proxies.deleteProxy')"
      :message="t('admin.proxies.deleteConfirm', { name: deletingProxy?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Batch Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showBatchDeleteDialog"
      :title="t('admin.proxies.batchDelete')"
      :message="t('admin.proxies.batchDeleteConfirm', { count: selectedCount })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmBatchDelete"
      @cancel="showBatchDeleteDialog = false"
    />
    <ConfirmDialog
      :show="showExportDataDialog"
      :title="t('admin.proxies.dataExport')"
      :message="t('admin.proxies.dataExportConfirmMessage')"
      :confirm-text="t('admin.proxies.dataExportConfirm')"
      :cancel-text="t('common.cancel')"
      @confirm="handleExportData"
      @cancel="showExportDataDialog = false"
    />

    <ImportDataModal
      :show="showImportData"
      @close="showImportData = false"
      @imported="handleDataImported"
    />

    <BaseDialog
      :show="showQualityReportDialog"
      :title="t('admin.proxies.qualityReportTitle')"
      width="normal"
      @close="closeQualityReportDialog"
    >
      <div v-if="qualityReport" class="space-y-4">
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-700">
          <div class="flex items-center justify-between gap-4">
            <div>
              <div class="text-sm text-gray-500 dark:text-gray-400">
                {{ qualityReportProxy?.name || '-' }}
              </div>
              <div class="mt-1 text-sm text-gray-700 dark:text-gray-200">
                {{ qualityReport.summary }}
              </div>
            </div>
            <div class="text-right">
              <div class="text-2xl font-semibold text-gray-900 dark:text-white">
                {{ qualityReport.score }}
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.proxies.qualityGrade', { grade: qualityReport.grade }) }}
              </div>
            </div>
          </div>
          <div class="mt-3 grid grid-cols-2 gap-2 text-xs text-gray-600 dark:text-gray-300">
            <div>{{ t('admin.proxies.qualityExitIP') }}: {{ qualityReport.exit_ip || '-' }}</div>
            <div>{{ t('admin.proxies.qualityCountry') }}: {{ qualityReport.country || '-' }}</div>
            <div>
              {{ t('admin.proxies.qualityBaseLatency') }}:
              {{ typeof qualityReport.base_latency_ms === 'number' ? `${qualityReport.base_latency_ms}ms` : '-' }}
            </div>
            <div>{{ t('admin.proxies.qualityCheckedAt') }}: {{ new Date(qualityReport.checked_at * 1000).toLocaleString() }}</div>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-900">
          <div class="flex items-center justify-between gap-3">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.proxies.reliabilityTitle') }}
            </h3>
            <span v-if="proxyReliabilityLoading" class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('common.loading') }}
            </span>
          </div>
          <div v-if="proxyReliability" class="mt-3 space-y-3">
            <div class="grid gap-2 md:grid-cols-2">
              <div
                v-for="window in proxyReliability.windows"
                :key="window.label"
                class="rounded-md border border-gray-200 p-3 text-sm dark:border-dark-700"
              >
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ window.label }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">
                  {{ t('admin.proxies.probeSuccessRate') }}:
                  {{ formatPercent(window.probe_success_rate) }}
                </div>
                <div class="mt-1 text-xs text-gray-600 dark:text-gray-300">
                  {{ t('admin.proxies.realSuccessCount') }}: {{ window.usage_success_count }}
                </div>
              </div>
            </div>
            <div class="grid gap-2 md:grid-cols-3">
              <div
                v-for="followup in proxyReliability.failure_followups"
                :key="followup.minutes"
                class="rounded-md bg-gray-50 p-3 text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300"
              >
                {{ t('admin.proxies.afterFailureWindow', { minutes: followup.minutes }) }}:
                <span class="font-medium text-gray-900 dark:text-white">{{ followup.usage_success_count }}</span>
              </div>
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ proxyReliability.interpretation_notes?.[0] || t('admin.proxies.reliabilityHint') }}
            </div>
          </div>
          <div v-else-if="!proxyReliabilityLoading" class="mt-3 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.proxies.noReliabilityData') }}
          </div>
        </div>

        <div class="max-h-80 overflow-auto rounded-lg border border-gray-200 dark:border-dark-600">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
              <tr>
                <th class="px-3 py-2 text-left">{{ t('admin.proxies.qualityTableTarget') }}</th>
                <th class="px-3 py-2 text-left">{{ t('admin.proxies.qualityTableStatus') }}</th>
                <th class="px-3 py-2 text-left">HTTP</th>
                <th class="px-3 py-2 text-left">{{ t('admin.proxies.qualityTableLatency') }}</th>
                <th class="px-3 py-2 text-left">{{ t('admin.proxies.qualityTableMessage') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-for="item in qualityReport.items" :key="item.target">
                <td class="px-3 py-2 text-gray-900 dark:text-white">{{ qualityTargetLabel(item.target) }}</td>
                <td class="px-3 py-2">
                  <span class="badge" :class="qualityStatusClass(item.status)">{{ qualityStatusLabel(item.status) }}</span>
                </td>
                <td class="px-3 py-2 text-gray-600 dark:text-gray-300">{{ item.http_status ?? '-' }}</td>
                <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                  {{ typeof item.latency_ms === 'number' ? `${item.latency_ms}ms` : '-' }}
                </td>
                <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                  <span>{{ item.message || '-' }}</span>
                  <span v-if="item.cf_ray" class="ml-1 text-xs text-gray-400">(cf-ray: {{ item.cf_ray }})</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button @click="closeQualityReportDialog" class="btn btn-secondary">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Proxy Accounts Dialog -->
    <BaseDialog
      :show="showAccountsModal"
      :title="t('admin.proxies.accountsTitle', { name: accountsProxy?.name || '' })"
      width="wide"
      @close="closeAccountsModal"
    >
      <div v-if="accountsLoading" class="flex items-center justify-center py-8 text-sm text-gray-500">
        <Icon name="refresh" size="md" class="mr-2 animate-spin" />
        {{ t('common.loading') }}
      </div>
      <div v-else-if="proxyAccounts.length === 0" class="py-6 text-center text-sm text-gray-500">
        {{ t('admin.proxies.accountsEmpty') }}
      </div>
      <div v-else class="max-h-80 overflow-x-auto overflow-y-auto">
        <table class="min-w-full table-fixed divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
            <tr>
              <th class="w-[28%] px-4 py-2 text-left">{{ t('admin.proxies.accountName') }}</th>
              <th class="w-[20%] px-4 py-2 text-left">{{ t('admin.accounts.columns.platformType') }}</th>
              <th class="w-[22%] px-4 py-2 text-left">{{ t('admin.proxies.accountNotes') }}</th>
              <th class="w-[30%] min-w-[18rem] px-4 py-2 text-left">{{ t('admin.proxies.accountTransferTarget') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
            <tr v-for="account in proxyAccounts" :key="account.id">
              <td class="px-4 py-3 align-top font-medium leading-6 text-gray-900 dark:text-white">
                <span class="block break-all whitespace-normal">{{ account.name }}</span>
              </td>
              <td class="px-4 py-3 align-top">
                <PlatformTypeBadge :platform="account.platform" :type="account.type" />
              </td>
              <td class="px-4 py-3 align-top text-gray-600 dark:text-gray-300">
                <span class="block break-all whitespace-normal leading-6">{{ account.notes || '-' }}</span>
              </td>
              <td class="px-4 py-3 align-top">
                <div class="grid min-w-[16rem] gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
                  <Select
                    :model-value="accountTransferSelections[account.id] ?? ''"
                    :options="accountTransferOptions"
                    :disabled="switchingAccountIds.has(account.id) || accountTransferOptions.length <= 1"
                    :data-testid="`proxy-transfer-select-${account.id}`"
                    class="min-w-0 w-full"
                    @update:model-value="setAccountTransferSelection(account.id, $event)"
                  />
                  <button
                    type="button"
                    class="btn btn-secondary w-full shrink-0 sm:w-auto"
                    :disabled="!canTransferAccount(account.id)"
                    :data-testid="`proxy-transfer-submit-${account.id}`"
                    @click="transferAccountToProxy(account)"
                  >
                    <Icon
                      v-if="switchingAccountIds.has(account.id)"
                      name="refresh"
                      size="sm"
                      class="mr-2 animate-spin"
                    />
                    {{ t('admin.proxies.accountTransferAction') }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button @click="closeAccountsModal" class="btn btn-secondary">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Proxy, ProxyAccountSummary, ProxyProtocol, ProxyQualityCheckResult, ProxyReliabilityReport } from '@/types'
import type { ProxyFailoverSettings } from '@/api/admin/settings'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ImportDataModal from '@/components/admin/proxy/ImportDataModal.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useSwipeSelect } from '@/composables/useSwipeSelect'
import { useTableSelection } from '@/composables/useTableSelection'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const columns = computed<Column[]>(() => [
  { key: 'select', label: '', sortable: false },
  { key: 'name', label: t('admin.proxies.columns.name'), sortable: true },
  { key: 'protocol', label: t('admin.proxies.columns.protocol'), sortable: true },
  { key: 'address', label: t('admin.proxies.columns.address'), sortable: false },
  { key: 'auth', label: t('admin.proxies.columns.auth'), sortable: false },
  { key: 'location', label: t('admin.proxies.columns.location'), sortable: false },
  { key: 'account_count', label: t('admin.proxies.columns.accounts'), sortable: true },
  { key: 'latency', label: t('admin.proxies.columns.latency'), sortable: false },
  { key: 'status', label: t('admin.proxies.columns.status'), sortable: true },
  { key: 'actions', label: t('admin.proxies.columns.actions'), sortable: false }
])

// Filter options
const protocolOptions = computed(() => [
  { value: '', label: t('admin.proxies.allProtocols') },
  { value: 'http', label: 'HTTP' },
  { value: 'https', label: 'HTTPS' },
  { value: 'socks5', label: 'SOCKS5' },
  { value: 'socks5h', label: 'SOCKS5H' }
])

const statusOptions = computed(() => [
  { value: '', label: t('admin.proxies.allStatus') },
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') }
])

// Form options
const protocolSelectOptions = computed(() => [
  { value: 'http', label: t('admin.proxies.protocols.http') },
  { value: 'https', label: t('admin.proxies.protocols.https') },
  { value: 'socks5', label: t('admin.proxies.protocols.socks5') },
  { value: 'socks5h', label: t('admin.proxies.protocols.socks5h') }
])

const editStatusOptions = computed(() => [
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') }
])

const isTransferTargetAvailable = (proxy: Proxy) => {
  if (proxy.status !== 'active') return false
  if (proxy.latency_status === 'failed') return false
  if (proxy.quality_status === 'failed' || proxy.quality_status === 'challenge') return false
  return true
}

const transferTargetQualityRank = (status?: Proxy['quality_status']) => {
  switch (status) {
    case 'healthy':
      return 0
    case 'warn':
      return 1
    default:
      return 2
  }
}

const transferTargetLatencyRank = (proxy: Proxy) => {
  if (proxy.latency_status === 'success' && typeof proxy.latency_ms === 'number') {
    return proxy.latency_ms
  }
  return Number.MAX_SAFE_INTEGER
}

const compareTransferTargets = (left: Proxy, right: Proxy) => {
  const qualityRankDiff = transferTargetQualityRank(left.quality_status) - transferTargetQualityRank(right.quality_status)
  if (qualityRankDiff !== 0) return qualityRankDiff

  const latencyDiff = transferTargetLatencyRank(left) - transferTargetLatencyRank(right)
  if (latencyDiff !== 0) return latencyDiff

  const qualityScoreDiff = (right.quality_score ?? -1) - (left.quality_score ?? -1)
  if (qualityScoreDiff !== 0) return qualityScoreDiff

  const accountCountDiff = (left.account_count ?? 0) - (right.account_count ?? 0)
  if (accountCountDiff !== 0) return accountCountDiff

  return left.name.localeCompare(right.name, 'zh-CN')
}

const buildTransferTargetLabel = (proxy: Proxy) => {
  const parts = [proxy.name, `${proxy.host}:${proxy.port}`]
  const location = formatLocation(proxy)
  if (location) {
    parts.push(location)
  }
  if (typeof proxy.latency_ms === 'number' && proxy.latency_status === 'success') {
    parts.push(`${proxy.latency_ms}ms`)
  }
  if (proxy.quality_grade) {
    parts.push(`Q${proxy.quality_grade}`)
  }
  return parts.join(' · ')
}

const proxies = ref<Proxy[]>([])
const visiblePasswordIds = reactive(new Set<number>())
const copyMenuProxyId = ref<number | null>(null)
const loading = ref(false)
const loadingFailoverSettings = ref(false)
const savingFailoverSettings = ref(false)
const searchQuery = ref('')
const filters = reactive({
  protocol: '',
  status: ''
})
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = reactive({
  sort_by: 'account_count',
  sort_order: 'desc' as 'asc' | 'desc'
})

const showCreateModal = ref(false)
const createPasswordVisible = ref(false)
const showEditModal = ref(false)
const editPasswordVisible = ref(false)
const editPasswordDirty = ref(false)
const showImportData = ref(false)
const showDeleteDialog = ref(false)
const showBatchDeleteDialog = ref(false)
const showExportDataDialog = ref(false)
const showAccountsModal = ref(false)
const submitting = ref(false)
const exportingData = ref(false)
const testingProxyIds = ref<Set<number>>(new Set())
const qualityCheckingProxyIds = ref<Set<number>>(new Set())
const batchTesting = ref(false)
const batchQualityChecking = ref(false)
const proxyTableRef = ref<HTMLElement | null>(null)
const {
  selectedSet: selectedProxyIds,
  selectedCount,
  allVisibleSelected,
  isSelected,
  select,
  deselect,
  clear: clearSelectedProxies,
  removeMany: removeSelectedProxies,
  toggleVisible
} = useTableSelection<Proxy>({
  rows: proxies,
  getId: (proxy) => proxy.id
})
useSwipeSelect(proxyTableRef, {
  isSelected,
  select,
  deselect
})
const accountsProxy = ref<Proxy | null>(null)
const proxyAccounts = ref<ProxyAccountSummary[]>([])
const accountsLoading = ref(false)
const accountTransferSelections = reactive<Record<number, number | ''>>({})
const accountTransferCandidateProxies = ref<Proxy[]>([])
const switchingAccountIds = ref<Set<number>>(new Set())
const editingProxy = ref<Proxy | null>(null)
const deletingProxy = ref<Proxy | null>(null)
const showQualityReportDialog = ref(false)
const qualityReportProxy = ref<Proxy | null>(null)
const qualityReport = ref<ProxyQualityCheckResult | null>(null)
const proxyReliability = ref<ProxyReliabilityReport | null>(null)
const proxyReliabilityLoading = ref(false)
const isProxyFailoverPanelExpanded = ref(false)
const proxyFailoverSettings = reactive<ProxyFailoverSettings>({
  enabled: true,
  auto_test_enabled: true,
  probe_interval_minutes: 5,
  failure_threshold: 3,
  failure_window_minutes: 10,
  cooldown_minutes: 15,
  half_open_probe_accounts: 2,
  cooldown_backoff_factor: 2,
  max_cooldown_minutes: 120,
  max_accounts_per_proxy: 6,
  max_migrations_per_cycle: 12,
  prefer_same_country: true,
  only_openai_oauth: false,
  temp_unsched_minutes: 10
})

function toggleProxyFailoverPanel() {
  isProxyFailoverPanelExpanded.value = !isProxyFailoverPanelExpanded.value
}

const availableTransferTargets = computed(() => {
  const currentProxyID = accountsProxy.value?.id
  return accountTransferCandidateProxies.value
    .filter((proxy) => proxy.id !== currentProxyID && isTransferTargetAvailable(proxy))
    .slice()
    .sort(compareTransferTargets)
})

const accountTransferOptions = computed(() => [
  {
    value: '',
    label:
      availableTransferTargets.value.length > 0
        ? t('admin.proxies.accountTransferPlaceholder')
        : t('admin.proxies.accountTransferUnavailable'),
    disabled: true
  },
  ...availableTransferTargets.value.map((proxy) => ({
    value: proxy.id,
    label: buildTransferTargetLabel(proxy)
  }))
])

// Batch import state
const createMode = ref<'standard' | 'batch'>('standard')
const batchInput = ref('')
const batchParseResult = reactive({
  total: 0,
  valid: 0,
  invalid: 0,
  duplicate: 0,
  proxies: [] as Array<{
    protocol: ProxyProtocol
    host: string
    port: number
    username: string
    password: string
  }>
})

const createForm = reactive({
  name: '',
  protocol: 'http' as ProxyProtocol,
  host: '',
  port: 8080,
  username: '',
  password: ''
})

const editForm = reactive({
  name: '',
  protocol: 'http' as ProxyProtocol,
  host: '',
  port: 8080,
  username: '',
  password: '',
  status: 'active' as 'active' | 'inactive'
})

let abortController: AbortController | null = null

const isAbortError = (error: unknown) => {
  if (!error || typeof error !== 'object') return false
  const maybeError = error as { name?: string; code?: string }
  return maybeError.name === 'AbortError' || maybeError.code === 'ERR_CANCELED'
}

const toggleSelectRow = (id: number, event: Event) => {
  const target = event.target as HTMLInputElement
  if (target.checked) {
    select(id)
    return
  }
  deselect(id)
}

const toggleSelectAllVisible = (event: Event) => {
  const target = event.target as HTMLInputElement
  toggleVisible(target.checked)
}

const buildProxyQueryFilters = () => ({
  protocol: filters.protocol || undefined,
  status: (filters.status || undefined) as 'active' | 'inactive' | undefined,
  search: searchQuery.value || undefined,
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order
})

const loadProxies = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentAbortController = new AbortController()
  abortController = currentAbortController
  loading.value = true
  try {
    const response = await adminAPI.proxies.list(
      pagination.page,
      pagination.page_size,
      buildProxyQueryFilters(),
      { signal: currentAbortController.signal }
    )
    if (currentAbortController.signal.aborted || abortController !== currentAbortController) {
      return
    }
    proxies.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error) {
    if (isAbortError(error)) {
      return
    }
    appStore.showError(t('admin.proxies.failedToLoad'))
    console.error('Error loading proxies:', error)
  } finally {
    if (abortController === currentAbortController) {
      loading.value = false
      abortController = null
    }
  }
}

const loadProxyFailoverSettings = async () => {
  loadingFailoverSettings.value = true
  try {
    const payload = await adminAPI.settings.getProxyFailoverSettings()
    Object.assign(proxyFailoverSettings, payload)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('common.error'))
  } finally {
    loadingFailoverSettings.value = false
  }
}

const saveProxyFailoverSettings = async () => {
  savingFailoverSettings.value = true
  try {
    proxyFailoverSettings.prefer_same_country = true
    const updated = await adminAPI.settings.updateProxyFailoverSettings({ ...proxyFailoverSettings })
    Object.assign(proxyFailoverSettings, updated)
    appStore.showSuccess(t('admin.proxies.failoverSettingsSaved'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failoverSettingsSaveFailed'))
  } finally {
    savingFailoverSettings.value = false
  }
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadProxies()
  }, 300)
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadProxies()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadProxies()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadProxies()
}

const closeCreateModal = () => {
  showCreateModal.value = false
  createMode.value = 'standard'
  createForm.name = ''
  createForm.protocol = 'http'
  createForm.host = ''
  createForm.port = 8080
  createForm.username = ''
  createForm.password = ''
  createPasswordVisible.value = false
  batchInput.value = ''
  batchParseResult.total = 0
  batchParseResult.valid = 0
  batchParseResult.invalid = 0
  batchParseResult.duplicate = 0
  batchParseResult.proxies = []
}

const handleDataImported = () => {
  showImportData.value = false
  loadProxies()
}

// Parse proxy URL: protocol://user:pass@host:port or protocol://host:port
const parseProxyUrl = (
  line: string
): {
  protocol: ProxyProtocol
  host: string
  port: number
  username: string
  password: string
} | null => {
  const trimmed = line.trim()
  if (!trimmed) return null

  // Regex to parse proxy URL (supports http, https, socks5, socks5h)
  const regex = /^(https?|socks5h?):\/\/(?:([^:@]+):([^@]+)@)?([^:]+):(\d+)$/i
  const match = trimmed.match(regex)

  if (!match) return null

  const [, protocol, username, password, host, port] = match
  const portNum = parseInt(port, 10)

  if (portNum < 1 || portNum > 65535) return null

  return {
    protocol: protocol.toLowerCase() as ProxyProtocol,
    host: host.trim(),
    port: portNum,
    username: username?.trim() || '',
    password: password?.trim() || ''
  }
}

const parseBatchInput = () => {
  const lines = batchInput.value.split('\n').filter((l) => l.trim())
  const seen = new Set<string>()
  const proxies: typeof batchParseResult.proxies = []
  let invalid = 0
  let duplicate = 0

  for (const line of lines) {
    const parsed = parseProxyUrl(line)
    if (!parsed) {
      invalid++
      continue
    }

    // Check for duplicates (same host:port:username:password)
    const key = `${parsed.host}:${parsed.port}:${parsed.username}:${parsed.password}`
    if (seen.has(key)) {
      duplicate++
      continue
    }
    seen.add(key)
    proxies.push(parsed)
  }

  batchParseResult.total = lines.length
  batchParseResult.valid = proxies.length
  batchParseResult.invalid = invalid
  batchParseResult.duplicate = duplicate
  batchParseResult.proxies = proxies
}

const handleBatchCreate = async () => {
  if (batchParseResult.valid === 0) return

  submitting.value = true
  try {
    const result = await adminAPI.proxies.batchCreate(batchParseResult.proxies)
    const created = result.created || 0
    const skipped = result.skipped || 0

    if (created > 0) {
      appStore.showSuccess(t('admin.proxies.batchImportSuccess', { created, skipped }))
    } else {
      appStore.showInfo(t('admin.proxies.batchImportAllSkipped', { skipped }))
    }

    closeCreateModal()
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToImport'))
    console.error('Error batch creating proxies:', error)
  } finally {
    submitting.value = false
  }
}

const handleCreateProxy = async () => {
  if (!createForm.name.trim()) {
    appStore.showError(t('admin.proxies.nameRequired'))
    return
  }
  if (!createForm.host.trim()) {
    appStore.showError(t('admin.proxies.hostRequired'))
    return
  }
  if (createForm.port < 1 || createForm.port > 65535) {
    appStore.showError(t('admin.proxies.portInvalid'))
    return
  }
  submitting.value = true
  try {
    await adminAPI.proxies.create({
      name: createForm.name.trim(),
      protocol: createForm.protocol,
      host: createForm.host.trim(),
      port: createForm.port,
      username: createForm.username.trim() || null,
      password: createForm.password.trim() || null
    })
    appStore.showSuccess(t('admin.proxies.proxyCreated'))
    closeCreateModal()
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToCreate'))
    console.error('Error creating proxy:', error)
  } finally {
    submitting.value = false
  }
}

const handleEdit = (proxy: Proxy) => {
  editingProxy.value = proxy
  editForm.name = proxy.name
  editForm.protocol = proxy.protocol
  editForm.host = proxy.host
  editForm.port = proxy.port
  editForm.username = proxy.username || ''
  editForm.password = proxy.password || ''
  editForm.status = proxy.status
  editPasswordVisible.value = false
  editPasswordDirty.value = false
  showEditModal.value = true
}

const closeEditModal = () => {
  showEditModal.value = false
  editingProxy.value = null
  editPasswordVisible.value = false
  editPasswordDirty.value = false
}

const handleUpdateProxy = async () => {
  if (!editingProxy.value) return
  if (!editForm.name.trim()) {
    appStore.showError(t('admin.proxies.nameRequired'))
    return
  }
  if (!editForm.host.trim()) {
    appStore.showError(t('admin.proxies.hostRequired'))
    return
  }
  if (editForm.port < 1 || editForm.port > 65535) {
    appStore.showError(t('admin.proxies.portInvalid'))
    return
  }

  submitting.value = true
  try {
    const updateData: any = {
      name: editForm.name.trim(),
      protocol: editForm.protocol,
      host: editForm.host.trim(),
      port: editForm.port,
      username: editForm.username.trim() || null,
      status: editForm.status
    }

    // Only include password if user actually modified the field
    if (editPasswordDirty.value) {
      updateData.password = editForm.password.trim() || null
    }

    await adminAPI.proxies.update(editingProxy.value.id, updateData)
    appStore.showSuccess(t('admin.proxies.proxyUpdated'))
    closeEditModal()
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToUpdate'))
    console.error('Error updating proxy:', error)
  } finally {
    submitting.value = false
  }
}

const applyLatencyResult = (
  proxyId: number,
  result: {
    success: boolean
    latency_ms?: number
    message?: string
    ip_address?: string
    country?: string
    country_code?: string
    region?: string
    city?: string
  }
) => {
  const target = proxies.value.find((proxy) => proxy.id === proxyId)
  if (!target) return
  if (result.success) {
    target.latency_status = 'success'
    target.latency_ms = result.latency_ms
    target.ip_address = result.ip_address
    target.country = result.country
    target.country_code = result.country_code
    target.region = result.region
    target.city = result.city
  } else {
    target.latency_status = 'failed'
    target.latency_ms = undefined
    target.ip_address = undefined
    target.country = undefined
    target.country_code = undefined
    target.region = undefined
    target.city = undefined
  }
  target.latency_message = result.message
}

const summarizeQualityStatus = (result: ProxyQualityCheckResult): Proxy['quality_status'] => {
  if (result.challenge_count > 0) return 'challenge'
  if (result.failed_count > 0) return 'failed'
  if (result.warn_count > 0) return 'warn'
  return 'healthy'
}

const applyQualityResult = (proxyId: number, result: ProxyQualityCheckResult) => {
  const target = proxies.value.find((proxy) => proxy.id === proxyId)
  if (!target) return
  target.quality_status = summarizeQualityStatus(result)
  target.quality_score = result.score
  target.quality_grade = result.grade
  target.quality_summary = result.summary
  target.quality_checked = result.checked_at
}

const formatLocation = (proxy: Proxy) => {
  const parts = [proxy.country, proxy.city].filter(Boolean) as string[]
  return parts.join(' · ')
}

const flagUrl = (code: string) =>
  `https://unpkg.com/flag-icons/flags/4x3/${code.toLowerCase()}.svg`

const startTestingProxy = (proxyId: number) => {
  testingProxyIds.value = new Set([...testingProxyIds.value, proxyId])
}

const stopTestingProxy = (proxyId: number) => {
  const next = new Set(testingProxyIds.value)
  next.delete(proxyId)
  testingProxyIds.value = next
}

const startQualityCheckingProxy = (proxyId: number) => {
  qualityCheckingProxyIds.value = new Set([...qualityCheckingProxyIds.value, proxyId])
}

const stopQualityCheckingProxy = (proxyId: number) => {
  const next = new Set(qualityCheckingProxyIds.value)
  next.delete(proxyId)
  qualityCheckingProxyIds.value = next
}

const runProxyTest = async (proxyId: number, notify: boolean) => {
  startTestingProxy(proxyId)
  try {
    const result = await adminAPI.proxies.testProxy(proxyId)
    applyLatencyResult(proxyId, result)
    if (notify) {
      if (result.success) {
        const message = result.latency_ms
          ? t('admin.proxies.proxyWorkingWithLatency', { latency: result.latency_ms })
          : t('admin.proxies.proxyWorking')
        appStore.showSuccess(message)
      } else {
        appStore.showError(result.message || t('admin.proxies.proxyTestFailed'))
      }
    }
    return result
  } catch (error: any) {
    const message = error.response?.data?.detail || t('admin.proxies.failedToTest')
    applyLatencyResult(proxyId, { success: false, message })
    if (notify) {
      appStore.showError(message)
    }
    console.error('Error testing proxy:', error)
    return null
  } finally {
    stopTestingProxy(proxyId)
  }
}

const handleTestConnection = async (proxy: Proxy) => {
  await runProxyTest(proxy.id, true)
}

const handleQualityCheck = async (proxy: Proxy) => {
  startQualityCheckingProxy(proxy.id)
  proxyReliabilityLoading.value = true
  try {
    const [result, reliabilityResult] = await Promise.allSettled([
      adminAPI.proxies.checkProxyQuality(proxy.id),
      adminAPI.proxies.getReliability(proxy.id)
    ])
    if (result.status === 'rejected') {
      throw result.reason
    }
    qualityReportProxy.value = proxy
    qualityReport.value = result.value
    proxyReliability.value = reliabilityResult.status === 'fulfilled' ? reliabilityResult.value : null
    showQualityReportDialog.value = true

    const baseStep = result.value.items.find((item) => item.target === 'base_connectivity')
    if (baseStep && baseStep.status === 'pass') {
      applyLatencyResult(proxy.id, {
        success: true,
        latency_ms: result.value.base_latency_ms,
        message: result.value.summary,
        ip_address: result.value.exit_ip,
        country: result.value.country,
        country_code: result.value.country_code
      })
    }
    applyQualityResult(proxy.id, result.value)

    appStore.showSuccess(
      t('admin.proxies.qualityCheckDone', { score: result.value.score, grade: result.value.grade })
    )
  } catch (error: any) {
    const message = error.response?.data?.detail || t('admin.proxies.qualityCheckFailed')
    appStore.showError(message)
    console.error('Error checking proxy quality:', error)
  } finally {
    proxyReliabilityLoading.value = false
    stopQualityCheckingProxy(proxy.id)
  }
}

const runBatchProxyQualityChecks = async (ids: number[]) => {
  if (ids.length === 0) return { total: 0, healthy: 0, warn: 0, challenge: 0, failed: 0 }

  const concurrency = 3
  let index = 0
  let healthy = 0
  let warn = 0
  let challenge = 0
  let failed = 0

  const worker = async () => {
    while (index < ids.length) {
      const current = ids[index]
      index++
      startQualityCheckingProxy(current)
      try {
        const result = await adminAPI.proxies.checkProxyQuality(current)
        const target = proxies.value.find((proxy) => proxy.id === current)
        if (target) {
          const baseStep = result.items.find((item) => item.target === 'base_connectivity')
          if (baseStep && baseStep.status === 'pass') {
            applyLatencyResult(current, {
              success: true,
              latency_ms: result.base_latency_ms,
              message: result.summary,
              ip_address: result.exit_ip,
              country: result.country,
              country_code: result.country_code
            })
          }
        }
        applyQualityResult(current, result)
        if (result.challenge_count > 0) {
          challenge++
        } else if (result.failed_count > 0) {
          failed++
        } else if (result.warn_count > 0) {
          warn++
        } else {
          healthy++
        }
      } catch {
        failed++
      } finally {
        stopQualityCheckingProxy(current)
      }
    }
  }

  const workers = Array.from({ length: Math.min(concurrency, ids.length) }, () => worker())
  await Promise.all(workers)
  return {
    total: ids.length,
    healthy,
    warn,
    challenge,
    failed
  }
}

const closeQualityReportDialog = () => {
  showQualityReportDialog.value = false
  qualityReportProxy.value = null
  qualityReport.value = null
  proxyReliability.value = null
  proxyReliabilityLoading.value = false
}

const formatPercent = (value?: number) => {
  if (typeof value !== 'number') return '-'
  return `${Math.round(value * 100)}%`
}

const qualityStatusClass = (status: string) => {
  if (status === 'pass') return 'badge-success'
  if (status === 'warn') return 'badge-warning'
  if (status === 'challenge') return 'badge-danger'
  return 'badge-danger'
}

const qualityStatusLabel = (status: string) => {
  if (status === 'pass') return t('admin.proxies.qualityStatusPass')
  if (status === 'warn') return t('admin.proxies.qualityStatusWarn')
  if (status === 'challenge') return t('admin.proxies.qualityStatusChallenge')
  return t('admin.proxies.qualityStatusFail')
}

const qualityOverallClass = (status?: string) => {
  if (status === 'healthy') return 'badge-success'
  if (status === 'warn') return 'badge-warning'
  if (status === 'challenge') return 'badge-danger'
  return 'badge-danger'
}

const qualityOverallLabel = (status?: string) => {
  if (status === 'healthy') return t('admin.proxies.qualityStatusHealthy')
  if (status === 'warn') return t('admin.proxies.qualityStatusWarn')
  if (status === 'challenge') return t('admin.proxies.qualityStatusChallenge')
  return t('admin.proxies.qualityStatusFail')
}

const qualityTargetLabel = (target: string) => {
  switch (target) {
    case 'base_connectivity':
      return t('admin.proxies.qualityTargetBase')
    case 'openai':
      return 'OpenAI'
    case 'anthropic':
      return 'Anthropic'
    case 'gemini':
      return 'Gemini'
    default:
      return target
  }
}

const fetchAllProxiesForBatch = async (): Promise<Proxy[]> => {
  const pageSize = 200
  const result: Proxy[] = []
  let page = 1
  let totalPages = 1

  while (page <= totalPages) {
    const response = await adminAPI.proxies.list(
      page,
      pageSize,
      {
        protocol: filters.protocol || undefined,
        status: filters.status as any,
        search: searchQuery.value || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      }
    )
    result.push(...response.items)
    totalPages = response.pages || 1
    page++
  }

  return result
}

const runBatchProxyTests = async (ids: number[]) => {
  if (ids.length === 0) return
  const concurrency = 5
  let index = 0

  const worker = async () => {
    while (index < ids.length) {
      const current = ids[index]
      index++
      await runProxyTest(current, false)
    }
  }

  const workers = Array.from({ length: Math.min(concurrency, ids.length) }, () => worker())
  await Promise.all(workers)
}

const handleBatchTest = async () => {
  if (batchTesting.value) return

  batchTesting.value = true
  try {
    let ids: number[] = []
    if (selectedCount.value > 0) {
      ids = Array.from(selectedProxyIds.value)
    } else {
      const allProxies = await fetchAllProxiesForBatch()
      ids = allProxies.map((proxy) => proxy.id)
    }

    if (ids.length === 0) {
      appStore.showInfo(t('admin.proxies.batchTestEmpty'))
      return
    }

    await runBatchProxyTests(ids)
    appStore.showSuccess(t('admin.proxies.batchTestDone', { count: ids.length }))
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.batchTestFailed'))
    console.error('Error batch testing proxies:', error)
  } finally {
    batchTesting.value = false
  }
}

const handleBatchQualityCheck = async () => {
  if (batchQualityChecking.value) return

  batchQualityChecking.value = true
  try {
    let ids: number[] = []
    if (selectedCount.value > 0) {
      ids = Array.from(selectedProxyIds.value)
    } else {
      const allProxies = await fetchAllProxiesForBatch()
      ids = allProxies.map((proxy) => proxy.id)
    }

    if (ids.length === 0) {
      appStore.showInfo(t('admin.proxies.batchQualityEmpty'))
      return
    }

    const summary = await runBatchProxyQualityChecks(ids)
    appStore.showSuccess(
      t('admin.proxies.batchQualityDone', {
        count: summary.total,
        healthy: summary.healthy,
        warn: summary.warn,
        challenge: summary.challenge,
        failed: summary.failed
      })
    )
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.batchQualityFailed'))
    console.error('Error batch checking quality:', error)
  } finally {
    batchQualityChecking.value = false
  }
}

const formatExportTimestamp = () => {
  const now = new Date()
  const pad2 = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad2(now.getMonth() + 1)}${pad2(now.getDate())}${pad2(now.getHours())}${pad2(now.getMinutes())}${pad2(now.getSeconds())}`
}

const handleExportData = async () => {
  if (exportingData.value) return
  exportingData.value = true
  try {
    const dataPayload = await adminAPI.proxies.exportData(
      selectedCount.value > 0
        ? { ids: Array.from(selectedProxyIds.value) }
        : {
            filters: buildProxyQueryFilters()
          }
    )
    const timestamp = formatExportTimestamp()
    const filename = `sub2api-proxy-${timestamp}.json`
    const blob = new Blob([JSON.stringify(dataPayload, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    link.click()
    URL.revokeObjectURL(url)
    appStore.showSuccess(t('admin.proxies.dataExported'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.proxies.dataExportFailed'))
  } finally {
    exportingData.value = false
    showExportDataDialog.value = false
  }
}

const handleDelete = (proxy: Proxy) => {
  if ((proxy.account_count || 0) > 0) {
    appStore.showError(t('admin.proxies.deleteBlockedInUse'))
    return
  }
  deletingProxy.value = proxy
  showDeleteDialog.value = true
}

const openBatchDelete = () => {
  if (selectedCount.value === 0) {
    return
  }
  showBatchDeleteDialog.value = true
}

const clearAccountTransferState = () => {
  for (const key of Object.keys(accountTransferSelections)) {
    delete accountTransferSelections[Number(key)]
  }
  accountTransferCandidateProxies.value = []
  switchingAccountIds.value = new Set()
}

const setAccountTransferSelection = (accountID: number, value: string | number | boolean | null) => {
  accountTransferSelections[accountID] = typeof value === 'number' ? value : ''
}

const canTransferAccount = (accountID: number) => {
  return typeof accountTransferSelections[accountID] === 'number' && !switchingAccountIds.value.has(accountID)
}

const startSwitchingAccount = (accountID: number) => {
  switchingAccountIds.value = new Set([...switchingAccountIds.value, accountID])
}

const stopSwitchingAccount = (accountID: number) => {
  const next = new Set(switchingAccountIds.value)
  next.delete(accountID)
  switchingAccountIds.value = next
}

const transferAccountToProxy = async (account: ProxyAccountSummary) => {
  const targetProxyID = accountTransferSelections[account.id]
  if (typeof targetProxyID !== 'number') {
    return
  }

  startSwitchingAccount(account.id)
  try {
    await adminAPI.accounts.update(account.id, { proxy_id: targetProxyID })
    proxyAccounts.value = proxyAccounts.value.filter((item) => item.id !== account.id)
    delete accountTransferSelections[account.id]

    const currentProxy = proxies.value.find((proxy) => proxy.id === accountsProxy.value?.id)
    if (currentProxy && typeof currentProxy.account_count === 'number') {
      currentProxy.account_count = Math.max(0, currentProxy.account_count - 1)
    }

    const targetProxy = accountTransferCandidateProxies.value.find((proxy) => proxy.id === targetProxyID)
    if (targetProxy && typeof targetProxy.account_count === 'number') {
      targetProxy.account_count += 1
    }

    const visibleTargetProxy = proxies.value.find((proxy) => proxy.id === targetProxyID)
    if (visibleTargetProxy && typeof visibleTargetProxy.account_count === 'number') {
      visibleTargetProxy.account_count += 1
    }

    appStore.showSuccess(
      t('admin.proxies.accountTransferSuccess', {
        account: account.name,
        proxy: targetProxy?.name || targetProxyID
      })
    )
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.accountTransferFailed'))
    console.error('Error transferring account proxy:', error)
  } finally {
    stopSwitchingAccount(account.id)
  }
}

const confirmDelete = async () => {
  if (!deletingProxy.value) return

  try {
    await adminAPI.proxies.delete(deletingProxy.value.id)
    appStore.showSuccess(t('admin.proxies.proxyDeleted'))
    showDeleteDialog.value = false
    removeSelectedProxies([deletingProxy.value.id])
    deletingProxy.value = null
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToDelete'))
    console.error('Error deleting proxy:', error)
  }
}

const confirmBatchDelete = async () => {
  const ids = Array.from(selectedProxyIds.value)
  if (ids.length === 0) {
    showBatchDeleteDialog.value = false
    return
  }

  try {
    const result = await adminAPI.proxies.batchDelete(ids)
    const deleted = result.deleted_ids?.length || 0
    const skipped = result.skipped?.length || 0

    if (deleted > 0) {
      appStore.showSuccess(t('admin.proxies.batchDeleteDone', { deleted, skipped }))
    } else if (skipped > 0) {
      appStore.showInfo(t('admin.proxies.batchDeleteSkipped', { skipped }))
    }

    clearSelectedProxies()
    showBatchDeleteDialog.value = false
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.batchDeleteFailed'))
    console.error('Error batch deleting proxies:', error)
  }
}

const openAccountsModal = async (proxy: Proxy) => {
  accountsProxy.value = proxy
  proxyAccounts.value = []
  clearAccountTransferState()
  accountsLoading.value = true
  showAccountsModal.value = true

  const [accountsResult, candidateProxiesResult] = await Promise.allSettled([
    adminAPI.proxies.getProxyAccounts(proxy.id),
    adminAPI.proxies.getAllWithCount()
  ])

  if (accountsResult.status === 'fulfilled') {
    proxyAccounts.value = accountsResult.value
  } else {
    const error: any = accountsResult.reason
    appStore.showError(error?.response?.data?.detail || t('admin.proxies.accountsFailed'))
    console.error('Error loading proxy accounts:', error)
  }

  if (candidateProxiesResult.status === 'fulfilled') {
    accountTransferCandidateProxies.value = candidateProxiesResult.value
  } else {
    const error: any = candidateProxiesResult.reason
    accountTransferCandidateProxies.value = proxies.value.filter((item) => item.status === 'active')
    appStore.showError(error?.response?.data?.detail || t('admin.proxies.accountTransferOptionsFailed'))
    console.error('Error loading proxy transfer candidates:', error)
  }

  accountsLoading.value = false
}

const closeAccountsModal = () => {
  showAccountsModal.value = false
  accountsProxy.value = null
  proxyAccounts.value = []
  clearAccountTransferState()
}

// ── Proxy URL copy ──
function buildAuthPart(row: any): string {
  const user = row.username ? encodeURIComponent(row.username) : ''
  const pass = row.password ? encodeURIComponent(row.password) : ''
  if (user && pass) return `${user}:${pass}@`
  if (user) return `${user}@`
  if (pass) return `:${pass}@`
  return ''
}

function buildProxyUrl(row: any): string {
  return `${row.protocol}://${buildAuthPart(row)}${row.host}:${row.port}`
}

function getCopyFormats(row: any) {
  const hasAuth = row.username || row.password
  const fullUrl = buildProxyUrl(row)
  const formats = [
    { label: fullUrl, value: fullUrl },
  ]
  if (hasAuth) {
    const withoutProtocol = fullUrl.replace(/^[^:]+:\/\//, '')
    formats.push({ label: withoutProtocol, value: withoutProtocol })
  }
  formats.push({ label: `${row.host}:${row.port}`, value: `${row.host}:${row.port}` })
  return formats
}

function copyProxyUrl(row: any) {
  copyToClipboard(buildProxyUrl(row), t('admin.proxies.urlCopied'))
  copyMenuProxyId.value = null
}

function toggleCopyMenu(id: number) {
  copyMenuProxyId.value = copyMenuProxyId.value === id ? null : id
}

function copyFormat(value: string) {
  copyToClipboard(value, t('admin.proxies.urlCopied'))
  copyMenuProxyId.value = null
}

function closeCopyMenu() {
  copyMenuProxyId.value = null
}

onMounted(() => {
  loadProxies()
  loadProxyFailoverSettings()
  document.addEventListener('click', closeCopyMenu)
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
  document.removeEventListener('click', closeCopyMenu)
})
</script>
