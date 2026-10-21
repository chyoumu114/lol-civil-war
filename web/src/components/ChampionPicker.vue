<template>
  <a-modal
    v-model:visible="visible"
    :title="title"
    :width="760"
    :footer="false"
    unmount-on-close
    class="champion-picker-modal"
  >
    <a-space direction="vertical" fill size="medium">
      <a-input-search v-model="query" placeholder="搜索英雄：中文名 / 称号 / 外号 / 英文名" allow-clear />
      <a-radio-group v-model="position" type="button" size="small">
        <a-radio value="ALL">全部</a-radio>
        <a-radio v-for="item in session.meta?.positions || []" :key="item.code" :value="item.code">
          {{ item.label }}
        </a-radio>
      </a-radio-group>

      <a-spin :loading="champions.loading" style="width: 100%">
        <div class="champion-grid">
          <button
            v-for="item in filtered"
            :key="item.key"
            type="button"
            class="champion-cell"
            :class="{ selected: item.key === modelValue }"
            @click="choose(item)"
          >
            <ChampionIcon :champion="item.key" :version="champions.latestVersion" :size="48" />
            <span class="cell-name">{{ item.cn }}</span>
            <span class="cell-title">{{ item.title }}</span>
          </button>
        </div>
        <a-empty v-if="!filtered.length" description="没有匹配的英雄" />
      </a-spin>
    </a-space>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import ChampionIcon from './ChampionIcon.vue'
import { champions, loadChampions } from '../stores/champions'
import { session } from '../stores/session'

const props = defineProps({
  visible: { type: Boolean, default: false },
  modelValue: { type: String, default: '' },
  title: { type: String, default: '选择英雄' },
})
const emit = defineEmits(['update:visible', 'update:modelValue', 'select'])

const query = ref('')
const position = ref('ALL')

const visible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

const filtered = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  return champions.list.filter((item) => {
    if (position.value !== 'ALL' && !(item.positions || []).includes(position.value)) return false
    if (!keyword) return true
    const haystack = [item.cn, item.en, item.title, item.key, ...(item.aliases || [])].join(' ').toLowerCase()
    return haystack.includes(keyword)
  })
})

watch(
  () => props.visible,
  (value) => {
    if (value) loadChampions()
  }
)

function choose(item) {
  emit('update:modelValue', item.key)
  emit('select', item)
  visible.value = false
}
</script>

<style scoped>
.champion-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(92px, 1fr));
  gap: 8px;
  max-height: 56vh;
  overflow: auto;
  padding: 4px;
}
.champion-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 8px 4px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: #fafbfc;
  cursor: pointer;
  transition: all 0.15s;
}
.champion-cell:hover {
  border-color: var(--app-primary);
  background: #f2f6ff;
}
.champion-cell.selected {
  border-color: var(--app-primary);
  background: #e8f0ff;
}
.cell-name {
  font-size: 12px;
  color: var(--app-text);
  font-weight: 600;
}
.cell-title {
  font-size: 10px;
  color: var(--app-muted);
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
