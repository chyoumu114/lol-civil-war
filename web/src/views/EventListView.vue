<template>
  <div class="page">
    <div class="list-head">
      <div>
        <h1 class="page-title">组织列表</h1>
        <p class="page-subtitle">当前有 {{ total }} 个组织</p>
      </div>
      <a-button type="primary" size="large" @click="createEvent">
        <template #icon><icon-plus /></template>
        发起组织
      </a-button>
    </div>

    <a-radio-group v-model="status" type="button" class="status-tabs" @change="reload">
        <a-radio value="all">全部·{{ counts.all || 0 }}</a-radio>
        <a-radio value="open">报名中·{{ counts.open || 0 }}</a-radio>
        <a-radio value="closed">已截止·{{ counts.closed || 0 }}</a-radio>
        <a-radio value="finished">已结束·{{ counts.finished || 0 }}</a-radio>
      </a-radio-group>

    <a-spin :loading="loading" style="width: 100%">
      <div v-if="events.length" class="event-grid">
        <div
          v-for="event in events"
          :key="event.id"
          class="event-card card"
          @click="router.push(`${base}/events/${event.id}`)"
        >
          <div class="event-card-top">
            <StateTag :status="event.status" />
            <a-tag v-if="event.hasBp" size="small" color="purple">BP</a-tag>
            <div class="spacer" />
            <span v-if="event.matchCount" class="event-score" title="系列赛比分（1队 : 2队）">
              <span class="event-score-team team1">1队</span>
              {{ event.team1Wins }} : {{ event.team2Wins }}
              <span class="event-score-team team2">2队</span>
            </span>
          </div>
          <div class="event-title">{{ eventDisplayTitle(event) }}</div>

          <div class="event-meta">
            <div><icon-clock-circle /> 报名截止 {{ formatDateTime(event.signupDeadline) }}</div>
            <div><icon-calendar /> 比赛开始 {{ formatDateTime(event.startsAt) }}</div>
            <div><icon-user-group /> 已报名 {{ event.registrationCount }} 人</div>
          </div>

          <div v-if="event.note" class="event-note">{{ event.note }}</div>

          <div class="event-foot">
            <span class="event-creator">组织者：{{ event.creator?.nickname || '—' }}</span>
            <span class="event-open">查看详情 <icon-right /></span>
          </div>
        </div>
      </div>

      <a-empty v-else-if="!loading" description="暂无组织，点右上角发起一场内战" />
    </a-spin>

    <div v-if="lastPage > 1" class="pager">
      <a-button :disabled="page <= 1" @click="goPage(page - 1)">上一页</a-button>
      <span class="pager-text">{{ page }} / {{ lastPage }}</span>
      <a-button :disabled="page >= lastPage" @click="goPage(page + 1)">下一页</a-button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import StateTag from '../components/StateTag.vue'
import { api } from '../api'
import { session } from '../stores/session'
import { formatDateTime } from '../utils/format'
import { eventDisplayTitle } from '../utils/event'

const route = useRoute()
const router = useRouter()

const base = computed(() => `/civil-war/${route.params.group}`)
const status = ref('all')
const loading = ref(false)
const events = ref([])
const counts = ref({})
const total = ref(0)
const page = ref(1)
const lastPage = ref(1)

async function reload() {
  loading.value = true
  page.value = 1
  await fetchPage()
  loading.value = false
}

async function fetchPage() {
  try {
    const data = await api.events(route.params.group, { status: status.value, page: page.value, page_size: 12 })
    events.value = data.data
    counts.value = data.meta.counts
    total.value = data.meta.total
    lastPage.value = data.meta.last_page
  } catch (error) {
    Message.error(error.message)
  }
}

function goPage(next) {
  page.value = next
  fetchPage()
}

function createEvent() {
  if (!session.profileComplete) {
    Message.warning('请先在个人资料中补全 QQ 号、主选段位和擅长位置')
    return
  }
  router.push(`${base.value}/events/new`)
}

onMounted(fetchPage)
</script>

<style scoped>
.status-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 14px;
}
.status-tabs :deep(.arco-radio-button) {
  padding: 0 8px;
  font-size: 12px;
  white-space: nowrap;
}
.list-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.event-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
  margin-top: 12px;
}
.event-card {
  padding: 18px;
  cursor: pointer;
  transition: all 0.18s;
  border: 1px solid transparent;
}
.event-card:hover {
  transform: translateY(-2px);
  border-color: #bedaff;
  box-shadow: 0 8px 24px rgba(22, 93, 255, 0.08);
}
.event-card-top {
  display: flex;
  align-items: center;
  gap: 8px;
}
.event-title {
  margin-top: 10px;
  font-size: 17px;
  font-weight: 800;
  letter-spacing: 0.3px;
}
.event-region {
  font-size: 12px;
  color: var(--app-muted);
}
.event-format {
  font-size: 12px;
  color: var(--app-muted);
}
.event-score {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-weight: 800;
  color: var(--app-primary);
  letter-spacing: 0;
}
.event-score-team {
  font-size: 11px;
  font-weight: 700;
}
.event-score-team.team1 {
  color: #d9a441;
}
.event-score-team.team2 {
  color: #ff6b6b;
}
.event-meta {
  display: grid;
  gap: 6px;
  margin-top: 14px;
  color: #4e5969;
  font-size: 12px;
}
.event-meta div {
  display: flex;
  align-items: center;
  gap: 6px;
}
.event-note {
  margin-top: 12px;
  padding: 8px 10px;
  border-radius: 8px;
  background: #f7f8fa;
  color: #4e5969;
  font-size: 12px;
  max-height: 52px;
  overflow: hidden;
}
.event-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14px;
  font-size: 12px;
  color: var(--app-muted);
}
.event-open {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  color: var(--app-primary);
}
.pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 24px;
}
.pager-text {
  color: var(--app-muted);
  font-size: 13px;
}
@media (max-width: 768px) {
  .list-head {
    flex-direction: column;
  }
  .event-grid {
    grid-template-columns: 1fr;
  }
}
</style>
