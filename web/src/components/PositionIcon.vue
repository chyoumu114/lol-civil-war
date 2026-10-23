<template>
  <span
    class="position-icon"
    :class="[normalized.toLowerCase(), { 'with-label': showLabel }]"
    :title="label"
    :style="{ width: size + 'px', height: size + 'px' }"
  >
    <svg
      v-if="normalized"
      class="position-glyph"
      :style="{ width: size + 'px', height: size + 'px' }"
      viewBox="0 0 24 24"
      role="img"
      :aria-label="label"
      focusable="false"
    >
      <title>{{ label }}</title>

      <!-- 上单：长剑 -->
      <g v-if="normalized === 'TOP'" fill="currentColor">
        <path d="M12 2.4 15.1 7v6.4H8.9V7L12 2.4Z" />
        <path d="M7.4 13.1h9.2v2H7.4z" />
        <path d="M10.7 15.1h2.6V19h-2.6z" />
        <path d="M9.1 19h5.8v2.2H9.1z" />
      </g>

      <!-- 打野：爪印 -->
      <g v-else-if="normalized === 'JUNGLE'" fill="currentColor">
        <ellipse cx="12" cy="15.4" rx="5.2" ry="4.4" />
        <circle cx="6.1" cy="9.6" r="2.1" />
        <circle cx="12" cy="6.8" r="2.2" />
        <circle cx="17.9" cy="9.6" r="2.1" />
      </g>

      <!-- 中单：菱形徽记 -->
      <g
        v-else-if="normalized === 'MIDDLE'"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linejoin="round"
      >
        <path d="M12 2.6 20 12l-8 9.4L4 12 12 2.6Z" />
        <path d="M12 7.3 16.2 12 12 16.7 7.8 12 12 7.3Z" fill="currentColor" stroke="none" />
      </g>

      <!-- 下路：弓与箭 -->
      <g
        v-else-if="normalized === 'BOTTOM'"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M4.4 19.6C6.8 11.8 12.2 6.4 20 4.4" />
        <path d="M4.4 19.6 19.6 4.4" />
        <path d="M13.6 4.4H19.6V10.4" />
      </g>

      <!-- 辅助：护盾与十字 -->
      <path
        v-else-if="normalized === 'UTILITY'"
        fill="currentColor"
        fill-rule="evenodd"
        d="M12 2.3 20 5v6.1c0 5-3.5 8.6-8 10.6C7.5 19.7 4 16.1 4 11.1V5l8-2.7Zm-1 4.9v3H8v2h3v3h2v-3h3v-2h-3v-3h-2Z"
      />

      <!-- 补位：四向箭头 -->
      <g v-else-if="normalized === 'ALL'" fill="currentColor">
        <path d="M12 2 15.2 5.2h-2.3v5h-1.8v-5H8.8L12 2Z" />
        <path d="M12 22 8.8 18.8h2.3v-5h1.8v5h2.3L12 22Z" />
        <path d="M2 12 5.2 8.8v2.3h5v1.8h-5v2.3L2 12Z" />
        <path d="M22 12 18.8 15.2v-2.3h-5v-1.8h5V8.8L22 12Z" />
        <circle cx="12" cy="12" r="1.4" />
      </g>

      <!-- 未知分路兜底 -->
      <circle
        v-else
        cx="12"
        cy="12"
        r="7.8"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
      />
    </svg>
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

const normalized = computed(() => (props.code || '').toUpperCase())
const label = computed(() => {
  if (!props.code) return ''
  if (normalized.value === 'ALL') return '补位'
  return session.meta?.positions?.find((item) => item.code === normalized.value)?.label || props.code
})
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
.position-glyph {
  display: block;
  flex: 0 0 auto;
  filter: drop-shadow(0 0 5px rgba(212, 175, 55, 0.16));
}
.position-icon.top { color: #e0b15a; }
.position-icon.jungle { color: #6fcf8e; }
.position-icon.middle { color: #6da8ff; }
.position-icon.bottom { color: #5fd0d0; }
.position-icon.utility { color: #c49bff; }
.position-icon.all { color: #aab4c5; }
.position-label {
  font-size: 12px;
  color: var(--app-muted);
  white-space: nowrap;
}
</style>
