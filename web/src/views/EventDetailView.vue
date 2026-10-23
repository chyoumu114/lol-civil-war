<template>
  <div class="page event-detail">
    <div class="detail-nav">
      <a-button type="text" @click="router.push(base)">
        <template #icon><icon-left /></template>组织列表
      </a-button>
    </div>

    <a-spin :loading="loading" style="width: 100%">
      <template v-if="detail">
        <div class="detail-head">
          <div class="detail-title-line">
            <h1 class="page-title">{{ eventTitle }}</h1>
            <StateTag :status="detail.event.status" />
            <a-tag v-if="detail.event.hasBp" size="small" color="purple">BP</a-tag>
          </div>

          <div class="detail-actions">
            <a-button v-if="canShowRegisterButton" type="primary" @click="openRegister">
              {{ detail.myRegistration ? '修改报名' : '报名' }}
            </a-button>
            <a-tooltip v-else-if="!detail.permissions.signupOpen" content="报名已截止，修改报名请联系组织者">
              <a-button disabled>报名已截止</a-button>
            </a-tooltip>
            <a-button
              v-if="detail.permissions.canSpectate"
              :type="detail.mySpectator ? 'secondary' : 'outline'"
              @click="toggleEventSpectator"
            >
              {{ detail.mySpectator ? `取消观战（${detail.spectatorCount}）` : `观战（${detail.spectatorCount}）` }}
            </a-button>

            <a-button v-if="detail.permissions.isOrganizer && !detail.permissions.isEnded" @click="editVisible = true">
              <template #icon><icon-settings /></template>
              编辑组织
            </a-button>
            <a-button
              v-if="detail.permissions.canEnd"
              status="danger"
              type="primary"
              @click="endEvent"
            >
              <template #icon><icon-check /></template>
              结束比赛
            </a-button>
            <a-button class="share-button" @click="openShare">分享</a-button>
          </div>
        </div>

        <a-row :gutter="[16, 16]">
          <!-- 左列：组织信息 + 队伍信息（含比赛记录） -->
          <a-col :xs="24" :lg="7">
            <a-card :bordered="false" class="section-card org-card" title="组织信息">
              <div class="org-grid">
                <div class="org-item">
                  <span>大区</span>
                  <b>{{ detail.event.region }}</b>
                </div>
                <div class="org-item">
                  <span>赛制</span>
                  <b>BO{{ detail.event.bestOf }}</b>
                </div>
                <div class="org-item">
                  <span>BP</span>
                  <b>{{ detail.event.hasBp ? '有' : '无' }}</b>
                </div>
                <div class="org-item">
                  <span>报名截止</span>
                  <b>{{ formatDateTime(detail.event.signupDeadline) }}</b>
                </div>
                <div class="org-item">
                  <span>比赛开始</span>
                  <b>{{ formatDateTime(detail.event.startsAt) }}</b>
                </div>
                <button type="button" class="org-item organizer" @click="openCreator">
                  <span>组织者</span>
                  <b>
                    <a-avatar :size="24" :image-url="detail.event.creator?.avatar || ''">
                      {{ (detail.event.creator?.nickname || '?').slice(0, 1) }}
                    </a-avatar>
                    {{ detail.event.creator?.nickname || '—' }}
                  </b>
                </button>
              </div>
              <div v-if="detail.event.note" class="org-note">
                <div class="org-note-label">备注</div>
                <div class="org-note-body">{{ detail.event.note }}</div>
              </div>
            </a-card>

            <a-collapse v-model:active-key="teamKeys" :bordered="false" class="team-collapse">
              <a-collapse-item key="team">
                <template #header>
                  <div class="team-panel-head">
                    <span class="panel-title">队伍信息</span>
                    <div class="team-panel-actions" @click.stop>
                      <a-button
                        v-if="detail.permissions.canArrange"
                        size="mini"
                        @click="rosterEditorVisible = true"
                      >
                        <template #icon><icon-edit /></template>
                        编排
                      </a-button>
                    </div>
                  </div>
                </template>

                <div v-if="hasRoster" class="versus">
                  <div class="versus-score">
                    <div class="score-team left">
                      <div class="score-team-name gold">
                        1队
                        <span v-if="detail.permissions.isEnded && detail.series.teamWinner === 1" class="win-mark">（胜）</span>
                      </div>
                      <PowerValue :value="detail.roster.team1Power" />
                    </div>
                    <div class="score-middle">
                      <b>{{ detail.series.team1Wins }}</b>
                      <span>VS</span>
                      <b>{{ detail.series.team2Wins }}</b>
                    </div>
                    <div class="score-team right">
                      <div class="score-team-name red">
                        2队
                        <span v-if="detail.permissions.isEnded && detail.series.teamWinner === 2" class="win-mark">（胜）</span>
                      </div>
                      <PowerValue :value="detail.roster.team2Power" />
                    </div>
                  </div>

                  <template v-for="position in positions" :key="position.code">
                    <div class="versus-block">
                    <div class="versus-row">
                      <button type="button" class="vs-player left" @click="openRosterPlayer(team1Item(position.code))">
                        <template v-if="team1Item(position.code)">
                          <span class="vs-name">{{ team1Item(position.code).name }}</span>
                          <a-avatar :size="28" :image-url="avatarFor(team1Item(position.code))">
                            {{ team1Item(position.code).name.slice(0, 1) }}
                          </a-avatar>
                        </template>
                        <span v-else class="muted">—</span>
                      </button>

                      <div class="vs-role-spacer"></div>
                      <div class="vs-role">
                        <PositionIcon :code="position.code" :size="36" />
                      </div>

                      <button type="button" class="vs-player right" @click="openRosterPlayer(team2Item(position.code))">
                        <template v-if="team2Item(position.code)">
                          <a-avatar :size="28" :image-url="avatarFor(team2Item(position.code))">
                            {{ team2Item(position.code).name.slice(0, 1) }}
                          </a-avatar>
                          <span class="vs-name">{{ team2Item(position.code).name }}</span>
                        </template>
                        <span v-else class="muted">—</span>
                      </button>
                    </div>

                    <div class="vs-detail-row">
                      <div class="vs-detail left">
                        <template v-if="team1Item(position.code)">
                          <div class="detail-rank-stack">
                            <div class="detail-power-rank">
                              <PowerValue :value="team1Item(position.code).power" compact />
                              <span class="detail-rank-kind">{{ rankKindForItem(team1Item(position.code)) }}</span>
                            </div>
                            <RankTag :rank="rankForItem(team1Item(position.code))" compact />
                          </div>
                        </template>
                      </div>
                      <div class="vs-detail-center"></div>
                      <div class="vs-detail right">
                        <template v-if="team2Item(position.code)">
                          <div class="detail-rank-stack">
                            <div class="detail-power-rank">
                              <PowerValue :value="team2Item(position.code).power" compact />
                              <span class="detail-rank-kind">{{ rankKindForItem(team2Item(position.code)) }}</span>
                            </div>
                            <RankTag :rank="rankForItem(team2Item(position.code))" compact />
                          </div>
                        </template>
                      </div>
                    </div>
                    </div>
                  </template>
                </div>
                <a-empty v-else description="尚未生成队伍信息" :image-style="{ height: '44px' }" />

                <!-- 比赛记录压缩在队伍信息里 -->
                <div class="match-section">
                  <div class="match-head">
                    <span class="match-title">比赛记录</span>
                    <div class="match-head-right">
                      <a-button
                        v-if="detail.permissions.canRecordMatch"
                        size="mini"
                        type="primary"
                        @click="newMatch"
                      >
                        <template #icon><icon-plus /></template>
                        记录
                      </a-button>
                    </div>
                  </div>

                  <div class="record-status" :class="{ 'series-result': seriesResult }">
                    <template v-if="!detail.permissions.isEnded">
                      记录中
                      <span v-if="detail.series.matches.length" class="muted">· 已记录 {{ detail.series.matches.length }} 局</span>
                    </template>
                    <template v-else-if="seriesResult">
                      <span class="series-result-team team1" :class="seriesResult.team1Class">{{ seriesResult.team1 }}</span>
                      <span class="series-result-score">{{ seriesResult.score }}</span>
                      <span class="series-result-team team2" :class="seriesResult.team2Class">{{ seriesResult.team2 }}</span>
                    </template>
                    <template v-else>比赛已结束</template>
                  </div>

                  <div v-if="detail.series.matches.length" class="match-list">
                    <button
                      v-for="match in detail.series.matches"
                      :key="match.id"
                      type="button"
                      class="match-row"
                      @click="openMatch(match.gameNo)"
                    >
                      <span class="match-no">第 {{ match.gameNo }} 局</span>
                      <span v-if="match.winnerTeam" class="match-result" :class="`team${match.winnerTeam}`">
                        {{ match.winnerTeam }}队 · 胜
                      </span>
                      <span v-else class="match-result">记录中</span>
                      <span class="muted small">{{ match.durationMs ? `· ${formatDuration(match.durationMs)}` : '· 时长未记录' }}</span>
                      <span class="match-open">查看记录 <icon-right /></span>
                    </button>
                  </div>
                  <a-empty v-else description="还没有对局记录" :image-style="{ height: '44px' }" />

                  <div class="match-hint">
                    <icon-info-circle />
                    {{ detail.permissions.isEnded ? '比赛已结束，所有记录已锁定并纳入排行榜。' : '比赛结束前记录可以修改；组织者点击「结束比赛」后锁定并计入排行榜。' }}
                  </div>
                </div>
              </a-collapse-item>
            </a-collapse>
          </a-col>

          <!-- 右列：报名列表 -->
          <a-col :xs="24" :lg="17">
            <a-card :bordered="false" class="section-card reg-card">
              <template #title>
                <div class="card-title">
                  <span>报名列表（{{ detail.registrations.length }}）</span>
                  <span class="reg-counts">
                    <span>参赛 {{ detail.participantCount }}</span>
                    <span v-if="detail.substituteCount">替补 {{ detail.substituteCount }}</span>
                    <span v-if="!detail.permissions.signupOpen" class="muted">已截止</span>
                  </span>
                </div>
              </template>

              <div class="reg-filters">
                <a-select v-model="filterMain" placeholder="主选位置" size="small" allow-clear>
                  <a-option value="">全部主选</a-option>
                  <a-option v-for="pos in positionOptions" :key="pos.code" :value="pos.code">
                    <PositionIcon :code="pos.code" :size="18" />
                    {{ pos.label }}
                  </a-option>
                </a-select>
                <a-select v-model="filterSub" placeholder="补位位置" size="small" allow-clear>
                  <a-option value="">全部补位</a-option>
                  <a-option v-for="pos in positionOptions" :key="pos.code" :value="pos.code">
                    <PositionIcon :code="pos.code" :size="18" />
                    {{ pos.label }}
                  </a-option>
                </a-select>
                <a-button size="small" @click="resetFilters">重置</a-button>
              </div>

              <div v-if="filteredRegistrations.length" class="reg-table">
                <div class="reg-header">
                  <span>玩家</span>
                  <span>主选段位</span>
                  <span>主选位置</span>
                  <span>补位段位</span>
                  <span>补位位置</span>
                  <span>备注</span>
                  <span>操作</span>
                </div>

                <div v-for="reg in filteredRegistrations" :key="reg.memberId" class="reg-row">
                  <button type="button" class="reg-player" @click="openRegistrationPlayer(reg)">
                    <a-avatar :size="28" :image-url="reg.avatar || ''">
                      {{ (reg.nickname || '?').slice(0, 1) }}
                    </a-avatar>
                    <span class="reg-name">{{ reg.nickname }}</span>
                    <a-tag v-if="reg.isSpectator" size="mini" color="gray">替补</a-tag>
                  </button>

                  <span class="reg-rank main-rank" data-label="主选段位"><RankTag :rank="reg.rank" compact /></span>

                  <span class="reg-pos main-pos" data-label="主选位置">
                    <PositionIcon v-if="reg.mainPosition" :code="reg.mainPosition" :size="28" />
                    <PositionIcon v-else code="ALL" :size="28" />
                  </span>

                  <span class="reg-rank sub-rank" data-label="补位段位"><RankTag :rank="reg.subRank" compact /></span>

                  <span class="reg-pos sub sub-pos" data-label="补位位置">
                    <template v-if="reg.subPositions?.length">
                      <PositionIcon v-for="code in reg.subPositions" :key="code" :code="code" :size="28" />
                    </template>
                    <span v-else class="muted">—</span>
                  </span>

                  <span class="reg-note" data-label="备注">{{ reg.note || '—' }}</span>

                  <span class="reg-actions">
                    <a-button v-if="canEditRegistration(reg)" size="mini" @click="editRegistration(reg)">编辑</a-button>
                    <a-button
                      v-if="detail.permissions.canManageRegistrations"
                      size="mini"
                      @click="toggleSpectator(reg)"
                    >
                      {{ reg.isSpectator ? '取消替补' : '替补' }}
                    </a-button>
                  </span>
                </div>
              </div>
              <a-empty v-else :description="hasFilters ? '没有符合筛选条件的报名' : '还没有人报名'" />
            </a-card>
          </a-col>
        </a-row>

        <RegistrationModal
          v-model:visible="registerVisible"
          :event="detail.event"
          :target="editingTarget"
          :mode="editingMode"
          @saved="reload"
        />
        <RosterEditorModal
          v-model:visible="rosterEditorVisible"
          :event="detail.event"
          :registrations="detail.registrations"
          :roster="detail.roster"
          @applied="reload"
        />
        <EventEditModal v-model:visible="editVisible" :event="detail.event" @saved="reload" />
        <PlayerInfoModal v-model:visible="playerVisible" :player="selectedPlayer" />
        <MatchRecordModal
          v-model:visible="matchModalVisible"
          :event-id="eventId"
          :game-no="matchGameNo"
          @changed="reload"
        />
        <a-modal v-model:visible="shareVisible" title="分享组织" :width="520" :footer="false">
          <a-textarea v-model="shareText" :auto-size="{ minRows: 4, maxRows: 8 }" />
          <div class="share-actions">
            <a-button @click="resetShare">恢复默认</a-button>
            <a-button type="primary" @click="copyShare">复制</a-button>
          </div>
        </a-modal>
      </template>

      <a-result v-else-if="error" status="error" :title="error" />
    </a-spin>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message, Modal } from '@arco-design/web-vue'
import EventEditModal from '../components/EventEditModal.vue'
import MatchRecordModal from '../components/MatchRecordModal.vue'
import PlayerInfoModal from '../components/PlayerInfoModal.vue'
import PowerValue from '../components/PowerValue.vue'
import RankTag from '../components/RankTag.vue'
import PositionIcon from '../components/PositionIcon.vue'
import RegistrationModal from '../components/RegistrationModal.vue'
import RosterEditorModal from '../components/RosterEditorModal.vue'
import StateTag from '../components/StateTag.vue'
import { api } from '../api'
import { session } from '../stores/session'
import { formatDateTime, formatDuration } from '../utils/format'
import { eventDisplayTitle } from '../utils/event'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const error = ref('')
const detail = ref(null)
const registerVisible = ref(false)
const rosterEditorVisible = ref(false)
const editVisible = ref(false)
const shareVisible = ref(false)
const shareText = ref('')
const playerVisible = ref(false)
const matchModalVisible = ref(false)
const matchGameNo = ref(0)
const selectedPlayer = ref(null)
const editingTarget = ref(null)
const editingMode = ref('create')
const teamKeys = ref([])
const filterMain = ref('')
const filterSub = ref('')

const base = computed(() => `/civil-war/${route.params.group}`)
const eventId = computed(() => route.params.eventId)
const positions = computed(() => session.meta?.positions || [])
const positionOptions = computed(() => session.meta?.positions || [])
const hasFilters = computed(() => Boolean(filterMain.value || filterSub.value))
const filteredRegistrations = computed(() => {
  const list = detail.value?.registrations || []
  return list.filter((reg) => {
    if (filterMain.value && reg.mainPosition !== filterMain.value) return false
    if (filterSub.value && !(reg.subPositions || []).includes(filterSub.value)) return false
    return true
  })
})

function resetFilters() {
  filterMain.value = ''
  filterSub.value = ''
}

async function toggleSpectator(reg) {
  try {
    detail.value = await api.toggleSpectator(
      route.params.group,
      eventId.value,
      reg.memberId,
      !reg.isSpectator,
    )
    Message.success(reg.isSpectator ? '已取消替补' : '已标记为替补')
    teamKeys.value = hasRoster.value ? ['team'] : []
  } catch (error) {
    Message.error(error.message)
  }
}

const registrationByMember = computed(() => {
  const map = {}
  for (const reg of detail.value?.registrations || []) {
    map[reg.memberId] = reg
  }
  return map
})

const hasRoster = computed(() => {
  const roster = detail.value?.roster
  return Boolean(roster?.team1?.length || roster?.team2?.length)
})

const eventTitle = computed(() => eventDisplayTitle(detail.value?.event))
const seriesResult = computed(() => {
  const series = detail.value?.series
  if (!series?.teamWinner) return null
  const team1Win = series.teamWinner === 1
  return {
    team1: `${team1Win ? '（胜）' : '（负）'}1队`,
    team2: `2队${team1Win ? '（负）' : '（胜）'}`,
    score: `${series.team1Wins} VS ${series.team2Wins}`,
    team1Class: team1Win ? 'winner' : 'loser',
    team2Class: team1Win ? 'loser' : 'winner',
  }
})

const canShowRegisterButton = computed(() => {
  if (!detail.value) return false
  return detail.value.permissions.signupOpen
    && !detail.value.permissions.isEnded
    && (detail.value.permissions.canRegister || Boolean(detail.value.myRegistration))
})

function team1Item(positionCode) {
  return detail.value?.roster?.team1?.find((item) => item.position === positionCode) || null
}

function team2Item(positionCode) {
  return detail.value?.roster?.team2?.find((item) => item.position === positionCode) || null
}

function avatarFor(item) {
  return registrationByMember.value[item.memberId]?.avatar || ''
}

function rankForItem(item) {
  if (!item) return null
  const reg = registrationByMember.value[item.memberId]
  if (!reg) return null
  return reg.mainPosition === item.position ? reg.rank : (reg.subRank || reg.rank)
}

function rankKindForItem(item) {
  if (!item) return ''
  const reg = registrationByMember.value[item.memberId]
  if (!reg) return ''
  return reg.mainPosition === item.position ? '主选段位' : '补位段位'
}

function openRegistrationPlayer(reg) {
  selectedPlayer.value = {
    nickname: reg.nickname,
    avatar: reg.avatar,
    qqNumber: reg.qqNumber,
    rankLabel: reg.rank?.label || '',
    positions: [reg.mainPosition, ...(reg.subPositions || [])].filter(Boolean),
    note: reg.note,
    isOrganizer: reg.isOrganizer,
  }
  playerVisible.value = true
}

function openRosterPlayer(item) {
  if (!item) return
  const reg = registrationByMember.value[item.memberId]
  if (reg) {
    openRegistrationPlayer(reg)
    return
  }
  selectedPlayer.value = {
    nickname: item.name,
    avatar: null,
    qqNumber: null,
    rankLabel: item.rankLabel || '',
    positions: [item.position],
    note: null,
    isOrganizer: false,
  }
  playerVisible.value = true
}

function openCreator() {
  const creator = detail.value?.event?.creator
  if (!creator) return
  const reg = registrationByMember.value[creator.memberId]
  if (reg) {
    openRegistrationPlayer(reg)
    return
  }
  selectedPlayer.value = {
    nickname: creator.nickname,
    avatar: creator.avatar,
    qqNumber: creator.qqNumber,
    rankLabel: '',
    positions: [],
    note: null,
    isOrganizer: true,
  }
  playerVisible.value = true
}

async function toggleEventSpectator() {
  if (!detail.value?.permissions.canSpectate) return
  const watching = !detail.value.mySpectator
  try {
    detail.value = await api.toggleEventSpectator(session.group.key, detail.value.event.id, watching)
    Message.success(watching ? '已观战' : '已取消观战')
  } catch (error) {
    Message.error(error.message)
  }
}

function openRegister() {
  if (!session.profileComplete) {
    Message.warning('请先在个人资料中补全 QQ 号、主选段位和擅长位置')
    return
  }
  editingTarget.value = detail.value.myRegistration || null
  editingMode.value = detail.value.myRegistration ? 'editSelf' : 'create'
  registerVisible.value = true
}

function editRegistration(reg) {
  editingTarget.value = reg
  editingMode.value = reg.memberId === session.member.id ? 'editSelf' : 'editOther'
  registerVisible.value = true
}

function canEditRegistration(reg) {
  if (!detail.value) return false
  if (detail.value.permissions.canManageRegistrations) return true
  return detail.value.permissions.signupOpen
    && !detail.value.permissions.isEnded
    && reg.memberId === session.member.id
}


async function reload() {
  loading.value = true
  error.value = ''
  try {
    detail.value = await api.event(route.params.group, eventId.value)
    // 已编排至少一名玩家时展开队伍信息；没有阵容时默认折叠
    teamKeys.value = hasRoster.value ? ['team'] : []
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

async function newMatch() {
  try {
    const data = await api.createMatch(route.params.group, eventId.value)
    matchGameNo.value = data.match.gameNo
    matchModalVisible.value = true
  } catch (err) {
    Message.error(err.message)
  }
}

function openMatch(gameNo) {
  matchGameNo.value = gameNo
  matchModalVisible.value = true
}

function defaultShareText() {
  const event = detail.value?.event
  if (!event) return ''
  return `【${eventDisplayTitle(event)}】报名截止日期：${formatDateTime(event.signupDeadline)}  点击链接报名 ${window.location.href}`
}

function openShare() {
  shareText.value = defaultShareText()
  shareVisible.value = true
}

function resetShare() {
  shareText.value = defaultShareText()
}

function fallbackCopy(text) {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  const ok = document.execCommand('copy')
  document.body.removeChild(textarea)
  if (!ok) throw new Error('copy failed')
}

async function copyShare() {
  const text = shareText.value.trim()
  if (!text) {
    Message.warning('请输入要分享的内容')
    return
  }
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      fallbackCopy(text)
    }
  } catch (err) {
    try {
      fallbackCopy(text)
    } catch (fallbackErr) {
      Message.error('复制失败，请手动选择内容复制')
      return
    }
  }
  Message.success('已复制到剪贴板')
}

function endEvent() {
  Modal.confirm({
    title: '结束比赛',
    content: '结束后所有对局记录将锁定并纳入排行榜，且不可撤销。确定结束吗？',
    okText: '确认结束',
    cancelText: '取消',
    onOk: async () => {
      try {
        await api.endEvent(route.params.group, eventId.value)
        Message.success('比赛已结束，记录已锁定')
        await reload()
      } catch (err) {
        Message.error(err.message)
      }
    },
  })
}

onMounted(reload)
</script>

<style scoped>
.event-detail {
  max-width: none;
}
.detail-nav {
  margin-bottom: 4px;
}
.detail-head {
  margin-bottom: 16px;
}
.detail-title-line {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.detail-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 12px;
}
.share-button {
  margin-left: auto;
}
.share-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}
.section-card {
  margin-bottom: 16px;
}
.card-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.small {
  font-size: 12px;
}

/* 组织信息 */
.org-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px 18px;
}
.org-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 0;
  border: 0;
  border-bottom: 1px solid var(--app-border);
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
}
.org-item > span {
  color: var(--app-muted);
  font-size: 12px;
}
.org-item > b {
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text);
}
.org-item.organizer {
  cursor: pointer;
  border-bottom-color: transparent;
}
.org-item.organizer:hover > b {
  color: #ff6b78;
}
.org-item.organizer b {
  display: flex;
  align-items: center;
  gap: 6px;
}
.org-note {
  margin-top: 14px;
}
.org-note-label {
  color: var(--app-muted);
  font-size: 12px;
  margin-bottom: 4px;
}
.org-note-body {
  white-space: pre-wrap;
  color: var(--app-text);
  font-size: 13px;
}

/* 队伍信息折叠 */
.team-collapse {
  margin-bottom: 16px;
  background: var(--app-card);
  border: 1px solid var(--app-border);
  border-radius: var(--app-radius);
  overflow: hidden;
}
.team-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.panel-title {
  font-size: 16px;
  font-weight: 700;
}
.team-panel-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.panel-powers {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}
.versus-score {
  display: grid;
  grid-template-columns: 1fr 72px 1fr;
  align-items: center;
  gap: 8px;
  padding: 8px 0 10px;
  border-bottom: 1px solid var(--app-border);
}
.score-team {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.score-team.left {
  align-items: flex-end;
  text-align: right;
}
.score-team.right {
  align-items: flex-start;
  text-align: left;
}
.score-team-name {
  font-size: 16px;
  font-weight: 800;
  letter-spacing: 2px;
}
.score-team-name.gold {
  color: #d4af37;
}
.score-team-name.red {
  color: #ff5b5b;
}
.win-mark {
  font-size: 11px;
  letter-spacing: 0;
  color: #7ee2a8;
}
.score-middle {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: var(--app-muted);
  font-size: 12px;
}
.score-middle b {
  color: #fff;
  font-size: 20px;
}
.score-middle span {
  color: var(--app-muted);
  font-weight: 700;
}
.vs-detail-row {
  display: grid;
  grid-template-columns: 1fr 64px 1fr;
  gap: 8px;
  align-items: center;
  padding: 0 0 6px;
}
.vs-detail {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex-wrap: wrap;
}
.vs-detail.left {
  justify-content: flex-end;
}
.vs-detail.right {
  justify-content: flex-start;
}
.vs-detail-center {
  width: 64px;
}
.detail-rank-kind {
  color: var(--app-muted);
  font-size: 9px;
  line-height: 1.25;
}
.detail-rank-stack {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-width: 0;
  max-width: 100%;
}
.detail-power-rank {
  display: flex;
  align-items: center;
  gap: 5px;
  white-space: nowrap;
}
.vs-detail.left .detail-rank-stack {
  align-items: flex-end;
  text-align: right;
}
.panel-powers .gold {
  color: #d4af37;
}
.panel-powers .red {
  color: #ff5b5b;
}
.vs-mark {
  color: var(--app-muted);
  font-weight: 800;
}

/* 1队 vs 2队 */
.versus {
  padding: 4px 0 10px;
}
.versus-head {
  display: grid;
  grid-template-columns: 1fr 64px 1fr;
  align-items: center;
  margin-bottom: 8px;
}
.vs-team {
  font-size: 14px;
  font-weight: 800;
  letter-spacing: 2px;
}
.vs-team.gold {
  color: #d4af37;
  text-align: right;
}
.vs-team.red {
  color: #ff5b5b;
  text-align: left;
}
.vs-center {
  text-align: center;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 800;
}
.versus-row {
  display: grid;
  grid-template-columns: 1fr 64px 1fr;
  align-items: center;
  min-height: 46px;
  border-top: 1px dashed var(--app-border);
}
.vs-player {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  border: none;
  background: transparent;
  color: var(--app-text);
  cursor: pointer;
  min-width: 0;
}
.vs-player.left {
  justify-content: flex-end;
  text-align: right;
}
.vs-player.right {
  justify-content: flex-start;
  text-align: left;
}
.vs-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 600;
}
.versus-block {
  position: relative;
}
.vs-role-spacer {
  width: 40px;
  height: 1px;
}
.vs-role {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  z-index: 1;
  display: flex;
  justify-content: center;
  align-items: center;
  width: 40px;
  height: 40px;
  margin: 0;
  border-radius: 50%;
  border: 1px solid var(--app-border);
  background: rgba(255, 70, 85, 0.06);
  pointer-events: none;
}

/* 比赛记录 */
.match-section {
  margin-top: 10px;
  padding-top: 14px;
  border-top: 1px solid var(--app-border);
}
.match-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 6px;
}
.match-head-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.record-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  padding: 4px 9px;
  border-radius: 6px;
  background: rgba(255, 70, 85, 0.09);
  color: #ff8a94;
  font-size: 12px;
  font-weight: 600;
}
.record-status.series-result {
  background: transparent;
  padding: 0;
  gap: 8px;
  font-size: 14px;
}
.series-result-team {
  font-weight: 800;
}
.series-result-team.team1 {
  color: #d9a441;
}
.series-result-team.team2 {
  color: #ff6b6b;
}
.series-result-team.loser {
  opacity: 0.55;
}
.series-result-score {
  color: var(--app-text);
  font-size: 14px;
  font-weight: 800;
}
.match-title {
  font-size: 15px;
  font-weight: 700;
}
.series-score {
  font-size: 13px;
  color: var(--app-muted);
}
.winner-line,
.draft-tip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  margin-bottom: 8px;
  border-radius: 8px;
  font-size: 13px;
}
.winner-line {
  background: rgba(212, 175, 55, 0.12);
  color: #d4af37;
}
.draft-tip {
  background: rgba(255, 91, 91, 0.12);
  color: #ff8a8a;
}
.match-list {
  display: flex;
  flex-direction: column;
}
.match-row {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 9px 2px;
  border: none;
  border-top: 1px solid var(--app-border);
  background: transparent;
  color: var(--app-text);
  cursor: pointer;
  text-align: left;
}
.match-no {
  font-weight: 600;
}
.match-result {
  color: #c9d4e2;
  font-size: 12px;
}
.match-result.team1 {
  color: #d9a441;
}
.match-result.team2 {
  color: #ff6b6b;
}
.match-open {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 2px;
  color: #ff5b5b;
  font-size: 12px;
}
.match-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 10px;
  color: var(--app-muted);
  font-size: 12px;
}

/* 报名列表 */
.reg-counts {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--app-muted);
  font-size: 11px;
  font-weight: 400;
}
.reg-filters {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 8px;
  margin-bottom: 10px;
}
.reg-table {
  width: 100%;
}
.reg-header,
.reg-row {
  display: grid;
  grid-template-columns: minmax(96px, 1.2fr) 104px 44px 104px 52px minmax(46px, 1fr) 98px;
  gap: 6px;
  align-items: center;
}
.reg-header {
  padding: 8px 6px;
  color: var(--app-muted);
  font-size: 10px;
  letter-spacing: 0.2px;
  white-space: nowrap;
  border-bottom: 1px solid var(--app-border);
}
.reg-row {
  min-height: 70px;
  padding: 8px 6px;
  border-bottom: 1px solid var(--app-border);
  font-size: 12px;
}
.reg-row:last-child {
  border-bottom: none;
}
.reg-player {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  border: none;
  background: transparent;
  color: var(--app-text);
  cursor: pointer;
  text-align: left;
}
.reg-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
}
.reg-rank {
  color: var(--app-muted);
  font-size: 10px;
  min-width: 0;
  overflow: hidden;
}
.reg-rank .rank-tag {
  max-width: 100%;
  padding: 2px 4px;
  font-size: 9px;
}
.reg-pos {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}
.reg-note {
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  color: var(--app-muted);
  font-size: 10px;
}
.reg-actions {
  display: flex;
  gap: 3px;
  justify-content: flex-end;
  flex-wrap: nowrap;
}

@media (max-width: 1320px) {
  .reg-header {
    display: none;
  }
  .reg-row {
    grid-template-columns: 1fr 1fr;
    gap: 8px 10px;
  }
  .reg-player,
  .reg-note,
  .reg-actions {
    grid-column: span 2;
  }
  .reg-rank,
  .reg-pos {
    grid-column: span 1;
  }
  .reg-pos {
    gap: 6px;
  }
  .reg-pos .position-icon {
    box-sizing: content-box;
    padding: 3px;
    border: 1px solid var(--app-border);
    border-radius: 6px;
    background: #0b1118;
  }
  .reg-row > [data-label]::before {
    content: attr(data-label);
    display: block;
    margin-bottom: 2px;
    color: var(--app-muted);
    font-size: 10px;
  }
  .reg-row .reg-pos {
    align-items: flex-start;
  }
  .reg-row .reg-pos::before {
    flex: 0 0 100%;
  }
}

@media (max-width: 1100px) {
  .org-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .org-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .reg-row {
    padding: 10px 4px;
  }
  .reg-filters {
    grid-template-columns: 1fr 1fr;
  }
  .reg-filters .arco-btn {
    grid-column: span 2;
  }
  .reg-note {
    -webkit-line-clamp: 3;
  }
  .versus-head,
  .versus-row {
    grid-template-columns: 1fr 48px 1fr;
  }
}

/* 组织详情页继承全局 PC 80vw 宽度 */
.page.event-detail {
  max-width: none;
}
/* PC 报名列表行高比原来增加约 50% */
.reg-row {
  min-height: 70px;
}
</style>
