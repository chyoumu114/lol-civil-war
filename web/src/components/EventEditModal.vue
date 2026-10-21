<template>
  <a-modal
    :visible="visible"
    title="编辑组织信息"
    :width="640"
    :ok-loading="saving"
    ok-text="保存修改"
    @ok="submit"
    @cancel="close"
  >
    <a-alert type="info" :closable="false" class="edit-tip">
      组织者点击「结束比赛」后不能再修改。修改赛制/BP 后可能需要重新套用阵容。
    </a-alert>

    <a-form :model="form" layout="vertical" @submit.prevent>
      <a-form-item label="标题（可选）">
        <a-input v-model="form.title" placeholder="例如：周五夜战 · 冠军争夺" :max-length="60" show-word-limit />
        <template #extra>不填则默认显示「大区 · 赛制」</template>
      </a-form-item>

      <a-form-item label="比赛大区" required>
        <a-select v-model="form.region" placeholder="选择大区" allow-search :options="regionOptions" />
      </a-form-item>

      <a-form-item label="赛制" required>
        <a-radio-group v-model="form.bestOf" type="button">
          <a-radio :value="1">BO1</a-radio>
          <a-radio :value="3">BO3</a-radio>
          <a-radio :value="5">BO5</a-radio>
        </a-radio-group>
      </a-form-item>

      <a-form-item label="BP 禁用">
        <a-switch v-model="form.hasBp" />
        <span class="form-hint">开启后对局记录会显示 BP 禁用模块</span>
      </a-form-item>

      <a-row :gutter="16">
        <a-col :xs="24" :md="12">
          <a-form-item label="报名截止时间" required>
            <a-date-picker
              v-model="form.signupDeadline"
              show-time
              format="YYYY-MM-DD HH:mm"
              value-format="YYYY-MM-DDTHH:mm:ssZ"
              placeholder="选择日期时间"
              style="width: 100%"
            />
            <template #extra>{{ weekdayText(form.signupDeadline) }}</template>
          </a-form-item>
        </a-col>
        <a-col :xs="24" :md="12">
          <a-form-item label="比赛开始时间" required>
            <a-date-picker
              v-model="form.startsAt"
              show-time
              format="YYYY-MM-DD HH:mm"
              value-format="YYYY-MM-DDTHH:mm:ssZ"
              placeholder="选择日期时间"
              style="width: 100%"
            />
            <template #extra>{{ weekdayText(form.startsAt) }}</template>
          </a-form-item>
        </a-col>
      </a-row>

      <a-form-item label="其他备注">
        <a-textarea
          v-model="form.note"
          placeholder="例如：请提前 10 分钟上线，语音频道 xxx"
          :max-length="500"
          show-word-limit
          :auto-size="{ minRows: 3, maxRows: 6 }"
        />
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import { api } from '../api'
import { session } from '../stores/session'

const props = defineProps({
  visible: { type: Boolean, default: false },
  event: { type: Object, required: true },
})
const emit = defineEmits(['update:visible', 'saved'])

const saving = ref(false)
const form = reactive({
  title: '',
  region: '',
  bestOf: 3,
  hasBp: true,
  signupDeadline: '',
  startsAt: '',
  note: '',
})

const regionOptions = computed(() => (session.meta?.regions || []).map((item) => ({ label: item, value: item })))

watch(
  () => props.visible,
  (value) => {
    if (!value) return
    form.title = props.event.title || ''
    form.region = props.event.region || session.meta?.regions?.[0] || ''
    form.bestOf = props.event.bestOf || 3
    form.hasBp = Boolean(props.event.hasBp)
    form.signupDeadline = props.event.signupDeadline || ''
    form.startsAt = props.event.startsAt || ''
    form.note = props.event.note || ''
  }
)

function weekdayText(value) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return '周' + '日一二三四五六'[date.getDay()]
}

function parseDate(value) {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

function close() {
  emit('update:visible', false)
}

async function submit() {
  const deadline = parseDate(form.signupDeadline)
  const startsAt = parseDate(form.startsAt)
  if (!form.region) {
    Message.warning('请选择比赛大区')
    return
  }
  if (!deadline || !startsAt) {
    Message.warning('请选择报名截止时间和比赛开始时间')
    return
  }
  // 修改时允许把截止时间设为过去（手动截止），只要求开始时间晚于截止时间
  if (startsAt.getTime() <= deadline.getTime()) {
    Message.warning('比赛开始时间必须晚于报名截止时间')
    return
  }

  saving.value = true
  try {
    await api.updateEvent(session.group.key, props.event.id, {
      title: form.title,
      region: form.region,
      bestOf: form.bestOf,
      hasBp: form.hasBp,
      signupDeadline: deadline.toISOString(),
      startsAt: startsAt.toISOString(),
      note: form.note,
    })
    Message.success('组织信息已更新')
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
.edit-tip {
  margin-bottom: 14px;
}
.form-hint {
  margin-left: 10px;
  color: var(--app-muted);
  font-size: 12px;
}
</style>
