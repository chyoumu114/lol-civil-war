<template>
  <a-modal
    :visible="visible"
    title="编排阵容"
    :width="1080"
    :ok-loading="saving"
    ok-text="保存阵容"
    @ok="submit"
    @cancel="close"
  >
    <div class="editor-toolbar">
      <div class="editor-totals">
        <div class="total-side">
          <span class="total-name gold">1队</span>
          <PowerValue :value="totalPower('team1')" />
        </div>
        <span class="total-vs">VS</span>
        <div class="total-side right">
          <span class="total-name team2">2队</span>
          <PowerValue :value="totalPower('team2')" />
        </div>
        <span class="diff">差值 {{ Math.abs(totalPower('team1') - totalPower('team2')) }}</span>
      </div>
      <a-button type="primary" @click="openRecommendations">
        <template #icon><icon-swap /></template>
        推荐编排
      </a-button>
    </div>

    <a-alert type="info" :closable="false" class="editor-tip">
      选择玩家后实时计算战力；分路图标在中间，左右分别是 1队 / 2队。同一名玩家不能重复。
    </a-alert>

    <div class="editor-table">
      <div v-for="position in positions" :key="position.code" class="editor-row">
        <div class="editor-side left">
          <div class="editor-player">
            <a-avatar :size="30" :image-url="avatarFor('team1', position.code)">
              {{ initialFor('team1', position.code) }}
            </a-avatar>
            <a-select
              v-model="form.team1[position.code]"
              placeholder="选择报名者"
              allow-search
              allow-clear
              class="editor-select"
            >
              <a-option
                v-for="reg in candidates(position.code)"
                :key="reg.id"
                :value="reg.id"
                :disabled="isUsedElsewhere(reg.id, 'team1', position.code)"
              >
                <span class="option-player">
                  <a-avatar :size="20" :image-url="reg.avatar || ''">{{ (reg.nickname || '?').slice(0, 1) }}</a-avatar>
                  <span>{{ reg.nickname }}</span>
                </span>
              </a-option>
            </a-select>
          </div>
          <div v-if="selected('team1', position.code)" class="editor-meta">
            <span class="rank-kind">{{ rankKind('team1', position.code) }}</span>
            <RankTag :rank="rankFor('team1', position.code)" compact />
            <PowerValue :value="powerFor('team1', position.code)" compact />
          </div>
        </div>

        <div class="editor-role">
          <PositionIcon :code="position.code" :size="42" />
        </div>

        <div class="editor-side right">
          <div class="editor-player">
            <a-avatar :size="30" :image-url="avatarFor('team2', position.code)">
              {{ initialFor('team2', position.code) }}
            </a-avatar>
            <a-select
              v-model="form.team2[position.code]"
              placeholder="选择报名者"
              allow-search
              allow-clear
              class="editor-select"
            >
              <a-option
                v-for="reg in candidates(position.code)"
                :key="reg.id"
                :value="reg.id"
                :disabled="isUsedElsewhere(reg.id, 'team2', position.code)"
              >
                <span class="option-player">
                  <a-avatar :size="20" :image-url="reg.avatar || ''">{{ (reg.nickname || '?').slice(0, 1) }}</a-avatar>
                  <span>{{ reg.nickname }}</span>
                </span>
              </a-option>
            </a-select>
          </div>
          <div v-if="selected('team2', position.code)" class="editor-meta right">
            <span class="rank-kind">{{ rankKind('team2', position.code) }}</span>
            <RankTag :rank="rankFor('team2', position.code)" compact />
            <PowerValue :value="powerFor('team2', position.code)" compact />
          </div>
        </div>
      </div>
    </div>

    <div class="editor-foot">
      <span class="muted small">保存后会按服务端数据重新计算两队总战力并记录为当前套用方案。</span>
    </div>

    <!-- 推荐编排 -->
    <a-modal v-model:visible="recommendVisible" title="推荐编排" :footer="false" :width="980">
      <a-spin :loading="recommendLoading" style="width: 100%">
        <div v-if="plans.length" class="recommend-list">
          <div v-for="(plan, index) in plans" :key="index" class="recommend-card">
            <div class="recommend-head">
              <span class="recommend-title">方案 {{ index + 1 }}</span>
              <span class="muted small">差值 {{ plan.diff }}</span>
              <a-button size="mini" type="primary" @click="applyPlan(plan)">使用此方案</a-button>
            </div>
            <div class="recommend-teams">
              <div v-for="side in sides" :key="side" class="recommend-side" :class="side">
                <div class="recommend-side-head">
                  <b>{{ side === 'team1' ? '1队' : '2队' }}</b>
                  <PowerValue :value="side === 'team1' ? plan.team1Power : plan.team2Power" compact />
                </div>
                <div
                  v-for="item in plan[side]"
                  :key="item.registrationId"
                  class="recommend-row"
                  title="查看玩家详情"
                  @click="openRecommendPlayer(item)"
                >
                  <a-avatar :size="24" :image-url="item.avatar || ''">
                    {{ (item.name || '?').slice(0, 1) }}
                  </a-avatar>
                  <div class="recommend-info">
                    <div class="recommend-name-line">
                      <PositionIcon :code="item.position" :size="16" />
                      <span class="recommend-name">{{ item.name }}</span>
                    </div>
                    <div class="recommend-rank-line">
                      <span class="recommend-rank-kind">
                        {{ item.positionRankKind === 'main' ? '主选' : '补位' }}
                      </span>
                      <RankTag :rank="item.rank" compact />
                    </div>
                  </div>
                  <PowerValue :value="item.power" compact />
                </div>
              </div>
            </div>
          </div>
        </div>
        <a-empty v-else-if="!recommendLoading" description="暂无可行方案" />
      </a-spin>
    </a-modal>
    <PlayerInfoModal v-model:visible="playerInfoVisible" :player="selectedPlanPlayer" />
  </a-modal>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import PositionIcon from './PositionIcon.vue'
import PowerValue from './PowerValue.vue'
import RankTag from './RankTag.vue'
import PlayerInfoModal from './PlayerInfoModal.vue'
import { api } from '../api'
import { session } from '../stores/session'

const props = defineProps({
  visible: { type: Boolean, default: false },
  event: { type: Object, required: true },
  registrations: { type: Array, default: () => [] },
  roster: { type: Object, default: null },
})
const emit = defineEmits(['update:visible', 'applied'])

const sides = ['team1', 'team2']
const saving = ref(false)
const form = reactive({ team1: {}, team2: {} })
const recommendVisible = ref(false)
const recommendLoading = ref(false)
const plans = ref([])
const playerInfoVisible = ref(false)
const selectedPlanPlayer = ref(null)

const positions = computed(() => session.meta?.positions || [])
const participants = computed(() => props.registrations.filter((reg) => !reg.isSpectator))
const byId = computed(() => Object.fromEntries(participants.value.map((reg) => [reg.id, reg])))

watch(
  () => props.visible,
  (value) => {
    if (!value) return
    for (const side of sides) {
      form[side] = {}
      for (const position of positions.value) form[side][position.code] = null
    }
    if (props.roster) {
      for (const side of sides) {
        for (const item of props.roster[side] || []) {
          if (item.position) form[side][item.position] = item.registrationId
        }
      }
    }
    plans.value = []
    recommendVisible.value = false
  }
)

function candidates(positionCode) {
  return participants.value.filter(
    (reg) => reg.mainPosition === positionCode || (reg.subPositions || []).includes(positionCode)
  )
}

function selected(side, positionCode) {
  const id = form[side][positionCode]
  return id ? byId.value[id] || null : null
}

function rankFor(side, positionCode) {
  const reg = selected(side, positionCode)
  if (!reg) return null
  return reg.mainPosition === positionCode ? reg.rank : (reg.subRank || reg.rank)
}

function rankKind(side, positionCode) {
  const reg = selected(side, positionCode)
  if (!reg) return ''
  return reg.mainPosition === positionCode ? '主选段位' : '补位段位'
}

function powerFor(side, positionCode) {
  const reg = selected(side, positionCode)
  if (!reg) return 0
  return Number(reg.positionPowers?.[positionCode] || 0)
}

function totalPower(side) {
  return positions.value.reduce((sum, position) => sum + powerFor(side, position.code), 0)
}

function avatarFor(side, positionCode) {
  return selected(side, positionCode)?.avatar || ''
}

function initialFor(side, positionCode) {
  return (selected(side, positionCode)?.nickname || '?').slice(0, 1)
}

function isUsedElsewhere(registrationId, side, positionCode) {
  for (const otherSide of sides) {
    for (const position of positions.value) {
      if (otherSide === side && position.code === positionCode) continue
      if (form[otherSide][position.code] === registrationId) return true
    }
  }
  return false
}

function close() {
  emit('update:visible', false)
}

function openRecommendPlayer(item) {
  const reg = byId.value[item.registrationId]
    || participants.value.find((entry) => entry.memberId === item.memberId)
  if (!reg) return

  selectedPlanPlayer.value = {
    nickname: reg.nickname || item.name,
    avatar: reg.avatar || item.avatar,
    qqNumber: reg.qqNumber,
    rankLabel: reg.rank?.label || item.mainRankLabel,
    positions: [reg.mainPosition, ...(reg.subPositions || [])].filter(Boolean),
    note: reg.note,
    isOrganizer: reg.isOrganizer,
  }
  playerInfoVisible.value = true
}

function applyPlan(plan) {
  for (const side of sides) {
    for (const item of plan[side] || []) {
      form[side][item.position] = item.registrationId
    }
  }
  recommendVisible.value = false
  Message.success('已填充推荐编排，可继续调整')
}

async function openRecommendations() {
  recommendVisible.value = true
  recommendLoading.value = true
  plans.value = []
  try {
    const data = await api.autoMatch(session.group.key, props.event.id)
    plans.value = data.plans || []
  } catch (error) {
    Message.error(error.message)
  } finally {
    recommendLoading.value = false
  }
}

async function submit() {
  const unique = new Set()
  let incomplete = false
  for (const side of sides) {
    for (const position of positions.value) {
      const id = form[side][position.code]
      if (!id) {
        incomplete = true
        continue
      }
      if (unique.has(id)) {
        Message.warning('同一名玩家不能出现在多个位置或两队中')
        return
      }
      unique.add(id)
    }
  }

  saving.value = true
  try {
    const payload = {}
    for (const side of sides) {
      payload[side] = positions.value
        .filter((position) => form[side][position.code])
        .map((position) => ({
          registrationId: form[side][position.code],
          position: position.code,
        }))
    }
    await api.applyRoster(session.group.key, props.event.id, payload)
    Message.success(incomplete ? '阵容已保存（当前未满员）' : '阵容已保存')
    emit('applied')
    close()
  } catch (error) {
    Message.error(error.message)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.editor-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.editor-totals {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
}
.total-side {
  display: flex;
  align-items: center;
  gap: 6px;
}
.total-name {
  font-size: 15px;
  font-weight: 800;
}
.total-name.gold {
  color: #d4af37;
}
.total-name.team2 {
  color: #ff5b5b;
}
.total-vs {
  color: var(--app-muted);
  font-weight: 800;
}
.diff {
  color: var(--app-muted);
}
.editor-tip {
  margin-bottom: 12px;
  background: rgba(122, 167, 255, 0.08) !important;
  border: 1px solid rgba(122, 167, 255, 0.25) !important;
  color: #b9c5d3 !important;
}
.editor-tip :deep(.arco-alert-icon) {
  color: #7aa7ff !important;
}
.editor-table {
  border: 1px solid var(--app-border);
  border-radius: 12px;
  overflow: hidden;
}
.editor-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 72px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-top: 1px solid var(--app-border);
}
.editor-row:first-child {
  border-top: none;
}
.editor-side {
  min-width: 0;
}
.editor-side.left {
  text-align: right;
}
.editor-side.right {
  text-align: left;
}
.editor-player {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.editor-side.left .editor-player {
  flex-direction: row-reverse;
}
.editor-select {
  flex: 1;
  min-width: 0;
}
.editor-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
  flex-wrap: wrap;
}
.editor-meta.right {
  justify-content: flex-start;
}
.editor-meta:not(.right) {
  justify-content: flex-end;
}
.rank-kind {
  color: var(--app-muted);
  font-size: 10px;
}
.editor-role {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 54px;
  height: 54px;
  margin: 0 auto;
  border-radius: 50%;
  border: 1px solid var(--app-border);
  background: rgba(255, 70, 85, 0.06);
}
.option-player {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.editor-foot {
  margin-top: 12px;
}
.small {
  font-size: 12px;
}

.recommend-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 62vh;
  overflow-y: auto;
}
.recommend-card {
  border: 1px solid var(--app-border);
  border-radius: 10px;
  padding: 10px 12px;
  background: #0b1118;
}
.recommend-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}
.recommend-title {
  font-weight: 700;
}
.recommend-head .arco-btn {
  margin-left: auto;
}
.recommend-teams {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.recommend-side {
  border: 1px solid var(--app-border);
  border-radius: 8px;
  overflow: hidden;
}
.recommend-side-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 5px 8px;
  background: rgba(255, 255, 255, 0.04);
}
.recommend-side.team1 .recommend-side-head b {
  color: #7aa7ff;
}
.recommend-side.team2 .recommend-side-head b {
  color: #ff8a94;
}
.recommend-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-top: 1px solid var(--app-border);
  font-size: 12px;
  cursor: pointer;
  transition: background 0.15s;
}
.recommend-row:hover {
  background: rgba(255, 255, 255, 0.045);
}
.recommend-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.recommend-name-line {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
}
.recommend-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.recommend-rank-line {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
}
.recommend-rank-kind {
  color: var(--app-muted);
  font-size: 10px;
  flex: 0 0 auto;
}
.recommend-rank-line :deep(.rank-tag) {
  max-width: 100%;
}

@media (max-width: 768px) {
  .editor-row {
    grid-template-columns: minmax(0, 1fr) 44px minmax(0, 1fr);
    padding: 6px 4px;
  }
  .editor-role {
    width: 38px;
    height: 38px;
  }
  .editor-role :deep(.position-icon) {
    width: 30px !important;
    height: 30px !important;
  }
  .editor-player {
    gap: 4px;
  }
  .editor-player :deep(.arco-avatar) {
    width: 24px !important;
    height: 24px !important;
  }
  .recommend-teams {
    grid-template-columns: 1fr;
  }
}
</style>
