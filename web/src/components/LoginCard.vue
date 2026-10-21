<template>
  <div class="login-card card">
    <div class="login-brand">
      <span class="login-badge">LOL</span>
      <div>
        <div class="login-title">登录后继续</div>
        <div class="login-sub">使用 QQ 登录，数据按群号隔离</div>
      </div>
    </div>

    <a-button type="primary" size="large" long :disabled="!qqConfigured" @click="goQq">
      <template #icon><icon-qq /></template>
      使用 QQ 登录
    </a-button>
    <div v-if="!qqConfigured" class="login-tip">
      QQ 互联尚未配置（QQ_CLIENT_ID / QQ_CLIENT_SECRET），本地开发请使用下方开发登录。
    </div>

    <div v-if="devLogin" class="dev-login">
      <a-divider orientation="center" style="margin: 16px 0">本地开发登录</a-divider>
      <a-space direction="vertical" fill>
        <a-input v-model="devQq" placeholder="测试 QQ 号（5-12 位数字）" allow-clear>
          <template #prefix><icon-user /></template>
        </a-input>
        <a-input v-model="devNickname" placeholder="昵称（可选）" allow-clear />
        <a-button long @click="goDev">开发登录</a-button>
      </a-space>
      <div class="dev-tip">仅 CIVILWAR_DEV_LOGIN=true 时可用，生产环境请关闭。</div>
    </div>

    <div v-if="error" class="login-error">{{ error }}</div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { devLoginUrl, loginUrl, session } from '../stores/session'

const route = useRoute()
const devQq = ref('10001')
const devNickname = ref('测试玩家')
const error = ref('')

const redirect = computed(() => route.fullPath)
const devLogin = computed(() => session.meta?.devLogin === true || session.me?.devLogin === true)
const qqConfigured = computed(() => session.meta?.qqConfigured !== false)

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
  margin-bottom: 24px;
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
