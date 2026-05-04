<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ t('profile.editProfile') }}
      </h2>
    </div>
    <div class="px-6 py-6">
      <form @submit.prevent="handleUpdateProfile" class="space-y-6">
        <section class="grid gap-5 md:grid-cols-[144px,1fr]">
          <div class="flex flex-col items-center gap-3">
            <div class="rounded-[1.35rem] bg-white p-2 shadow-lg shadow-primary-500/15 ring-1 ring-gray-200 dark:bg-dark-900/60 dark:ring-dark-700">
              <UserAvatar
                :user="user"
                :preview-type="avatarType"
                :preview-style="avatarStyle"
                :preview-url="previewUrl"
                size="xl"
                rounded="2xl"
              />
            </div>
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
              <div class="flex flex-wrap items-center justify-between gap-3">
                <label for="avatarFile" class="input-label mb-0">
                  {{ t('profile.avatar.uploadFile') }}
                </label>
                <div v-if="selectedFile" class="text-xs text-gray-500 dark:text-gray-400">
                  {{ selectedFile.name }}
                </div>
              </div>
              <input
                id="avatarFile"
                ref="fileInputRef"
                type="file"
                class="sr-only"
                accept="image/png,image/jpeg,image/webp"
                @change="handleFileChange"
              />
              <div class="rounded-xl border border-dashed border-gray-300 bg-gray-50/80 p-4 dark:border-dark-600 dark:bg-dark-900/40">
                <div class="flex flex-wrap items-center gap-3">
                  <label for="avatarFile" class="btn btn-secondary btn-sm cursor-pointer">
                    {{ t('profile.avatar.chooseFile') }}
                  </label>
                  <button
                    v-if="cropSourceUrl"
                    type="button"
                    class="btn btn-ghost btn-sm"
                    @click="resetCrop"
                  >
                    {{ t('profile.avatar.recenter') }}
                  </button>
                  <button
                    v-if="selectedFile"
                    type="button"
                    class="btn btn-ghost btn-sm"
                    @click="clearSelectedFile()"
                  >
                    {{ t('profile.avatar.removeUpload') }}
                  </button>
                </div>
                <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('profile.avatar.uploadHint') }}
                </p>

                <div v-if="cropSourceUrl" class="mt-4 grid gap-4 xl:grid-cols-[minmax(0,320px),1fr]">
                  <div>
                    <div
                      ref="cropFrameRef"
                      class="relative aspect-square w-full max-w-80 touch-none select-none overflow-hidden rounded-2xl border border-gray-200 bg-gray-950 shadow-inner dark:border-dark-600"
                      @pointerdown="startDrag"
                    >
                      <img
                        ref="cropImageRef"
                        :src="cropSourceUrl"
                        alt=""
                        class="absolute left-1/2 top-1/2 max-w-none cursor-grab select-none"
                        :class="{ 'cursor-grabbing': isDragging }"
                        :style="cropImageStyle"
                        draggable="false"
                        @load="handleCropImageLoad"
                      />
                      <div class="pointer-events-none absolute inset-0 ring-1 ring-inset ring-white/25"></div>
                      <div class="pointer-events-none absolute left-1/3 top-0 h-full w-px bg-white/20"></div>
                      <div class="pointer-events-none absolute left-2/3 top-0 h-full w-px bg-white/20"></div>
                      <div class="pointer-events-none absolute left-0 top-1/3 h-px w-full bg-white/20"></div>
                      <div class="pointer-events-none absolute left-0 top-2/3 h-px w-full bg-white/20"></div>
                    </div>
                  </div>

                  <div class="flex min-w-0 flex-col justify-between gap-4">
                    <div>
                      <div class="mb-2 flex items-center justify-between gap-3">
                        <span class="text-sm font-medium text-gray-700 dark:text-gray-200">
                          {{ t('profile.avatar.zoom') }}
                        </span>
                        <span class="font-mono text-xs text-gray-500 dark:text-gray-400">
                          {{ Math.round(cropZoom * 100) }}%
                        </span>
                      </div>
                      <input
                        v-model.number="cropZoom"
                        type="range"
                        min="1"
                        max="3"
                        step="0.01"
                        class="w-full accent-primary-500"
                        @input="clampCropOffset"
                        @change="applyCrop"
                      />
                      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                        {{ t('profile.avatar.cropHint') }}
                      </p>
                    </div>
                    <div class="flex flex-wrap gap-2">
                      <button type="button" class="btn btn-primary btn-sm" @click="applyCrop">
                        {{ t('profile.avatar.applyCrop') }}
                      </button>
                      <button type="button" class="btn btn-secondary btn-sm" @click="resetCrop">
                        {{ t('profile.avatar.resetCrop') }}
                      </button>
                    </div>
                  </div>
                </div>
              </div>
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
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
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
const selectedSourceFileName = ref('')
const selectedFileUrl = ref('')
const cropSourceUrl = ref('')
const loading = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)
const cropFrameRef = ref<HTMLElement | null>(null)
const cropImageRef = ref<HTMLImageElement | null>(null)
const cropZoom = ref(1)
const cropOffset = ref({ x: 0, y: 0 })
const naturalSize = ref({ width: 0, height: 0 })
const isDragging = ref(false)
const dragStart = ref({ pointerId: 0, x: 0, y: 0, offsetX: 0, offsetY: 0 })

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

const cropFrameSize = computed(() => cropFrameRef.value?.clientWidth || 320)

const cropBaseScale = computed(() => {
  if (!naturalSize.value.width || !naturalSize.value.height) return 1
  return Math.max(cropFrameSize.value / naturalSize.value.width, cropFrameSize.value / naturalSize.value.height)
})

const cropScale = computed(() => cropBaseScale.value * cropZoom.value)

const cropImageStyle = computed(() => {
  if (!naturalSize.value.width || !naturalSize.value.height) {
    return {
      width: '100%',
      height: '100%',
      transform: 'translate(-50%, -50%)'
    }
  }
  return {
    width: `${naturalSize.value.width * cropScale.value}px`,
    height: `${naturalSize.value.height * cropScale.value}px`,
    transform: `translate(calc(-50% + ${cropOffset.value.x}px), calc(-50% + ${cropOffset.value.y}px))`
  }
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
  selectedSourceFileName.value = file.name
  cropSourceUrl.value = URL.createObjectURL(file)
  selectedFileUrl.value = cropSourceUrl.value
  resetCrop()
}

function clearSelectedFile(resetInput = true) {
  selectedFile.value = null
  selectedSourceFileName.value = ''
  naturalSize.value = { width: 0, height: 0 }
  cropZoom.value = 1
  cropOffset.value = { x: 0, y: 0 }
  revokeSelectedFileUrl()
  if (resetInput && fileInputRef.value) {
    fileInputRef.value.value = ''
  }
}

function revokeSelectedFileUrl() {
  const previewUrl = selectedFileUrl.value
  const sourceUrl = cropSourceUrl.value
  if (previewUrl) {
    URL.revokeObjectURL(previewUrl)
  }
  if (sourceUrl && sourceUrl !== previewUrl) {
    URL.revokeObjectURL(sourceUrl)
  }
  selectedFileUrl.value = ''
  cropSourceUrl.value = ''
}

function handleCropImageLoad() {
  const image = cropImageRef.value
  if (!image) return
  naturalSize.value = {
    width: image.naturalWidth,
    height: image.naturalHeight
  }
  nextTick(() => {
    clampCropOffset()
    void applyCrop()
  })
}

function resetCrop() {
  cropZoom.value = 1
  cropOffset.value = { x: 0, y: 0 }
  nextTick(() => {
    clampCropOffset()
    if (cropSourceUrl.value) void applyCrop()
  })
}

function clampCropOffset() {
  const displayWidth = naturalSize.value.width * cropScale.value
  const displayHeight = naturalSize.value.height * cropScale.value
  const frameSize = cropFrameSize.value
  const maxX = Math.max(0, (displayWidth - frameSize) / 2)
  const maxY = Math.max(0, (displayHeight - frameSize) / 2)
  cropOffset.value = {
    x: Math.min(maxX, Math.max(-maxX, cropOffset.value.x)),
    y: Math.min(maxY, Math.max(-maxY, cropOffset.value.y))
  }
}

function startDrag(event: PointerEvent) {
  if (!cropSourceUrl.value) return
  isDragging.value = true
  dragStart.value = {
    pointerId: event.pointerId,
    x: event.clientX,
    y: event.clientY,
    offsetX: cropOffset.value.x,
    offsetY: cropOffset.value.y
  }
  cropFrameRef.value?.setPointerCapture(event.pointerId)
  window.addEventListener('pointermove', dragCrop)
  window.addEventListener('pointerup', stopDrag, { once: true })
}

function dragCrop(event: PointerEvent) {
  if (!isDragging.value) return
  cropOffset.value = {
    x: dragStart.value.offsetX + event.clientX - dragStart.value.x,
    y: dragStart.value.offsetY + event.clientY - dragStart.value.y
  }
  clampCropOffset()
}

function stopDrag(event: PointerEvent) {
  if (!isDragging.value) return
  isDragging.value = false
  cropFrameRef.value?.releasePointerCapture(dragStart.value.pointerId)
  window.removeEventListener('pointermove', dragCrop)
  if (event.pointerId === dragStart.value.pointerId) {
    void applyCrop()
  }
}

async function applyCrop() {
  const image = cropImageRef.value
  if (!image || !cropSourceUrl.value || !naturalSize.value.width || !naturalSize.value.height) return

  const outputSize = 512
  const frameSize = cropFrameSize.value
  const sourceSize = frameSize / cropScale.value
  const sourceX = (naturalSize.value.width / 2) - (sourceSize / 2) - (cropOffset.value.x / cropScale.value)
  const sourceY = (naturalSize.value.height / 2) - (sourceSize / 2) - (cropOffset.value.y / cropScale.value)
  const canvas = document.createElement('canvas')
  canvas.width = outputSize
  canvas.height = outputSize
  const context = canvas.getContext('2d')
  if (!context) return

  context.drawImage(
    image,
    Math.max(0, sourceX),
    Math.max(0, sourceY),
    Math.min(naturalSize.value.width, sourceSize),
    Math.min(naturalSize.value.height, sourceSize),
    0,
    0,
    outputSize,
    outputSize
  )

  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.9))
  if (!blob) return

  if (selectedFileUrl.value && selectedFileUrl.value !== cropSourceUrl.value) {
    URL.revokeObjectURL(selectedFileUrl.value)
  }
  const fileName = selectedSourceFileName.value.replace(/\.[^.]+$/, '') || 'avatar'
  selectedFile.value = new File([blob], `${fileName}-cropped.jpg`, { type: 'image/jpeg' })
  selectedFileUrl.value = URL.createObjectURL(blob)
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
