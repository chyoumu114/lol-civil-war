<template>
  <a-modal
    :visible="visible"
    :title="`第 ${gameNo} 局`"
    :width="'clamp(1360px, 75vw, 1800px)'"
    :footer="false"
    unmount-on-close
    class="match-record-modal"
    @cancel="close"
  >
    <a-spin :loading="loading" style="width: 100%">
      <template v-if="data">
        <div class="scoreboard-top">
          <span>版本 {{ data.rofl?.gameVersion || '—' }}</span>
          <span><icon-clock-circle /> {{ durationText }}</span>
          <span :class="['win-text', data.match.blueWin === null ? '' : (data.match.blueWin ? 'blue' : 'red')]">
            {{ winnerText }}
          </span>
          <span>上传时间 {{ data.rofl?.uploadedAt ? formatDateTime(data.rofl.uploadedAt) : '—' }}</span>
          <div class="spacer" />
          <a-button v-if="editable && !editing" size="small" @click="startEdit">
            <template #icon><icon-edit /></template>
            编辑
          </a-button>
          <a-button v-if="editing" size="small" type="primary" :loading="saving" @click="completeEdit">
            完成
          </a-button>
          <a-button v-if="editable" size="small" status="danger" @click="removeMatch">
            删除记录
          </a-button>
        </div>

        <div class="board">
          <div class="board-head">
            <template v-for="side in sides" :key="`head-${side}`">
              <div class="team-header" :class="side">
                <div class="team-title-cell">
                  <div class="team-title">{{ side === 'blue' ? '蓝色方' : '红色方' }} <span>{{ teamName(side) }}</span></div>
                  <div class="team-result-slot">
                    <span
                      v-if="data.match.blueWin !== null && data.match.blueWin !== undefined"
                      class="match-result-circle"
                      :class="side === (data.match.blueWin ? 'blue' : 'red') ? 'win' : 'lose'"
                    >
                      {{ side === (data.match.blueWin ? 'blue' : 'red') ? '胜' : '负' }}
                    </span>
                  </div>
                </div>
                <div class="head-stat"><span>KDA</span><b>{{ sideStat(side, 'kills') }}/{{ sideStat(side, 'deaths') }}/{{ sideStat(side, 'assists') }}</b></div>
                <div class="head-stat"><span>伤害</span><b>{{ formatNumber(sideStat(side, 'damageToChampions')) }}</b></div>
                <div class="head-stat"><span>承伤</span><b>{{ formatNumber(sideStat(side, 'damageTaken')) }}</b></div>
                <div class="head-stat"><span>视野分</span><b>{{ sideStat(side, 'visionScore') }}</b></div>
                <div class="head-stat"><span>补刀数</span><b>{{ formatNumber(sideStat(side, 'cs')) }}</b></div>
                <div class="head-stat"><span>总经济</span><b>{{ formatNumber(sideGold(side)) }}</b><em v-if="goldDiff(side) > 0">(+{{ formatNumber(goldDiff(side)) }})</em></div>
              </div>
              <div v-if="side === 'blue'" class="center-head">
                <a-button size="mini" class="swap-btn" :disabled="!editable" @click="swapTeams">
                  <template #icon><icon-swap /></template>
                  交换队伍
                </a-button>
              </div>
            </template>
          </div>

          <div v-for="position in positions" :key="position.code" class="board-row">
            <template v-for="side in sides" :key="`${side}-${position.code}`">
              <div class="team-stat-row" :class="side">
                <template v-if="playerAt(side, position.code)">
                  <div class="champ-cell">
                    <button class="champ-line" :disabled="!editable" :title="championName(playerAt(side, position.code).champion) || '选择英雄'" @click="openPlayerPicker(side, position.code)">
                      <ChampionIcon :champion="playerAt(side, position.code).champion" :version="iconVersion" :size="44" />
                    </button>
                  </div>
                  <div class="player-cell">
                    <div class="player-top">
                      <a-avatar :size="22" :image-url="playerAt(side, position.code).memberAvatar || ''">
                        {{ nicknameAt(side, position.code).slice(0, 1) }}
                      </a-avatar>
                      <b>{{ nicknameAt(side, position.code) }}</b>
                    </div>
                    <div class="rank-line">
                      <span class="rank-kind">{{ rankKindForPlayer(side, position.code) }}</span>
                      <RankTag :rank="rankForPlayer(side, position.code)" compact />
                    </div>
                  </div>

                  <div class="stat-cell kda-cell">
                    <template v-if="editing">
                      <div class="kda-edit">
                        <a-input-number v-model="playerAt(side, position.code).kills" :min="0" size="mini" />
                        <span>/</span>
                        <a-input-number v-model="playerAt(side, position.code).deaths" :min="0" size="mini" />
                        <span>/</span>
                        <a-input-number v-model="playerAt(side, position.code).assists" :min="0" size="mini" />
                      </div>
                    </template>
                    <b v-else>{{ playerAt(side, position.code).kills }}/{{ playerAt(side, position.code).deaths }}/{{ playerAt(side, position.code).assists }}</b>
                  </div>

                  <div class="stat-cell">
                    <a-input-number v-if="editing" v-model="playerAt(side, position.code).damageToChampions" :min="0" size="mini" class="stat-input" />
                    <b v-else>{{ formatNumber(playerAt(side, position.code).damageToChampions) }}</b>
                    <div class="progress damage"><i :style="{ width: barWidth(playerAt(side, position.code).damageToChampions, maxDamage) }" /></div>
                  </div>

                  <div class="stat-cell">
                    <a-input-number v-if="editing" v-model="playerAt(side, position.code).damageTaken" :min="0" size="mini" class="stat-input" />
                    <b v-else>{{ formatNumber(playerAt(side, position.code).damageTaken) }}</b>
                    <div class="progress taken"><i :style="{ width: barWidth(playerAt(side, position.code).damageTaken, maxTaken) }" /></div>
                  </div>

                  <div class="stat-cell">
                    <a-input-number v-if="editing" v-model="playerAt(side, position.code).visionScore" :min="0" size="mini" class="stat-input" />
                    <b v-else>{{ playerAt(side, position.code).visionScore }}</b>
                  </div>

                  <div class="stat-cell">
                    <a-input-number v-if="editing" v-model="playerAt(side, position.code).cs" :min="0" size="mini" class="stat-input" />
                    <b v-else>{{ formatNumber(playerAt(side, position.code).cs) }}</b>
                  </div>

                  <div class="stat-cell">
                    <a-input-number v-if="editing" v-model="playerAt(side, position.code).gold" :min="0" size="mini" class="stat-input" />
                    <b v-else>{{ formatNumber(playerAt(side, position.code).gold) }}</b>
                  </div>
                </template>
                <template v-else>
                  <div class="champ-cell empty">—</div>
                  <div class="player-cell empty">—</div>
                  <div class="stat-cell">—</div>
                  <div class="stat-cell">—</div>
                  <div class="stat-cell">—</div>
                  <div class="stat-cell">—</div>
                  <div class="stat-cell">—</div>
                  <div class="stat-cell">—</div>
                </template>
              </div>
              <div v-if="side === 'blue'" class="center-cell">
                <PositionIcon :code="position.code" :size="34" />
              </div>
            </template>
          </div>
        </div>

        <div class="mobile-board">
          <div v-for="side in sides" :key="`mobile-${side}`" class="mobile-team" :class="side">
            <div class="mobile-team-head">
              <div class="team-title">{{ side === 'blue' ? '蓝色方' : '红色方' }} <span>{{ teamName(side) }}</span></div>
              <a-button v-if="side === 'blue'" size="mini" class="swap-btn" :disabled="!editable" @click="swapTeams">
                <template #icon><icon-swap /></template>
                交换队伍
              </a-button>
            </div>
            <div class="mobile-team-scroll">
            <div class="mobile-team-row mobile-table-head">
              <div class="mobile-pos"></div>
              <div class="mobile-content">
                <div class="mobile-identity mobile-identity-head">
                  <span
                    v-if="data.match.blueWin !== null && data.match.blueWin !== undefined"
                    class="match-result-circle"
                    :class="side === (data.match.blueWin ? 'blue' : 'red') ? 'win' : 'lose'"
                  >
                    {{ side === (data.match.blueWin ? 'blue' : 'red') ? '胜' : '负' }}
                  </span>
                </div>
                <div class="mobile-stats mobile-stats-head">
                  <div class="mobile-stat"><em>KDA</em><b>{{ sideStat(side, 'kills') }}/{{ sideStat(side, 'deaths') }}/{{ sideStat(side, 'assists') }}</b></div>
                  <div class="mobile-stat"><em>伤害</em></div>
                  <div class="mobile-stat"><em>承伤</em></div>
                  <div class="mobile-stat"><em>视野分</em></div>
                  <div class="mobile-stat"><em>补刀数</em></div>
                  <div class="mobile-stat"><em>总经济</em><b>{{ formatNumber(sideGold(side)) }}<em v-if="goldDiff(side) > 0" class="total-economy-diff">(+{{ formatNumber(goldDiff(side)) }})</em></b></div>
                </div>
              </div>
            </div>
            <div v-for="position in positions" :key="`mobile-${side}-${position.code}`" class="mobile-team-row">
              <div class="mobile-pos"><PositionIcon :code="position.code" :size="26" /></div>
              <div v-if="playerAt(side, position.code)" class="mobile-content">
                <div class="mobile-identity">
                  <button class="mobile-champ" :disabled="!editable" :title="championName(playerAt(side, position.code).champion) || '选择英雄'" @click="openPlayerPicker(side, position.code)">
                    <ChampionIcon :champion="playerAt(side, position.code).champion" :version="iconVersion" :size="44" />
                  </button>
                  <div class="mobile-player">
                    <div class="player-top">
                      <a-avatar :size="20" :image-url="playerAt(side, position.code).memberAvatar || ''">
                        {{ nicknameAt(side, position.code).slice(0, 1) }}
                      </a-avatar>
                      <b>{{ nicknameAt(side, position.code) }}</b>
                    </div>
                    <div class="rank-line">
                      <span class="rank-kind">{{ rankKindForPlayer(side, position.code) }}</span>
                      <RankTag :rank="rankForPlayer(side, position.code)" compact />
                    </div>
                  </div>
                </div>
                <div class="mobile-stats">
                  <div class="mobile-stat">
                    <div v-if="editing" class="kda-edit">
                      <a-input-number v-model="playerAt(side, position.code).kills" :min="0" size="mini" />
                      <span>/</span>
                      <a-input-number v-model="playerAt(side, position.code).deaths" :min="0" size="mini" />
                      <span>/</span>
                      <a-input-number v-model="playerAt(side, position.code).assists" :min="0" size="mini" />
                    </div>
                    <b v-else>{{ playerAt(side, position.code).kills }}/{{ playerAt(side, position.code).deaths }}/{{ playerAt(side, position.code).assists }}</b>
                  </div>
                  <div class="mobile-stat">
                    <a-input-number v-if="editing" v-model="playerAt(side, position.code).damageToChampions" :min="0" size="mini" class="stat-input" />
                    <b v-else>{{ formatNumber(playerAt(side, position.code).damageToChampions) }}</b>
                    <div class="progress damage"><i :style="{ width: barWidth(playerAt(side, position.code).damageToChampions, maxDamage) }" /></div>
                  </div>
                  <div class="mobile-stat">
                    <a-input-number v-if="editing" v-model="playerAt(side, position.code).damageTaken" :min="0" size="mini" class="stat-input" />
                    <b v-else>{{ formatNumber(playerAt(side, position.code).damageTaken) }}</b>
                    <div class="progress taken"><i :style="{ width: barWidth(playerAt(side, position.code).damageTaken, maxTaken) }" /></div>
                  </div>
                  <div class="mobile-stat">
                    <a-input-number v-if="editing" v-model="playerAt(side, position.code).visionScore" :min="0" size="mini" class="stat-input" />
                    <b v-else>{{ playerAt(side, position.code).visionScore }}</b>
                  </div>
                  <div class="mobile-stat">
                    <a-input-number v-if="editing" v-model="playerAt(side, position.code).cs" :min="0" size="mini" class="stat-input" />
                    <b v-else>{{ formatNumber(playerAt(side, position.code).cs) }}</b>
                  </div>
                  <div class="mobile-stat">
                    <a-input-number v-if="editing" v-model="playerAt(side, position.code).gold" :min="0" size="mini" class="stat-input" />
                    <b v-else>{{ formatNumber(playerAt(side, position.code).gold) }}</b>
                  </div>
                </div>
              </div>
              <div v-else class="mobile-content empty">—</div>
            </div>
            </div>
          </div>
        </div>

        <div class="winner-box">
          <span class="winner-label">本局胜方</span>
          <a-radio-group v-if="editing" v-model="data.match.blueWin" type="button">
            <a-radio :value="true">蓝色方胜</a-radio>
            <a-radio :value="false">红色方胜</a-radio>
          </a-radio-group>
          <b v-else :class="data.match.blueWin === null ? '' : (data.match.blueWin ? 'blue' : 'red')">{{ winnerText }}</b>
          <span class="winner-label">游戏时间</span>
          <b>{{ durationText }}</b>
        </div>

        <div v-if="data.event.hasBp" class="bp-section">
          <div class="section-title">BP 禁用</div>
          <div class="bp-grid">
            <div v-for="side in sides" :key="`bp-${side}`" class="bp-side" :class="side">
              <div class="bp-head">{{ side === 'blue' ? '蓝色方禁用' : '红色方禁用' }}</div>
              <div class="bp-slots">
                <button
                  v-for="index in 5"
                  :key="index"
                  class="ban-slot"
                  :class="{ empty: !banAt(side, index - 1) }"
                  :disabled="!editable"
                  @click="openBanPicker(side, index - 1)"
                >
                  <ChampionIcon v-if="banAt(side, index - 1)" :champion="banAt(side, index - 1)" :version="iconVersion" :size="68" />
                  <icon-close v-else />
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="rofl-section">
          <div class="section-title">录像</div>
          <div class="rofl-line">
            <span class="muted">{{ data.rofl ? `${data.rofl.fileName} · ${formatDateTime(data.rofl.uploadedAt)}` : '尚未上传 .rofl 录像' }}</span>
            <input ref="fileInput" type="file" accept=".rofl" hidden @change="onRoflFile" />
            <a-button size="mini" :loading="uploading" :disabled="!editable" @click="fileInput?.click()">
              上传录像
            </a-button>
          </div>
        </div>

        <div v-if="data.event.isEnded" class="rating-section">
          <div class="section-title">选手评分 <span class="muted small">（{{ data.canRate ? '点击星星评分' : '仅展示' }}）</span></div>
          <div class="rating-legend">
            <span v-for="score in 5" :key="score">{{ score }} {{ data.ratingLabels?.[score] }}</span>
          </div>
          <div class="rating-board">
            <div v-for="side in sides" :key="`rating-${side}`" class="rating-team" :class="side">
              <div class="rating-team-head">{{ side === 'blue' ? '蓝色方' : '红色方' }} <span>{{ teamName(side) }}</span></div>
              <div v-for="position in positions" :key="`rating-${side}-${position.code}`" class="rating-row">
                <template v-if="playerAt(side, position.code)">
                  <div class="rating-pos"><PositionIcon :code="position.code" :size="24" /></div>
                  <div class="rating-champ">
                    <ChampionIcon :champion="playerAt(side, position.code).champion" :version="iconVersion" :size="44" />
                  </div>
                  <div class="rating-player">
                    <div class="player-top">
                      <a-avatar :size="22" :image-url="playerAt(side, position.code).memberAvatar || ''">
                        {{ nicknameAt(side, position.code).slice(0, 1) }}
                      </a-avatar>
                      <b>{{ nicknameAt(side, position.code) }}</b>
                    </div>
                    <div class="rank-line">
                      <span class="rank-kind">{{ rankKindForPlayer(side, position.code) }}</span>
                      <RankTag :rank="rankForPlayer(side, position.code)" compact />
                    </div>
                  </div>
                  <div class="rating-stars">
                    <button
                      v-for="score in 5"
                      :key="score"
                      class="star-btn"
                      :class="{ active: ratingOf(playerAt(side, position.code).memberId).myScore >= score }"
                      :disabled="!data.canRate || playerAt(side, position.code).memberId === session.member?.id"
                      :title="data.ratingLabels?.[score]"
                      @click="rate(playerAt(side, position.code), score)"
                    >★</button>
                    <span class="rating-label">{{ ratingLabel(ratingOf(playerAt(side, position.code).memberId).myScore) }}</span>
                  </div>
                </template>
                <div v-else class="rating-empty">—</div>
              </div>
            </div>
          </div>
        </div>

        <ChampionPicker
          v-model:visible="pickerVisible"
          v-model="pickerChampion"
          :title="pickerTitle"
          @select="onChampionSelected"
        />
      </template>
    </a-spin>
  </a-modal>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import ChampionIcon from './ChampionIcon.vue'
import ChampionPicker from './ChampionPicker.vue'
import PositionIcon from './PositionIcon.vue'
import RankTag from './RankTag.vue'
import { api } from '../api'
import { session } from '../stores/session'
import { champions, championName, loadChampions } from '../stores/champions'
import { formatDateTime, formatDuration, formatNumber } from '../utils/format'

const props = defineProps({
  visible: { type: Boolean, default: false },
  eventId: { type: [String, Number], required: true },
  gameNo: { type: [String, Number], required: true },
})
const emit = defineEmits(['update:visible', 'changed'])

const loading = ref(false)
const saving = ref(false)
const uploading = ref(false)
const editing = ref(false)
const data = ref(null)
const fileInput = ref(null)
const pickerVisible = ref(false)
const pickerChampion = ref('')
const pickerTitle = ref('选择英雄')
const pickerTarget = ref(null)

const sides = ['blue', 'red']
const positions = computed(() => session.meta?.positions || [])
const editable = computed(() => Boolean(data.value?.match?.editable))
const iconVersion = computed(() => data.value?.rofl?.ddragonVersion || champions.latestVersion)
const maxDamage = computed(() => Math.max(1, ...(data.value?.players || []).map((player) => Number(player.damageToChampions || 0))))
const maxTaken = computed(() => Math.max(1, ...(data.value?.players || []).map((player) => Number(player.damageTaken || 0))))

const durationText = computed(() => {
  const ms = Number(data.value?.match?.durationMs || 0)
  return ms > 0 ? formatDuration(ms) : '—'
})
const winnerText = computed(() => {
  const win = data.value?.match?.blueWin
  if (win === null || win === undefined) return '未记录胜方'
  const side = win ? 'blue' : 'red'
  const team = (data.value?.match?.team1Side || 'blue') === side ? '1队' : '2队'
  return `${team} · 胜 · ${side === 'blue' ? '蓝方' : '红方'}`
})

function playerAt(side, position) {
  return data.value?.players?.find((player) => player.side === side && player.position === position) || null
}

function nicknameAt(side, position) {
  const player = playerAt(side, position)
  return player?.memberNickname || player?.playerName || '—'
}
function nicknameOf(player) {
  return player.memberNickname || player.playerName || '—'
}
function teamName(side) {
  return data.value?.match?.team1Side === side ? '1队' : '2队'
}
function sideStat(side, key) {
  return (data.value?.players || [])
    .filter((player) => player.side === side)
    .reduce((sum, player) => sum + Number(player[key] || 0), 0)
}
function sideGold(side) {
  return (data.value?.players || [])
    .filter((player) => player.side === side)
    .reduce((sum, player) => sum + (player.gold || 0), 0)
}
function goldDiff(side) {
  return sideGold(side) - sideGold(side === 'blue' ? 'red' : 'blue')
}
function rankForPlayer(side, position) {
  const player = playerAt(side, position)
  if (!player) return null
  const reg = (data.value?.registrations || []).find((item) => item.memberId === player.memberId)
  if (!reg) return null
  return reg.mainPosition === player.position ? reg.rank : (reg.subRank || reg.rank)
}
function rankKindForPlayer(side, position) {
  const player = playerAt(side, position)
  if (!player) return ''
  const reg = (data.value?.registrations || []).find((item) => item.memberId === player.memberId)
  if (!reg) return ''
  return reg.mainPosition === player.position ? '主选段位' : '补位段位'
}
function barWidth(value, max) {
  const amount = Number(value || 0)
  if (amount <= 0) return '0%'
  return `${Math.max(2, Math.round((amount / Math.max(1, max)) * 100))}%`
}
function banAt(side, index) {
  return data.value?.bans?.find((ban) => ban.side === side && ban.slot === index + 1)?.champion || ''
}
function ratingOf(memberId) {
  return data.value?.ratings?.[memberId] || { average: 0, count: 0, myScore: null }
}
function ratingLabel(score) {
  return score ? (data.value?.ratingLabels?.[score] || '') : ''
}

async function load() {
  if (!props.visible || !props.gameNo) return
  loading.value = true
  try {
    data.value = await api.match(session.group.key, props.eventId, props.gameNo)
    editing.value = false
  } catch (error) {
    Message.error(error.message)
    close()
  } finally {
    loading.value = false
  }
}

function close() {
  emit('update:visible', false)
  emit('changed')
}

function startEdit() {
  editing.value = true
}

function payloadFromData() {
  return {
    blueWin: data.value.match.blueWin,
    durationMs: data.value.match.durationMs,
    team1Side: data.value.match.team1Side,
    players: (data.value.players || []).map((player) => ({
      side: player.side,
      position: player.position,
      slot: player.slot,
      memberId: player.memberId,
      playerName: player.memberNickname || player.playerName || '',
      champion: player.champion || '',
      kills: Number(player.kills || 0),
      deaths: Number(player.deaths || 0),
      assists: Number(player.assists || 0),
      cs: Number(player.cs || 0),
      gold: Number(player.gold || 0),
      damageToChampions: Number(player.damageToChampions || 0),
      damageTaken: Number(player.damageTaken || 0),
      visionScore: Number(player.visionScore || 0),
    })),
    bans: (data.value.bans || []).map((ban) => ({ side: ban.side, slot: ban.slot, champion: ban.champion })),
  }
}

async function saveNow() {
  saving.value = true
  try {
    data.value = await api.saveMatch(session.group.key, props.eventId, props.gameNo, payloadFromData())
    emit('changed')
    return true
  } catch (error) {
    Message.error(error.message)
    return false
  } finally {
    saving.value = false
  }
}

async function completeEdit() {
  if (await saveNow()) editing.value = false
}

async function swapTeams() {
  if (!editable.value) return
  // 只交换「网站玩家」，英雄、KDA、经济、伤害、承伤、视野、补刀都留在原位置/原阵营。
  for (const position of positions.value) {
    const blue = playerAt('blue', position.code)
    const red = playerAt('red', position.code)
    if (!blue || !red) continue
    for (const field of ['memberId', 'memberNickname', 'memberAvatar', 'playerName']) {
      const tmp = blue[field]
      blue[field] = red[field]
      red[field] = tmp
    }
  }
  data.value.match.team1Side = data.value.match.team1Side === 'blue' ? 'red' : 'blue'
  await saveNow()
}

function openPlayerPicker(side, position) {
  pickerTarget.value = { kind: 'player', side, position }
  pickerChampion.value = playerAt(side, position)?.champion || ''
  pickerTitle.value = '选择英雄'
  pickerVisible.value = true
}

function openBanPicker(side, index) {
  pickerTarget.value = { kind: 'ban', side, index }
  pickerChampion.value = banAt(side, index)
  pickerTitle.value = '选择禁用英雄'
  pickerVisible.value = true
}

async function onChampionSelected() {
  if (!pickerTarget.value) return
  const target = pickerTarget.value
  if (target.kind === 'player') {
    const player = playerAt(target.side, target.position)
    if (player) player.champion = pickerChampion.value
  } else {
    const existing = data.value.bans.find((ban) => ban.side === target.side && ban.slot === target.index + 1)
    if (existing) existing.champion = pickerChampion.value
    else data.value.bans.push({ side: target.side, slot: target.index + 1, champion: pickerChampion.value })
  }
  pickerTarget.value = null
  await saveNow()
}

async function onRoflFile(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return
  uploading.value = true
  try {
    await api.uploadRofl(session.group.key, props.eventId, props.gameNo, file)
    Message.success('录像已解析，胜方、时长和战绩已自动填入')
    await load()
    emit('changed')
  } catch (error) {
    Message.error(error.message)
  } finally {
    uploading.value = false
  }
}

function removeMatch() {
  Modal.confirm({
    title: '删除对局记录',
    content: '删除后该局录像、战绩和评分都会一并删除，且不可恢复。确定删除吗？',
    okText: '删除',
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      try {
        await api.deleteMatch(session.group.key, props.eventId, props.gameNo)
        Message.success('对局记录已删除')
        emit('changed')
        emit('update:visible', false)
      } catch (error) {
        Message.error(error.message)
      }
    },
  })
}

async function rate(player, score) {
  try {
    const result = await api.rateMatch(session.group.key, props.eventId, props.gameNo, player.memberId, score)
    data.value.ratings[player.memberId] = result
    Message.success(`已评分：${data.value.ratingLabels[score]}`)
  } catch (error) {
    Message.error(error.message)
  }
}

watch(
  () => [props.visible, props.gameNo],
  () => {
    if (props.visible) {
      loadChampions()
      load()
    }
  }
)
onMounted(() => {
  if (props.visible) {
    loadChampions()
    load()
  }
})
</script>

<style scoped>
.match-record-modal :deep(.arco-modal-body) {
  max-height: 82vh;
  overflow-y: auto;
}
.scoreboard-top {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
  padding: 8px 10px;
  border-radius: 8px;
  background: #0b1118;
  border: 1px solid var(--app-border);
  color: var(--app-muted);
  font-size: 12px;
}
.win-text.blue {
  color: #7aa7ff;
}
.win-text.red {
  color: #ff8a94;
}
.board {
  border: 1px solid var(--app-border);
  border-radius: 12px;
  overflow-x: auto;
}
.mobile-board {
  display: none;
}
.board-head,
.board-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 54px minmax(0, 1fr);
  align-items: stretch;
  min-width: 980px;
}
.team-header,
.team-stat-row {
  display: grid;
  grid-template-columns: minmax(64px, 0.8fr) minmax(112px, 1.3fr) repeat(6, minmax(0, 1fr));
  gap: 4px;
  align-items: center;
}
.team-header {
  padding: 6px 8px;
  border-bottom: 1px solid var(--app-border);
}
.team-header.blue {
  background: rgba(22, 93, 255, 0.14);
}
.team-header.red {
  background: rgba(255, 70, 85, 0.13);
}
.team-title-cell {
  display: flex;
  align-items: center;
  gap: 0;
  min-width: 0;
  grid-column: span 2;
}
.team-title {
  flex: 0 0 auto;
}
.team-result-slot {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
  min-width: 30px;
}
.match-result-circle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  flex: 0 0 auto;
  border: 2px solid currentColor;
  border-radius: 50%;
  font-size: 14px;
  font-weight: 800;
  line-height: 1;
  background: transparent;
}
.match-result-circle.win {
  color: #22c55e;
}
.match-result-circle.lose {
  color: #ff4655;
}
.team-title {
  font-size: 14px;
  font-weight: 800;
  white-space: nowrap;
}
.team-title span {
  color: var(--app-muted);
  font-size: 10px;
  font-weight: 500;
  margin-left: 3px;
}
.head-stat {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 1px;
  min-width: 0;
}
.head-stat span {
  color: var(--app-muted);
  font-size: 8px;
  white-space: nowrap;
}
.head-stat b {
  color: #e8edf5;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.head-stat em {
  color: #4ade80;
  font-style: normal;
  font-size: 9px;
  white-space: nowrap;
}
.center-head {
  display: flex;
  align-items: center;
  justify-content: center;
  border-left: 1px solid var(--app-border);
  border-right: 1px solid var(--app-border);
  background: #0b1118;
}
.swap-btn {
  font-size: 9px;
  padding: 0 4px;
}
.board-row {
  border-top: 1px solid var(--app-border);
}
.team-stat-row {
  padding: 6px 8px;
  min-width: 0;
}
.team-stat-row.left {
  border-right: 1px solid var(--app-border);
}
.team-stat-row.right {
  border-left: 1px solid var(--app-border);
}
.center-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0b1118;
  border-left: 1px solid var(--app-border);
  border-right: 1px solid var(--app-border);
}
.champ-cell {
  display: flex;
  align-items: flex-start;
  min-width: 0;
}
.champ-cell .champ-line {
  flex-direction: column;
  align-items: flex-start;
  gap: 1px;
  width: 100%;
}
.champ-cell :deep(.champion-icon) {
  width: 44px !important;
  height: 44px !important;
  border-radius: 50%;
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.12);
}
.player-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-width: 0;
}
.rank-line {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 1px;
  min-width: 0;
  max-width: 100%;
}
.rank-kind {
  color: var(--app-muted);
  font-size: 9px;
  line-height: 1.3;
  white-space: nowrap;
}
.mobile-player .rank-line {
  align-items: center;
  gap: 3px;
  padding-top: 1px;
  width: 100%;
}
.rating-player .rank-line {
  align-items: flex-start;
  gap: 3px;
  box-sizing: border-box;
  width: 100%;
  padding: 1px 0 0 26px;
}
.rank-line :deep(.rank-tag) {
  min-width: 0;
}
.player-cell.empty {
  color: var(--app-muted);
  font-size: 12px;
}
.player-top {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}
.player-top b {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
}
.player-cell :deep(.rank-tag) {
  align-self: flex-start;
  transform: scale(0.92);
  transform-origin: left center;
}
.champ-line {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--app-muted);
  font-size: 9px;
  cursor: pointer;
  text-align: left;
}
.champ-line:disabled {
  cursor: default;
}
.champ-name-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.stat-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-width: 0;
  font-size: 10px;
}
.stat-cell b {
  color: #d8e2ef;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.stat-input {
  width: 100%;
}
.kda-edit {
  display: flex;
  align-items: center;
  gap: 1px;
}
.kda-edit :deep(.arco-input-number) {
  width: 34px;
}
.kda-edit span {
  color: var(--app-muted);
  font-size: 10px;
}
.progress {
  width: 100%;
  height: 3px;
  border-radius: 999px;
  background: #1a2430;
  overflow: hidden;
}
.progress i {
  display: block;
  height: 100%;
  border-radius: inherit;
}
.progress.damage i {
  background: linear-gradient(90deg, #ff4655, #ff8a94);
}
.progress.taken i {
  background: linear-gradient(90deg, #f8fafc, #8b98a8);
}
.winner-box {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 10px;
  padding: 10px 12px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: #0b1118;
}
.winner-label {
  color: var(--app-muted);
  font-size: 12px;
}
.winner-box .blue {
  color: #7aa7ff;
}
.winner-box .red {
  color: #ff8a94;
}
.section-title {
  font-size: 14px;
  font-weight: 700;
  margin-bottom: 8px;
}
.bp-section,
.rofl-section,
.rating-section {
  margin-top: 14px;
}
.bp-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.bp-side {
  border: 1px solid var(--app-border);
  border-radius: 8px;
  overflow: hidden;
}
.bp-head {
  padding: 6px 9px;
  font-size: 12px;
  font-weight: 700;
}
.bp-side.blue .bp-head {
  background: rgba(22, 93, 255, 0.14);
  color: #7aa7ff;
}
.bp-side.red .bp-head {
  background: rgba(255, 70, 85, 0.13);
  color: #ff8a94;
}
.bp-slots {
  display: flex;
  gap: 10px;
  padding: 8px;
  flex-wrap: wrap;
}
.ban-slot {
  width: 72px;
  height: 72px;
  border: 1px dashed var(--app-border);
  border-radius: 8px;
  background: #0b1118;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--app-muted);
}
.ban-slot.empty {
  background: #080d13;
}
.rofl-line {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 10px 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #0b1118;
}
.rating-legend {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 6px;
  color: var(--app-muted);
  font-size: 10px;
}
.rating-board {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.rating-team {
  border: 1px solid var(--app-border);
  border-radius: 10px;
  overflow: hidden;
}
.rating-team-head {
  padding: 7px 10px;
  background: #0b1118;
  border-bottom: 1px solid var(--app-border);
  font-size: 12px;
  font-weight: 700;
}
.rating-team.blue .rating-team-head {
  background: rgba(22, 93, 255, 0.16);
}
.rating-team.red .rating-team-head {
  background: rgba(255, 70, 85, 0.14);
}
.rating-row {
  display: grid;
  grid-template-columns: 28px 64px minmax(110px, 1.25fr) minmax(150px, 1.5fr);
  align-items: center;
  gap: 6px;
  padding: 7px 9px;
  border-top: 1px solid var(--app-border);
  font-size: 12px;
}
.rating-row:first-of-type {
  border-top: none;
}
.rating-pos {
  display: flex;
  align-items: center;
  justify-content: center;
}
.rating-champ {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  min-width: 0;
  color: var(--app-muted);
  font-size: 9px;
}
.rating-champ :deep(.champion-icon) {
  width: 44px !important;
  height: 44px !important;
  border-radius: 50%;
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.12);
}
.rating-champ span {
  max-width: 64px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rating-player {
  min-width: 0;
}
.rating-stars {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  flex-wrap: nowrap;
  width: 100%;
}
.rating-empty {
  grid-column: 1 / -1;
  color: var(--app-muted);
  font-size: 11px;
}
.star-btn {
  flex: 1;
  border: none;
  background: transparent;
  color: #34404f;
  font-size: 24px;
  line-height: 1;
  cursor: pointer;
  padding: 0;
  text-align: center;
}
.star-btn.active {
  color: #ffd166;
  text-shadow: 0 0 8px rgba(255, 209, 102, 0.4);
}
.star-btn:disabled {
  cursor: default;
  opacity: 0.55;
}
.rating-label {
  flex: 0 0 28px;
  color: #ffb347;
  font-size: 10px;
  text-align: right;
}
.small {
  font-size: 11px;
}
@media (max-width: 900px) {
  .board {
    display: none;
  }
  .mobile-board {
    display: block;
  }
  .mobile-team {
    border: 1px solid var(--app-border);
    border-radius: 12px;
    overflow: hidden;
  }
  .mobile-team-scroll {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    touch-action: pan-x pan-y;
  }
  .mobile-table-head {
    padding-top: 5px;
    padding-bottom: 4px;
    background: #0f1620;
  }
  .mobile-table-head .mobile-stats {
    margin-top: 0;
  }
  .mobile-table-head .mobile-stat {
    justify-content: flex-end;
  }
  .mobile-table-head .mobile-stat em {
    font-size: 8px;
  }
  .mobile-table-head .mobile-stat b {
    color: #e8edf5;
    font-size: 11px;
  }
  .mobile-table-head .mobile-stat:last-child b {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    overflow: visible;
    line-height: 1.15;
  }
  .total-economy-diff {
    display: block;
    color: #4ade80;
    font-size: 8px;
    font-style: normal;
    line-height: 1.1;
    margin-left: 0;
  }
  .match-result-circle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 30px;
    height: 30px;
    border: 2px solid currentColor;
    border-radius: 50%;
    font-size: 14px;
    font-weight: 800;
    line-height: 1;
    background: transparent;
  }
  .match-result-circle.win {
    color: #22c55e;
  }
  .match-result-circle.lose {
    color: #ff4655;
  }
  .mobile-team-scroll::-webkit-scrollbar {
    height: 5px;
  }
  .mobile-team-scroll::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.18);
    border-radius: 999px;
  }
  .mobile-team-row {
    min-width: 630px;
  }
  .mobile-team + .mobile-team {
    margin-top: 12px;
  }
  .mobile-team-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 8px 10px;
    background: #0b1118;
    border-bottom: 1px solid var(--app-border);
  }
  .mobile-team.blue .mobile-team-head {
    background: rgba(22, 93, 255, 0.16);
  }
  .mobile-team.red .mobile-team-head {
    background: rgba(255, 70, 85, 0.14);
  }
  .mobile-team-totals {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 5px 10px;
    color: var(--app-muted);
    font-size: 9px;
    border-bottom: 1px solid var(--app-border);
  }
  .mobile-team-totals em {
    color: #4ade80;
    font-style: normal;
    margin-left: 2px;
  }
  .mobile-team-row {
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr);
    gap: 6px;
    padding: 7px 8px;
    border-top: 1px solid var(--app-border);
  }
  .mobile-pos {
    display: flex;
    align-items: center;
    justify-content: center;
    position: sticky;
    left: 8px;
    z-index: 3;
    background: #0f1620;
    box-shadow: -8px 0 0 #0f1620, 6px 0 0 #0f1620;
  }
  .mobile-content {
    display: grid;
    grid-template-columns: 170px minmax(420px, 1fr);
    gap: 8px;
    align-items: center;
    min-width: 0;
  }
  .mobile-content.empty {
    color: var(--app-muted);
    font-size: 12px;
  }
  .mobile-identity-head {
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
  }
  .mobile-identity {
    display: grid;
    grid-template-columns: 60px minmax(0, 1fr);
    gap: 6px;
    align-items: center;
    min-width: 0;
    position: sticky;
    left: 42px;
    z-index: 2;
    align-self: stretch;
    padding: 2px 4px 2px 0;
    background: #0f1620;
    box-shadow: 8px 0 0 #0f1620;
  }
  .mobile-champ {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 2px;
    padding: 0;
    border: none;
    background: transparent;
    color: var(--app-muted);
    font-size: 9px;
    cursor: pointer;
    min-width: 0;
  }
  .mobile-champ :deep(.champion-icon) {
    width: 44px !important;
    height: 44px !important;
    border-radius: 50%;
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.12);
  }
  .mobile-champ span {
    max-width: 64px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .mobile-player {
    min-width: 0;
  }
  .mobile-stats {
    display: grid;
    grid-template-columns: repeat(6, minmax(0, 1fr));
    gap: 5px;
    margin-top: 0;
  }
  .mobile-stat {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    font-size: 9px;
    color: #d8e2ef;
  }
  .mobile-stat em {
    color: var(--app-muted);
    font-size: 8px;
    font-style: normal;
    white-space: nowrap;
  }
  .mobile-stat b {
    font-size: 12px;
    font-weight: 700;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .mobile-stat .stat-input,
  .mobile-stat :deep(.arco-input-number) {
    width: 100%;
  }
  .mobile-stat .kda-edit {
    gap: 1px;
  }
  .mobile-stat .kda-edit :deep(.arco-input-number) {
    width: 100%;
    min-width: 0;
  }
  .mobile-stat .kda-edit span {
    font-size: 8px;
  }
  .bp-grid {
    grid-template-columns: 1fr;
  }
  .bp-slots {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 6px;
    flex-wrap: nowrap;
  }
  .ban-slot {
    width: 100%;
    height: auto;
    aspect-ratio: 1;
  }
  .ban-slot :deep(.champion-icon) {
    width: 100% !important;
    height: 100% !important;
  }
  .swap-btn {
    font-size: 9px;
    padding: 0 6px;
  }
}

@media (max-width: 900px) {
  .rating-board {
    grid-template-columns: 1fr;
  }
  .rating-row {
    grid-template-columns: 24px 60px minmax(0, 1fr);
    grid-template-areas:
      "pos champ player"
      "pos champ stars";
    align-items: center;
    row-gap: 5px;
  }
  .rating-pos {
    grid-area: pos;
    align-self: center;
  }
  .rating-champ {
    grid-area: champ;
    align-self: center;
  }
  .rating-champ :deep(.champion-icon) {
    width: 46px !important;
    height: 46px !important;
  }
  .rating-player {
    grid-area: player;
  }
  .rating-player .rank-line {
    padding-left: 26px;
  }
  .rating-stars {
    grid-area: stars;
    width: 100%;
    justify-content: space-between;
    gap: 1px;
  }
  .rating-stars .star-btn {
    flex: 1;
    font-size: 24px;
    text-align: center;
  }
  .rating-label {
    flex: 0 0 26px;
    text-align: right;
  }
  .rating-team + .rating-team {
    margin-top: 10px;
  }
}
</style>
