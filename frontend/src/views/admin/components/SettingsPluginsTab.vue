<template>
  <div class="space-y-6">
    <section class="overflow-hidden rounded-3xl border border-slate-200 bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.16),_transparent_34%),linear-gradient(135deg,_#f8fafc,_#eef2ff)] p-6 shadow-sm dark:border-dark-700 dark:bg-[radial-gradient(circle_at_top_left,_rgba(56,189,248,0.2),_transparent_32%),linear-gradient(135deg,_rgba(15,23,42,0.95),_rgba(30,41,59,0.96))]">
      <div class="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
        <div class="max-w-2xl">
          <p class="text-xs font-semibold uppercase tracking-[0.24em] text-sky-600 dark:text-sky-300">
            {{ t('admin.settings.plugins.eyebrow') }}
          </p>
          <h2 class="mt-2 text-2xl font-semibold text-slate-900 dark:text-white">
            {{ t('admin.settings.plugins.title') }}
          </h2>
          <p class="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-300">
            {{ t('admin.settings.plugins.description') }}
          </p>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div class="rounded-2xl border border-white/70 bg-white/80 px-4 py-3 shadow-sm backdrop-blur dark:border-white/10 dark:bg-white/5">
            <div class="text-xs font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">
              {{ t('admin.settings.plugins.metrics.instances') }}
            </div>
            <div class="mt-1 text-2xl font-semibold text-slate-900 dark:text-white">{{ plugins.length }}</div>
          </div>
          <div class="rounded-2xl border border-white/70 bg-white/80 px-4 py-3 shadow-sm backdrop-blur dark:border-white/10 dark:bg-white/5">
            <div class="text-xs font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">
              {{ t('admin.settings.plugins.metrics.enabled') }}
            </div>
            <div class="mt-1 text-2xl font-semibold text-slate-900 dark:text-white">{{ enabledCount }}</div>
          </div>
          <div class="rounded-2xl border border-white/70 bg-white/80 px-4 py-3 shadow-sm backdrop-blur dark:border-white/10 dark:bg-white/5">
            <div class="text-xs font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">
              {{ t('admin.settings.plugins.metrics.templates') }}
            </div>
            <div class="mt-1 text-2xl font-semibold text-slate-900 dark:text-white">{{ templateCount }}</div>
          </div>
        </div>
      </div>
    </section>

    <section class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.plugins.create.title') }}
        </h3>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.plugins.create.description') }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div class="grid gap-4 lg:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.settings.plugins.fields.name') }}</label>
            <input v-model.trim="createForm.name" class="input" :placeholder="t('admin.settings.plugins.placeholders.name')" />
            <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
              {{ t('admin.settings.plugins.hints.directoryRule') }}
            </p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.settings.plugins.fields.type') }}</label>
            <div class="input flex items-center justify-between bg-slate-50 dark:bg-dark-800">
              <span class="font-medium text-slate-900 dark:text-white">api-prompt</span>
              <span class="rounded-full bg-sky-100 px-2.5 py-1 text-xs font-medium text-sky-700 dark:bg-sky-900/30 dark:text-sky-200">
                {{ t('admin.settings.plugins.typeLabels.apiPrompt') }}
              </span>
            </div>
          </div>
          <div class="lg:col-span-2">
            <label class="input-label">{{ t('admin.settings.plugins.fields.description') }}</label>
            <textarea
              v-model.trim="createForm.description"
              rows="3"
              class="input min-h-[96px]"
              :placeholder="t('admin.settings.plugins.placeholders.description')"
            />
          </div>
        </div>

        <div class="flex flex-col gap-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50/70 p-4 dark:border-dark-600 dark:bg-dark-800/60">
          <div class="flex items-center justify-between">
            <div>
              <div class="font-medium text-slate-900 dark:text-white">{{ t('admin.settings.plugins.create.defaultTemplates') }}</div>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
                {{ t('admin.settings.plugins.create.defaultTemplatesHint') }}
              </p>
            </div>
            <Toggle v-model="createForm.enabled" />
          </div>
          <div class="grid gap-3 md:grid-cols-3">
            <div
              v-for="template in defaultTemplatePreview"
              :key="template.id"
              class="rounded-2xl border border-white bg-white p-4 shadow-sm dark:border-dark-600 dark:bg-dark-900/60"
            >
              <div class="flex items-center justify-between gap-2">
                <h4 class="font-medium text-slate-900 dark:text-white">{{ template.name }}</h4>
                <span class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-semibold text-slate-600 dark:bg-dark-700 dark:text-slate-300">
                  {{ t('admin.settings.plugins.labels.builtin') }}
                </span>
              </div>
              <p class="mt-2 text-sm text-slate-500 dark:text-slate-400">{{ template.description }}</p>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-end gap-3">
          <button type="button" class="btn btn-primary" :disabled="creating" @click="handleCreate">
            <Icon v-if="creating" name="refresh" size="sm" class="mr-2 animate-spin" />
            {{ creating ? t('common.creating') : t('admin.settings.plugins.create.submit') }}
          </button>
        </div>
      </div>
    </section>

    <section v-if="loading" class="flex items-center justify-center py-10 text-sm text-gray-500">
      <Icon name="refresh" size="sm" class="mr-2 animate-spin" />
      {{ t('common.loading') }}
    </section>

    <section v-else-if="plugins.length === 0" class="rounded-3xl border border-dashed border-slate-300 bg-slate-50/70 px-6 py-12 text-center dark:border-dark-600 dark:bg-dark-800/40">
      <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-white shadow-sm dark:bg-dark-700">
        <Icon name="cube" size="lg" class="text-sky-600 dark:text-sky-300" />
      </div>
      <h3 class="mt-4 text-lg font-semibold text-slate-900 dark:text-white">
        {{ t('admin.settings.plugins.empty.title') }}
      </h3>
      <p class="mt-2 text-sm text-slate-500 dark:text-slate-400">
        {{ t('admin.settings.plugins.empty.description') }}
      </p>
    </section>

    <section v-else class="space-y-5">
      <article
        v-for="plugin in plugins"
        :key="plugin.name"
        class="overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900/70"
      >
        <div class="border-b border-slate-100 bg-slate-50/90 px-6 py-4 dark:border-dark-700 dark:bg-dark-800/60">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-lg font-semibold text-slate-900 dark:text-white">{{ plugin.name }}</h3>
                <span class="rounded-full bg-sky-100 px-2.5 py-1 text-xs font-semibold text-sky-700 dark:bg-sky-900/30 dark:text-sky-200">
                  {{ t('admin.settings.plugins.typeLabels.apiPrompt') }}
                </span>
                <span
                  :class="plugin.enabled
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
                    : 'bg-slate-200 text-slate-600 dark:bg-dark-700 dark:text-slate-300'"
                  class="rounded-full px-2.5 py-1 text-xs font-semibold"
                >
                  {{ plugin.enabled ? t('common.enabled') : t('common.disabled') }}
                </span>
                <span class="rounded-full bg-slate-200 px-2.5 py-1 text-xs font-semibold text-slate-600 dark:bg-dark-700 dark:text-slate-300">
                  {{ t('admin.settings.plugins.labels.localMode') }}
                </span>
              </div>
              <p class="mt-2 text-sm text-slate-500 dark:text-slate-400">
                {{ plugin.description || t('admin.settings.plugins.labels.noDescription') }}
              </p>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="testingName === plugin.name" @click="handleTest(plugin.name)">
                <Icon v-if="testingName === plugin.name" name="refresh" size="sm" class="mr-1.5 animate-spin" />
                <Icon v-else name="beaker" size="sm" class="mr-1.5" />
                {{ t('admin.settings.plugins.actions.test') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="togglingName === plugin.name"
                @click="handleToggle(plugin)"
              >
                <Icon v-if="togglingName === plugin.name" name="refresh" size="sm" class="mr-1.5 animate-spin" />
                <Icon v-else :name="plugin.enabled ? 'ban' : 'play'" size="sm" class="mr-1.5" />
                {{ plugin.enabled ? t('admin.settings.plugins.actions.disable') : t('admin.settings.plugins.actions.enable') }}
              </button>
              <button type="button" class="btn btn-primary btn-sm" :disabled="savingName === plugin.name" @click="handleSave(plugin)">
                <Icon v-if="savingName === plugin.name" name="refresh" size="sm" class="mr-1.5 animate-spin" />
                {{ savingName === plugin.name ? t('common.saving') : t('common.save') }}
              </button>
            </div>
          </div>

          <div v-if="testResults[plugin.name]" class="mt-4 rounded-2xl px-4 py-3 text-sm"
            :class="testResults[plugin.name]!.ok
              ? 'border border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/40 dark:bg-emerald-900/10 dark:text-emerald-200'
              : 'border border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/40 dark:bg-amber-900/10 dark:text-amber-200'">
            {{ testResults[plugin.name]!.message }}
          </div>
        </div>

        <div class="space-y-6 px-6 py-5">
          <div class="grid gap-4">
            <div>
              <label class="input-label">{{ t('admin.settings.plugins.fields.description') }}</label>
              <textarea
                v-model.trim="plugin.description"
                rows="3"
                class="input min-h-[96px]"
                :placeholder="t('admin.settings.plugins.placeholders.description')"
              />
            </div>
          </div>

          <div class="rounded-3xl border border-slate-200 bg-slate-50/70 p-5 dark:border-dark-700 dark:bg-dark-800/50">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h4 class="text-base font-semibold text-slate-900 dark:text-white">
                  {{ t('admin.settings.plugins.templates.title') }}
                </h4>
                <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
                  {{ t('admin.settings.plugins.templates.description') }}
                </p>
                <div class="mt-3 flex flex-wrap items-center gap-2 text-xs">
                  <span class="rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-1 font-medium text-emerald-700 dark:border-emerald-900/40 dark:bg-emerald-900/10 dark:text-emerald-200">
                    {{ t('admin.settings.plugins.templates.statusLocal') }}
                  </span>
                  <span class="rounded-full border border-slate-200 bg-white px-2.5 py-1 text-slate-600 dark:border-dark-600 dark:bg-dark-900 dark:text-slate-300">
                    {{ t('admin.settings.plugins.metrics.templates') }}: {{ plugin.api_prompt?.templates.length ?? 0 }}
                  </span>
                </div>
              </div>
              <button type="button" class="btn btn-secondary btn-sm" @click="addTemplate(plugin)">
                <Icon name="plus" size="sm" class="mr-1.5" />
                {{ t('admin.settings.plugins.templates.add') }}
              </button>
            </div>

            <div class="mt-4 space-y-4">
              <div
                v-if="(plugin.api_prompt?.templates.length ?? 0) === 0"
                class="rounded-2xl border border-dashed border-slate-300 bg-white px-4 py-6 text-sm text-slate-500 dark:border-dark-600 dark:bg-dark-900/60 dark:text-slate-400"
              >
                {{ t('admin.settings.plugins.templates.emptyLocal') }}
              </div>
              <div
                v-for="(template, index) in plugin.api_prompt?.templates ?? []"
                :key="template.id"
                class="rounded-2xl border border-white bg-white p-4 shadow-sm dark:border-dark-600 dark:bg-dark-900/70"
              >
                <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                  <div class="grid flex-1 gap-4 lg:grid-cols-2">
                    <div>
                      <label class="input-label">{{ t('admin.settings.plugins.templates.fields.name') }}</label>
                      <input v-model.trim="template.name" class="input" :placeholder="t('admin.settings.plugins.templates.placeholders.name')" />
                    </div>
                    <div>
                      <label class="input-label">{{ t('admin.settings.plugins.templates.fields.id') }}</label>
                      <input v-model.trim="template.id" class="input font-mono text-sm" :placeholder="t('admin.settings.plugins.templates.placeholders.id')" />
                    </div>
                    <div class="lg:col-span-2">
                      <label class="input-label">{{ t('admin.settings.plugins.templates.fields.description') }}</label>
                      <input v-model.trim="template.description" class="input" :placeholder="t('admin.settings.plugins.templates.placeholders.description')" />
                    </div>
                    <div class="lg:col-span-2">
                      <label class="input-label">{{ t('admin.settings.plugins.templates.fields.prompt') }}</label>
                      <textarea
                        v-model="template.prompt"
                        rows="5"
                        class="input min-h-[140px] font-mono text-sm leading-6"
                        :placeholder="t('admin.settings.plugins.templates.placeholders.prompt')"
                      />
                    </div>
                  </div>

                  <div class="flex min-w-[180px] flex-row items-start justify-between gap-3 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800 lg:flex-col">
                    <div class="space-y-2">
                      <div class="flex flex-wrap gap-2">
                        <span v-if="template.builtin" class="rounded-full bg-sky-100 px-2 py-0.5 text-[11px] font-semibold text-sky-700 dark:bg-sky-900/30 dark:text-sky-200">
                          {{ t('admin.settings.plugins.labels.builtin') }}
                        </span>
                        <span class="rounded-full bg-slate-200 px-2 py-0.5 text-[11px] font-semibold text-slate-600 dark:bg-dark-700 dark:text-slate-300">
                          #{{ index + 1 }}
                        </span>
                      </div>
                      <p class="text-xs text-slate-500 dark:text-slate-400">
                        {{ t('admin.settings.plugins.templates.hints.injection') }}
                      </p>
                    </div>
                    <div class="flex items-center gap-3">
                      <Toggle v-model="template.enabled" />
                      <button
                        type="button"
                        class="rounded-full p-2 text-slate-400 transition hover:bg-rose-50 hover:text-rose-600 dark:hover:bg-rose-900/20 dark:hover:text-rose-300"
                        @click="removeTemplate(plugin, index)"
                      >
                        <Icon name="trash" size="sm" />
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </article>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { adminPluginsAPI, type PluginTestResult } from '@/api/admin/plugins'
import type { APIPromptPluginConfig, APIPromptTemplate, Plugin } from '@/types'

type PluginFormState = Plugin

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const creating = ref(false)
const savingName = ref('')
const testingName = ref('')
const togglingName = ref('')
const plugins = ref<PluginFormState[]>([])
const testResults = ref<Record<string, PluginTestResult>>({})

const createForm = ref({
  name: '',
  description: '',
  enabled: true
})

const defaultTemplatePreview = computed(() => [
  {
    id: 'general-writing',
    name: t('admin.settings.plugins.defaults.generalWriting.name'),
    description: t('admin.settings.plugins.defaults.generalWriting.description')
  },
  {
    id: 'engineering-review',
    name: t('admin.settings.plugins.defaults.engineeringReview.name'),
    description: t('admin.settings.plugins.defaults.engineeringReview.description')
  },
  {
    id: 'product-ops',
    name: t('admin.settings.plugins.defaults.productOps.name'),
    description: t('admin.settings.plugins.defaults.productOps.description')
  }
])

const enabledCount = computed(() => plugins.value.filter((plugin) => plugin.enabled).length)
const templateCount = computed(() =>
  plugins.value.reduce((total, plugin) => total + (plugin.api_prompt?.templates.length ?? 0), 0)
)

function cloneTemplates(config?: APIPromptPluginConfig): APIPromptPluginConfig {
  return {
    templates: (config?.templates ?? []).map((template) => ({ ...template })),
    source: 'local'
  }
}

function hydratePlugin(plugin: Plugin): PluginFormState {
  return {
    ...plugin,
    api_prompt: cloneTemplates(plugin.api_prompt)
  }
}

function makeTemplate(): APIPromptTemplate {
  const id = `custom-${Math.random().toString(36).slice(2, 10)}`
  return {
    id,
    name: '',
    description: '',
    prompt: '',
    enabled: true,
    builtin: false,
    sort_order: Date.now()
  }
}

async function loadPlugins() {
  loading.value = true
  try {
    const data = await adminPluginsAPI.list()
    plugins.value = data.map(hydratePlugin)
  } catch (error) {
    appStore.showError(t('admin.settings.plugins.messages.loadFailed'))
  } finally {
    loading.value = false
  }
}

function validateCreateForm() {
  if (!createForm.value.name.trim()) {
    appStore.showError(t('admin.settings.plugins.messages.nameRequired'))
    return false
  }
  return true
}

function validatePlugin(plugin: PluginFormState) {
  for (const template of plugin.api_prompt?.templates ?? []) {
    if (!template.name.trim() || !template.id.trim() || !template.prompt.trim()) {
      appStore.showError(t('admin.settings.plugins.messages.templateInvalid'))
      return false
    }
  }
  return true
}

async function handleCreate() {
  if (!validateCreateForm()) return
  creating.value = true
  try {
    const plugin = await adminPluginsAPI.create({
      name: createForm.value.name.trim(),
      type: 'api-prompt',
      description: createForm.value.description.trim(),
      enabled: createForm.value.enabled
    })
    plugins.value.unshift(hydratePlugin(plugin))
    createForm.value = {
      name: '',
      description: '',
      enabled: true
    }
    appStore.showSuccess(t('admin.settings.plugins.messages.createSuccess'))
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || t('admin.settings.plugins.messages.createFailed'))
  } finally {
    creating.value = false
  }
}

async function handleSave(plugin: PluginFormState) {
  if (!validatePlugin(plugin)) return
  savingName.value = plugin.name
  try {
    const updated = await adminPluginsAPI.update(plugin.name, {
      description: plugin.description?.trim() || '',
      enabled: plugin.enabled,
      api_prompt: cloneTemplates(plugin.api_prompt)
    })
    const next = hydratePlugin(updated)
    plugins.value = plugins.value.map((item) => (item.name === plugin.name ? next : item))
    appStore.showSuccess(t('admin.settings.plugins.messages.saveSuccess'))
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || t('admin.settings.plugins.messages.saveFailed'))
  } finally {
    savingName.value = ''
  }
}

async function handleTest(name: string) {
  testingName.value = name
  try {
    const result = await adminPluginsAPI.test(name)
    testResults.value = { ...testResults.value, [name]: result }
    appStore.showSuccess(result.ok ? t('admin.settings.plugins.messages.testSuccess') : t('admin.settings.plugins.messages.testWarning'))
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || t('admin.settings.plugins.messages.testFailed'))
  } finally {
    testingName.value = ''
  }
}

async function handleToggle(plugin: PluginFormState) {
  togglingName.value = plugin.name
  try {
    const updated = await adminPluginsAPI.setEnabled(plugin.name, !plugin.enabled)
    plugins.value = plugins.value.map((item) => (item.name === plugin.name ? hydratePlugin(updated) : item))
    appStore.showSuccess(updated.enabled ? t('admin.settings.plugins.messages.enableSuccess') : t('admin.settings.plugins.messages.disableSuccess'))
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || t('admin.settings.plugins.messages.toggleFailed'))
  } finally {
    togglingName.value = ''
  }
}

function addTemplate(plugin: PluginFormState) {
  plugin.api_prompt ??= { templates: [] }
  plugin.api_prompt.templates.push(makeTemplate())
}

function removeTemplate(plugin: PluginFormState, index: number) {
  if ((plugin.api_prompt?.templates.length ?? 0) <= 1) {
    appStore.showError(t('admin.settings.plugins.messages.keepOneTemplate'))
    return
  }
  plugin.api_prompt?.templates.splice(index, 1)
}

onMounted(() => {
  loadPlugins()
})
</script>
