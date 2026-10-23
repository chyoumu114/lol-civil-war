<template>
  <svg class="tier-badge" viewBox="0 0 48 58" role="img" :aria-label="`T${safeTier}`">
    <title>T{{ safeTier }}</title>
    <path :d="outerPath" :fill="palette.fill" />
    <path :d="innerPath" fill="none" stroke="rgba(255,255,255,.32)" stroke-width="1.2" />
    <path :d="topFacet" fill="rgba(255,255,255,.10)" />
    <rect x="18" y="44" width="12" height="2" rx="1" fill="rgba(255,255,255,.35)" />
    <text
      x="24"
      y="37"
      text-anchor="middle"
      font-family="Arial, Helvetica, sans-serif"
      font-size="22"
      font-weight="800"
      fill="#fff"
    >T{{ safeTier }}</text>
  </svg>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  tier: { type: Number, default: 0 },
})

// 原创段位徽章：不复制任何第三方图标，只保留 T0-T4 的语义和配色层级。
const safeTier = computed(() => Math.min(4, Math.max(0, Number(props.tier) || 0)))
const outerPath = 'M10 1H19L24 6L29 1H38L47 10V35L24 57L1 35V10Z'
const innerPath = 'M12 4H18.5L24 9L29.5 4H36L44 12V33L24 52L4 33V12Z'
const topFacet = 'M10 1H19L24 6L29 1H38L47 10H1Z'
const palettes = [
  { fill: '#e04b5f' },
  { fill: '#2f7de1' },
  { fill: '#0aa88f' },
  { fill: '#e2a21a' },
  { fill: '#7f8ca3' },
]
const palette = computed(() => palettes[safeTier.value])
</script>

<style scoped>
.tier-badge {
  display: block;
  height: 30px;
  width: auto;
  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.25));
}
</style>
