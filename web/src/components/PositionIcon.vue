<template>
  <span
    class="position-icon"
    :class="[code?.toLowerCase(), { 'with-label': showLabel }]"
    :title="label"
    :style="{ width: size + 'px', height: size + 'px' }"
  >
    <img :src="src" :alt="label" loading="lazy" />
    <span v-if="showLabel" class="position-label">{{ label }}</span>
  </span>
</template>

<script setup>
import { computed } from 'vue'
import { session } from '../stores/session'

const props = defineProps({
  code: { type: String, default: '' },
  size: { type: Number, default: 24 },
  showLabel: { type: Boolean, default: false },
})

const label = computed(() => {
  if (!props.code) return ''
  return session.meta?.positions?.find((item) => item.code === props.code)?.label || props.code
})
// 图标在 2026-10-19 换过一轮，带版本参数绕过旧的 30 天静态缓存
const src = computed(() => `/positions/${(props.code || '').toLowerCase()}.png?v=2`)
</script>

<style scoped>
.position-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  vertical-align: middle;
}
.position-icon.with-label {
  width: auto !important;
  height: auto !important;
  gap: 5px;
}
.position-icon img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  filter: brightness(1.7) contrast(1.15) drop-shadow(0 0 6px rgba(212, 175, 55, 0.2));
}
.position-icon.with-label img {
  width: auto;
  height: 1em;
}
.position-label {
  font-size: 12px;
  color: var(--app-muted);
  white-space: nowrap;
}
</style>
