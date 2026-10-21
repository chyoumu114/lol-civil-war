<template>
  <a-modal
    :visible="visible"
    title="自动匹配阵容"
    :width="920"
    unmount-on-close
    @cancel="close"
  >
    <a-spin :loading="loading" style="width: 100%">
      <div v-if="error" class="plan-error">
        <a-result status="warning" :title="error" />
      </div>

      <template v-else-if="result">
        <div class="plan-head">
          <div>
            共 {{ result.candidateCount }} 人报名，返回 {{ result.plans.length }} 个最平衡方案
            <span v-if="result.exhausted" class="muted">（搜索达到节点上限，结果可能不是全局最优）</span>
          </div>
          <a-radio-group v-model="selected" type="button" size="small" class="plan-switch">
            <a-radio v-for="(plan, index) in result.plans" :key="index" :value="index">
              方案 {{ index + 1 }} · 差 {{ plan.diff }}
            </a-radio>
          </a-radio-group>
        </div>

        <div v-if="currentPlan" class="plan-body">
          <div class="plan-side team1">
            <div class="side-head">
              <span>1队</span>
              <b>{{ currentPlan.team1Power }}</b>
            </div>
            <div v-for="item in currentPlan.team1" :key="item.registrationId" class="side-row">
              <PositionIcon :code="item.position" :size="22" />
              <span class="side-name">{{ item.name }}</span>
              <span class="side-rank">{{ item.rankLabel }}</span>
              <span class="side-power">{{ item.power }}</span>
            </div>
          </div>
          <div class="plan-side team2">
            <div class="side-head">
              <span>2队</span>
              <b>{{ currentPlan.team2Power }}</b>
            </div>
            <div v-for="item in currentPlan.team2" :key="item.registrationId" class="side-row">
              <PositionIcon :code="item.position" :size="22" />
              <span class="side-name">{{ item.name }}</span>
              <span class="side-rank">{{ item.rankLabel }}</span>
              <span class="side-power">{{ item.power }}</span>
            </div>
          </div>
        </div>

      </template>
    </a-spin>

    <template #footer>
      <div class="plan-foot">
        <span class="muted">战力 = 段位基础分 × 大区系数 × (1 + 位置影响力)，四舍五入。</span>
        <a-button type="primary" :loading="applying" :disabled="!currentPlan" @click="apply">套用到表单</a-button>
      </div>
    </template>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import PositionIcon from './PositionIcon.vue'
import { api } from '../api'
import { session } from '../stores/session'

const props = defineProps({
  visible: { type: Boolean, default: false },
  event: { type: Object, required: true },
})
const emit = defineEmits(['update:visible', 'applied'])

const loading = ref(false)
const applying = ref(false)
const error = ref('')
const result = ref(null)
const selected = ref(0)

const currentPlan = computed(() => result.value?.plans?.[selected.value] || null)

watch(
  () => props.visible,
  async (value) => {
    if (!value) return
    loading.value = true
    error.value = ''
    result.value = null
    selected.value = 0
    try {
      result.value = await api.autoMatch(session.group.key, props.event.id)
      if (!result.value.plans?.length) {
        error.value = '没有找到可行的阵容方案'
      }
    } catch (e) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }
)

function close() {
  emit('update:visible', false)
}

async function apply() {
  if (!currentPlan.value) return
  applying.value = true
  try {
    await api.applyRoster(session.group.key, props.event.id, currentPlan.value)
    Message.success('阵容已套用')
    emit('applied')
    close()
  } catch (e) {
    Message.error(e.message)
  } finally {
    applying.value = false
  }
}
</script>

<style scoped>
.plan-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
  font-size: 13px;
}
.plan-body {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.plan-side {
  border: 1px solid var(--app-border);
  border-radius: 12px;
  overflow: hidden;
}
.plan-side.team1 .side-head { background: #e8f0ff; color: #165dff; }
.plan-side.team2 .side-head { background: #ffece8; color: #d4380d; }
.side-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  font-weight: 700;
}
.side-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  border-top: 1px solid #f2f3f5;
  font-size: 13px;
}
.side-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.side-rank {
  color: var(--app-muted);
  font-size: 12px;
}
.side-power {
  font-weight: 700;
  color: #4e5969;
  min-width: 42px;
  text-align: right;
}
.plan-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.plan-switch {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.plan-switch :deep(.arco-radio-button) {
  white-space: nowrap;
  font-size: 12px;
}
.plan-error {
  padding: 12px 0;
}
@media (max-width: 640px) {
  .plan-body {
    grid-template-columns: 1fr;
  }
}
</style>
