<template>
  <div class="page">
    <div class="champ-head">
      <a-button type="text" @click="router.push('/')"><template #icon><icon-left /></template>返回首页</a-button>
      <h1 class="page-title">英雄资料库</h1>
      <p class="page-subtitle">按位置筛选，支持中文名 / 称号 / 英文名 / 外号搜索。</p>
    </div>

    <a-space direction="vertical" fill class="champ-tools">
      <a-input-search v-model="query" placeholder="搜索英雄，如：寡妇、快乐风男、eve" allow-clear />
      <a-radio-group v-model="position" type="button">
        <a-radio value="ALL">全部</a-radio>
        <a-radio v-for="item in session.meta?.positions || []" :key="item.code" :value="item.code">
          <PositionIcon :code="item.code" :size="16" />
          {{ item.label }}
        </a-radio>
      </a-radio-group>
    </a-space>

    <a-spin :loading="champions.loading" style="width: 100%">
      <div class="champ-grid">
        <div v-for="item in filtered" :key="item.key" class="champ-card card">
          <ChampionIcon :champion="item.key" :version="champions.latestVersion" :size="56" />
          <div class="champ-info">
            <div class="champ-name">{{ item.cn }}</div>
            <div class="champ-title">{{ item.title }}</div>
            <div class="champ-en">{{ item.en }}</div>
            <div class="champ-pos">
              <PositionIcon v-for="pos in item.positions" :key="pos" :code="pos" :size="20" show-label />
            </div>
            <div v-if="item.aliases?.length" class="champ-alias">外号：{{ item.aliases.join(' / ') }}</div>
          </div>
        </div>
      </div>
      <a-empty v-if="!filtered.length && !champions.loading" description="没有匹配的英雄" />
    </a-spin>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import ChampionIcon from '../components/ChampionIcon.vue'
import PositionIcon from '../components/PositionIcon.vue'
import { champions, loadChampions } from '../stores/champions'
import { session, loadMeta } from '../stores/session'

const router = useRouter()
const query = ref('')
const position = ref('ALL')

const filtered = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  return champions.list.filter((item) => {
    if (position.value !== 'ALL' && !(item.positions || []).includes(position.value)) return false
    if (!keyword) return true
    const haystack = [item.cn, item.en, item.title, item.key, ...(item.aliases || [])].join(' ').toLowerCase()
    return haystack.includes(keyword)
  })
})

function positionLabel(code) {
  return session.meta?.positions?.find((item) => item.code === code)?.label || code
}

onMounted(async () => {
  await loadMeta()
  await loadChampions()
})
</script>

<style scoped>
.champ-head {
  margin-bottom: 8px;
}
.champ-tools {
  margin-bottom: 16px;
}
.champ-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}
.champ-card {
  display: flex;
  gap: 12px;
  padding: 14px;
}
.champ-info {
  min-width: 0;
}
.champ-name {
  font-size: 15px;
  font-weight: 700;
}
.champ-title {
  font-size: 12px;
  color: var(--app-muted);
}
.champ-en {
  font-size: 11px;
  color: #c9cdd4;
}
.champ-pos {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 6px;
}
.champ-alias {
  margin-top: 6px;
  font-size: 11px;
  color: var(--app-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
</style>
