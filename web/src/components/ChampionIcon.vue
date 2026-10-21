<template>
  <span class="champion-icon" :style="{ width: size + 'px', height: size + 'px' }">
    <img
      v-if="currentUrl && !failed"
      :src="currentUrl"
      :alt="name"
      loading="lazy"
      @error="onError"
    />
    <span v-else class="champion-fallback">{{ initial }}</span>
  </span>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { championIconUrls, championName } from '../stores/champions'

const props = defineProps({
  champion: { type: String, default: '' },
  version: { type: String, default: '' },
  size: { type: Number, default: 40 },
})

const urls = computed(() => championIconUrls(props.champion, props.version))
const index = ref(0)
const failed = ref(false)
const currentUrl = computed(() => urls.value[index.value] || '')
const name = computed(() => championName(props.champion) || props.champion || '')
const initial = computed(() => (name.value || '?').slice(0, 1))

watch(
  () => [props.champion, props.version],
  () => {
    index.value = 0
    failed.value = false
  }
)

function onError() {
  if (index.value < urls.value.length - 1) {
    index.value++
  } else {
    failed.value = true
  }
}
</script>

<style scoped>
.champion-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  overflow: hidden;
  background: #f2f3f5;
  flex: 0 0 auto;
}
.champion-icon img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.champion-fallback {
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 700;
}
</style>
