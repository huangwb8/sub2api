<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ t('profile.editProfile') }}
      </h2>
    </div>
    <div class="px-6 py-6">
      <form @submit.prevent="handleUpdateProfile" class="space-y-6">
        <section class="grid gap-5 lg:grid-cols-[144px,1fr]">
          <div class="flex flex-col items-center gap-3">
            <UserAvatar
              :user="user"
              :preview-type="avatarType"
              :preview-style="avatarStyle"
              :preview-url="previewUrl"
              size="xl"
              rounded="2xl"
              class="shadow-lg shadow-primary-500/15"
            />
            <button
              v-if="avatarType !== 'generated'"
              type="button"
              class="btn btn-secondary btn-sm"
              @click="useGeneratedAvatar"
            >
              {{ t('profile.avatar.useGenerated') }}
            </button>
          </div>

          <div class="space-y-5">
            <div>
              <label for="username" class="input-label">
                {{ t('profile.username') }}
              </label>
              <input
                id="username"
                v-model="username"
                type="text"
                class="input"
                :placeholder="t('profile.enterUsername')"
                maxlength="100"
              />
            </div>

            <div>
              <div class="input-label">{{ t('profile.avatar.source') }}</div>
              <div class="grid gap-3 md:grid-cols-3">
                <label
                  v-for="source in avatarSources"
                  :key="source.value"
                  class="cursor-pointer rounded-lg border p-3 transition hover:border-primary-300 dark:hover:border-primary-700"
                  :class="avatarType === source.value ? 'border-primary-400 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/20' : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800'"
                >
                  <div class="flex items-start gap-3">
                    <input v-model="avatarType" type="radio" class="mt-1" :value="source.value" />
                    <div>
                      <div class="text-sm font-medium text-gray-900 dark:text-white">
                        {{ source.label }}
                      </div>
                      <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                        {{ source.description }}
                      </div>
                    </div>
                  </div>
                </label>
              </div>
            </div>

            <div v-if="avatarType === 'external'" class="space-y-2">
              <label for="avatarUrl" class="input-label">
                {{ t('profile.avatar.externalUrl') }}
              </label>
              <input
                id="avatarUrl"
                v-model="avatarUrl"
                type="url"
                class="input"
                placeholder="https://example.com/avatar.png"
                maxlength="2048"
              />
            </div>

            <div v-if="avatarType === 'uploaded'" class="space-y-2">
              <label for="avatarFile" class="input-label">
                {{ t('profile.avatar.uploadFile') }}
              </label>
              <input
                id="avatarFile"
                ref="fileInputRef"
                type="file"
                class="input h-auto p-3"
                accept="image/png,image/jpeg,image/webp"
                @change="handleFileChange"
              />
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('profile.avatar.uploadHint') }}
              </p>
            </div>

            <div class="space-y-3">
              <div>
                <div class="input-label">{{ t('profile.avatar.style') }}</div>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('profile.avatar.styleHint') }}
                </p>
              </div>
              <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                <label
                  v-for="style in avatarStyles"
                  :key="style.value"
                  class="cursor-pointer rounded-lg border p-3 transition hover:border-primary-300 dark:hover:border-primary-700"
                  :class="avatarStyle === style.value ? 'border-primary-400 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/20' : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800'"
                >
                  <div class="flex items-center gap-3">
                    <input v-model="avatarStyle" type="radio" :value="style.value" />
                    <UserAvatar
                      :user="user"
                      preview-type="generated"
                      :preview-style="style.value"
                      size="md"
                      rounded="xl"
                    />
                    <div class="min-w-0">
                      <div class="truncate text-sm font-medium text-gray-900 dark:text-white">
                        {{ style.label }}
                      </div>
                      <div class="text-xs text-gray-500 dark:text-gray-400">
                        {{ style.description }}
                      </div>
                    </div>
                  </div>
                </label>
              </div>
            </div>
          </div>
        </section>

        <div class="flex justify-end pt-2">
          <button type="submit" :disabled="loading" class="btn btn-primary">
            {{ loading ? t('profile.updating') : t('profile.updateProfile') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { userAPI } from '@/api'
import UserAvatar from '@/components/user/UserAvatar.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { User } from '@/types'

const props = defineProps<{
  user: User | null
}>()

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const username = ref('')
const avatarType = ref<User['avatar_type']>('generated')
const avatarStyle = ref<User['avatar_style']>('classic_letter')
const avatarUrl = ref('')
const selectedFile = ref<File | null>(null)
const selectedFileUrl = ref('')
const loading = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)

const avatarSources = computed(() => [
  {
    value: 'generated' as const,
    label: t('profile.avatar.generated'),
    description: t('profile.avatar.generatedDescription')
  },
  {
    value: 'uploaded' as const,
    label: t('profile.avatar.uploaded'),
    description: t('profile.avatar.uploadedDescription')
  },
  {
    value: 'external' as const,
    label: t('profile.avatar.external'),
    description: t('profile.avatar.externalDescription')
  }
])

const avatarStyles = computed(() => [
  { value: 'classic_letter' as const, label: t('profile.avatar.styles.classicLetter'), description: t('profile.avatar.styles.classicLetterDesc') },
  { value: 'aurora_ring' as const, label: t('profile.avatar.styles.auroraRing'), description: t('profile.avatar.styles.auroraRingDesc') },
  { value: 'orbit_burst' as const, label: t('profile.avatar.styles.orbitBurst'), description: t('profile.avatar.styles.orbitBurstDesc') },
  { value: 'pixel_patch' as const, label: t('profile.avatar.styles.pixelPatch'), description: t('profile.avatar.styles.pixelPatchDesc') },
  { value: 'paper_cut' as const, label: t('profile.avatar.styles.paperCut'), description: t('profile.avatar.styles.paperCutDesc') }
])

const previewUrl = computed(() => {
  if (avatarType.value === 'uploaded') {
    return selectedFileUrl.value || props.user?.avatar_url || ''
  }
  if (avatarType.value === 'external') {
    return avatarUrl.value
  }
  return ''
})

watch(() => props.user, (user) => {
  username.value = user?.username || ''
  avatarType.value = user?.avatar_type || 'generated'
  avatarStyle.value = user?.avatar_style || 'classic_letter'
  avatarUrl.value = user?.avatar_type === 'external' ? user.avatar_url : ''
  clearSelectedFile()
}, { immediate: true })

watch(avatarType, (type) => {
  if (type !== 'uploaded') {
    clearSelectedFile()
  }
})

onBeforeUnmount(() => {
  revokeSelectedFileUrl()
})

function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] || null
  clearSelectedFile(false)
  if (!file) return

  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
    appStore.showError(t('profile.avatar.unsupportedFile'))
    input.value = ''
    return
  }
  if (file.size > 1024 * 1024) {
    appStore.showError(t('profile.avatar.fileTooLarge'))
    input.value = ''
    return
  }

  selectedFile.value = file
  selectedFileUrl.value = URL.createObjectURL(file)
}

function clearSelectedFile(resetInput = true) {
  selectedFile.value = null
  revokeSelectedFileUrl()
  if (resetInput && fileInputRef.value) {
    fileInputRef.value.value = ''
  }
}

function revokeSelectedFileUrl() {
  if (selectedFileUrl.value) {
    URL.revokeObjectURL(selectedFileUrl.value)
    selectedFileUrl.value = ''
  }
}

function useGeneratedAvatar() {
  avatarType.value = 'generated'
  avatarUrl.value = ''
  clearSelectedFile()
}

const handleUpdateProfile = async () => {
  if (!username.value.trim()) {
    appStore.showError(t('profile.usernameRequired'))
    return
  }
  if (avatarType.value === 'external' && !avatarUrl.value.trim()) {
    appStore.showError(t('profile.avatar.urlRequired'))
    return
  }
  if (avatarType.value === 'uploaded' && !selectedFile.value && !(props.user?.avatar_type === 'uploaded' && props.user.avatar_url)) {
    appStore.showError(t('profile.avatar.fileRequired'))
    return
  }

  const form = new FormData()
  form.set('username', username.value.trim())
  form.set('avatar_type', avatarType.value)
  form.set('avatar_style', avatarStyle.value)
  if (avatarType.value === 'external') {
    form.set('avatar_url', avatarUrl.value.trim())
  }
  if (selectedFile.value) {
    form.set('avatar_file', selectedFile.value)
  }

  loading.value = true
  try {
    const updatedUser = await userAPI.updateProfile(form)
    authStore.user = updatedUser
    localStorage.setItem('auth_user', JSON.stringify(updatedUser))
    clearSelectedFile()
    appStore.showSuccess(t('profile.updateSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('profile.updateFailed')))
  } finally {
    loading.value = false
  }
}
</script>
