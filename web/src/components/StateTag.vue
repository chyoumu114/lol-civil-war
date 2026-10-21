<template>
  <a-tag :color="color" size="small">{{ label }}</a-tag>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: String, default: '' },
  kind: { type: String, default: 'event' },
})

const MAP = {
  event: {
    open: { label: '报名中', color: 'green' },
    closed: { label: '已截止', color: 'orange' },
    finished: { label: '已结束', color: 'gray' },
  },
  match: {
    draft: { label: '草稿', color: 'orange' },
    submitted: { label: '已提交', color: 'green' },
  },
}

const current = computed(() => MAP[props.kind]?.[props.status] || { label: props.status || '-', color: 'gray' })
const label = computed(() => current.value.label)
const color = computed(() => current.value.color)
</script>
