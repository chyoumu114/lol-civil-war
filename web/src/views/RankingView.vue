<template>
  <div class="page ranking-page">
    <h1 class="page-title">排行榜</h1>
    <p class="page-subtitle">只统计组织者已结束的比赛，按群隔离；点击表头可按该指标倒序。</p>

    <a-spin :loading="loading" style="width: 100%">
      <div v-if="items.length" class="rank-table-wrap card">
        <table class="rank-table">
          <thead>
            <tr>
              <th>#</th>
              <th>玩家</th>
              <th class="sortable" @click="sortBy('tier')">梯级 <span v-if="sortKey === 'tier'">↓</span></th>
              <th class="sortable" @click="sortBy('kda')">KDA <span v-if="sortKey === 'kda'">↓</span></th>
              <th class="sortable" @click="sortBy('rating')">评分 <span v-if="sortKey === 'rating'">↓</span></th>
              <th class="sortable" @click="sortBy('games')">场次 <span v-if="sortKey === 'games'">↓</span></th>
              <th class="sortable" @click="sortBy('winRate')">胜率 <span v-if="sortKey === 'winRate'">↓</span></th>
              <th class="sortable" @click="sortBy('kills')">击杀 <span v-if="sortKey === 'kills'">↓</span></th>
              <th class="sortable" @click="sortBy('deaths')">死亡 <span v-if="sortKey === 'deaths'">↓</span></th>
              <th class="sortable" @click="sortBy('assists')">助攻 <span v-if="sortKey === 'assists'">↓</span></th>
              <th class="sortable" @click="sortBy('cs')">补刀 <span v-if="sortKey === 'cs'">↓</span></th>
              <th class="sortable" @click="sortBy('gold')">经济 <span v-if="sortKey === 'gold'">↓</span></th>
              <th class="sortable" @click="sortBy('damage')">总伤害 <span v-if="sortKey === 'damage'">↓</span></th>
              <th class="sortable" @click="sortBy('damageTaken')">抗伤 <span v-if="sortKey === 'damageTaken'">↓</span></th>
              <th class="sortable" @click="sortBy('visionScore')">视野分 <span v-if="sortKey === 'visionScore'">↓</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(item, index) in sortedItems" :key="item.memberId" @click="openPlayer(item.memberId)">
              <td class="rank-no" :class="{ top: index < 3 }">{{ index + 1 }}</td>
              <td class="rank-player-cell">
                <div class="player-inner">
                  <a-avatar :size="28" :image-url="item.avatar || ''">{{ (item.nickname || '?').slice(0, 1) }}</a-avatar>
                  <b>{{ item.nickname }}</b>
                </div>
              </td>
              <td>
                <img class="tier-icon" :src="`/t/T${tierOf(item.rating)}.png`" :alt="`T${tierOf(item.rating)}`" :title="`T${tierOf(item.rating)}`" />
              </td>
              <td class="kda-value">{{ Number(item.kda || 0).toFixed(1) }}</td>
              <td class="rating-cell">
                <div class="rating-inner">
                  <StarRating :value="item.rating" />
                  <span class="rating-count">（{{ item.ratingCount }}人评分）</span>
                </div>
              </td>
              <td>{{ item.games }}</td>
              <td>{{ item.winRate }}%</td>
              <td>{{ formatNumber(item.kills) }}</td>
              <td>{{ formatNumber(item.deaths) }}</td>
              <td>{{ formatNumber(item.assists) }}</td>
              <td>{{ formatNumber(item.cs) }}</td>
              <td>{{ formatNumber(item.gold) }}</td>
              <td>{{ formatNumber(item.damage) }}</td>
              <td>{{ formatNumber(item.damageTaken) }}</td>
              <td>{{ formatNumber(item.visionScore) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <a-empty v-else-if="!loading" description="还没有已结束的比赛数据" />
    </a-spin>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import StarRating from '../components/StarRating.vue'
import { api } from '../api'
import { formatNumber } from '../utils/format'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const items = ref([])
const sortKey = ref('rating')
const sortDir = ref('desc')

const base = computed(() => `/civil-war/${route.params.group}`)

const sortedItems = computed(() => {
  const key = sortKey.value
  return [...items.value].sort((a, b) => {
    if (key === 'tier') {
      return Number(b.rating || 0) - Number(a.rating || 0)
    }
    const left = Number(a[key] || 0)
    const right = Number(b[key] || 0)
    return sortDir.value === 'desc' ? right - left : left - right
  })
})

function tierOf(rating) {
  const value = Number(rating || 0)
  if (value >= 4.5) return 0
  if (value >= 3.5) return 1
  if (value >= 3.0) return 2
  if (value >= 2.0) return 3
  return 4
}

function sortBy(key) {
  sortKey.value = key
  sortDir.value = 'desc'
}

function openPlayer(memberId) {
  router.push(`${base.value}/players/${memberId}`)
}

async function load() {
  loading.value = true
  try {
    const data = await api.ranking(route.params.group, 'all')
    items.value = data.items || []
  } catch (error) {
    Message.error(error.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.rank-table-wrap {
  overflow-x: auto;
}
.rank-table {
  width: 100%;
  table-layout: fixed;
  border-collapse: collapse;
  font-size: 13px;
}
.rank-table th,
.rank-table td {
  padding: 10px 6px;
  text-align: left;
  vertical-align: middle;
  border-bottom: 1px solid var(--app-border);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.rank-table th {
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
  background: #0b1118;
  position: sticky;
  top: 0;
  z-index: 1;
}
.rank-table th.sortable {
  cursor: pointer;
  user-select: none;
}
.rank-table th.sortable:hover {
  color: #ff8a94;
}
.rank-table tbody tr {
  height: 70px;
  cursor: pointer;
}
.rank-table tbody tr:hover {
  background: #141c26;
}
.rank-no {
  width: 36px;
  font-size: 14px;
  font-weight: 700;
  color: #8b98a8;
}
.rank-no.top {
  color: #ff5b5b;
}
.player-inner {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.rank-player-cell b {
  max-width: 140px;
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.tier-icon {
  height: 30px;
  width: auto;
  display: block;
}
.kda-value {
  font-size: 14px;
  color: #7aa7ff;
  font-weight: 700;
}
.rating-inner {
  display: flex;
  align-items: center;
  gap: 8px;
}
.rating-count {
  color: var(--app-muted);
  font-size: 10px;
}
.rank-table th:nth-child(1) { width: 30px; }
.rank-table th:nth-child(2) { width: 130px; }
.rank-table th:nth-child(3) { width: 46px; }
.rank-table th:nth-child(4) { width: 46px; }
.rank-table th:nth-child(5) { width: 138px; }
.rank-table th:nth-child(6) { width: 44px; }
.rank-table th:nth-child(7) { width: 48px; }
.rank-table th:nth-child(8),
.rank-table th:nth-child(9),
.rank-table th:nth-child(10) { width: 44px; }
.rank-table th:nth-child(11) { width: 50px; }
.rank-table th:nth-child(12),
.rank-table th:nth-child(13) { width: 62px; }
.rank-table th:nth-child(14),
.rank-table th:nth-child(15) { width: 50px; }
</style>
