<template>
  <div class="rank-picker">
    <a-select
      :model-value="modelValue?.region"
      placeholder="大区"
      allow-search
      :options="regionOptions"
      @change="(value) => update({ region: value })"
    />
    <a-select
      :model-value="modelValue?.tier"
      placeholder="段位"
      :options="tierOptions"
      @change="onTierChange"
    />
    <a-radio-group
      v-if="hasDivisions"
      :model-value="modelValue?.division"
      type="button"
      class="division-group"
      @change="(value) => update({ division: value, lp: 0 })"
    >
      <a-radio :value="4">IV</a-radio>
      <a-radio :value="3">III</a-radio>
      <a-radio :value="2">II</a-radio>
      <a-radio :value="1">I</a-radio>
    </a-radio-group>
    <a-input-number
      v-else
      :model-value="modelValue?.lp"
      placeholder="分数"
      :min="0"
      :max="100000"
      class="lp-input"
      @change="(value) => update({ lp: value || 0, division: null })"
    >
      <template #suffix>分</template>
    </a-input-number>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: {
    type: Object,
    default: () => ({ region: '', tier: '', division: 4, lp: 0 }),
  },
  regions: { type: Array, default: () => [] },
  tiers: { type: Array, default: () => [] },
})
const emit = defineEmits(['update:modelValue'])

const regionOptions = computed(() => props.regions.map((item) => ({ label: item, value: item })))
const tierOptions = computed(() => props.tiers.map((item) => ({ label: item.label, value: item.code })))

const hasDivisions = computed(() => {
  const tier = props.tiers.find((item) => item.code === props.modelValue?.tier)
  return tier?.hasDivisions !== false
})

function update(patch) {
  emit('update:modelValue', { ...props.modelValue, ...patch })
}

function onTierChange(code) {
  const tier = props.tiers.find((item) => item.code === code)
  const divisioned = tier?.hasDivisions !== false
  update({
    tier: code,
    division: divisioned ? (props.modelValue?.division || 4) : null,
    lp: divisioned ? 0 : (props.modelValue?.lp || 0),
  })
}
</script>

<style scoped>
.rank-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  width: 100%;
}
.rank-picker :deep(.arco-select) {
  min-width: 110px;
  flex: 1 1 110px;
}
.division-group {
  flex: 0 0 auto;
}
.lp-input {
  width: 130px;
}
</style>
