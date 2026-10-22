<template>
  <div class="page">
    <h1 class="page-title">个人资料</h1>
    <p class="page-subtitle">资料仅在当前群生效；昵称和 QQ 号均为必填，QQ 号可修改但不能与群内成员重复。</p>

    <a-card title="账号" :bordered="false" class="account-card">
      <a-descriptions :column="1" size="medium">
        <a-descriptions-item label="用户编号">{{ meUser?.userNo || '-' }}</a-descriptions-item>
        <a-descriptions-item label="邮箱">{{ meUser?.email || '未绑定邮箱' }}</a-descriptions-item>
        <a-descriptions-item label="QQ 快捷登录">
          <a-tag v-if="qqBound" color="green">已绑定</a-tag>
          <a-tag v-else>未绑定</a-tag>
          <a-button
            v-if="qqConfigured && !qqBound"
            size="small"
            type="primary"
            style="margin-left: 12px"
            @click="bindQQ"
          >
            绑定 QQ
          </a-button>
          <span v-if="!qqConfigured" class="account-muted">服务端未配置 QQ 互联</span>
        </a-descriptions-item>
      </a-descriptions>
    </a-card>

    <a-card :bordered="false" class="profile-card">
      <a-form :model="form" layout="vertical" @submit.prevent>
        <a-form-item label="昵称" required>
          <a-input v-model="form.nickname" placeholder="群内展示昵称" :max-length="16" show-word-limit />
        </a-form-item>

        <a-form-item label="QQ 号" required>
          <a-input v-model="form.qqNumber" placeholder="5-12 位数字" :max-length="12" />
          <template #extra>
            <span>5-12 位数字；当前值已脱敏显示，留空或保留 * 表示不修改；输入新的完整 QQ 号即可修改，但不能与其他成员重复。</span>
          </template>
        </a-form-item>

        <a-form-item label="最高段位" required>
          <RankPicker
            v-model="form.rank"
            :regions="session.meta?.regions || []"
            :tiers="session.meta?.rankTiers || []"
          />
        </a-form-item>

        <a-form-item label="擅长位置" required>
          <PositionPicker v-model="form.positions" :positions="session.meta?.positions || []" />
        </a-form-item>

        <a-space>
          <a-button type="primary" :loading="saving" @click="save">保存资料</a-button>
          <a-button @click="reset">重置</a-button>
        </a-space>
      </a-form>
    </a-card>
  </div>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import PositionPicker from '../components/PositionPicker.vue'
import RankPicker from '../components/RankPicker.vue'
import { api } from '../api'
import { qqBindUrl, session } from '../stores/session'

const route = useRoute()
const saving = ref(false)

const form = reactive({
  nickname: '',
  qqNumber: '',
  rank: { region: '', tier: '', division: 4, lp: 0 },
  positions: [],
})

const meUser = computed(() => session.me?.user || null)
const qqBound = computed(() => meUser.value?.qqBound === true)
const qqConfigured = computed(() => session.meta?.qqConfigured === true)

function fill() {
  const member = session.member
  if (!member) return
  form.nickname = member.nickname || ''
  form.qqNumber = member.qqNumber || ''
  form.rank = member.rank
    ? {
        region: member.rank.region,
        tier: member.rank.tier,
        division: member.rank.division || 4,
        lp: member.rank.lp || 0,
      }
    : { region: session.meta?.regions?.[0] || '', tier: 'GOLD', division: 4, lp: 0 }
  form.positions = member.positions || []
}

function reset() {
  fill()
}

function bindQQ() {
  window.location.href = qqBindUrl(route.fullPath)
}

async function save() {
  if (!form.nickname.trim()) {
    Message.warning('请填写昵称')
    return
  }
  saving.value = true
  try {
    const data = await api.updateProfile(session.group.key, {
      nickname: form.nickname,
      qqNumber: form.qqNumber,
      rank: form.rank,
      positions: form.positions,
    })
    session.member = data.member
    session.profileComplete = data.profileComplete
    Message.success('资料已保存')
  } catch (error) {
    Message.error(error.message)
  } finally {
    saving.value = false
  }
}

watch(() => session.member, fill, { immediate: true })
</script>

<style scoped>
.account-card,
.profile-card {
  max-width: 560px;
  margin-bottom: 16px;
}
.account-muted {
  margin-left: 12px;
  color: var(--app-muted);
  font-size: 12px;
}
</style>
