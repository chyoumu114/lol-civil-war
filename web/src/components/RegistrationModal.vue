<template>
  <a-modal
    :visible="visible"
    :title="title"
    :width="580"
    :ok-loading="saving"
    :ok-text="okText"
    @ok="submit"
    @cancel="close"
  >
    <div v-if="target" class="target-line">
      <span class="target-label">报名成员</span>
      <b>{{ target.nickname }}</b>
      <span class="muted">QQ {{ target.qqNumber || '—' }}</span>
      <a-tag v-if="target.isOrganizer" size="small" color="arcoblue">组织者</a-tag>
      <a-tag v-if="form.isSpectator" size="small" color="gray">替补</a-tag>
    </div>

    <a-form :model="form" layout="vertical" @submit.prevent>
      <a-form-item label="参与方式">
        <a-radio-group v-model="form.isSpectator" type="button">
          <a-radio :value="false">参赛</a-radio>
          <a-radio :value="true">替补</a-radio>
        </a-radio-group>
        <template #extra>替补不参与队伍编排，也不进入比赛阵容；分路会保留，取消替补后自动恢复。</template>
      </a-form-item>

        <a-form-item label="主选位置">
          <div class="main-positions">
            <button
              type="button"
              class="main-choice"
              :class="{ selected: !form.mainPosition }"
              title="补位"
              aria-label="补位"
              @click="selectMain('')"
            >
              <PositionIcon code="ALL" :size="34" />
            </button>
            <button
              v-for="item in session.meta?.positions || []"
              :key="item.code"
              type="button"
              class="main-choice"
              :class="{ selected: form.mainPosition === item.code }"
              :title="item.label"
              @click="selectMain(item.code)"
            >
              <PositionIcon :code="item.code" :size="34" />
            </button>
          </div>
        </a-form-item>

        <a-form-item label="补位位置（不能与主选相同）">
          <PositionPicker
            v-model="form.subPositions"
            :positions="session.meta?.positions || []"
            :exclude="form.mainPosition ? [form.mainPosition] : []"
          />
          <template #extra>参赛时主选和补位至少填写一个；替补可留空。主选为「补位」时只按补位位置排。</template>
        </a-form-item>

      <a-form-item label="主选段位" required>
        <RankPicker v-model="form.rank" :regions="session.meta?.regions || []" :tiers="session.meta?.rankTiers || []" />
      </a-form-item>

      <a-form-item label="补位段位">
        <RankPicker
          v-model="form.subRank"
          :regions="session.meta?.regions || []"
          :tiers="session.meta?.rankTiers || []"
        />
<!--        <template #extra>默认与主选段位相同，可单独填写。</template>-->
      </a-form-item>

      <a-form-item label="备注">
        <a-textarea v-model="form.note" placeholder="英雄池、语音、时间等" :max-length="200" show-word-limit :auto-size="{ minRows: 2, maxRows: 4 }" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import PositionIcon from './PositionIcon.vue'
import PositionPicker from './PositionPicker.vue'
import RankPicker from './RankPicker.vue'
import { api } from '../api'
import { session } from '../stores/session'

const props = defineProps({
  visible: { type: Boolean, default: false },
  event: { type: Object, required: true },
  target: { type: Object, default: null },
  mode: { type: String, default: 'create' }, // create | editSelf | editOther
})
const emit = defineEmits(['update:visible', 'saved'])

const saving = ref(false)
const form = reactive({
  isSpectator: false,
  rank: { region: '', tier: '', division: 4, lp: 0 },
  subRank: { region: '', tier: '', division: 4, lp: 0 },
  mainPosition: '',
  subPositions: [],
  note: '',
})

const title = computed(() => {
  if (props.mode === 'editOther') return `编辑 ${props.target?.nickname || ''} 的报名 / 替补`
  if (props.mode === 'editSelf') return '修改我的报名 / 替补'
  return '报名 / 替补'
})
const okText = computed(() => (form.isSpectator ? '参与替补' : '保存报名'))

function normalizeRank(rank) {
  if (!rank) return null
  return {
    region: rank.region || '',
    tier: rank.tier || '',
    division: rank.division || 4,
    lp: rank.lp || 0,
  }
}

function fill() {
  const target = props.target || {}
  const memberRank = normalizeRank(target.rank) || normalizeRank(session.member?.rank) || {
    region: session.meta?.regions?.[0] || '',
    tier: 'GOLD',
    division: 4,
    lp: 0,
  }
  form.isSpectator = Boolean(target.isSpectator)
  form.rank = memberRank
  form.subRank = normalizeRank(target.subRank) || { ...memberRank }
  form.mainPosition = target.mainPosition || ''
  form.subPositions = [...(target.subPositions || [])]
  form.note = target.note || ''
}

watch(
  () => props.visible,
  (value) => {
    if (value) fill()
  }
)

// 先选补位再选主选时，自动把重复的位置从补位中移除；
// 补位图标仍显示，只是变成灰色禁用状态。
watch(
  () => form.mainPosition,
  (value) => {
    if (value) form.subPositions = form.subPositions.filter((code) => code !== value)
  }
)

function selectMain(code) {
  form.mainPosition = code
}

function close() {
  emit('update:visible', false)
}

async function submit() {
  if (!form.isSpectator) {
    if (!form.mainPosition && form.subPositions.length === 0) {
      Message.warning('主选位置和补位位置至少填写一个')
      return
    }
    if (form.subPositions.includes(form.mainPosition)) {
      Message.warning('补位位置不能与主选位置相同')
      return
    }
  }

  saving.value = true
  try {
    const payload = {
      isSpectator: form.isSpectator,
      rank: form.rank,
      subRank: form.subRank,
      mainPosition: form.mainPosition || null,
      subPositions: form.subPositions,
      note: form.note,
    }
    if (props.mode === 'editOther') {
      await api.updateRegistration(
        session.group.key,
        props.event.id,
        props.target.memberId,
        payload
      )
    } else {
      await api.register(session.group.key, props.event.id, payload)
    }
    Message.success(form.isSpectator ? '已参与替补' : '报名已保存')
    emit('saved')
    close()
  } catch (error) {
    Message.error(error.message)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.target-line {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  padding: 10px 12px;
  border-radius: 8px;
  background: #f7f8fa;
  font-size: 13px;
}
.target-label {
  color: var(--app-muted);
}
.main-positions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.main-choice {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  padding: 4px;
  border: 1px solid var(--app-border);
  border-radius: 50%;
  background: #0b1118;
  cursor: pointer;
  transition: all 0.18s ease;
}
.main-choice:hover {
  border-color: rgba(255, 70, 85, 0.55);
  transform: translateY(-1px);
}
.main-choice.selected {
  border-color: #ff4655;
  background: rgba(255, 70, 85, 0.18);
  box-shadow: 0 0 0 2px rgba(255, 70, 85, 0.22), 0 0 16px rgba(255, 70, 85, 0.35);
}
</style>
