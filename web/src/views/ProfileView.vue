<template>
  <div class="page">
    <h1 class="page-title">个人资料</h1>
    <p class="page-subtitle">资料仅在当前群生效；QQ 号首次填写后不可修改。</p>

    <a-card :bordered="false" class="profile-card">
      <a-form :model="form" layout="vertical" @submit.prevent>
        <a-form-item label="昵称" required>
          <a-input v-model="form.nickname" placeholder="群内展示昵称" :max-length="16" show-word-limit />
        </a-form-item>

        <a-form-item label="QQ 号" required>
          <a-input
            v-model="form.qqNumber"
            placeholder="5-12 位数字"
            :disabled="qqLocked"
            :max-length="12"
          />
          <template #extra>
            <span v-if="qqLocked">QQ 号已锁定，不可修改。如需变更请联系管理员。</span>
            <span v-else>5-12 位数字；仅用于展示，首次保存后不可修改，请仔细核对。</span>
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
import { Message } from '@arco-design/web-vue'
import PositionPicker from '../components/PositionPicker.vue'
import RankPicker from '../components/RankPicker.vue'
import { api } from '../api'
import { session } from '../stores/session'

const saving = ref(false)

const form = reactive({
  nickname: '',
  qqNumber: '',
  rank: { region: '', tier: '', division: 4, lp: 0 },
  positions: [],
})

const qqLocked = computed(() => Boolean(session.member?.qqNumber))

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

async function save() {
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
.profile-card {
  max-width: 560px;
}
</style>
