<template>
  <div v-if="loadingSettings" class="min-h-screen bg-gray-50 dark:bg-dark-950"></div>
  <HomeView v-else-if="useCustomHomePricing" />
  <PublicPricingView v-else />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import HomeView from './HomeView.vue'
import PublicPricingView from './PublicPricingView.vue'

const route = useRoute()
const appStore = useAppStore()
const loadingSettings = ref(!appStore.publicSettingsLoaded)

const homeContent = computed(() => appStore.cachedPublicSettings?.home_content?.trim() || '')
const isHomeContentUrl = computed(() => {
  return homeContent.value.startsWith('http://') || homeContent.value.startsWith('https://')
})

const useCustomHomePricing = computed(() => {
  return route.query.ui_mode !== 'embedded' && !!homeContent.value && !isHomeContentUrl.value
})

onMounted(async () => {
  if (!appStore.publicSettingsLoaded) {
    await appStore.fetchPublicSettings()
  }
  loadingSettings.value = false
})
</script>
