<template>
  <div class="complete-page">
    <a-card :bordered="false" class="complete-card">
      <div class="complete-brand">
        <span class="complete-badge">QQ</span>
        <div>
          <div class="complete-title">完成账号注册</div>
          <div class="complete-sub">首次使用 QQ 快捷登录，需要设置邮箱和密码</div>
        </div>
      </div>

      <a-alert v-if="error" type="error" :content="error" style="margin-bottom: 16px" />

      <a-spin :loading="checking" style="width: 100%">
        <template v-if="pending">
          <div class="qq-info">
            <a-avatar :size="44" :image-url="pending.avatar || undefined">
              {{ (pending.nickname || 'Q').slice(0, 1) }}
            </a-avatar>
            <div>
              <div class="qq-name">{{ pending.nickname || 'QQ 用户' }}</div>
              <div class="qq-tip">邮箱已存在时，会校验密码并绑定到该账号</div>
            </div>
          </div>

          <a-space direction="vertical" fill size="medium">
            <a-input v-model="email" placeholder="邮箱">
              <template #prefix><icon-email /></template>
            </a-input>
            <a-input-password v-model="password" placeholder="密码（至少 8 位）">
              <template #prefix><icon-lock /></template>
            </a-input-password>
            <a-input-password v-model="confirm" placeholder="确认密码" @press-enter="submit">
              <template #prefix><icon-lock /></template>
            </a-input-password>
            <a-button type="primary" size="large" long :loading="submitting" @click="submit">
              完成注册并登录
            </a-button>
            <a-button long @click="goHome">返回首页</a-button>
          </a-space>
        </template>
        <template v-else>
          <a-result status="warning" title="没有待完成的 QQ 登录" sub-title="请重新发起 QQ 快捷登录，或使用邮箱登录。">
            <template #extra>
              <a-button type="primary" @click="goHome">返回首页</a-button>
            </template>
          </a-result>
        </template>
      </a-spin>
    </a-card>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'

const route = useRoute()
const pending = ref(null)
const checking = ref(true)
const submitting = ref(false)
const error = ref('')

const email = ref('')
const password = ref('')
const confirm = ref('')

onMounted(async () => {
  try {
    const data = await api.qqPending()
    pending.value = data.pending ? data : null
  } catch (e) {
    error.value = e.message
  } finally {
    checking.value = false
  }
})

function goHome() {
  window.location.href = '/'
}

function safeRedirect() {
  const raw = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
  if (!raw.startsWith('/') || raw.startsWith('//')) return '/'
  return raw
}

async function submit() {
  error.value = ''
  if (!email.value.trim() || !password.value) {
    error.value = '请输入邮箱和密码'
    return
  }
  if (password.value.length < 8) {
    error.value = '密码至少 8 位'
    return
  }
  if (password.value !== confirm.value) {
    error.value = '两次输入的密码不一致'
    return
  }
  submitting.value = true
  try {
    await api.completeQQ({ email: email.value.trim(), password: password.value })
    window.location.href = safeRedirect()
  } catch (e) {
    error.value = e.message
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.complete-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
  background:
    radial-gradient(900px 480px at 15% 10%, rgba(22, 93, 255, 0.35), transparent 60%),
    radial-gradient(800px 500px at 90% 85%, rgba(114, 46, 209, 0.35), transparent 60%),
    #0b1020;
}
.complete-card {
  width: 100%;
  max-width: 440px;
  padding: 8px;
}
.complete-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}
.complete-badge {
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  border-radius: 12px;
  background: linear-gradient(135deg, #165dff, #722ed1);
  color: #fff;
  font-weight: 800;
  font-size: 14px;
}
.complete-title {
  font-size: 18px;
  font-weight: 700;
}
.complete-sub {
  margin-top: 2px;
  color: var(--app-muted);
  font-size: 12px;
}
.qq-info {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  margin-bottom: 16px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: #fafbfc;
}
.qq-name {
  font-weight: 700;
}
.qq-tip {
  margin-top: 2px;
  color: var(--app-muted);
  font-size: 12px;
}
</style>
