<template>
  <div class="civil-shell">
    <header v-if="ready" class="civil-header">
      <div class="header-inner">
        <router-link to="/" class="brand">
          <span class="brand-mark">LOL</span>
          <span class="brand-text">工具</span>
        </router-link>
        <span v-if="session.group" class="group-chip">{{ session.group.name || '内战群组' }}</span>

        <nav v-if="!needLogin" class="desktop-nav">
          <router-link :to="base" exact-active-class="active">组织列表</router-link>
          <router-link :to="`${base}/ranking`" active-class="active">排行榜</router-link>
          <router-link :to="`${base}/profile`" active-class="active">个人资料</router-link>
        </nav>

        <div class="spacer" />

        <a-dropdown v-if="session.member" trigger="click">
          <button type="button" class="user-chip">
            <a-avatar :size="28" :image-url="session.me?.user?.avatar || ''">
              {{ (session.member.nickname || '?').slice(0, 1) }}
            </a-avatar>
            <span class="user-name">{{ session.member.nickname }}</span>
            <icon-down />
          </button>
          <template #content>
            <a-doption @click="router.push(`${base}/profile`)">个人资料</a-doption>
            <a-doption @click="doLogout">退出登录</a-doption>
          </template>
        </a-dropdown>
      </div>
    </header>

    <div v-if="ready && session.member && !session.profileComplete" class="profile-banner">
      <a-alert type="warning" :closable="false">
        <template #title>
          个人资料尚未完善：报名和发起组织前需要填写 QQ 号、主选段位和擅长位置。
          <router-link :to="`${base}/profile`" class="banner-link">去完善</router-link>
        </template>
      </a-alert>
    </div>

    <main class="civil-main">
      <div v-if="loading" class="center-box">
        <a-spin dot :size="28" tip="加载中…" />
      </div>

      <div v-else-if="needLogin" class="page">
        <LoginCard />
      </div>

      <a-result v-else-if="error" status="error" :title="error" :subtitle="'请确认群号是否已开通，或稍后重试'">
        <template #extra>
          <a-button type="primary" @click="router.push('/civil-war')">重新输入群号</a-button>
        </template>
      </a-result>

      <router-view v-else />

      <nav v-if="ready" class="mobile-tab">
        <router-link :to="base" exact-active-class="active">
          <icon-apps />
          <span>组织</span>
        </router-link>
        <router-link :to="`${base}/ranking`" active-class="active">
          <icon-trophy />
          <span>排行</span>
        </router-link>
        <router-link :to="`${base}/profile`" active-class="active">
          <icon-user />
          <span>我的</span>
        </router-link>
      </nav>
    </main>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import LoginCard from '../components/LoginCard.vue'
import { api } from '../api'
import { loadGroup, loadMe, loadMeta, session } from '../stores/session'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const needLogin = ref(false)
const error = ref('')
const ready = ref(false)

const base = computed(() => `/civil-war/${route.params.group}`)

async function load() {
  loading.value = true
  ready.value = false
  needLogin.value = false
  error.value = ''
  try {
    await loadMeta()
    await loadMe()
    await loadGroup(String(route.params.group))
    // 兼容旧的数字群号链接：地址栏立即换成公开 key
    if (session.group?.key && String(route.params.group) !== session.group.key) {
      router.replace(route.fullPath.replace(
        `/civil-war/${route.params.group}`,
        `/civil-war/${session.group.key}`,
      ))
    }
  } catch (e) {
    if (e.status === 401) {
      needLogin.value = true
      // 旧数字群号链接在登录页也立即换成公开 key，避免地址栏继续暴露群号
      const identifier = String(route.params.group)
      if (/^\d{5,12}$/.test(identifier)) {
        try {
          const resolved = await api.resolveGroup(identifier)
          await router.replace(route.fullPath.replace(
            `/civil-war/${identifier}`,
            `/civil-war/${resolved.key}`,
          ))
        } catch {
          // 未开通的旧群号仍交给登录/错误页处理
        }
      }
    } else {
      error.value = e.message
    }
  } finally {
    loading.value = false
    ready.value = !error.value
  }
}

async function doLogout() {
  try {
    await api.logout()
  } catch (e) {
    // 即使失败也刷新页面
  }
  window.location.href = `/civil-war/${route.params.group}`
}

function onUnauthorized() {
  session.group = null
  session.member = null
  session.profileComplete = false
  needLogin.value = true
  error.value = ''
  loading.value = false
}

onMounted(() => {
  window.addEventListener('lol:unauthorized', onUnauthorized)
  load()
})
onUnmounted(() => {
  window.removeEventListener('lol:unauthorized', onUnauthorized)
})

watch(() => route.params.group, (value, oldValue) => {
  if (value && value !== oldValue) load()
})
</script>

<style scoped>
.civil-shell {
  min-height: 100vh;
  background: var(--app-bg);
}
.civil-header {
  position: sticky;
  top: 0;
  z-index: 20;
  background: rgba(255, 255, 255, 0.88);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid var(--app-border);
}
.header-inner {
  display: flex;
  align-items: center;
  gap: 20px;
  max-width: 1600px;
  margin: 0 auto;
  padding: 10px 20px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 8px;
}
.brand-mark {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 9px;
  background: linear-gradient(135deg, #165dff, #722ed1);
  color: #fff;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.5px;
}
.brand-text {
  font-weight: 700;
  font-size: 15px;
}
.group-chip {
  padding: 2px 10px;
  border-radius: 999px;
  background: #f2f3f5;
  color: #4e5969;
  font-size: 12px;
  white-space: nowrap;
}
.desktop-nav {
  display: flex;
  gap: 4px;
  margin-left: 8px;
}
.desktop-nav a {
  padding: 6px 12px;
  border-radius: 8px;
  color: #4e5969;
  font-size: 14px;
}
.desktop-nav a:hover {
  background: #f2f3f5;
  color: var(--app-text);
}
.desktop-nav a.active {
  background: #e8f0ff;
  color: var(--app-primary);
  font-weight: 600;
}
.user-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  cursor: pointer;
  color: var(--app-text);
}
.user-chip:hover {
  background: #f2f3f5;
}
.user-name {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}
.profile-banner {
  max-width: 1600px;
  margin: 12px auto 0;
  padding: 0 20px;
}
.banner-link {
  margin-left: 6px;
  color: var(--app-primary);
  font-weight: 600;
}
.civil-main {
  min-height: calc(100vh - 56px);
}
.center-box {
  display: grid;
  place-items: center;
  padding: 120px 0;
}
.mobile-tab {
  display: none;
}
@media (min-width: 769px) {
  .header-inner {
    width: 80vw;
    max-width: none;
  }
  .profile-banner {
    width: 80vw;
    max-width: none;
  }
}

@media (max-width: 768px) {
  .header-inner {
    padding: 8px 12px;
    gap: 10px;
  }
  .desktop-nav,
  .brand-text,
  .group-chip {
    display: none;
  }
  .profile-banner {
    padding: 0 12px;
  }
  .mobile-tab {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 30;
    display: flex;
    background: rgba(255, 255, 255, 0.96);
    border-top: 1px solid var(--app-border);
    padding: 6px 0 calc(6px + env(safe-area-inset-bottom));
    backdrop-filter: blur(12px);
  }
  .mobile-tab a {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    font-size: 11px;
    color: var(--app-muted);
  }
  .mobile-tab a.active {
    color: var(--app-primary);
    font-weight: 600;
  }
  .mobile-tab .arco-icon {
    font-size: 20px;
  }
}
</style>
