<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <!-- Row 1: Core Stats -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <!-- Total API Keys -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30">
                <Icon name="key" size="md" class="text-blue-600 dark:text-blue-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.apiKeys') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ stats.total_api_keys }}
                </p>
                <p class="text-xs text-green-600 dark:text-green-400">
                  {{ stats.active_api_keys }} {{ t('common.active') }}
                </p>
              </div>
            </div>
          </div>

          <!-- Service Accounts -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30">
                <Icon name="server" size="md" class="text-purple-600 dark:text-purple-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.accounts') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ stats.total_accounts }}
                </p>
                <p class="text-xs">
                  <span class="text-green-600 dark:text-green-400"
                    >{{ stats.normal_accounts }} {{ t('common.active') }}</span
                  >
                  <span v-if="stats.error_accounts > 0" class="ml-1 text-red-500"
                    >{{ stats.error_accounts }} {{ t('common.error') }}</span
                  >
                </p>
              </div>
            </div>
          </div>

          <!-- Today Requests -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30">
                <Icon name="chart" size="md" class="text-green-600 dark:text-green-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.todayRequests') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ stats.today_requests }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('common.total') }}: {{ formatNumber(stats.total_requests) }}
                </p>
              </div>
            </div>
          </div>

          <!-- New Users Today -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-emerald-100 p-2 dark:bg-emerald-900/30">
                <Icon name="userPlus" size="md" class="text-emerald-600 dark:text-emerald-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.users') }}
                </p>
                <p class="text-xl font-bold text-emerald-600 dark:text-emerald-400">
                  +{{ stats.today_new_users }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('common.total') }}: {{ formatNumber(stats.total_users) }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Row 2: Token Stats -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <!-- Today Tokens -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
                <Icon name="cube" size="md" class="text-amber-600 dark:text-amber-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.todayTokens') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ formatTokens(stats.today_tokens) }}
                </p>
                <p class="text-xs">
                  <span
                    class="text-amber-600 dark:text-amber-400"
                    :title="t('admin.dashboard.actual')"
                    >${{ formatCost(stats.today_actual_cost) }}</span
                  >
                  <span
                    class="text-gray-400 dark:text-gray-500"
                    :title="t('admin.dashboard.standard')"
                  >
                    / ${{ formatCost(stats.today_cost) }}</span
                  >
                </p>
              </div>
            </div>
          </div>

          <!-- Total Tokens -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-indigo-100 p-2 dark:bg-indigo-900/30">
                <Icon name="database" size="md" class="text-indigo-600 dark:text-indigo-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.totalTokens') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ formatTokens(stats.total_tokens) }}
                </p>
                <p class="text-xs">
                  <span
                    class="text-indigo-600 dark:text-indigo-400"
                    :title="t('admin.dashboard.actual')"
                    >${{ formatCost(stats.total_actual_cost) }}</span
                  >
                  <span
                    class="text-gray-400 dark:text-gray-500"
                    :title="t('admin.dashboard.standard')"
                  >
                    / ${{ formatCost(stats.total_cost) }}</span
                  >
                </p>
              </div>
            </div>
          </div>

          <!-- Performance (RPM/TPM) -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-violet-100 p-2 dark:bg-violet-900/30">
                <Icon name="bolt" size="md" class="text-violet-600 dark:text-violet-400" :stroke-width="2" />
              </div>
              <div class="flex-1">
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.performance') }}
                </p>
                <div class="flex items-baseline gap-2">
                  <p class="text-xl font-bold text-gray-900 dark:text-white">
                    {{ formatTokens(stats.rpm) }}
                  </p>
                  <span class="text-xs text-gray-500 dark:text-gray-400">RPM</span>
                </div>
                <div class="flex items-baseline gap-2">
                  <p class="text-sm font-semibold text-violet-600 dark:text-violet-400">
                    {{ formatTokens(stats.tpm) }}
                  </p>
                  <span class="text-xs text-gray-500 dark:text-gray-400">TPM</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Avg Response Time -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-rose-100 p-2 dark:bg-rose-900/30">
                <Icon name="clock" size="md" class="text-rose-600 dark:text-rose-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.avgResponse') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ formatDuration(stats.average_duration_ms) }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ stats.active_users }} {{ t('admin.dashboard.activeUsers') }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <div class="card p-5">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.dashboard.recommendations.title') }}
              </h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.dashboard.recommendations.description') }}
              </p>
            </div>
            <div
              v-if="recommendations"
              class="flex flex-wrap items-center justify-end gap-2 text-xs text-gray-500 dark:text-gray-400"
            >
              <span class="rounded-full bg-gray-100 px-3 py-1 dark:bg-dark-700">
                {{
                  t('admin.dashboard.recommendations.poolsAndGroups', {
                    pools: recommendations.summary.pool_count,
                    groups: recommendations.summary.group_count
                  })
                }}
              </span>
              <span class="rounded-full bg-amber-50 px-3 py-1 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
                {{
                  t('admin.dashboard.recommendations.toAddSchedulable', {
                    count: recommendations.summary.recommended_additional_schedulable_accounts
                  })
                }}
              </span>
              <span class="rounded-full bg-blue-50 px-3 py-1 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300">
                {{
                  t('admin.dashboard.recommendations.recoverable', {
                    count: recommendations.summary.recoverable_unschedulable_accounts
                  })
                }}
              </span>
              <span class="rounded-full bg-rose-50 px-3 py-1 text-rose-700 dark:bg-rose-900/20 dark:text-rose-300">
                {{ t('admin.dashboard.recommendations.urgent', { count: recommendations.summary.urgent_pool_count }) }}
              </span>
            </div>
          </div>

          <div v-if="recommendationsLoading" class="flex items-center justify-center py-8">
            <LoadingSpinner size="md" />
          </div>
          <div
            v-else-if="recommendations && recommendations.pools.length > 0"
            class="mt-4 overflow-x-auto"
          >
            <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-gray-700">
              <thead>
                <tr class="text-left text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  <th class="px-3 py-3 font-medium">{{ t('admin.dashboard.recommendations.pool') }}</th>
                  <th class="px-3 py-3 font-medium">{{ t('admin.dashboard.recommendations.status') }}</th>
                  <th class="px-3 py-3 font-medium">{{ t('admin.dashboard.recommendations.current') }}</th>
                  <th class="px-3 py-3 font-medium">{{ t('admin.dashboard.recommendations.recommended') }}</th>
                  <th class="px-3 py-3 font-medium">{{ t('admin.dashboard.recommendations.gap') }}</th>
                  <th class="px-3 py-3 font-medium">{{ t('admin.dashboard.recommendations.utilization') }}</th>
                  <th class="px-3 py-3 font-medium">{{ t('admin.dashboard.recommendations.reason') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-gray-800">
                <tr v-for="item in recommendations.pools.slice(0, 8)" :key="item.pool_key" class="align-top">
                  <td class="px-3 py-3">
                    <div class="font-medium text-gray-900 dark:text-white">{{ item.pool_key }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ item.platform }} · {{ item.recommended_account_type }}
                    </div>
                    <div
                      v-if="item.group_names.length > 0"
                      class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-gray-400"
                    >
                      {{ item.group_names.join(' / ') }}
                    </div>
                    <div
                      v-if="item.plan_names.length > 0"
                      class="mt-1 line-clamp-2 text-xs text-gray-400 dark:text-gray-500"
                    >
                      {{ t('admin.dashboard.recommendations.contributors') }}: {{ item.plan_names.join(' / ') }}
                    </div>
                  </td>
                  <td class="px-3 py-3">
                    <span
                      class="inline-flex rounded-full px-2.5 py-1 text-xs font-medium"
                      :class="recommendationStatusClass(item.status)"
                    >
                      {{ t(`admin.dashboard.recommendations.statusMap.${item.status}`) }}
                    </span>
                  </td>
                  <td class="px-3 py-3 text-gray-700 dark:text-gray-300">
                    {{ item.current_schedulable_accounts }} / {{ item.current_total_accounts }}
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.dashboard.recommendations.subscriptions', { count: item.metrics.active_subscriptions }) }}
                    </div>
                  </td>
                  <td class="px-3 py-3">
                    <div class="font-semibold text-gray-900 dark:text-white">
                      {{ item.recommended_schedulable_accounts }}
                    </div>
                    <div
                      class="mt-1 text-xs"
                      :class="item.recommended_additional_schedulable_accounts > 0 ? 'text-amber-600 dark:text-amber-300' : 'text-gray-500 dark:text-gray-400'"
                    >
                      {{ item.recommended_additional_schedulable_accounts > 0
                        ? t('admin.dashboard.recommendations.addSchedulableCount', { count: item.recommended_additional_schedulable_accounts })
                        : t('admin.dashboard.recommendations.noAction')
                      }}
                    </div>
                  </td>
                  <td class="px-3 py-3 text-gray-700 dark:text-gray-300">
                    <div class="font-semibold text-gray-900 dark:text-white">
                      {{ item.recommended_additional_schedulable_accounts }}
                    </div>
                    <div class="mt-1 text-xs text-blue-600 dark:text-blue-300">
                      {{ t('admin.dashboard.recommendations.recoverableInline', { count: item.recoverable_unschedulable_accounts }) }}
                    </div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.dashboard.recommendations.newAccountsInline', { count: estimatedNewAccounts(item) }) }}
                    </div>
                  </td>
                  <td class="px-3 py-3 text-gray-700 dark:text-gray-300">
                    {{ formatPercent(item.metrics.capacity_utilization) }}
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.dashboard.recommendations.projectedCost', { amount: formatCost(item.metrics.projected_daily_cost) }) }}
                    </div>
                  </td>
                  <td class="max-w-xs px-3 py-3 text-xs leading-5 text-gray-600 dark:text-gray-300">
                    {{ item.reason }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div
            v-else
            class="mt-4 rounded-xl border border-dashed border-gray-200 px-4 py-8 text-center text-sm text-gray-500 dark:border-gray-700 dark:text-gray-400"
          >
            {{ t('admin.dashboard.recommendations.empty') }}
          </div>
        </div>

        <div class="calculator-shell">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="flex items-start gap-3">
              <div class="hidden h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-slate-900 text-cyan-300 shadow-lg shadow-cyan-500/10 dark:bg-cyan-400/10 dark:text-cyan-200 sm:flex">
                <Icon name="calculator" size="md" :stroke-width="2" />
              </div>
              <div>
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.dashboard.oversell.title') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.oversell.description') }}
                </p>
              </div>
            </div>
            <div
              v-if="oversellCalculator?.estimate"
              class="flex max-w-4xl flex-wrap items-center justify-end gap-2 text-xs text-gray-500 dark:text-gray-400"
            >
              <span
                v-if="Number.isFinite(oversellCalculator.estimate.estimated_light_user_ratio)"
                class="calculator-evidence-pill calculator-evidence-pill--strong"
              >
                {{ oversellEstimateSummary }}
              </span>
              <span class="calculator-evidence-pill">
                {{
                  t('admin.dashboard.oversell.costBadge', {
                    cost: formatCost(oversellCalculator.defaults.actual_cost_cny),
                    capacity: formatDecimal(oversellCalculator.defaults.capacity_units_per_product, 1)
                  })
                }}
              </span>
              <span
                v-if="oversellCalculator.generated_at"
                class="calculator-evidence-pill"
              >
                {{ t('admin.dashboard.oversell.updatedAt', { time: formatShortDateTime(oversellCalculator.generated_at) }) }}
              </span>
            </div>
          </div>

          <div v-if="oversellLoading" class="flex items-center justify-center py-8">
            <LoadingSpinner size="md" />
          </div>
          <div v-else-if="oversellCalculator?.estimate" class="mt-5 space-y-5">
            <section class="calculator-results-panel">
              <div class="mb-4 flex flex-wrap items-end justify-between gap-3">
                <div>
                  <h4 class="text-xs font-semibold uppercase tracking-[0.2em] text-slate-500 dark:text-slate-400">
                    {{ t('admin.dashboard.oversell.sections.results') }}
                  </h4>
                  <p class="mt-1 text-sm text-slate-600 dark:text-slate-300">
                    {{ t('admin.dashboard.oversell.result.note') }}
                  </p>
                </div>
              </div>
              <div class="grid grid-cols-1 gap-3 lg:grid-cols-12">
                <div class="calculator-result-card calculator-result-card--hero lg:col-span-5">
                  <span class="calculator-result-card__stripe bg-cyan-400"></span>
                  <div class="flex items-center gap-2">
                    <span class="h-2 w-2 rounded-full bg-cyan-300 shadow-[0_0_18px_rgba(34,211,238,0.75)]"></span>
                    <p class="text-xs font-semibold uppercase tracking-[0.2em] text-cyan-100/80">
                      {{ t('admin.dashboard.oversell.result.recommendedPrice') }}
                    </p>
                  </div>
                  <p data-testid="oversell-recommended-price" class="calculator-result-card__value calculator-result-card__value--hero">
                    {{ oversellScenario ? formatCny(oversellScenario.requiredPrice) : '--' }}
                  </p>
                  <p class="calculator-result-card__meta !text-cyan-50/70">
                    {{ oversellScenario ? t('admin.dashboard.oversell.result.floorPriceHint', { floor: formatCost(oversellScenario.floorPrice) }) : '--' }}
                  </p>
                </div>

                <div class="calculator-result-card calculator-result-card--amber lg:col-span-3">
                  <span class="calculator-result-card__stripe bg-amber-400"></span>
                  <div class="calculator-result-card__label">
                    <span class="h-1.5 w-1.5 rounded-full bg-amber-400"></span>
                    <p>{{ t('admin.dashboard.oversell.result.plannedProfit') }}</p>
                  </div>
                  <p class="calculator-result-card__value">
                    {{ oversellScenario ? formatSignedCny(oversellScenario.plannedMonthlyProfit) : '--' }}
                  </p>
                  <p class="calculator-result-card__meta">
                    {{ oversellScenario ? t('admin.dashboard.oversell.result.revenueHint', { value: formatCost(oversellScenario.plannedMonthlyRevenue) }) : '--' }}
                  </p>
                </div>

                <div class="calculator-result-card calculator-result-card--emerald lg:col-span-2">
                  <span class="calculator-result-card__stripe bg-emerald-400"></span>
                  <div class="calculator-result-card__label">
                    <span class="h-1.5 w-1.5 rounded-full bg-emerald-400"></span>
                    <p>{{ t('admin.dashboard.oversell.result.conservativeCost') }}</p>
                  </div>
                  <p class="calculator-result-card__value">
                    {{ oversellScenario ? formatCny(oversellScenario.conservativeMonthlyCost) : '--' }}
                  </p>
                  <p class="calculator-result-card__meta">
                    {{ oversellScenario ? `${formatDecimal(oversellScenario.riskAdjustedMeanUnits, 3)} ${t('admin.dashboard.oversell.form.units')} / ${t('admin.dashboard.oversell.form.users')}` : '--' }}
                  </p>
                </div>

                <div class="calculator-result-card calculator-result-card--sky lg:col-span-2">
                  <span class="calculator-result-card__stripe bg-sky-400"></span>
                  <div class="calculator-result-card__label">
                    <span class="h-1.5 w-1.5 rounded-full bg-sky-400"></span>
                    <p>{{ t('admin.dashboard.oversell.result.buffer') }}</p>
                  </div>
                  <p
                    class="calculator-result-card__value"
                    :class="oversellScenario && oversellScenario.safetyBuffer < 0 ? 'text-rose-600 dark:text-rose-300' : ''"
                  >
                    {{ oversellScenario ? `${formatSignedDecimal(oversellScenario.safetyBuffer, 3)} ${t('admin.dashboard.oversell.form.units')}` : '--' }}
                  </p>
                  <p class="calculator-result-card__meta">
                    {{ oversellScenario ? t('admin.dashboard.oversell.result.priceGapHint', { gap: formatSignedCny(oversellScenario.priceGap) }) : '--' }}
                  </p>
                </div>
              </div>
            </section>

            <section class="calculator-parameters-panel">
              <h4 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.dashboard.oversell.sections.parameters') }}
              </h4>
              <div class="grid grid-cols-1 gap-3 xl:grid-cols-3">
                <div class="calculator-parameter-group">
                  <h5 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.oversell.sections.cost') }}
                  </h5>
                  <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-1">
                    <div class="calculator-field">
                      <div class="calculator-field__header">
                        <label class="input-label min-w-0 flex-1">{{ t('admin.dashboard.oversell.form.procurementCost') }}</label>
                        <HelpTooltip
                          data-testid="oversell-procurement-cost-help"
                          class="shrink-0"
                          :content="t('admin.dashboard.oversell.tooltips.procurementCost')"
                        >
                          <template #trigger>
                            <span class="inline-flex h-4 w-4 items-center justify-center rounded-full bg-gray-200 text-[10px] font-semibold text-gray-500 transition-colors hover:bg-gray-300 hover:text-gray-700 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500 dark:hover:text-gray-200">?</span>
                          </template>
                        </HelpTooltip>
                      </div>
                      <input
                        v-model.number="oversellForm.procurementCost"
                        type="number"
                        min="0"
                        step="0.01"
                        class="input calculator-field__control"
                      />
                      <p class="calculator-field__hint">{{ t('admin.dashboard.oversell.form.cnyPerItem') }}</p>
                    </div>

                    <div class="calculator-field">
                      <div class="calculator-field__header">
                        <label class="input-label min-w-0 flex-1">{{ t('admin.dashboard.oversell.form.capacity') }}</label>
                        <HelpTooltip
                          data-testid="oversell-capacity-help"
                          class="shrink-0"
                          :content="t('admin.dashboard.oversell.tooltips.capacity')"
                        >
                          <template #trigger>
                            <span class="inline-flex h-4 w-4 items-center justify-center rounded-full bg-gray-200 text-[10px] font-semibold text-gray-500 transition-colors hover:bg-gray-300 hover:text-gray-700 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500 dark:hover:text-gray-200">?</span>
                          </template>
                        </HelpTooltip>
                      </div>
                      <input
                        v-model.number="oversellForm.capacityPerItem"
                        type="number"
                        min="0.1"
                        step="0.1"
                        class="input calculator-field__control"
                      />
                      <p class="calculator-field__hint">{{ t('admin.dashboard.oversell.form.units') }}</p>
                    </div>
                  </div>
                </div>

                <div class="calculator-parameter-group">
                  <h5 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.oversell.sections.users') }}
                  </h5>
                  <div class="grid grid-cols-1 gap-3 sm:grid-cols-3 xl:grid-cols-1">
                    <div class="calculator-field">
                      <div class="calculator-field__header">
                        <label class="input-label min-w-0 flex-1">{{ t('admin.dashboard.oversell.form.userCount') }}</label>
                        <HelpTooltip
                          data-testid="oversell-user-count-help"
                          class="shrink-0"
                          :content="t('admin.dashboard.oversell.tooltips.userCount')"
                        >
                          <template #trigger>
                            <span class="inline-flex h-4 w-4 items-center justify-center rounded-full bg-gray-200 text-[10px] font-semibold text-gray-500 transition-colors hover:bg-gray-300 hover:text-gray-700 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500 dark:hover:text-gray-200">?</span>
                          </template>
                        </HelpTooltip>
                      </div>
                      <input
                        v-model.number="oversellForm.userCount"
                        data-testid="oversell-user-count"
                        type="number"
                        min="1"
                        step="1"
                        class="input calculator-field__control"
                      />
                      <p class="calculator-field__hint">{{ t('admin.dashboard.oversell.form.users') }}</p>
                    </div>

                    <div class="calculator-field">
                      <div class="calculator-field__header">
                        <label class="input-label min-w-0 flex-1">{{ t('admin.dashboard.oversell.form.plannedPrice') }}</label>
                        <HelpTooltip
                          data-testid="oversell-planned-price-help"
                          class="shrink-0"
                          :content="t('admin.dashboard.oversell.tooltips.plannedPrice')"
                        >
                          <template #trigger>
                            <span class="inline-flex h-4 w-4 items-center justify-center rounded-full bg-gray-200 text-[10px] font-semibold text-gray-500 transition-colors hover:bg-gray-300 hover:text-gray-700 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500 dark:hover:text-gray-200">?</span>
                          </template>
                        </HelpTooltip>
                      </div>
                      <input
                        v-model.number="oversellForm.plannedPrice"
                        data-testid="oversell-planned-price"
                        type="number"
                        min="0"
                        step="0.01"
                        class="input calculator-field__control"
                      />
                      <p class="calculator-field__hint">{{ t('admin.dashboard.oversell.form.cnyPerMonth') }}</p>
                    </div>

                    <div class="calculator-field">
                      <div class="calculator-field__header">
                        <label class="input-label min-w-0 flex-1">{{ t('admin.dashboard.oversell.form.heavyUsage') }}</label>
                        <HelpTooltip
                          data-testid="oversell-heavy-usage-help"
                          class="shrink-0"
                          :content="t('admin.dashboard.oversell.tooltips.heavyUsage')"
                        >
                          <template #trigger>
                            <span class="inline-flex h-4 w-4 items-center justify-center rounded-full bg-gray-200 text-[10px] font-semibold text-gray-500 transition-colors hover:bg-gray-300 hover:text-gray-700 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500 dark:hover:text-gray-200">?</span>
                          </template>
                        </HelpTooltip>
                      </div>
                      <input
                        v-model.number="oversellForm.heavyUsage"
                        type="number"
                        min="0.1"
                        step="0.1"
                        class="input calculator-field__control"
                      />
                      <p class="calculator-field__hint">{{ t('admin.dashboard.oversell.form.units') }}</p>
                    </div>
                  </div>
                </div>

                <div class="calculator-parameter-group">
                  <h5 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.oversell.sections.profitRisk') }}
                  </h5>
                  <div class="grid grid-cols-1 gap-3 sm:grid-cols-3 xl:grid-cols-1">
                    <div class="calculator-field">
                      <div class="calculator-field__header">
                        <label class="input-label min-w-0 flex-1">{{ t('admin.dashboard.oversell.form.profitRate') }}</label>
                        <HelpTooltip
                          data-testid="oversell-profit-rate-help"
                          class="shrink-0"
                          :content="t('admin.dashboard.oversell.tooltips.profitRate')"
                        >
                          <template #trigger>
                            <span class="inline-flex h-4 w-4 items-center justify-center rounded-full bg-gray-200 text-[10px] font-semibold text-gray-500 transition-colors hover:bg-gray-300 hover:text-gray-700 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500 dark:hover:text-gray-200">?</span>
                          </template>
                        </HelpTooltip>
                      </div>
                      <input
                        v-model.number="oversellForm.profitRatePercent"
                        type="number"
                        min="0"
                        max="95"
                        step="1"
                        class="input calculator-field__control"
                      />
                      <p class="calculator-field__hint">{{ t('admin.dashboard.oversell.form.percent') }}</p>
                    </div>

                    <div class="calculator-field">
                      <div class="calculator-field__header">
                        <label class="input-label min-w-0 flex-1">{{ t('admin.dashboard.oversell.form.profitMode') }}</label>
                        <HelpTooltip
                          data-testid="oversell-profit-mode-help"
                          class="shrink-0"
                          :content="t('admin.dashboard.oversell.tooltips.profitMode')"
                        >
                          <template #trigger>
                            <span class="inline-flex h-4 w-4 items-center justify-center rounded-full bg-gray-200 text-[10px] font-semibold text-gray-500 transition-colors hover:bg-gray-300 hover:text-gray-700 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500 dark:hover:text-gray-200">?</span>
                          </template>
                        </HelpTooltip>
                      </div>
                      <select v-model="oversellForm.profitMode" class="input calculator-field__control">
                        <option value="costPlus">{{ t('admin.dashboard.oversell.form.costPlus') }}</option>
                        <option value="netMargin">{{ t('admin.dashboard.oversell.form.netMargin') }}</option>
                      </select>
                      <p class="calculator-field__hint"></p>
                    </div>

                    <div class="calculator-field">
                      <div class="calculator-field__header">
                        <label class="input-label min-w-0 flex-1">{{ t('admin.dashboard.oversell.form.confidence') }}</label>
                        <HelpTooltip
                          data-testid="oversell-confidence-help"
                          class="shrink-0"
                          :content="t('admin.dashboard.oversell.tooltips.confidence')"
                        >
                          <template #trigger>
                            <span class="inline-flex h-4 w-4 items-center justify-center rounded-full bg-gray-200 text-[10px] font-semibold text-gray-500 transition-colors hover:bg-gray-300 hover:text-gray-700 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500 dark:hover:text-gray-200">?</span>
                          </template>
                        </HelpTooltip>
                      </div>
                      <select v-model.number="oversellForm.confidenceLevel" class="input calculator-field__control">
                        <option :value="95">{{ t('admin.dashboard.oversell.form.confidence95') }}</option>
                        <option :value="99">{{ t('admin.dashboard.oversell.form.confidence99') }}</option>
                      </select>
                      <p class="calculator-field__hint"></p>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <section v-if="oversellPlanRecommendations.length > 0">
              <h4 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.dashboard.oversell.table.title') }}
              </h4>
              <div class="calculator-table-wrap">
                <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
                  <thead class="bg-gray-50/80 dark:bg-dark-700/50">
                    <tr class="text-left text-xs uppercase tracking-[0.16em] text-gray-500 dark:text-gray-400">
                      <th class="px-4 py-3 font-medium">{{ t('admin.dashboard.oversell.table.plan') }}</th>
                      <th class="px-4 py-3 font-medium">{{ t('admin.dashboard.oversell.table.basis') }}</th>
                      <th class="px-4 py-3 font-medium">{{ t('admin.dashboard.oversell.table.duration') }}</th>
                      <th class="px-4 py-3 font-medium">{{ t('admin.dashboard.oversell.table.currentMonthlyEquivalent') }}</th>
                      <th class="px-4 py-3 font-medium">{{ t('admin.dashboard.oversell.table.currentPrice') }}</th>
                      <th class="px-4 py-3 font-medium">{{ t('admin.dashboard.oversell.table.recommendedPrice') }}</th>
                      <th class="px-4 py-3 font-medium">{{ t('admin.dashboard.oversell.table.delta') }}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                    <tr
                      v-for="plan in oversellPlanRecommendations"
                      :key="plan.plan_id"
                      class="transition-colors hover:bg-gray-50 dark:hover:bg-dark-800/30"
                    >
                      <td class="px-4 py-3">
                        <div class="font-medium text-gray-900 dark:text-white">{{ plan.plan_name }}</div>
                        <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ plan.group_name }}</div>
                      </td>
                      <td class="px-4 py-3 text-xs text-gray-500 dark:text-gray-400">
                        <div class="font-medium text-gray-700 dark:text-gray-300">{{ formatOversellPricingBasis(plan.pricing_basis) }}</div>
                        <div class="mt-1">
                          {{
                            t('admin.dashboard.oversell.table.basisDetail', {
                              quota: formatCost(plan.monthly_quota_usd),
                              units: formatDecimal(plan.effective_capacity_units, 2),
                              ratio: formatDecimal(plan.capacity_ratio, 2)
                            })
                          }}
                        </div>
                      </td>
                      <td class="px-4 py-3 text-gray-700 dark:text-gray-300">
                        {{ plan.validity_days }}{{ plan.validity_unit === 'day' ? '天' : plan.validity_unit }}
                      </td>
                      <td class="px-4 py-3 text-gray-700 dark:text-gray-300">
                        {{ formatCny(plan.current_monthly_price_cny) }}
                      </td>
                      <td class="px-4 py-3 text-gray-700 dark:text-gray-300">
                        {{ formatCny(plan.current_price_cny) }}
                      </td>
                      <td
                        class="px-4 py-3 font-semibold text-gray-900 dark:text-white"
                        data-testid="oversell-plan-recommended-price"
                      >
                        {{ formatCny(plan.recommended_price_cny) }}
                      </td>
                      <td
                        class="px-4 py-3 font-semibold"
                        :class="plan.price_delta_cny >= 0 ? 'text-rose-600 dark:text-rose-300' : 'text-emerald-600 dark:text-emerald-300'"
                      >
                        {{ formatSignedCny(plan.price_delta_cny) }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>

          </div>
          <div
            v-else
            class="mt-5 rounded-xl border border-dashed border-gray-200 px-4 py-8 text-center text-sm text-gray-500 dark:border-gray-700 dark:text-gray-400"
          >
            {{ t('admin.dashboard.oversell.noEstimate') }}
          </div>
        </div>

        <!-- Charts Section -->
        <div class="space-y-6">
          <!-- Date Range Filter -->
          <div class="card p-4">
            <div class="flex flex-wrap items-center gap-4">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.timeRange') }}:</span
                >
                <DateRangePicker
                  v-model:start-date="startDate"
                  v-model:end-date="endDate"
                  @change="onDateRangeChange"
                />
              </div>
              <button @click="loadDashboardStats" :disabled="chartsLoading" class="btn btn-secondary">
                {{ t('common.refresh') }}
              </button>
              <div class="ml-auto flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.granularity') }}:</span
                >
                <div class="w-28">
                  <Select
                    v-model="granularity"
                    :options="granularityOptions"
                    @change="loadChartData"
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- Charts Grid -->
          <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <ModelDistributionChart
              :model-stats="modelStats"
              :enable-ranking-view="true"
              :ranking-items="rankingItems"
              :ranking-total-actual-cost="rankingTotalActualCost"
              :ranking-total-requests="rankingTotalRequests"
              :ranking-total-tokens="rankingTotalTokens"
              :loading="chartsLoading"
              :ranking-loading="rankingLoading"
              :ranking-error="rankingError"
              :start-date="startDate"
              :end-date="endDate"
              @ranking-click="goToUserUsage"
            />
            <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
          </div>

          <div class="card p-4">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.dashboard.profitability.title') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.profitability.description') }}
                </p>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.timeRange') }}:
                </span>
                <DateRangePicker
                  v-model:start-date="profitabilityStartDate"
                  v-model:end-date="profitabilityEndDate"
                  :default-preset="profitabilityDefaultPreset"
                  :enable-all-time="Boolean(profitabilityAllTimeStartDate)"
                  :all-time-start-date="profitabilityAllTimeStartDate"
                  @change="onProfitabilityRangeChange"
                />
              </div>
              <div class="grid min-w-full grid-cols-2 gap-2 text-xs sm:min-w-0 sm:grid-cols-5">
                <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
                  <div class="text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.profitability.balanceRevenue') }}
                  </div>
                  <div class="mt-1 font-semibold text-gray-900 dark:text-white">
                    {{ formatCny(profitabilitySummary.revenueBalanceCNY) }}
                  </div>
                </div>
                <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
                  <div class="text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.profitability.subscriptionRevenue') }}
                  </div>
                  <div class="mt-1 font-semibold text-gray-900 dark:text-white">
                    {{ formatCny(profitabilitySummary.revenueSubscriptionCNY) }}
                  </div>
                </div>
                <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
                  <div class="text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.profitability.estimatedCost') }}
                  </div>
                  <div class="mt-1 font-semibold text-gray-900 dark:text-white">
                    {{ formatCny(profitabilitySummary.estimatedCostCNY) }}
                  </div>
                </div>
                <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
                  <div class="text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.profitability.profit') }}
                  </div>
                  <div
                    class="mt-1 font-semibold"
                    :class="profitabilitySummary.profitCNY >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'"
                  >
                    {{ formatSignedCny(profitabilitySummary.profitCNY) }}
                  </div>
                </div>
                <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
                  <div class="text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.profitability.extraProfitRate') }}
                  </div>
                  <div class="mt-1 font-semibold text-blue-600 dark:text-blue-400">
                    {{ formatExtraProfitRate(profitabilitySummary.extraProfitRatePercent) }}
                  </div>
                  <div
                    v-if="profitabilitySummary.extraProfitRatePercent == null"
                    class="mt-1 text-[11px] text-gray-500 dark:text-gray-400"
                  >
                    {{ t('admin.dashboard.profitability.extraProfitRateUnavailableHint') }}
                  </div>
                </div>
              </div>
            </div>

            <div class="mt-4 h-72">
              <ProfitabilityTrendChart
                :trend-data="profitabilityTrend"
                :loading="profitabilityLoading"
                :granularity="profitabilityGranularity"
                :start-date="profitabilityStartDate"
                :end-date="profitabilityEndDate"
              />
            </div>
          </div>

          <!-- User Usage Trend (Full Width) -->
          <div class="card p-4">
            <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.dashboard.recentUsage') }} (Top 12)
            </h3>
            <div class="h-64">
              <div v-if="userTrendLoading" class="flex h-full items-center justify-center">
                <LoadingSpinner size="md" />
              </div>
              <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
              <div
                v-else
                class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
import { adminAPI } from '@/api/admin'
import type {
  DashboardStats,
  DashboardOversellCalculatorResponse,
  DashboardRecommendationsResponse,
  TrendDataPoint,
  ProfitabilityTrendPoint,
  ModelStat,
  UserUsageTrendPoint,
  UserSpendingRankingItem
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import ProfitabilityTrendChart from '@/components/charts/ProfitabilityTrendChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import {
  summarizeProfitabilityTrend
} from './dashboardProfitability'

import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'

// Register Chart.js components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
)

const appStore = useAppStore()
const router = useRouter()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartsLoading = ref(false)
const recommendationsLoading = ref(false)
const userTrendLoading = ref(false)
const profitabilityLoading = ref(false)
const rankingLoading = ref(false)
const rankingError = ref(false)
const recommendations = ref<DashboardRecommendationsResponse | null>(null)
const oversellLoading = ref(false)
const oversellCalculator = ref<DashboardOversellCalculatorResponse | null>(null)

type OversellProfitMode = 'costPlus' | 'netMargin'

const oversellForm = reactive({
  userCount: 30,
  plannedPrice: 50,
  procurementCost: 50,
  capacityPerItem: 3,
  profitRatePercent: 20,
  profitMode: 'costPlus' as OversellProfitMode,
  heavyUsage: 3,
  confidenceLevel: 99 as 95 | 99
})


// Chart data
const trendData = ref<TrendDataPoint[]>([])
const profitabilityTrend = ref<ProfitabilityTrendPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const userTrend = ref<UserUsageTrendPoint[]>([])
const rankingItems = ref<UserSpendingRankingItem[]>([])
const rankingTotalActualCost = ref(0)
const rankingTotalRequests = ref(0)
const rankingTotalTokens = ref(0)
let chartLoadSeq = 0
let usersTrendLoadSeq = 0
let profitabilityLoadSeq = 0
let rankingLoadSeq = 0
const rankingLimit = 12

// Helper function to format date in local timezone
const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const profitabilityStartDate = ref(formatLocalDate(new Date()))
const profitabilityEndDate = ref(formatLocalDate(new Date()))
const profitabilityAllTimeStartDate = ref<string | null>(null)
const profitabilityGranularity = ref<'day' | 'hour'>('day')
const profitabilityBoundsLoaded = ref(false)

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end)
  }
}

// Date range
const granularity = ref<'day' | 'hour'>('hour')
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const profitabilityDefaultPreset = computed(() =>
  profitabilityAllTimeStartDate.value ? 'allTime' : 'last24Hours'
)

// Granularity options for Select component
const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') }
])

// Dark mode detection
const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

// Chart colors
const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

// Line chart options (for user trend chart)
const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      itemSort: (a: any, b: any) => {
        const aValue = typeof a?.raw === 'number' ? a.raw : Number(a?.parsed?.y ?? 0)
        const bValue = typeof b?.raw === 'number' ? b.raw : Number(b?.parsed?.y ?? 0)
        return bValue - aValue
      },
      callbacks: {
        label: (context: any) => {
          return `${context.dataset.label}: ${formatTokens(context.raw)}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    }
  }
}))

// User trend chart data
const userTrendChartData = computed(() => {
  if (!userTrend.value?.length) return null

  const getDisplayName = (point: UserUsageTrendPoint): string => {
    const username = point.username?.trim()
    if (username) {
      return username
    }

    const email = point.email?.trim()
    if (email) {
      return email
    }

    return t('admin.redeem.userPrefix', { id: point.user_id })
  }

  // Group by user_id to avoid merging different users with the same display name
  const userGroups = new Map<number, { name: string; data: Map<string, number> }>()
  const allDates = new Set<string>()

  userTrend.value.forEach((point) => {
    allDates.add(point.date)
    const key = point.user_id
    if (!userGroups.has(key)) {
      userGroups.set(key, { name: getDisplayName(point), data: new Map() })
    }
    userGroups.get(key)!.data.set(point.date, point.tokens)
  })

  const sortedDates = Array.from(allDates).sort()
  const colors = [
    '#3b82f6',
    '#10b981',
    '#f59e0b',
    '#ef4444',
    '#8b5cf6',
    '#ec4899',
    '#14b8a6',
    '#f97316',
    '#6366f1',
    '#84cc16',
    '#06b6d4',
    '#a855f7'
  ]

  const datasets = Array.from(userGroups.values()).map((group, idx) => ({
    label: group.name,
    data: sortedDates.map((date) => group.data.get(date) || 0),
    borderColor: colors[idx % colors.length],
    backgroundColor: `${colors[idx % colors.length]}20`,
    fill: false,
    tension: 0.3
  }))

  return {
    labels: sortedDates,
    datasets
  }
})

const profitabilitySummary = computed(() => summarizeProfitabilityTrend(profitabilityTrend.value))
const oversellEstimateSummary = computed(() => {
  const estimate = oversellCalculator.value?.estimate
  if (!estimate || !Number.isFinite(estimate.estimated_light_user_ratio)) {
    return ''
  }

  return t('admin.dashboard.oversell.estimateDescription', {
    days: 30,
    share: formatPercentDetailed(estimate.estimated_light_user_ratio, 0),
    threshold: formatDecimal(estimate.light_user_threshold_units, 2)
  })
})

const resolveLossRisk = (confidenceLevel: 95 | 99): number => {
  return confidenceLevel === 99 ? 0.01 : 0.05
}

const resolveProfitRate = (percent: number): number => {
  return Math.min(Math.max(percent / 100, 0), 0.95)
}

const resolvePriceMultiplier = (
  profitMode: OversellProfitMode,
  profitRate: number
): number => {
  return profitMode === 'netMargin'
    ? 1 / Math.max(1 - profitRate, 0.05)
    : 1 + profitRate
}

const resolveUsageDistribution = (
  estimate: DashboardOversellCalculatorResponse['estimate'],
  heavyUsageInput: number
) => {
  const lightUserShare = Math.min(Math.max(estimate.estimated_light_user_ratio, 0), 1)
  const lightUserThreshold = Math.max(estimate.light_user_threshold_units, 0)
  const heavyUsage = Math.max(heavyUsageInput || 0, lightUserThreshold, 0.0001)
  const meanUpperBound = lightUserShare * lightUserThreshold + (1 - lightUserShare) * heavyUsage

  return {
    lightUserShare,
    lightUserThreshold,
    heavyUsage,
    meanUpperBound
  }
}

const oversellScenario = computed(() => {
  const estimate = oversellCalculator.value?.estimate
  if (!estimate || !Number.isFinite(estimate.estimated_light_user_ratio)) {
    return null
  }

  const userCount = Math.max(Math.round(oversellForm.userCount || 0), 1)
  const distribution = resolveUsageDistribution(estimate, oversellForm.heavyUsage)
  const rangeWidth = distribution.heavyUsage
  const procurementCost = Math.max(oversellForm.procurementCost || 0, 0)
  const capacityPerItem = Math.max(oversellForm.capacityPerItem || 0, 0.0001)
  const plannedPrice = Math.max(oversellForm.plannedPrice || 0, 0)
  const profitRate = resolveProfitRate(oversellForm.profitRatePercent || 0)
  const priceMultiplier = resolvePriceMultiplier(oversellForm.profitMode, profitRate)
  const lossRisk = resolveLossRisk(oversellForm.confidenceLevel)
  const unitCostPerTheoretical = procurementCost / capacityPerItem
  const riskBufferUnits = rangeWidth * Math.sqrt(Math.log(1 / lossRisk) / (2 * userCount))
  const riskAdjustedMeanUnits = distribution.meanUpperBound + riskBufferUnits
  const expectedCostPerUser = unitCostPerTheoretical * distribution.meanUpperBound
  const riskAdjustedCostPerUser = unitCostPerTheoretical * riskAdjustedMeanUnits
  const conservativeMonthlyCost = riskAdjustedCostPerUser * userCount
  const floorPrice = riskAdjustedCostPerUser * priceMultiplier
  const requiredPrice = floorPrice
  const plannedMonthlyRevenue = plannedPrice * userCount
  const plannedMonthlyProfit = plannedMonthlyRevenue - conservativeMonthlyCost
  const affordableUsageThreshold =
    oversellForm.profitMode === 'netMargin'
      ? (plannedPrice * Math.max(1 - profitRate, 0.0001)) / Math.max(unitCostPerTheoretical, 0.0001)
      : plannedPrice / Math.max(unitCostPerTheoretical * priceMultiplier, 0.0001)
  const safetyBuffer = affordableUsageThreshold - riskAdjustedMeanUnits
  const priceGap = plannedPrice - requiredPrice

  return {
    userCount,
    meanUpperBound: distribution.meanUpperBound,
    riskAdjustedMeanUnits,
    riskBufferUnits,
    unitCostPerTheoretical,
    expectedCostPerUser,
    riskAdjustedCostPerUser,
    conservativeMonthlyCost,
    floorPrice,
    requiredPrice,
    plannedMonthlyRevenue,
    plannedMonthlyProfit,
    affordableUsageThreshold,
    safetyBuffer,
    priceGap,
    lossRiskLabel: formatPercentDetailed(lossRisk, 0)
  }
})

const oversellPlanRecommendations = computed(() => {
  const plans = oversellCalculator.value?.plans ?? []
  const requiredMonthlyPrice = oversellScenario.value?.requiredPrice ?? 0

  return plans.map((plan) => {
    const capacityRatio = plan.capacity_ratio > 0 ? plan.capacity_ratio : 1
    const recommendedMonthlyPrice =
      requiredMonthlyPrice > 0
        ? requiredMonthlyPrice * capacityRatio
        : plan.recommended_monthly_price_cny ?? 0
    const derivedRecommendedPrice =
      recommendedMonthlyPrice > 0
        ? (recommendedMonthlyPrice * plan.duration_days_equivalent) / 30
        : plan.recommended_price_cny ?? 0

    return {
      ...plan,
      monthly_quota_usd: plan.monthly_quota_usd ?? 0,
      effective_capacity_units: plan.effective_capacity_units ?? 0,
      capacity_ratio: capacityRatio,
      pricing_basis: plan.pricing_basis ?? '',
      current_monthly_price_cny:
        plan.current_monthly_price_cny ??
        (plan.duration_days_equivalent > 0
          ? (plan.current_price_cny * 30) / plan.duration_days_equivalent
          : plan.current_price_cny),
      recommended_monthly_price_cny: recommendedMonthlyPrice,
      recommended_price_cny: derivedRecommendedPrice,
      price_delta_cny: derivedRecommendedPrice - plan.current_price_cny
    }
  })
})


// Format helpers
const formatTokens = (value: number | undefined): string => {
  if (value === undefined || value === null) return '0'
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatOversellPricingBasis = (basis: string | undefined): string => {
  switch (basis) {
    case 'monthly_limit_usd':
      return t('admin.dashboard.oversell.table.basisMonthly')
    case 'weekly_limit_usd':
      return t('admin.dashboard.oversell.table.basisWeekly')
    case 'daily_limit_usd':
      return t('admin.dashboard.oversell.table.basisDaily')
    case 'duration_only':
      return t('admin.dashboard.oversell.table.basisDurationOnly')
    default:
      return basis || '--'
  }
}

const formatNumber = (value: number): string => {
  return value.toLocaleString()
}

const formatCost = (value: number): string => {
  if (value >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  } else if (value >= 1) {
    return value.toFixed(2)
  } else if (value >= 0.01) {
    return value.toFixed(3)
  }
  return value.toFixed(4)
}

const formatCny = (value: number): string => `¥${formatCost(value)}`

const formatSignedCny = (value: number): string => `${value >= 0 ? '+' : '-'}${formatCny(Math.abs(value))}`

const formatExtraProfitRate = (value: number | null | undefined): string => {
  if (value == null || Number.isNaN(value)) {
    return '--'
  }
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
}

const formatDecimal = (value: number, digits = 2): string => {
  if (!Number.isFinite(value)) {
    return '--'
  }

  return value.toFixed(digits).replace(/\.0+$/, '').replace(/(\.\d*[1-9])0+$/, '$1')
}

const formatSignedDecimal = (value: number, digits = 2): string => {
  if (!Number.isFinite(value)) {
    return '--'
  }

  return `${value >= 0 ? '+' : ''}${formatDecimal(value, digits)}`
}

const formatPercentDetailed = (value: number, digits = 1): string => {
  if (!Number.isFinite(value)) {
    return '--'
  }

  return `${formatDecimal(value * 100, digits)}%`
}

const formatShortDateTime = (value: string): string => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString()
}

const formatDuration = (ms: number): string => {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

const formatPercent = (value: number): string => `${Math.round(value * 100)}%`

const estimatedNewAccounts = (
  item: DashboardRecommendationsResponse['pools'][number]
): number => {
  return Math.max(
    item.recommended_additional_schedulable_accounts - item.recoverable_unschedulable_accounts,
    0
  )
}

const recommendationStatusClass = (status: 'healthy' | 'watch' | 'action') => {
  if (status === 'action') {
    return 'bg-rose-50 text-rose-700 dark:bg-rose-900/20 dark:text-rose-300'
  }
  if (status === 'watch') {
    return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
  }
  return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
}

const goToUserUsage = (item: UserSpendingRankingItem) => {
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(item.user_id),
      start_date: startDate.value,
      end_date: endDate.value
    }
  })
}

const resolveGranularityForRange = (start: string, end: string): 'day' | 'hour' => {
  const startDateValue = new Date(start)
  const endDateValue = new Date(end)
  const daysDiff = Math.ceil((endDateValue.getTime() - startDateValue.getTime()) / (1000 * 60 * 60 * 24))
  return daysDiff <= 1 ? 'hour' : 'day'
}

// Date range change handler
const onDateRangeChange = (range: {
  startDate: string
  endDate: string
  preset: string | null
}) => {
  granularity.value = resolveGranularityForRange(range.startDate, range.endDate)

  loadChartData()
}

const onProfitabilityRangeChange = (range: {
  startDate: string
  endDate: string
  preset: string | null
}) => {
  profitabilityGranularity.value = resolveGranularityForRange(range.startDate, range.endDate)
  loadProfitabilityTrend()
}

// Load data
const loadDashboardSnapshot = async (includeStats: boolean) => {
  const currentSeq = ++chartLoadSeq
  if (includeStats && !stats.value) {
    loading.value = true
  }
  chartsLoading.value = true
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      include_stats: includeStats,
      include_trend: true,
      include_model_stats: true,
      include_group_stats: false,
      include_users_trend: false
    })
    if (currentSeq !== chartLoadSeq) return
    if (includeStats && response.stats) {
      stats.value = response.stats
    }
    trendData.value = response.trend || []
    modelStats.value = response.models || []
  } catch (error) {
    if (currentSeq !== chartLoadSeq) return
    appStore.showError(t('admin.dashboard.failedToLoad'))
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (currentSeq === chartLoadSeq) {
      loading.value = false
      chartsLoading.value = false
    }
  }
}

const loadUsersTrend = async () => {
  const currentSeq = ++usersTrendLoadSeq
  userTrendLoading.value = true
  try {
    const response = await adminAPI.dashboard.getUserUsageTrend({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      limit: 12
    })
    if (currentSeq !== usersTrendLoadSeq) return
    userTrend.value = response.trend || []
  } catch (error) {
    if (currentSeq !== usersTrendLoadSeq) return
    console.error('Error loading users trend:', error)
    userTrend.value = []
  } finally {
    if (currentSeq === usersTrendLoadSeq) {
      userTrendLoading.value = false
    }
  }
}

const loadProfitabilityBounds = async () => {
  try {
    const bounds = await adminAPI.dashboard.getProfitabilityBounds()
    const today = formatLocalDate(new Date())

    if (bounds.has_data && bounds.earliest_date) {
      profitabilityAllTimeStartDate.value = bounds.earliest_date
      profitabilityStartDate.value = bounds.earliest_date
      profitabilityEndDate.value = today
      profitabilityGranularity.value = resolveGranularityForRange(bounds.earliest_date, today)
      profitabilityBoundsLoaded.value = true
      return
    }

    profitabilityAllTimeStartDate.value = null
    profitabilityStartDate.value = today
    profitabilityEndDate.value = today
    profitabilityGranularity.value = 'hour'
    profitabilityBoundsLoaded.value = true
  } catch (error) {
    console.error('Error loading profitability bounds:', error)
    const today = formatLocalDate(new Date())
    profitabilityAllTimeStartDate.value = null
    profitabilityStartDate.value = today
    profitabilityEndDate.value = today
    profitabilityGranularity.value = 'hour'
    profitabilityBoundsLoaded.value = true
  }
}

const loadProfitabilityTrend = async () => {
  const currentSeq = ++profitabilityLoadSeq
  profitabilityLoading.value = true
  try {
    const response = await adminAPI.dashboard.getProfitabilityTrend({
      start_date: profitabilityStartDate.value,
      end_date: profitabilityEndDate.value,
      granularity: profitabilityGranularity.value
    })
    if (currentSeq !== profitabilityLoadSeq) return
    profitabilityTrend.value = response.trend || []
  } catch (error) {
    if (currentSeq !== profitabilityLoadSeq) return
    console.error('Error loading profitability trend:', error)
    profitabilityTrend.value = []
  } finally {
    if (currentSeq === profitabilityLoadSeq) {
      profitabilityLoading.value = false
    }
  }
}

const loadUserSpendingRanking = async () => {
  const currentSeq = ++rankingLoadSeq
  rankingLoading.value = true
  rankingError.value = false
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({
      start_date: startDate.value,
      end_date: endDate.value,
      limit: rankingLimit
    })
    if (currentSeq !== rankingLoadSeq) return
    rankingItems.value = response.ranking || []
    rankingTotalActualCost.value = response.total_actual_cost || 0
    rankingTotalRequests.value = response.total_requests || 0
    rankingTotalTokens.value = response.total_tokens || 0
  } catch (error) {
    if (currentSeq !== rankingLoadSeq) return
    console.error('Error loading user spending ranking:', error)
    rankingItems.value = []
    rankingTotalActualCost.value = 0
    rankingTotalRequests.value = 0
    rankingTotalTokens.value = 0
    rankingError.value = true
  } finally {
    if (currentSeq === rankingLoadSeq) {
      rankingLoading.value = false
    }
  }
}

const loadRecommendations = async () => {
  recommendationsLoading.value = true
  try {
    recommendations.value = await adminAPI.dashboard.getRecommendations()
  } catch (error) {
    console.error('Error loading dashboard recommendations:', error)
    recommendations.value = null
  } finally {
    recommendationsLoading.value = false
  }
}

const loadOversellMathBaseline = async () => {
  oversellLoading.value = true
  try {
    const response = await adminAPI.dashboard.getOversellCalculator()
    oversellCalculator.value = response

    const initialInput = response.input || response.defaults
    oversellForm.procurementCost = initialInput.actual_cost_cny
    oversellForm.capacityPerItem = initialInput.capacity_units_per_product
    oversellForm.heavyUsage = initialInput.capacity_units_per_product
    oversellForm.userCount = Math.max(response.estimate.sampled_subscription_count || 0, 1)
    oversellForm.profitRatePercent = initialInput.profit_rate_percent
    oversellForm.profitMode = initialInput.profit_mode === 'net_margin' ? 'netMargin' : 'costPlus'
    oversellForm.confidenceLevel = initialInput.confidence_level >= 0.99 ? 99 : 95
    oversellForm.plannedPrice = response.estimate.current_cheapest_monthly_price_cny || oversellForm.plannedPrice
  } catch (error) {
    console.error('Error loading oversell math baseline:', error)
    oversellCalculator.value = null
  } finally {
    oversellLoading.value = false
  }
}

const loadDashboardStats = async () => {
  if (!profitabilityBoundsLoaded.value) {
    await loadProfitabilityBounds()
  }
  await Promise.all([
    loadDashboardSnapshot(true),
    loadRecommendations(),
    loadOversellMathBaseline(),
    loadProfitabilityTrend(),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

const loadChartData = async () => {
  await Promise.all([
    loadDashboardSnapshot(false),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

onMounted(() => {
  loadDashboardStats()
})
</script>

<style scoped>
.calculator-form-grid {
  @apply grid grid-cols-1 items-stretch gap-3 sm:grid-cols-2 lg:grid-cols-4;
}

.calculator-shell {
  @apply relative overflow-hidden rounded-3xl border border-slate-200/80 bg-slate-50/90 p-5 shadow-sm;
  @apply dark:border-dark-700 dark:bg-dark-900/40;
}

.calculator-shell::before {
  content: '';
  @apply pointer-events-none absolute inset-x-0 top-0 h-32 opacity-80;
  background:
    radial-gradient(circle at 12% 0%, rgba(34, 211, 238, 0.16), transparent 28%),
    radial-gradient(circle at 78% 12%, rgba(16, 185, 129, 0.12), transparent 30%),
    linear-gradient(135deg, rgba(15, 23, 42, 0.05), transparent 42%);
}

.calculator-shell > * {
  @apply relative;
}

.calculator-evidence-pill {
  @apply rounded-full border border-slate-200/80 bg-white/80 px-3 py-1 text-slate-600 shadow-sm backdrop-blur dark:border-dark-700 dark:bg-dark-800/70 dark:text-slate-300;
}

.calculator-evidence-pill--strong {
  @apply border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-700/60 dark:bg-emerald-900/20 dark:text-emerald-300;
}

.calculator-results-panel {
  @apply rounded-3xl border border-slate-200/80 bg-white/90 p-4 shadow-sm ring-1 ring-white/70;
  @apply dark:border-dark-700 dark:bg-dark-800/50 dark:ring-dark-700/60;
}

.calculator-parameters-panel {
  @apply rounded-3xl border border-slate-200/80 bg-white/70 p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800/30;
}

.calculator-parameter-group {
  @apply rounded-2xl border border-slate-200/80 bg-slate-50/80 p-4 shadow-inner shadow-white/60;
  @apply dark:border-dark-700 dark:bg-dark-700/30 dark:shadow-none;
}

.calculator-field {
  @apply flex h-full min-h-[96px] flex-col rounded-2xl border border-slate-200/80 bg-white/95 p-3 shadow-sm ring-1 ring-white/70 transition-all;
  @apply focus-within:border-cyan-300 focus-within:shadow-md focus-within:shadow-cyan-500/10 dark:border-dark-700 dark:bg-dark-800/60 dark:ring-dark-700/80 dark:focus-within:border-cyan-700;
}

.calculator-field__header {
  @apply flex min-h-[1.75rem] items-center justify-between gap-2;
}

.calculator-field__control {
  @apply mt-2 w-full border-slate-200 bg-slate-50/70 font-medium text-slate-900 shadow-inner dark:border-dark-600 dark:bg-dark-900/40 dark:text-white;
}

.calculator-field__hint {
  @apply mt-auto min-h-[1.25rem] pt-2 text-xs leading-4 text-gray-500 dark:text-gray-400;
}

.calculator-result-card {
  @apply relative flex h-full min-h-[148px] flex-col overflow-hidden rounded-2xl border border-slate-200 bg-white p-4 shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-lg;
  @apply dark:border-dark-700 dark:bg-dark-800/60;
}

.calculator-result-card::after {
  content: '';
  @apply pointer-events-none absolute inset-x-0 top-0 h-20 opacity-70;
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.04), transparent);
}

.calculator-result-card--hero {
  @apply min-h-[176px] border-slate-800 bg-slate-950 text-white shadow-xl shadow-slate-900/15 dark:border-cyan-900/70;
  background:
    radial-gradient(circle at 82% 8%, rgba(34, 211, 238, 0.24), transparent 30%),
    radial-gradient(circle at 12% 92%, rgba(16, 185, 129, 0.18), transparent 26%),
    #020617;
}

.calculator-result-card--amber {
  @apply border-amber-200/80 bg-amber-50/70 dark:border-amber-900/50 dark:bg-amber-950/20;
}

.calculator-result-card--emerald {
  @apply border-emerald-200/80 bg-emerald-50/60 dark:border-emerald-900/50 dark:bg-emerald-950/20;
}

.calculator-result-card--sky {
  @apply border-sky-200/80 bg-sky-50/60 dark:border-sky-900/50 dark:bg-sky-950/20;
}

.calculator-result-card__stripe {
  @apply absolute left-0 top-0 h-full w-1;
}

.calculator-result-card__label {
  @apply flex items-center gap-1.5 text-xs font-semibold uppercase tracking-[0.16em] text-gray-500 dark:text-gray-400;
}

.calculator-result-card__value {
  @apply mt-3 text-2xl font-black tracking-tight text-gray-950 tabular-nums dark:text-white;
}

.calculator-result-card__value--hero {
  @apply text-4xl text-white sm:text-5xl;
}

.calculator-result-card__support {
  @apply mt-2 text-sm text-gray-600 dark:text-gray-300;
}

.calculator-result-card__meta {
  @apply mt-auto min-h-[2.75rem] pt-3 text-xs leading-5 text-gray-500 dark:text-gray-400;
}

.calculator-table-wrap {
  @apply overflow-x-auto rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800/40;
}
</style>
