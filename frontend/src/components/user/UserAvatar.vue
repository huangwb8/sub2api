<template>
  <span
    class="relative inline-flex shrink-0 items-center justify-center overflow-hidden bg-gray-100 text-white ring-1 ring-black/5 dark:bg-dark-700 dark:ring-white/10"
    :class="[sizeClass, roundedClass]"
    :style="generatedStyle"
  >
    <img
      v-if="showImage"
      :src="imageUrl"
      :alt="altText"
      class="h-full w-full object-cover"
      @error="imageFailed = true"
    />
    <span v-else-if="avatarStyle === 'orbit_burst'" class="absolute inset-0 opacity-80">
      <span class="absolute left-1/2 top-1/2 h-[70%] w-[36%] -translate-x-1/2 -translate-y-1/2 rotate-45 rounded-full border-2 border-white/45"></span>
      <span class="absolute left-1/2 top-1/2 h-[36%] w-[70%] -translate-x-1/2 -translate-y-1/2 -rotate-12 rounded-full border-2 border-white/35"></span>
      <span class="absolute right-[18%] top-[22%] h-1.5 w-1.5 rounded-full bg-white/80"></span>
    </span>
    <span v-else-if="avatarStyle === 'pixel_patch'" class="absolute inset-0 grid grid-cols-3 grid-rows-3 gap-1 p-2 opacity-80">
      <span v-for="index in 9" :key="index" class="rounded bg-white/20" :style="{ opacity: String(0.18 + ((seed + index * 13) % 45) / 100) }"></span>
    </span>
    <span v-else-if="avatarStyle === 'paper_cut'" class="absolute inset-0">
      <span class="absolute -bottom-[12%] left-[-8%] h-[72%] w-[120%] -rotate-12 rounded-[45%] bg-white/25"></span>
      <span class="absolute -bottom-[22%] left-[10%] h-[66%] w-[110%] -rotate-6 rounded-[42%] bg-black/20"></span>
    </span>
    <span v-else-if="avatarStyle === 'aurora_ring'" class="absolute inset-0">
      <span class="absolute inset-[18%] rounded-full border-[6px] border-white/25"></span>
      <span class="absolute inset-[31%] rounded-full border-2 border-black/20"></span>
      <span class="absolute right-[20%] top-[18%] h-2 w-2 rounded-full bg-white/40"></span>
    </span>
    <span v-else class="absolute right-[16%] top-[16%] h-[18%] w-[18%] rounded-full bg-white/25"></span>
    <span v-if="!showImage" class="relative z-10 font-semibold leading-none" :class="textClass">{{ initials }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { User } from '@/types'

const props = withDefaults(defineProps<{
  user: User | null
  previewUrl?: string
  previewStyle?: User['avatar_style']
  previewType?: User['avatar_type']
  size?: 'sm' | 'md' | 'lg' | 'xl'
  rounded?: 'full' | 'xl' | '2xl'
}>(), {
  size: 'md',
  rounded: 'xl'
})

const imageFailed = ref(false)

watch(() => [props.user?.avatar_url, props.previewUrl, props.previewType], () => {
  imageFailed.value = false
})

const avatarType = computed(() => props.previewType || props.user?.avatar_type || 'generated')
const avatarStyle = computed(() => props.previewStyle || props.user?.avatar_style || 'classic_letter')
const imageUrl = computed(() => props.previewUrl || props.user?.avatar_url || '')
const showImage = computed(() => !imageFailed.value && avatarType.value !== 'generated' && imageUrl.value.trim() !== '')

const displayName = computed(() => props.user?.username || props.user?.email?.split('@')[0] || 'U')
const initials = computed(() => displayName.value.trim().slice(0, 2).toUpperCase() || 'U')
const altText = computed(() => `${displayName.value} avatar`)

const seed = computed(() => {
  const raw = `${props.user?.id || 0}|${props.user?.email || ''}|${displayName.value}`
  let hash = 0
  for (let i = 0; i < raw.length; i += 1) {
    hash = (hash * 31 + raw.charCodeAt(i)) >>> 0
  }
  return hash
})

const palettes = [
  ['#0f766e', '#14b8a6', '#99f6e4', '#042f2e'],
  ['#1d4ed8', '#38bdf8', '#dbeafe', '#082f49'],
  ['#be123c', '#fb7185', '#ffe4e6', '#4c0519'],
  ['#7c3aed', '#c084fc', '#f3e8ff', '#2e1065'],
  ['#c2410c', '#fb923c', '#ffedd5', '#431407']
]

const palette = computed(() => palettes[seed.value % palettes.length])

const generatedStyle = computed(() => {
  if (showImage.value) return undefined
  const [a, b, c, d] = palette.value
  if (avatarStyle.value === 'aurora_ring') {
    return { background: `radial-gradient(circle at 28% 18%, ${c}, ${b} 45%, ${a} 100%)` }
  }
  if (avatarStyle.value === 'orbit_burst') {
    return { background: d }
  }
  if (avatarStyle.value === 'pixel_patch') {
    return { background: `linear-gradient(145deg, ${a}, ${d})` }
  }
  if (avatarStyle.value === 'paper_cut') {
    return { background: c, color: '#ffffff' }
  }
  return { background: `linear-gradient(135deg, ${a}, ${b})` }
})

const sizeClass = computed(() => ({
  sm: 'h-8 w-8',
  md: 'h-10 w-10',
  lg: 'h-16 w-16',
  xl: 'h-24 w-24'
}[props.size]))

const textClass = computed(() => ({
  sm: 'text-xs',
  md: 'text-sm',
  lg: 'text-2xl',
  xl: 'text-4xl'
}[props.size]))

const roundedClass = computed(() => ({
  full: 'rounded-full',
  xl: 'rounded-xl',
  '2xl': 'rounded-2xl'
}[props.rounded]))
</script>
