<script setup lang="ts">
import { ref, useTemplateRef, nextTick } from 'vue'

defineProps<{
  content?: string
}>()

const show = ref(false)
const triggerRef = useTemplateRef<HTMLElement>('trigger')
const tooltipRef = useTemplateRef<HTMLElement>('tooltip')
const tooltipStyle = ref({
  top: '0px',
  left: '0px',
  transform: 'translateX(-50%) translateY(-100%)',
  arrowLeft: '50%',
})
const tooltipPlacement = ref<'top' | 'bottom'>('top')

function onEnter() {
  show.value = true
  nextTick(updatePosition)
}

function onLeave() {
  show.value = false
}

function toggle() {
  show.value = !show.value
  if (show.value) {
    nextTick(updatePosition)
  }
}

function updatePosition() {
  const el = triggerRef.value
  if (!el) return

  const rect = el.getBoundingClientRect()

  const viewportPadding = 16
  const viewportWidth = window.innerWidth
  const tooltipWidth = Math.min(tooltipRef.value?.offsetWidth || 256, viewportWidth - viewportPadding * 2)
  const tooltipHeight = tooltipRef.value?.offsetHeight || 0
  const triggerCenter = rect.left + rect.width / 2
  const minLeft = viewportPadding + tooltipWidth / 2
  const maxLeft = viewportWidth - viewportPadding - tooltipWidth / 2
  const left = Math.min(Math.max(triggerCenter, minLeft), Math.max(minLeft, maxLeft))
  const hasSpaceAbove = rect.top >= tooltipHeight + viewportPadding + 8

  tooltipPlacement.value = hasSpaceAbove ? 'top' : 'bottom'
  tooltipStyle.value = {
    top: `${hasSpaceAbove ? rect.top - 8 : rect.bottom + 8}px`,
    left: `${left}px`,
    transform: hasSpaceAbove ? 'translateX(-50%) translateY(-100%)' : 'translateX(-50%)',
    arrowLeft: `${triggerCenter - left + tooltipWidth / 2}px`,
  }
}
</script>

<template>
  <div
    ref="trigger"
    class="group relative ml-1 inline-flex items-center align-middle"
    role="button"
    tabindex="0"
    @mouseenter="onEnter"
    @mouseleave="onLeave"
    @focusin="onEnter"
    @focusout="onLeave"
    @click="toggle"
    @keydown.enter.prevent="toggle"
    @keydown.space.prevent="toggle"
    @keydown.esc.prevent="onLeave"
  >
    <!-- Trigger Icon -->
    <slot name="trigger">
      <svg
        class="h-4 w-4 cursor-help text-gray-400 transition-colors hover:text-primary-600 dark:text-gray-500 dark:hover:text-primary-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="2"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    </slot>

    <!-- Teleport to body to escape modal overflow clipping -->
    <Teleport to="body">
      <div
        ref="tooltip"
        v-show="show"
        class="fixed z-[99999] w-64 max-w-[calc(100vw-2rem)] rounded-lg bg-gray-900 p-3 text-xs leading-relaxed text-white shadow-xl ring-1 ring-white/10 dark:bg-gray-800"
        :style="{ top: tooltipStyle.top, left: tooltipStyle.left, transform: tooltipStyle.transform }"
      >
        <slot>{{ content }}</slot>
        <div
          class="absolute h-2 w-2 -translate-x-1/2 rotate-45 bg-gray-900 dark:bg-gray-800"
          :class="tooltipPlacement === 'top' ? '-bottom-1' : '-top-1'"
          :style="{ left: tooltipStyle.arrowLeft }"
        ></div>
      </div>
    </Teleport>
  </div>
</template>
