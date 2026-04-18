<template>
  <div class="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(20,184,166,0.18),_transparent_35%),linear-gradient(180deg,_#f5f7fb_0%,_#eef4f7_45%,_#f8fafc_100%)] dark:bg-[radial-gradient(circle_at_top_left,_rgba(45,212,191,0.15),_transparent_30%),linear-gradient(180deg,_#08111a_0%,_#0f172a_48%,_#111827_100%)]">
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div class="absolute -left-24 top-24 h-64 w-64 rounded-full bg-emerald-300/25 blur-3xl dark:bg-emerald-400/15"></div>
      <div class="absolute right-0 top-0 h-72 w-72 translate-x-1/4 -translate-y-1/4 rounded-full bg-sky-300/30 blur-3xl dark:bg-sky-400/15"></div>
      <div class="absolute bottom-0 left-1/2 h-80 w-80 -translate-x-1/2 translate-y-1/3 rounded-full bg-cyan-200/40 blur-3xl dark:bg-cyan-500/10"></div>
    </div>

    <header class="relative z-10 px-6 py-5">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4">
        <router-link
          to="/home"
          class="inline-flex items-center gap-3 rounded-full border border-white/70 bg-white/80 px-4 py-2 shadow-sm backdrop-blur dark:border-white/10 dark:bg-slate-900/70"
        >
          <div class="flex h-10 w-10 items-center justify-center overflow-hidden rounded-2xl bg-white shadow-inner dark:bg-slate-950">
            <img v-if="siteLogo" :src="siteLogo" alt="Logo" class="h-full w-full object-contain" />
            <span v-else class="text-sm font-semibold text-slate-700 dark:text-slate-200">
              {{ siteName.slice(0, 1).toUpperCase() }}
            </span>
          </div>
          <div class="text-left">
            <p class="text-xs uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">
              {{ t('legal.badge') }}
            </p>
            <p class="text-sm font-semibold text-slate-900 dark:text-white">
              {{ siteName }}
            </p>
          </div>
        </router-link>

        <div class="flex flex-wrap items-center gap-2">
          <router-link
            v-for="item in documentTabs"
            :key="item.to"
            :to="item.to"
            class="rounded-full border px-4 py-2 text-sm font-medium transition-all"
            :class="item.active
              ? 'border-emerald-500 bg-emerald-500 text-white shadow-lg shadow-emerald-500/25'
              : item.available
                ? 'border-slate-200 bg-white/80 text-slate-700 hover:border-emerald-300 hover:text-emerald-700 dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-200 dark:hover:border-emerald-400 dark:hover:text-emerald-300'
                : 'cursor-not-allowed border-slate-200/80 bg-white/50 text-slate-400 dark:border-slate-800 dark:bg-slate-900/40 dark:text-slate-500'"
          >
            {{ item.label }}
          </router-link>
        </div>
      </div>
    </header>

    <main class="relative z-10 px-6 pb-16 pt-6">
      <div class="mx-auto max-w-4xl">
        <section class="mb-8 rounded-[2rem] border border-white/70 bg-white/75 p-8 shadow-[0_30px_80px_-45px_rgba(15,23,42,0.45)] backdrop-blur dark:border-white/10 dark:bg-slate-900/75 md:p-10">
          <div class="flex flex-wrap items-start justify-between gap-6">
            <div class="max-w-2xl">
              <div class="mb-4 inline-flex items-center gap-2 rounded-full bg-emerald-500/10 px-4 py-1.5 text-xs font-semibold uppercase tracking-[0.22em] text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300">
                <span class="h-2 w-2 rounded-full bg-current"></span>
                {{ t(documentMeta.badgeKey) }}
              </div>
              <h1 class="text-3xl font-semibold tracking-tight text-slate-900 dark:text-white md:text-4xl">
                {{ t(documentMeta.titleKey) }}
              </h1>
              <p class="mt-3 max-w-2xl text-sm leading-7 text-slate-600 dark:text-slate-300 md:text-base">
                {{ t(documentMeta.descriptionKey, { siteName }) }}
              </p>
            </div>

            <div class="rounded-3xl border border-slate-200/80 bg-slate-50/80 px-5 py-4 text-sm text-slate-500 dark:border-slate-800 dark:bg-slate-950/60 dark:text-slate-400">
              <p class="font-medium text-slate-700 dark:text-slate-200">
                {{ t('legal.entryHintTitle') }}
              </p>
              <p class="mt-1 max-w-xs leading-6">
                {{ t('legal.entryHintBody') }}
              </p>
            </div>
          </div>
        </section>

        <section class="rounded-[2rem] border border-white/70 bg-white/88 p-6 shadow-[0_30px_80px_-45px_rgba(15,23,42,0.4)] backdrop-blur dark:border-white/10 dark:bg-slate-900/88 md:p-10">
          <div v-if="renderedContent" class="legal-markdown prose prose-slate max-w-none dark:prose-invert" v-html="renderedContent"></div>

          <div v-else class="rounded-[1.5rem] border border-dashed border-slate-300/80 bg-slate-50/80 p-10 text-center dark:border-slate-700 dark:bg-slate-950/60">
            <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-900 text-white dark:bg-white dark:text-slate-900">
              <Icon :name="documentMeta.icon" size="lg" />
            </div>
            <h2 class="mt-5 text-xl font-semibold text-slate-900 dark:text-white">
              {{ t('legal.notPublishedTitle') }}
            </h2>
            <p class="mx-auto mt-3 max-w-xl text-sm leading-7 text-slate-600 dark:text-slate-300">
              {{ t('legal.notPublishedDescription', { document: t(documentMeta.titleKey), siteName }) }}
            </p>
            <router-link
              to="/home"
              class="mt-6 inline-flex items-center rounded-full bg-slate-900 px-5 py-2.5 text-sm font-medium text-white transition hover:bg-slate-700 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-200"
            >
              {{ t('legal.backHome') }}
            </router-link>
          </div>
        </section>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'

type DocumentType = 'terms' | 'privacy'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()

marked.setOptions({
  breaks: true,
  gfm: true
})

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const currentDocumentType = computed<DocumentType>(() =>
  route.name === 'LegalPrivacy' ? 'privacy' : 'terms'
)

const documentMeta = computed(() => {
  if (currentDocumentType.value === 'privacy') {
    return {
      titleKey: 'legal.privacy.title',
      badgeKey: 'legal.privacy.badge',
      descriptionKey: 'legal.privacy.description',
      icon: 'shield' as const
    }
  }

  return {
    titleKey: 'legal.terms.title',
    badgeKey: 'legal.terms.badge',
    descriptionKey: 'legal.terms.description',
    icon: 'book' as const
  }
})

const termsContent = computed(() => appStore.cachedPublicSettings?.terms_of_service_content?.trim() || '')
const privacyContent = computed(() => appStore.cachedPublicSettings?.privacy_policy_content?.trim() || '')
const currentContent = computed(() =>
  currentDocumentType.value === 'privacy' ? privacyContent.value : termsContent.value
)

const documentTabs = computed(() => [
  {
    to: '/legal/terms',
    label: t('legal.terms.shortTitle'),
    active: currentDocumentType.value === 'terms',
    available: Boolean(termsContent.value)
  },
  {
    to: '/legal/privacy',
    label: t('legal.privacy.shortTitle'),
    active: currentDocumentType.value === 'privacy',
    available: Boolean(privacyContent.value)
  }
])

const renderedContent = computed(() => {
  if (!currentContent.value) return ''
  const html = marked.parse(currentContent.value) as string
  return DOMPurify.sanitize(html)
})

onMounted(() => {
  if (!appStore.publicSettingsLoaded) {
    void appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.legal-markdown {
  color: rgb(51 65 85);
  line-height: 1.9;
}

.legal-markdown :deep(h1),
.legal-markdown :deep(h2),
.legal-markdown :deep(h3),
.legal-markdown :deep(h4) {
  color: rgb(15 23 42);
  font-weight: 650;
  letter-spacing: -0.02em;
}

.legal-markdown :deep(h1) {
  margin-top: 0;
  font-size: 2rem;
}

.legal-markdown :deep(h2) {
  margin-top: 2rem;
  padding-top: 0.5rem;
  border-top: 1px solid rgb(226 232 240);
}

.legal-markdown :deep(p),
.legal-markdown :deep(li) {
  color: rgb(71 85 105);
}

.legal-markdown :deep(a) {
  color: rgb(5 150 105);
  text-decoration: none;
  font-weight: 600;
}

.legal-markdown :deep(a:hover) {
  text-decoration: underline;
}

.legal-markdown :deep(blockquote) {
  border-left: 4px solid rgb(16 185 129);
  background: rgb(236 253 245);
  border-radius: 1rem;
  padding: 1rem 1.25rem;
  color: rgb(6 95 70);
}

.legal-markdown :deep(code) {
  border-radius: 0.5rem;
  background: rgb(241 245 249);
  padding: 0.15rem 0.45rem;
  color: rgb(15 23 42);
}

.legal-markdown :deep(pre) {
  border-radius: 1.25rem;
  background: rgb(15 23 42);
  padding: 1rem 1.25rem;
  color: rgb(226 232 240);
}

.legal-markdown :deep(pre code) {
  background: transparent;
  padding: 0;
  color: inherit;
}

.legal-markdown :deep(table) {
  display: block;
  overflow-x: auto;
  border-radius: 1rem;
  border: 1px solid rgb(226 232 240);
}

.legal-markdown :deep(th) {
  background: rgb(248 250 252);
  color: rgb(15 23 42);
}

.legal-markdown :deep(th),
.legal-markdown :deep(td) {
  border-color: rgb(226 232 240);
}

:global(.dark) .legal-markdown {
  color: rgb(203 213 225);
}

:global(.dark) .legal-markdown :deep(h1),
:global(.dark) .legal-markdown :deep(h2),
:global(.dark) .legal-markdown :deep(h3),
:global(.dark) .legal-markdown :deep(h4) {
  color: rgb(248 250 252);
}

:global(.dark) .legal-markdown :deep(h2) {
  border-top-color: rgb(51 65 85);
}

:global(.dark) .legal-markdown :deep(p),
:global(.dark) .legal-markdown :deep(li) {
  color: rgb(203 213 225);
}

:global(.dark) .legal-markdown :deep(blockquote) {
  background: rgba(16, 185, 129, 0.12);
  color: rgb(167 243 208);
}

:global(.dark) .legal-markdown :deep(code) {
  background: rgb(30 41 59);
  color: rgb(226 232 240);
}

:global(.dark) .legal-markdown :deep(table) {
  border-color: rgb(51 65 85);
}

:global(.dark) .legal-markdown :deep(th) {
  background: rgb(15 23 42);
  color: rgb(241 245 249);
}

:global(.dark) .legal-markdown :deep(th),
:global(.dark) .legal-markdown :deep(td) {
  border-color: rgb(51 65 85);
}
</style>
