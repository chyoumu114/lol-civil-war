<template>
  <span class="star-rating" :class="{ compact }" :aria-label="`${value} 星`">
    <span class="stars-bg" aria-hidden="true">★★★★★</span>
    <span class="stars-fill" :style="{ width: fillPercent + '%' }" aria-hidden="true">★★★★★</span>
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  value: { type: Number, default: 0 },
  compact: { type: Boolean, default: false },
})

const fillPercent = computed(() => {
  const value = Math.min(5, Math.max(0, Number(props.value) || 0))
  return (value / 5) * 100
})
</script>

<style scoped>
.star-rating {
  position: relative;
  display: inline-block;
  line-height: 1;
  font-size: 18px;
  letter-spacing: 2px;
  vertical-align: middle;
  white-space: nowrap;
}
.star-rating.compact {
  font-size: 13px;
  letter-spacing: 1px;
}
.stars-bg {
  color: #34404f;
  text-shadow: 0 1px 0 rgba(0, 0, 0, 0.35);
}
.stars-fill {
  position: absolute;
  top: 0;
  left: 0;
  overflow: hidden;
  color: #ffd166;
  text-shadow: 0 0 8px rgba(255, 209, 102, 0.35);
}
</style>
