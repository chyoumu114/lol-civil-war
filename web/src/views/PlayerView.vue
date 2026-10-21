<template>
  <div class="page">
    <a-spin :loading="loading" style="width: 100%">
      <template v-if="data">
        <div class="player-head card">
          <a-avatar :size="56">{{ (data.member.nickname || '?').slice(0, 1) }}</a-avatar>
          <div class="player-info">
            <div class="player-name">{{ data.member.nickname }}</div>
            <div class="player-meta">
              QQ {{ data.member.qqNumber || '—' }} ·
              <span class="member-positions">
                <template v-if="data.member.positions?.length">
                  <PositionIcon v-for="code in data.member.positions" :key="code" :code="code" :size="18" show-label />
                </template>
                <span v-else>未设置位置</span>
              </span>
            </div>
          </div>
          <div class="player-winrate">
            <div class="winrate-num">{{ data.summary.winRate }}%</div>
            <div class="winrate-label">胜率（{{ data.summary.wins }}/{{ data.summary.games }}）</div>
          </div>
        </div>

        <div class="stat-grid">
          <div class="stat-card card"><span>场次</span><b>{{ data.summary.games }}</b></div>
          <div class="stat-card card"><span>场均 KDA</span><b>{{ data.summary.avgKills }} / {{ data.summary.avgDeaths }} / {{ data.summary.avgAssists }}</b></div>
          <div class="stat-card card"><span>场均补刀</span><b>{{ data.summary.avgCs }}</b></div>
          <div class="stat-card card"><span>场均经济</span><b>{{ formatNumber(data.summary.avgGold) }}</b></div>
          <div class="stat-card card"><span>场均伤害</span><b>{{ formatNumber(data.summary.avgDamage) }}</b></div>
          <div class="stat-card card"><span>单场最高击杀</span><b>{{ data.summary.bestKills }}</b></div>
        </div>

        <a-row :gutter="16">
          <a-col :xs="24" :md="12">
            <a-card :bordered="false" class="section" title="最佳队友">
              <div v-if="data.teammates.best.length">
                <div v-for="mate in data.teammates.best" :key="mate.memberId" class="mate-row" @click="openPlayer(mate.memberId)">
                  <span class="mate-name">{{ mate.nickname }}</span>
                  <span class="mate-games">{{ mate.wins }}/{{ mate.games }} 胜</span>
                  <span class="mate-rate good">{{ mate.winRate }}%</span>
                </div>
              </div>
              <a-empty v-else description="同队场次不足 2 场" :image-style="{ height: '40px' }" />
            </a-card>
          </a-col>
          <a-col :xs="24" :md="12">
            <a-card :bordered="false" class="section" title="最差队友">
              <div v-if="data.teammates.worst.length">
                <div v-for="mate in data.teammates.worst" :key="mate.memberId" class="mate-row" @click="openPlayer(mate.memberId)">
                  <span class="mate-name">{{ mate.nickname }}</span>
                  <span class="mate-games">{{ mate.wins }}/{{ mate.games }} 胜</span>
                  <span class="mate-rate bad">{{ mate.winRate }}%</span>
                </div>
              </div>
              <a-empty v-else description="同队场次不足 2 场" :image-style="{ height: '40px' }" />
            </a-card>
          </a-col>
        </a-row>

        <a-card :bordered="false" class="section" title="最近对局">
          <div v-if="data.recent.length" class="recent-list">
            <div
              v-for="item in data.recent"
              :key="item.matchId"
              class="recent-row"
              @click="openMatch(item)"
            >
              <span class="recent-win" :class="item.win ? 'win' : 'lose'">{{ item.win ? '胜' : '负' }}</span>
              <span class="recent-champion">{{ championLabel(item.champion) }}</span>
              <span class="recent-pos"><PositionIcon :code="item.position" :size="18" /></span>
              <span class="recent-kda">{{ item.kills }}/{{ item.deaths }}/{{ item.assists }}</span>
              <span class="recent-cs">补刀 {{ item.cs }}</span>
              <span class="recent-dmg">伤害 {{ formatNumber(item.damage) }}</span>
              <span class="recent-region">{{ item.region }} · 第 {{ item.gameNo }} 局</span>
            </div>
          </div>
          <a-empty v-else description="暂无对局记录" />
        </a-card>

        <MatchRecordModal
          v-model:visible="matchModalVisible"
          :event-id="matchEventId"
          :game-no="matchGameNo"
          @changed="load"
        />
      </template>

      <a-result v-else-if="error" status="error" :title="error" />
    </a-spin>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import MatchRecordModal from '../components/MatchRecordModal.vue'
import PositionIcon from '../components/PositionIcon.vue'
import { api } from '../api'
import { session } from '../stores/session'
import { championName } from '../stores/champions'
import { formatNumber } from '../utils/format'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const error = ref('')
const data = ref(null)
const matchModalVisible = ref(false)
const matchEventId = ref(0)
const matchGameNo = ref(0)

const base = computed(() => `/civil-war/${route.params.group}`)

function positionLabel(code) {
  return session.meta?.positions?.find((item) => item.code === code)?.label || code || '—'
}

function championLabel(champion) {
  return championName(champion) || champion || '—'
}

function openPlayer(memberId) {
  router.push(`${base.value}/players/${memberId}`)
}

function openMatch(item) {
  matchEventId.value = item.eventId
  matchGameNo.value = item.gameNo
  matchModalVisible.value = true
}

async function load() {
  loading.value = true
  try {
    data.value = await api.player(route.params.group, route.params.memberId)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.player-head {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  margin-bottom: 16px;
}
.player-info {
  flex: 1;
  min-width: 0;
}
.player-name {
  font-size: 22px;
  font-weight: 800;
}
.player-meta {
  margin-top: 4px;
  color: var(--app-muted);
  font-size: 13px;
}
.player-winrate {
  text-align: right;
}
.winrate-num {
  font-size: 28px;
  font-weight: 800;
  color: var(--app-primary);
}
.winrate-label {
  color: var(--app-muted);
  font-size: 12px;
}
.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}
.stat-card {
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.stat-card span {
  color: var(--app-muted);
  font-size: 12px;
}
.stat-card b {
  font-size: 18px;
}
.section {
  margin-bottom: 16px;
}
.mate-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  border-bottom: 1px dashed #f2f3f5;
  cursor: pointer;
  font-size: 13px;
}
.mate-row:last-child {
  border-bottom: none;
}
.mate-name {
  flex: 1;
  font-weight: 600;
}
.mate-games {
  color: var(--app-muted);
  font-size: 12px;
}
.mate-rate {
  font-weight: 700;
    min-width: 56px;
  text-align: right;
}
.mate-rate.good { color: #00b42a; }
.mate-rate.bad { color: #f53f3f; }
.recent-list {
  display: flex;
  flex-direction: column;
}
.recent-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  border-bottom: 1px dashed #f2f3f5;
  font-size: 12px;
  cursor: pointer;
}
.recent-row:hover {
  background: #f7f8fa;
}
.recent-row:last-child {
  border-bottom: none;
}
.recent-win {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  border-radius: 6px;
  font-weight: 700;
  flex: 0 0 auto;
}
.recent-win.win { color: #00b42a; background: #e8ffea; }
.recent-win.lose { color: #f53f3f; background: #ffece8; }
.recent-champion {
  min-width: 64px;
  font-weight: 600;
}
.member-positions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.recent-pos,
.recent-cs,
.recent-dmg,
.recent-region {
  color: var(--app-muted);
  white-space: nowrap;
}
.recent-region {
  margin-left: auto;
}
@media (max-width: 640px) {
  .recent-dmg,
  .recent-region {
    display: none;
  }
}
</style>
