<template>
  <div class="page">
    <div class="match-nav">
      <a-button type="text" @click="router.push(`${base}/events/${eventId}`)">
        <template #icon><icon-left /></template>返回组织详情
      </a-button>
    </div>

    <a-spin :loading="loading" style="width: 100%">
      <template v-if="data">
        <div class="match-head">
          <div>
            <div class="match-title-line">
              <h1 class="page-title">第 {{ data.match.gameNo }} 局</h1>
              <StateTag :status="data.match.status" kind="match" />
            </div>
            <p class="page-subtitle">
              {{ data.event.region }} · BO{{ data.event.bestOf }} ·
              系列比分 蓝 {{ data.event.series.blueWins }} : {{ data.event.series.redWins }} 红
              <span v-if="data.match.submittedAt"> · 提交于 {{ formatDateTime(data.match.submittedAt) }}</span>
            </p>
          </div>

          <div class="match-actions">
            <a-button :disabled="!data.match.editable" @click="swapTeams">
              <template #icon><icon-swap /></template>
              交换 1队 / 2队
            </a-button>
            <a-button :disabled="!data.match.editable" :loading="saving" @click="saveDraft">保存记录</a-button>
          </div>
        </div>

        <a-alert v-if="!data.match.editable" type="success" class="lock-alert">
          {{ data.event.isEnded ? '比赛已结束，记录已锁定并纳入排行榜，不能再修改。' : '只有组织者可以编辑本局记录。' }}
        </a-alert>

        <div class="winner-box">
          <span class="winner-label">本局胜方</span>
          <a-radio-group v-model="form.blueWin" type="button" :disabled="!data.match.editable">
            <a-radio :value="true">蓝色方胜</a-radio>
            <a-radio :value="false">红色方胜</a-radio>
          </a-radio-group>
          <span class="winner-label">时长</span>
          <a-input-number
            v-model="form.durationMinutes"
            :min="0"
            :max="1440"
            :disabled="!data.match.editable"
            style="width: 120px"
          >
            <template #suffix>分钟</template>
          </a-input-number>
        </div>

        <!-- 模块一：队伍 -->
        <div class="module-card card">
          <div class="module-title">
            <icon-user-group /> <span class="module-name">队伍与战绩</span>
            <span class="module-hint">左蓝右红固定；可交换 1队 / 2队 的红蓝方</span>
          </div>
          <div class="teams">
            <div v-for="side in ['blue', 'red']" :key="side" class="team-block" :class="side">
              <div class="team-head">
                <span>{{ teamName(side) }} · {{ side === 'blue' ? '蓝色方' : '红色方' }}</span>
                <span v-if="form.blueWin !== null" class="team-result">
                  {{ (form.blueWin === (side === 'blue')) ? '胜' : '负' }}
                </span>
              </div>

              <div v-for="row in form.players[side]" :key="row.position" class="team-row">
                <div class="row-main">
                  <PositionIcon :code="row.position" :size="22" class="pos-icon" />
                  <a-select
                    v-model="row.memberId"
                    placeholder="选择报名玩家"
                    allow-search
                    allow-clear
                    :disabled="!data.match.editable"
                    class="member-select"
                    @change="(value) => onMemberChange(row, value)"
                  >
                    <a-option v-for="reg in data.registrations" :key="reg.memberId" :value="reg.memberId">
                      <span class="option-player">
                        <PositionIcon v-if="reg.mainPosition" :code="reg.mainPosition" :size="16" />
                        {{ reg.nickname }}
                      </span>
                    </a-option>
                  </a-select>
                  <button
                    type="button"
                    class="champion-btn"
                    :disabled="!data.match.editable"
                    @click="openPicker(side, row, 'player')"
                  >
                    <ChampionIcon :champion="row.champion" :version="iconVersion" :size="30" />
                    <span>{{ championName(row.champion) || '选英雄' }}</span>
                  </button>
                </div>

                <div class="row-stats">
                  <label><span>K</span><a-input-number v-model="row.kills" size="mini" :min="0" :disabled="!data.match.editable" /></label>
                  <label><span>D</span><a-input-number v-model="row.deaths" size="mini" :min="0" :disabled="!data.match.editable" /></label>
                  <label><span>A</span><a-input-number v-model="row.assists" size="mini" :min="0" :disabled="!data.match.editable" /></label>
                  <label><span>补刀</span><a-input-number v-model="row.cs" size="mini" :min="0" :disabled="!data.match.editable" /></label>
                  <label><span>经济</span><a-input-number v-model="row.gold" size="mini" :min="0" :disabled="!data.match.editable" /></label>
                  <label><span>伤害</span><a-input-number v-model="row.damageToChampions" size="mini" :min="0" :disabled="!data.match.editable" /></label>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 模块二：BP -->
        <div v-if="data.event.hasBp" class="module-card card">
          <div class="module-title">
            <icon-close-circle /> <span class="module-name">BP 禁用</span>
            <span class="module-hint">组织者手动记录；rofl 录像不包含 Ban 数据</span>
          </div>
          <div class="teams">
            <div v-for="side in ['blue', 'red']" :key="`ban-${side}`" class="team-block" :class="side">
              <div class="team-head"><span>{{ side === 'blue' ? '蓝色方禁用' : '红色方禁用' }}</span></div>
              <div class="ban-grid">
                <button
                  v-for="(champion, index) in form.bans[side]"
                  :key="index"
                  type="button"
                  class="ban-slot"
                  :class="{ empty: !champion }"
                  :disabled="!data.match.editable"
                  @click="openBanPicker(side, index)"
                >
                  <ChampionIcon v-if="champion" :champion="champion" :version="iconVersion" :size="34" />
                  <icon-close v-else />
                  <span class="ban-name">{{ championName(champion) || '禁用' }}</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- 模块三：ROFL -->
        <div class="module-card card">
          <div class="module-title">
            <icon-file /> <span class="module-name">ROFL 录像战绩</span>
            <span class="module-hint">上传后自动读取 10 名玩家的 KDA / 补刀 / 经济 / 伤害</span>
          </div>

          <div class="rofl-actions">
            <input ref="fileInput" type="file" accept=".rofl" hidden @change="uploadRofl" />
            <a-button :disabled="!data.match.editable" :loading="uploading" @click="fileInput?.click()">
              <template #icon><icon-upload /></template>
              上传 .rofl 文件
            </a-button>
            <a-button v-if="rofl" :disabled="!data.match.editable" type="outline" @click="applyRofl">
              <template #icon><icon-download /></template>
              自动填入队伍模块
            </a-button>
            <span v-if="rofl" class="rofl-meta">
              {{ rofl.fileName }} · 版本 {{ rofl.gameVersion }} · {{ formatDuration(rofl.gameLengthMs) }} ·
              {{ rofl.blueWin ? '蓝色方胜' : '红色方胜' }}
            </span>
          </div>

          <div v-if="rofl?.players?.length" class="rofl-table">
            <div class="rofl-row rofl-header">
              <span>方</span><span>位置</span><span>英雄</span><span>玩家</span>
              <span>K/D/A</span><span>补刀</span><span>经济</span><span>伤害</span>
            </div>
            <div v-for="player in rofl.players" :key="`${player.team}-${player.slot}`" class="rofl-row">
              <span :class="['side-dot', player.team]">{{ player.team === 'blue' ? '蓝' : '红' }}</span>
              <span class="rofl-pos"><PositionIcon :code="player.position" :size="18" /></span>
              <span class="rofl-champion">
                <ChampionIcon :champion="player.champion" :version="iconVersion" :size="22" />
                {{ championName(player.champion) || player.champion }}
              </span>
              <span class="rofl-player">{{ player.playerName }}</span>
              <span>{{ player.kills }}/{{ player.deaths }}/{{ player.assists }}</span>
              <span>{{ player.cs }}</span>
              <span>{{ formatNumber(player.gold) }}</span>
              <span>{{ formatNumber(player.damageToChampions) }}</span>
            </div>
            <div class="rofl-note">录像从游戏加载开始记录，不包含 Ban/Pick 阶段，BP 需手动填写。</div>
          </div>
          <a-empty v-else description="还没有上传录像，可先手动填写或上传 rofl 自动填充" />
        </div>

        <ChampionPicker
          v-model:visible="pickerVisible"
          :model-value="pickerValue"
          title="选择英雄"
          @select="onChampionSelected"
        />
      </template>

      <a-result v-else-if="error" status="error" :title="error" />
    </a-spin>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import ChampionIcon from '../components/ChampionIcon.vue'
import ChampionPicker from '../components/ChampionPicker.vue'
import PositionIcon from '../components/PositionIcon.vue'
import StateTag from '../components/StateTag.vue'
import { api } from '../api'
import { session } from '../stores/session'
import { championName } from '../stores/champions'
import { formatDateTime, formatDuration, formatNumber } from '../utils/format'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const error = ref('')
const data = ref(null)
const saving = ref(false)
const uploading = ref(false)
const fileInput = ref(null)
const rofl = ref(null)
const pickerVisible = ref(false)
const pickerValue = ref('')
const pickerTarget = ref(null)

const POSITIONS = ['TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY']
const POSITION_LABEL = { TOP: '上单', JUNGLE: '打野', MIDDLE: '中单', BOTTOM: '下路', UTILITY: '辅助' }

const form = reactive({
  // 新对局默认不选胜方/时长，避免草稿里写入并不存在的比赛结果
  blueWin: null,
  durationMinutes: undefined,
  team1Side: 'blue',
  players: { blue: [], red: [] },
  bans: { blue: [], red: [] },
})

const base = computed(() => `/civil-war/${route.params.group}`)
const eventId = computed(() => route.params.eventId)
const gameNo = computed(() => route.params.gameNo)
const iconVersion = computed(() => rofl.value?.ddragonVersion || '')

function blankPlayers() {
  const result = {}
  for (const side of ['blue', 'red']) {
    result[side] = POSITIONS.map((position, index) => ({
      side,
      position,
      slot: index + 1,
      memberId: null,
      playerName: '',
      champion: '',
      kills: 0,
      deaths: 0,
      assists: 0,
      cs: 0,
      gold: 0,
      damageToChampions: 0,
      visionScore: 0,
    }))
  }
  return result
}

function positionLabel(code) {
  return POSITION_LABEL[code] || ''
}

function positionText(code) {
  return POSITION_LABEL[code] || ''
}

function onMemberChange(row, memberId) {
  const registration = data.value?.registrations?.find((item) => item.memberId === memberId)
  // 清空选择时同步清空名字，避免留下“幽灵玩家”
  row.playerName = registration?.nickname || ''
}

function openPicker(side, row, kind) {
  if (!data.value?.match?.editable) return
  pickerTarget.value = { side, row, kind }
  pickerValue.value = row.champion
  pickerVisible.value = true
}

function openBanPicker(side, index) {
  if (!data.value?.match?.editable) return
  pickerTarget.value = { side, index, kind: 'ban' }
  pickerValue.value = form.bans[side][index] || ''
  pickerVisible.value = true
}

function onChampionSelected(item) {
  const target = pickerTarget.value
  if (!target) return
  if (target.kind === 'ban') {
    form.bans[target.side][target.index] = item.key
  } else {
    target.row.champion = item.key
  }
}

function teamName(side) {
  const sideOfTeam1 = form.team1Side || 'blue'
  return side === sideOfTeam1 ? '1队' : '2队'
}

function swapTeams() {
  if (!data.value?.match?.editable) return
  const bluePlayers = form.players.blue.map((row) => ({ ...row, side: 'red' }))
  const redPlayers = form.players.red.map((row) => ({ ...row, side: 'blue' }))
  form.players.blue = redPlayers
  form.players.red = bluePlayers
  form.team1Side = form.team1Side === 'blue' ? 'red' : 'blue'
}

function applyMatch(payload) {
  data.value = payload
  rofl.value = payload.rofl
  form.blueWin = payload.match.blueWin ?? null
  form.durationMinutes = payload.match.durationMs ? Math.round(payload.match.durationMs / 60000) : undefined
  form.team1Side = payload.match.team1Side || 'blue'
  form.players = blankPlayers()
  form.bans = { blue: Array(5).fill(''), red: Array(5).fill('') }

  for (const player of payload.players || []) {
    const row = form.players[player.side]?.[player.slot - 1]
    if (row) {
      Object.assign(row, {
        memberId: player.memberId,
        playerName: player.playerName,
        champion: player.champion,
        kills: player.kills,
        deaths: player.deaths,
        assists: player.assists,
        cs: player.cs,
        gold: player.gold,
        damageToChampions: player.damageToChampions,
        visionScore: player.visionScore,
      })
    }
  }
  for (const ban of payload.bans || []) {
    if (form.bans[ban.side]) form.bans[ban.side][ban.slot - 1] = ban.champion
  }
}

async function prefillFromRoster() {
  if (data.value.players?.length) return
  try {
    const detail = await api.event(route.params.group, eventId.value)
    if (!detail.roster) return
    const team1Side = form.team1Side === 'red' ? 'red' : 'blue'
    const team2Side = team1Side === 'blue' ? 'red' : 'blue'
    for (const [rosterKey, side] of [['team1', team1Side], ['team2', team2Side]]) {
      for (const item of detail.roster[rosterKey] || []) {
        const row = form.players[side].find((entry) => entry.position === item.position)
        if (row) {
          row.memberId = item.memberId
          row.playerName = item.name
        }
      }
    }
  } catch (e) {
    // 预填失败不影响手动记录
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const payload = await api.match(route.params.group, eventId.value, gameNo.value)
    applyMatch(payload)
    await nextTick()
    await prefillFromRoster()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function buildPayload() {
  const players = []
  for (const side of ['blue', 'red']) {
    for (const row of form.players[side]) {
      players.push({
        side,
        position: row.position,
        slot: row.slot,
        memberId: row.memberId || null,
        playerName: row.playerName || '',
        champion: row.champion || '',
        kills: row.kills || 0,
        deaths: row.deaths || 0,
        assists: row.assists || 0,
        cs: row.cs || 0,
        gold: row.gold || 0,
        damageToChampions: row.damageToChampions || 0,
        visionScore: row.visionScore || 0,
      })
    }
  }
  const bans = []
  for (const side of ['blue', 'red']) {
    form.bans[side].forEach((champion, index) => {
      if (champion) bans.push({ side, slot: index + 1, champion })
    })
  }
  return {
    blueWin: form.blueWin,
    durationMs: form.durationMinutes ? form.durationMinutes * 60000 : null,
    team1Side: form.team1Side,
    players,
    bans,
  }
}

async function saveDraft() {
  saving.value = true
  try {
    const payload = await api.saveMatch(route.params.group, eventId.value, gameNo.value, buildPayload())
    applyMatch(payload)
    Message.success('记录已保存')
  } catch (e) {
    Message.error(e.message)
  } finally {
    saving.value = false
  }
}

async function uploadRofl(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.rofl')) {
    Message.warning('请选择 .rofl 格式的录像文件')
    return
  }
  uploading.value = true
  try {
    const result = await api.uploadRofl(route.params.group, eventId.value, gameNo.value, file)
    rofl.value = result.rofl
    // 录像解析结果会写回后端草稿；同步到表单，避免随后“保存草稿”用空值覆盖胜方/时长
    if (result.rofl.blueWin !== null && form.blueWin === null) {
      form.blueWin = Boolean(result.rofl.blueWin)
    }
    if (result.rofl.gameLengthMs && !form.durationMinutes) {
      form.durationMinutes = Math.round(result.rofl.gameLengthMs / 60000)
    }
    Message.success('录像已解析，可点击「自动填入队伍模块」')
  } catch (e) {
    Message.error(e.message)
  } finally {
    uploading.value = false
  }
}

function applyRofl() {
  if (!rofl.value?.players) return
  const unmatched = []
  for (const player of rofl.value.players) {
    const side = player.team
    if (!form.players[side]) continue
    let row = form.players[side].find((entry) => entry.position === player.position)
    if (!row) {
      row = form.players[side].find((entry) => entry.champion && entry.champion.toLowerCase() === String(player.champion).toLowerCase())
    }
    if (!row) {
      row = form.players[side].find((entry) => !entry.champion)
    }
    if (!row) {
      unmatched.push(player.playerName)
      continue
    }
    row.champion = player.champion
    row.kills = player.kills
    row.deaths = player.deaths
    row.assists = player.assists
    row.cs = player.cs
    row.gold = player.gold
    row.damageToChampions = player.damageToChampions
    row.damageTaken = player.damageTaken || 0
    row.visionScore = player.visionScore
  }
  if (rofl.value.blueWin !== null) {
    form.blueWin = Boolean(rofl.value.blueWin)
  }
  if (rofl.value.gameLengthMs) {
    form.durationMinutes = Math.round(rofl.value.gameLengthMs / 60000)
  }
  Message.success(unmatched.length ? `已填入，${unmatched.length} 人未匹配到位置` : '已按位置自动填入')
}

onMounted(load)
</script>

<style scoped>
.match-nav {
  margin-bottom: 4px;
}
.match-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.match-title-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.match-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.lock-alert {
  margin-bottom: 12px;
}
.winner-box {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 16px;
  margin-bottom: 16px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(29, 33, 41, 0.04);
}
.winner-label {
  font-size: 13px;
  color: var(--app-muted);
}
.module-card {
  padding: 18px;
  margin-bottom: 16px;
}
.module-title {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  font-size: 16px;
  font-weight: 700;
  margin-bottom: 14px;
}
.module-name {
  white-space: nowrap;
}
.module-hint {
  margin-left: 8px;
  font-size: 12px;
  font-weight: 400;
  color: var(--app-muted);
}
@media (max-width: 768px) {
  .module-hint {
    flex: 1 1 100%;
    margin-left: 0;
  }
}
.teams {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.team-block {
  border: 1px solid var(--app-border);
  border-radius: 12px;
  overflow: hidden;
}
.team-block.blue .team-head {
  background: #e8f0ff;
  color: #165dff;
}
.team-block.red .team-head {
  background: #ffece8;
  color: #d4380d;
}
.team-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  font-weight: 700;
  font-size: 14px;
}
.team-result {
  font-size: 12px;
}
.team-row {
  padding: 8px 10px;
  border-top: 1px solid #f2f3f5;
}
.row-main {
  display: flex;
  align-items: center;
  gap: 6px;
}
.pos-icon {
  flex: 0 0 auto;
}
.option-player {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.member-select {
  flex: 1;
  min-width: 0;
}
.champion-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fff;
  cursor: pointer;
  font-size: 12px;
  color: #4e5969;
  max-width: 116px;
}
.champion-btn span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.champion-btn:disabled {
  cursor: default;
  opacity: 0.7;
}
.row-stats {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 4px;
  margin-top: 6px;
}
.row-stats label {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 10px;
  color: var(--app-muted);
}
.ban-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 6px;
  padding: 10px;
}
.ban-slot {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 6px;
  border: 1px dashed var(--app-border);
  border-radius: 8px;
  background: #fafbfc;
  cursor: pointer;
  color: var(--app-muted);
  font-size: 10px;
}
.ban-slot.empty {
  color: #c9cdd4;
}
.ban-name {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rofl-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.rofl-meta {
  color: var(--app-muted);
  font-size: 12px;
}
.rofl-table {
  margin-top: 14px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  overflow: hidden;
}
.rofl-row {
  display: grid;
  grid-template-columns: 34px 52px 1.2fr 1.4fr 70px 56px 70px 80px;
  gap: 6px;
  align-items: center;
  padding: 6px 10px;
  font-size: 12px;
  border-top: 1px solid #f2f3f5;
}
.rofl-header {
  background: #f7f8fa;
  color: var(--app-muted);
  border-top: none;
}
.side-dot {
  font-weight: 700;
}
.side-dot.blue { color: #165dff; }
.side-dot.red { color: #d4380d; }
.rofl-champion {
  display: flex;
  align-items: center;
  gap: 4px;
  overflow: hidden;
}
.rofl-player {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rofl-note {
  padding: 8px 10px;
  background: #f7f8fa;
  color: var(--app-muted);
  font-size: 11px;
}
@media (max-width: 900px) {
  .teams {
    grid-template-columns: 1fr;
  }
  .row-stats {
    grid-template-columns: repeat(3, 1fr);
  }
  .rofl-row {
    grid-template-columns: 28px 46px 1fr 1fr 64px;
  }
  .rofl-row > span:nth-child(6),
  .rofl-row > span:nth-child(7),
  .rofl-row > span:nth-child(8) {
    display: none;
  }
}
</style>
