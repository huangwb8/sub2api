<template>
  <div :class="pageClass">
    <template v-if="!isEmbedded">
      <header class="relative z-20 px-6 py-4">
        <nav class="mx-auto flex max-w-6xl items-center justify-between gap-4">
          <div class="flex items-center gap-4">
            <router-link to="/home" class="flex items-center gap-3">
              <div class="h-10 w-10 overflow-hidden rounded-xl shadow-md">
                <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
              </div>
              <span class="hidden text-lg font-semibold text-gray-900 dark:text-white sm:inline">
                {{ siteName }}
              </span>
            </router-link>

            <div class="hidden items-center gap-2 rounded-full border border-gray-200/70 bg-white/80 p-1 backdrop-blur md:flex dark:border-dark-700/60 dark:bg-dark-800/80">
              <router-link
                to="/home"
                class="rounded-full px-4 py-1.5 text-sm font-medium transition-colors"
                :class="navLinkClass('/home')"
              >
                {{ t('home.nav.home') }}
              </router-link>
              <router-link
                to="/pricing"
                class="rounded-full px-4 py-1.5 text-sm font-medium transition-colors"
                :class="navLinkClass('/pricing')"
              >
                {{ t('home.nav.pricing') }}
              </router-link>
            </div>
          </div>

          <div class="flex items-center gap-3">
            <LocaleSwitcher />

            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
              :title="t('home.viewDocs')"
            >
              <Icon name="book" size="md" />
            </a>

            <button
              @click="toggleTheme"
              class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
              :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            >
              <Icon v-if="isDark" name="sun" size="md" />
              <Icon v-else name="moon" size="md" />
            </button>

            <router-link
              v-if="isAuthenticated"
              :to="dashboardPath"
              class="inline-flex items-center rounded-full bg-gray-900 px-3 py-1 text-xs font-medium text-white transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
            >
              {{ t('home.dashboard') }}
            </router-link>
            <router-link
              v-else
              to="/login"
              class="inline-flex items-center rounded-full bg-gray-900 px-3 py-1 text-xs font-medium text-white transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
            >
              {{ t('home.login') }}
            </router-link>
          </div>
        </nav>
      </header>
    </template>

    <main :class="mainClass">
      <div class="mx-auto max-w-6xl">
        <div v-if="!isEmbedded" class="mb-10 text-center">
          <p class="mb-3 inline-flex items-center rounded-full border border-primary-200/70 bg-white/80 px-4 py-1 text-xs font-semibold uppercase tracking-[0.24em] text-primary-600 backdrop-blur dark:border-primary-900/50 dark:bg-dark-800/70 dark:text-primary-300">
            {{ t('pricing.eyebrow') }}
          </p>
          <h1 class="text-4xl font-bold text-gray-900 dark:text-white md:text-5xl">
            {{ t('pricing.title') }}
          </h1>
          <p class="mx-auto mt-4 max-w-3xl text-base leading-7 text-gray-600 dark:text-dark-300 md:text-lg">
            {{ t('pricing.subtitle') }}
          </p>
        </div>

        <div
          v-if="paymentEnabled && plans.length > 0 && !loading"
          class="mb-8 rounded-3xl border border-gray-200/70 bg-white/75 px-6 py-4 shadow-sm backdrop-blur dark:border-dark-700/60 dark:bg-dark-800/70"
        >
          <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('pricing.availableTitle', { count: plans.length }) }}
              </p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('pricing.availableDescription') }}
              </p>
            </div>
            <p class="text-xs text-gray-400 dark:text-dark-500">
              {{ t('pricing.ctaHint') }}
            </p>
          </div>
        </div>

        <div v-if="loading" class="flex items-center justify-center py-20">
          <div class="h-10 w-10 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
        </div>

        <div
          v-else-if="!paymentEnabled || plans.length === 0"
          class="rounded-3xl border border-dashed border-gray-300 bg-white/70 px-8 py-16 text-center shadow-sm backdrop-blur dark:border-dark-700 dark:bg-dark-800/70"
        >
          <Icon name="gift" size="xl" class="mx-auto mb-4 text-gray-300 dark:text-dark-600" />
          <h2 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ paymentEnabled ? t('pricing.emptyTitle') : t('pricing.disabledTitle') }}
          </h2>
          <p class="mx-auto mt-3 max-w-2xl text-sm leading-7 text-gray-500 dark:text-dark-400">
            {{ paymentEnabled ? t('pricing.emptyDescription') : t('pricing.disabledDescription') }}
          </p>
          <router-link
            v-if="!isEmbedded"
            to="/home"
            class="btn btn-secondary mt-6 inline-flex items-center px-6 py-2.5"
          >
            {{ t('pricing.backHome') }}
          </router-link>
        </div>

        <div v-else :class="planGridClass">
          <SubscriptionPlanCard
            v-for="plan in plans"
            :key="plan.id"
            :plan="plan"
            @select="handleSelectPlan"
          />
        </div>
      </div>
    </main>

    <footer
      v-if="!isEmbedded"
      class="relative z-10 border-t border-gray-200/50 px-6 py-8 dark:border-dark-800/50"
    >
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-center gap-4 text-center sm:flex-row sm:text-left"
      >
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <router-link
            to="/home"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.nav.home') }}
          </router-link>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore, useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import type { SubscriptionPlan } from '@/types/payment'
import { normalizeSubscriptionPlan, sortSubscriptionPlans } from '@/utils/subscriptionPlan'
import SubscriptionPlanCard from '@/components/payment/SubscriptionPlanCard.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { useTheme } from '@/composables/useTheme'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const plans = ref<SubscriptionPlan[]>([])
const loading = ref(false)
const { isDark, toggleTheme } = useTheme()

const isEmbedded = computed(() => route.query.ui_mode === 'embedded')
const paymentEnabled = computed(() => !!appStore.cachedPublicSettings?.payment_enabled)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const currentYear = computed(() => new Date().getFullYear())

const pageClass = computed(() => {
  if (isEmbedded.value) {
    return 'min-h-full bg-transparent'
  }
  return 'relative min-h-screen overflow-hidden bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950'
})

const mainClass = computed(() => (isEmbedded.value ? 'px-4 py-4' : 'relative z-10 px-6 py-14'))

const planGridClass = computed(() => {
  const count = plans.value.length
  if (count <= 1) return 'grid gap-6 md:grid-cols-1 md:max-w-md md:mx-auto'
  if (count === 2) return 'grid gap-6 lg:grid-cols-2'
  return 'grid gap-6 lg:grid-cols-3'
})

function navLinkClass(targetPath: string) {
  return route.path === targetPath
    ? 'bg-gray-900 text-white dark:bg-gray-700'
    : 'text-gray-500 hover:bg-gray-100 hover:text-gray-900 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white'
}

async function loadPlans() {
  loading.value = true
  try {
    const response = await paymentAPI.getPublicPlans()
    const normalized = (response.data || []).map((plan) => normalizeSubscriptionPlan(plan))
    plans.value = sortSubscriptionPlans(normalized)
  } catch (error) {
    console.error('[pricing] Failed to load public plans:', error)
    plans.value = []
  } finally {
    loading.value = false
  }
}

function handleSelectPlan() {
  if (authStore.isAuthenticated) {
    void router.push('/purchase')
    return
  }
  void router.push({
    path: '/login',
    query: { redirect: '/purchase' },
  })
}

onMounted(async () => {
  if (!appStore.publicSettingsLoaded) {
    await appStore.fetchPublicSettings()
  }
  await loadPlans()
})
</script>
