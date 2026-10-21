<template>
  <div class="login-card card">
    <div class="login-brand">
      <span class="login-badge">LOL</span>
      <div>
        <div class="login-title">登录后继续</div>
        <div class="login-sub">使用邮箱账号，或绑定 QQ 快捷登录</div>
      </div>
    </div>

    <a-tabs v-model:active-key="mode" type="rounded" size="large" class="login-tabs">
      <a-tab-pane key="login" title="登录">
        <a-space direction="vertical" fill>
          <a-input v-model="loginEmail" placeholder="邮箱" allow-clear @press-enter="doLogin">
            <template #prefix><icon-email /></template>
          </a-input>
          <a-input-password v-model="loginPassword" placeholder="密码" @press-enter="doLogin">
            <template #prefix><icon-lock /></template>
          </a-input-password>
          <a-button type="primary" size="large" long :loading="loading" @click="doLogin">登录</a-button>
        </a-space>
      </a-tab-pane>

      <a-tab-pane key="register" title="注册">
        <a-space direction="vertical" fill>
          <a-input v-model="registerEmail" placeholder="邮箱" allow-clear>
            <template #prefix><icon-email /></template>
          </a-input>
          <a-input-password v-model="registerPassword" placeholder="密码（至少 8 位）">
            <template #prefix><icon-lock /></template>
          </a-input-password>
          <a-input-password v-model="registerConfirm" placeholder="确认密码" @press-enter="doRegister">
            <template #prefix><icon-lock /></template>
          </a-input-password>
          <a-button type="primary" size="large" long :loading="loading" @click="doRegister">注册并登录</a-button>
        </a-space>
      </a-tab-pane>
    </a-tabs>

    <template v-if="qqConfigured">
      <a-divider orientation="center" style="margin: 16px 0">或</a-divider>
      <a-button size="large" long @click="goQq">
        <template #icon><icon-qq /></template>
        使用 QQ 快捷登录
      </a-button>
      <div class="login-tip">首次使用 QQ 快捷登录，需要先设置邮箱和密码。</div>
    </template>
    <div v-else class="login-tip">服务端未配置 QQ 互联，可使用邮箱注册/登录。</div>

    <div v-if="devLogin" class="dev-login">
      <a-divider orientation="center" style="margin: 16px 0">本地开发登录</a-divider>
      <a-space direction="vertical" fill>
        <a-input v-model="devQq" placeholder="测试 QQ 号（5-12 位数字）" allow-clear>
          <template #prefix><icon-user /></template>
        </a-input>
        <a-input v-model="devNickname" placeholder="昵称（可选）" allow-clear />
        <a-button long @click="goDev">开发登录</a-button>
      </a-space>
      <div class="dev-tip">仅 LCW_DEV_LOGIN=true 时可用，生产环境请关闭。</div>
    </div>

    <div v-if="error" class="login-error">{{ error }}</div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'
import { devLoginUrl, loginUrl, session } from '../stores/session'

const route = useRoute()
const mode = ref('login')
const loading = ref(false)
const error = ref('')

const loginEmail = ref('')
const loginPassword = ref('')
const registerEmail = ref('')
const registerPassword = ref('')
const registerConfirm = ref('')

const devQq = ref('10001')
const devNickname = ref('测试玩家')

const redirect = computed(() => route.fullPath)
const devLogin = computed(() => session.meta?.devLogin === true || session.me?.devLogin === true)
const qqConfigured = computed(() => session.meta?.qqConfigured === true)

async function doLogin() {
  error.value = ''
  if (!loginEmail.value.trim() || !loginPassword.value) {
    error.value = '请输入邮箱和密码'
    return
  }
  loading.value = true
  try {
    await api.loginEmail({ email: loginEmail.value.trim(), password: loginPassword.value })
    window.location.reload()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function doRegister() {
  error.value = ''
  if (!registerEmail.value.trim() || !registerPassword.value) {
    error.value = '请输入邮箱和密码'
    return
  }
  if (registerPassword.value !== registerConfirm.value) {
    error.value = '两次输入的密码不一致'
    return
  }
  loading.value = true
  try {
    await api.registerEmail({ email: registerEmail.value.trim(), password: registerPassword.value })
    window.location.reload()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function goQq() {
  window.location.href = loginUrl(redirect.value)
}

function goDev() {
  error.value = ''
  if (!/^\d{5,12}$/.test(devQq.value)) {
    error.value = '测试 QQ 号必须是 5-12 位数字'
    return
  }
  window.location.href = devLoginUrl(devQq.value, devNickname.value, redirect.value)
}
</script>

<style scoped>
.login-card {
  max-width: 420px;
  margin: 48px auto;
  padding: 28px;
}
.login-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}
.login-badge {
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  border-radius: 12px;
  background: linear-gradient(135deg, #165dff, #722ed1);
  color: #fff;
  font-weight: 800;
  font-size: 13px;
  letter-spacing: 0.5px;
}
.login-title {
  font-size: 18px;
  font-weight: 700;
}
.login-sub {
  margin-top: 2px;
  color: var(--app-muted);
  font-size: 13px;
}
.login-tabs {
  margin-bottom: 4px;
}
.dev-tip,
.login-tip {
  margin-top: 8px;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.6;
}
.login-error {
  margin-top: 12px;
  color: #f53f3f;
  font-size: 13px;
}
</style>
