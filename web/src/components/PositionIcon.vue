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
      viewBox="0 0 1024 1024"
      role="img"
      :aria-label="label"
      focusable="false"
    >
      <title>{{ label }}</title>

      <g v-if="normalized === 'TOP'">
        <path d="M81.14176 1024l186.13248-186.20416h570.5216V279.26528L1024 93.1328V1024H81.14176z" fill="#4B4B4B"></path>
        <path d="M372.33664 651.6736V372.39808H651.6736V651.6736H372.33664z" fill="#4B4B4B"></path>
        <path d="M186.20416 186.20416v558.53056L0 930.93888V0h907.6736L721.46944 186.20416H186.20416z" fill="#CCBA87"></path>
      </g>

      <g v-else-if="normalized === 'JUNGLE'">
        <path d="M595.36384 1024c-15.55456-166.07232-329.2672-232.74496-329.2672-232.74496-24.89344-260.11648-93.52192-394.73152-233.96352-605.06112C-108.2368-24.20736 442.88 551.36256 442.88 551.36256 480.4608 438.12864 328.4992 39.41376 328.4992 39.41376 732.44672 418.01728 595.36384 1024 595.36384 1024z m114.3808-472.63744C565.56544 297.1136 697.87648 191.08864 862.27968 0 734.06464 423.43424 709.74464 551.36256 709.74464 551.36256z m190.62784 78.76608c21.84192 145.27488-152.4736 377.67168-152.4736 236.31872 0-141.4144 66.84672-556.544 266.86464-590.77632-0.01024 0.01024-136.17152 209.18272-114.39104 354.4576z" fill="#CCBA87"></path>
      </g>

      <g v-else-if="normalized === 'MIDDLE'">
        <path d="M279.47008 1023.85664l171.81696-187.71968h384.84992V451.20512L1024 279.32672v744.52992H279.47008zM185.79456 187.00288v388.13696L0 760.9344V1.56672L744.81664 0.14336 573.93152 187.00288H185.79456z" fill="#4B4B4B"></path>
        <path d="M991.46752 163.03104L163.1744 991.25248 0.28672 1023.85664l32.60416-162.88768L861.11232 32.74752 1024 0.14336l-32.53248 162.88768z" fill="#CCBA87"></path>
      </g>

      <g v-else-if="normalized === 'BOTTOM'">
        <path d="M81.14176 1024l186.13248-186.20416h570.5216V279.26528L1024 93.1328V1024z" fill="#CCBA87"></path>
        <path d="M372.33664 651.6736V372.39808H651.6736V651.6736z" fill="#C9C9C9"></path>
        <path d="M186.20416 186.20416v558.53056L0 930.93888V0h907.6736L721.46944 186.20416z" fill="#C9C9C9"></path>
      </g>

      <g v-else-if="normalized === 'UTILITY'">
        <path d="M511.488 345.088S348.16 257.024 398.336 143.872c0 0 62.976-100.864 113.152 25.088 0 0 62.976-113.152 113.152-25.088 0 0 75.776 100.352-113.152 201.216zM310.272 395.264S272.384 320 20.992 307.2c0 0 239.104-62.976 364.544-12.8 0 0 88.064 62.976 0 188.416 0 0-25.088 50.176-163.328 50.176 0 0-37.888-37.888 37.888-62.976 75.264-24.576 50.176-74.752 50.176-74.752zM714.24 395.264S752.128 320 1003.52 307.2c0 0-239.104-62.976-364.544-12.8 0 0-88.064 62.976 0 188.416 0 0 25.088 50.176 163.328 50.176 0 0 37.888-37.888-37.888-62.976-75.264-24.576-50.176-74.752-50.176-74.752zM511.488 407.552S436.224 721.92 423.424 822.784c0 0 12.8 75.264 88.064 100.864 0 0 75.264-25.088 88.064-100.864 0 0-50.176-327.168-88.064-415.232z" fill="#CCBA87"></path>
      </g>

      <g v-else-if="normalized === 'ALL'">
        <path d="M512 128a42.666667 42.666667 0 0 1 42.666667 42.666667v682.666666a42.666667 42.666667 0 1 1-85.333334 0V170.666667a42.666667 42.666667 0 0 1 42.666667-42.666667z" fill="#CCBA87"></path>
        <path d="M311.168 311.168a42.666667 42.666667 0 0 1 60.330667 0l341.333333 341.333333a42.666667 42.666667 0 0 1-60.330667 60.330667l-341.333333-341.333333a42.666667 42.666667 0 0 1 0-60.330667z" fill="#CCBA87"></path>
        <path d="M128 512a42.666667 42.666667 0 0 1 42.666667-42.666667h682.666666a42.666667 42.666667 0 1 1 0 85.333334H170.666667a42.666667 42.666667 0 0 1-42.666667-42.666667z" fill="#CCBA87"></path>
        <path d="M311.168 712.832a42.666667 42.666667 0 0 1 0-60.330667l341.333333-341.333333a42.666667 42.666667 0 1 1 60.330667 60.330667l-341.333333 341.333333a42.666667 42.666667 0 0 1-60.330667 0z" fill="#CCBA87"></path>
        <path d="M256 768m-42.666667 0a42.666667 42.666667 0 1 0 85.333334 0 42.666667 42.666667 0 1 0-85.333334 0Z" fill="#CCBA87"></path>
        <path d="M256 256m-42.666667 0a42.666667 42.666667 0 1 0 85.333334 0 42.666667 42.666667 0 1 0-85.333334 0Z" fill="#CCBA87"></path>
        <path d="M768 256m-42.666667 0a42.666667 42.666667 0 1 0 85.333334 0 42.666667 42.666667 0 1 0-85.333334 0Z" fill="#CCBA87"></path>
        <path d="M768 768m-42.666667 0a42.666667 42.666667 0 1 0 85.333334 0 42.666667 42.666667 0 1 0-85.333334 0Z" fill="#CCBA87"></path>
      </g>

      <circle
        v-else
        cx="512"
        cy="512"
        r="360"
        fill="none"
        stroke="#CCBA87"
        stroke-width="64"
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
.position-label {
  font-size: 12px;
  color: var(--app-muted);
  white-space: nowrap;
}
</style>
