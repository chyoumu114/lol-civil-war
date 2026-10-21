<template>
  <div class="position-picker">
    <button
      v-for="item in options"
      :key="item.value"
      type="button"
      class="position-choice"
      :class="{ selected: isSelected(item.value), disabled: isDisabled(item.value) }"
      :disabled="isDisabled(item.value)"
      :title="item.label"
      @click="toggle(item.value)"
    >
      <PositionIcon :code="item.value" :size="34" />
    </button>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import PositionIcon from './PositionIcon.vue'

const props = defineProps({
  modelValue: { type: Array, default: () => [] },
  positions: { type: Array, default: () => [] },
  exclude: { type: Array, default: () => [] },
})
const emit = defineEmits(['update:modelValue'])

const options = computed(() =>
  props.positions.map((item) => ({
    label: item.label,
    value: item.code,
  }))
)

function isSelected(code) {
  return props.modelValue.includes(code)
}

function isDisabled(code) {
  return props.exclude.includes(code)
}

function toggle(code) {
  if (isDisabled(code)) return
  const next = isSelected(code)
    ? props.modelValue.filter((item) => item !== code)
    : [...props.modelValue, code]
  emit('update:modelValue', next)
}
</script>

<style scoped>
.position-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  padding: 2px 0;
}
.position-choice {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  padding: 4px;
  border: 1px solid var(--app-border);
  border-radius: 50%;
  background: #0b1118;
  cursor: pointer;
  transition: all 0.18s ease;
}
.position-choice:hover {
  border-color: rgba(255, 70, 85, 0.55);
  transform: translateY(-1px);
}
.position-choice.selected {
  border-color: #ff4655;
  background: rgba(255, 70, 85, 0.18);
  box-shadow: 0 0 0 2px rgba(255, 70, 85, 0.22), 0 0 16px rgba(255, 70, 85, 0.35);
}
.position-choice.disabled {
  opacity: 0.28;
  filter: grayscale(1);
  cursor: not-allowed;
  transform: none;
  box-shadow: none;
}
.position-choice.disabled:hover {
  border-color: var(--app-border);
}
</style>
