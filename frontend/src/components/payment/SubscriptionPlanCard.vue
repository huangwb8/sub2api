<template>
  <div
    :class="[
      'group relative flex flex-col overflow-hidden rounded-2xl border transition-all',
      'hover:shadow-xl hover:-translate-y-0.5',
      'border-gray-200 dark:border-dark-700',
      'bg-white dark:bg-dark-800',
    ]"
  >
    <!-- Top accent bar -->
    <div class="h-1.5 bg-gradient-to-r from-primary-400 to-primary-500" />

    <div class="flex flex-1 flex-col p-4">
      <!-- Header: name + badge + price -->
      <div class="mb-3 flex items-start justify-between gap-2">
        <div class="min-w-0 flex-1">
          <h3 class="truncate text-base font-bold text-gray-900 dark:text-white">{{ plan.name }}</h3>
          <p v-if="plan.description" class="mt-0.5 text-xs leading-relaxed text-gray-500 dark:text-dark-400 line-clamp-2">
            {{ plan.description }}
          </p>
        </div>
        <div class="shrink-0 text-right">
          <div class="flex items-baseline gap-1">
            <span class="text-xs text-gray-400 dark:text-dark-500">{{ currencySymbol }}</span>
            <span class="text-2xl font-extrabold tracking-tight text-primary-600 dark:text-primary-400">{{ formattedPriceValue }}</span>
          </div>
          <span class="text-[11px] text-gray-400 dark:text-dark-500">/ {{ validitySuffix }}</span>
          <div v-if="plan.original_price" class="mt-0.5 flex items-center justify-end gap-1.5">
            <span class="text-xs text-gray-400 line-through dark:text-dark-500">{{ formatPaymentAmount(plan.original_price) }}</span>
            <span class="rounded bg-red-100 px-1 py-0.5 text-[10px] font-semibold text-red-700 dark:bg-red-900/40 dark:text-red-300">{{ discountText }}</span>
          </div>
        </div>
      </div>

      <!-- Group quota info (compact) -->
      <div class="mb-3 grid grid-cols-2 gap-x-3 gap-y-1 rounded-lg bg-gray-50 px-3 py-2 text-xs dark:bg-dark-700/50">
        <div class="flex items-center justify-between">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.rate') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ rateDisplay }}</span>
        </div>
        <div v-if="plan.daily_limit_usd != null" class="flex items-center justify-between">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.dailyLimit') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ formatUsageCost(plan.daily_limit_usd) }}</span>
        </div>
        <div v-if="plan.weekly_limit_usd != null" class="flex items-center justify-between">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.weeklyLimit') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ formatUsageCost(plan.weekly_limit_usd) }}</span>
        </div>
        <div v-if="plan.monthly_limit_usd != null" class="flex items-center justify-between">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.monthlyLimit') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ formatUsageCost(plan.monthly_limit_usd) }}</span>
        </div>
        <div v-if="plan.daily_limit_usd == null && plan.weekly_limit_usd == null && plan.monthly_limit_usd == null" class="flex items-center justify-between">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.quota') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.planCard.unlimited') }}</span>
        </div>
        <div v-if="modelScopeLabels.length > 0" class="col-span-2 flex items-center justify-between">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.models') }}</span>
          <div class="flex flex-wrap justify-end gap-1">
            <span v-for="scope in modelScopeLabels" :key="scope"
              class="rounded bg-gray-200/80 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-600 dark:text-gray-300">
              {{ scope }}
            </span>
          </div>
        </div>
      </div>

      <!-- Idle dynamic billing highlight -->
      <div
        v-if="idleBillingVisible"
        class="mb-3 rounded-lg border border-emerald-200 bg-emerald-50/80 px-3 py-2 text-xs dark:border-emerald-900/60 dark:bg-emerald-950/30"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="flex min-w-0 items-center gap-1.5">
            <svg class="h-3.5 w-3.5 shrink-0 text-emerald-600 dark:text-emerald-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.25">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6l3.5 2M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
            </svg>
            <span class="truncate font-semibold text-emerald-800 dark:text-emerald-200">
              {{ t('payment.planCard.idleBilling') }}
            </span>
          </div>
          <span class="shrink-0 text-[11px] font-medium text-emerald-700 dark:text-emerald-300">
            {{ idleBillingWindowDisplay }}
          </span>
        </div>
        <div class="mt-2 flex flex-wrap gap-1.5">
          <span
            v-if="idleRateDisplay"
            class="rounded-md bg-white px-2 py-1 font-medium text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-950/50 dark:text-emerald-200 dark:ring-emerald-900/70"
          >
            {{ t('payment.planCard.idleRate') }} {{ idleRateDisplay }}
          </span>
          <span
            v-if="idleExtraProfitDisplay"
            class="rounded-md bg-white px-2 py-1 font-medium text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-950/50 dark:text-emerald-200 dark:ring-emerald-900/70"
          >
            {{ t('payment.planCard.idleExtraProfit') }} {{ idleExtraProfitDisplay }}
          </span>
        </div>
      </div>

      <!-- Features list (compact) -->
      <div v-if="plan.features.length > 0" class="mb-3 space-y-1">
        <div v-for="feature in plan.features" :key="feature" class="flex items-start gap-1.5">
          <svg class="mt-0.5 h-3.5 w-3.5 shrink-0 text-primary-500 dark:text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
          </svg>
          <span class="text-xs text-gray-600 dark:text-gray-300">{{ feature }}</span>
        </div>
      </div>

      <div class="flex-1" />

      <!-- Subscribe Button -->
      <button
        type="button"
        :class="['w-full rounded-xl py-2.5 text-sm font-semibold transition-all active:scale-[0.98]', 'bg-primary-500 text-white hover:bg-primary-600 dark:bg-primary-600 dark:hover:bg-primary-500']"
        @click="emit('select', plan)"
      >
        {{ isRenewal ? t('payment.renewNow') : t('payment.subscribeNow') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/types/payment'
import type { UserSubscription } from '@/types'
import { normalizePlanValidityUnit } from '@/utils/subscriptionPlan'
import { formatPaymentAmount, formatUsageCost, getCurrencySymbol } from '@/utils/format'

const props = defineProps<{ plan: SubscriptionPlan; activeSubscriptions?: UserSubscription[] }>()
const emit = defineEmits<{ select: [plan: SubscriptionPlan] }>()
const { t } = useI18n()

const isRenewal = computed(() =>
  props.activeSubscriptions?.some(s => s.group_id === props.plan.group_id && s.status === 'active') ?? false
)

const currencySymbol = getCurrencySymbol('CNY')
const formattedPriceValue = computed(() => formatPaymentAmount(props.plan.price).replace(currencySymbol, ''))

function formatCompactNumber(value: number) {
  return Number(value.toPrecision(10)).toString()
}

const discountText = computed(() => {
  if (!props.plan.original_price || props.plan.original_price <= 0) return ''
  const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
})

const rateDisplay = computed(() => {
  const rate = props.plan.rate_multiplier ?? 1
  return `×${formatCompactNumber(rate)}`
})

const idleBillingVisible = computed(() => {
  return Boolean(
    props.plan.idle_start_time &&
    props.plan.idle_end_time &&
    (props.plan.idle_rate_multiplier != null || props.plan.idle_extra_profit_rate_percent != null)
  )
})

function formatClockTime(value?: string | null) {
  const time = (value || '').trim()
  if (/^\d{2}:\d{2}:00$/.test(time)) {
    return time.slice(0, 5)
  }
  return time
}

const idleBillingWindowDisplay = computed(() => {
  const start = formatClockTime(props.plan.idle_start_time)
  const end = formatClockTime(props.plan.idle_end_time)
  return t('payment.planCard.idleWindow', { start, end })
})

const idleRateDisplay = computed(() => {
  if (props.plan.idle_rate_multiplier == null) return ''
  return `×${formatCompactNumber(props.plan.idle_rate_multiplier)}`
})

const idleExtraProfitDisplay = computed(() => {
  if (props.plan.idle_extra_profit_rate_percent == null) return ''
  return `${formatCompactNumber(props.plan.idle_extra_profit_rate_percent)}%`
})

const MODEL_SCOPE_LABELS: Record<string, string> = {
  claude: 'Claude',
  gemini_text: 'Gemini',
  gemini_image: 'Imagen',
}

const modelScopeLabels = computed(() => {
  const scopes = props.plan.supported_model_scopes
  if (!scopes || scopes.length === 0) return []
  return scopes.map(s => MODEL_SCOPE_LABELS[s] || s)
})

const validitySuffix = computed(() => {
  const u = normalizePlanValidityUnit(props.plan.validity_unit)
  if (u === 'month') return t('payment.perMonth')
  if (u === 'year') return t('payment.perYear')
  if (u === 'week') return `${props.plan.validity_days}${t('payment.admin.weeks')}`
  return `${props.plan.validity_days}${t('payment.days')}`
})
</script>
