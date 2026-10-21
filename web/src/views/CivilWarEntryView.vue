<template>
  <div class="entry">
    <div class="entry-card card">
      <router-link to="/" class="entry-back"><icon-left /> 返回首页</router-link>
      <div class="entry-brand">CIVIL WAR</div>
      <h1 class="entry-title">进入内战组织</h1>
      <p class="entry-sub">输入 QQ 群号进入对应群的内战工具，不同群的数据互不相通。</p>

      <a-input
        v-model="groupNumber"
        size="large"
        placeholder="输入 QQ 群号"
        allow-clear
        class="entry-input"
        @press-enter="enter"
      >
        <template #prefix><icon-usergroup /></template>
      </a-input>
      <a-button type="primary" size="large" long :loading="loading" @click="enter">进入</a-button>

      <div v-if="error" class="entry-error">{{ error }}</div>

      <div v-if="recent.length" class="entry-recent">
        <div class="recent-title">最近访问</div>
        <div class="recent-list">
          <a-tag v-for="item in recent" :key="item.key" size="large" @click="go(item)">
            {{ item.name || `群组 ${item.key.slice(0, 6)}` }}
          </a-tag>
        </div>
      </div>

      <div class="entry-tip">群号需已在服务端白名单中开通，首次进入会自动创建群。</div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()
const groupNumber = ref('')
const loading = ref(false)
const error = ref('')
const recent = ref([])

onMounted(() => {
  const saved = JSON.parse(localStorage.getItem('lol.civilwar.recent') || '[]')
  // 新格式只保存 { key, name }，不把 QQ 群号留在 localStorage
  recent.value = Array.isArray(saved)
    ? saved.filter((item) => item && typeof item === 'object' && item.key).slice(0, 5)
    : []
})

function go(item) {
  router.push(`/civil-war/${item.key}`)
}

async function enter() {
  error.value = ''
  const group = groupNumber.value.trim()
  if (!/^\d{5,12}$/.test(group)) {
    error.value = '请输入 5-12 位数字的 QQ 群号'
    return
  }
  loading.value = true
  try {
    // 先取一次 meta，让浏览器拿到 XSRF-TOKEN，后面的 POST resolve 才能通过 CSRF
    await api.meta()
    const resolved = await api.resolveGroup(group)
    const entry = { key: resolved.key, name: resolved.name || '' }
    const saved = JSON.parse(localStorage.getItem('lol.civilwar.recent') || '[]')
    const kept = Array.isArray(saved)
      ? saved.filter((item) => item && typeof item === 'object' && item.key !== entry.key)
      : []
    localStorage.setItem('lol.civilwar.recent', JSON.stringify([entry, ...kept].slice(0, 5)))
    router.push(`/civil-war/${entry.key}`)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.entry {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
  background: linear-gradient(160deg, #f4f6fb 0%, #eef2ff 100%);
}
.entry-card {
  width: 100%;
  max-width: 440px;
  padding: 28px;
}
.entry-back {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--app-muted);
  font-size: 13px;
  margin-bottom: 20px;
}
.entry-brand {
  font-size: 11px;
  letter-spacing: 3px;
  color: var(--app-primary);
  font-weight: 700;
}
.entry-title {
  margin: 6px 0 6px;
  font-size: 26px;
  font-weight: 800;
}
.entry-sub {
  margin: 0 0 20px;
  color: var(--app-muted);
  font-size: 13px;
  line-height: 1.6;
}
.entry-input {
  margin-bottom: 12px;
}
.entry-error {
  margin-top: 12px;
  color: #f53f3f;
  font-size: 13px;
}
.entry-recent {
  margin-top: 24px;
}
.recent-title {
  font-size: 12px;
  color: var(--app-muted);
  margin-bottom: 8px;
}
.recent-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.recent-list .arco-tag {
  cursor: pointer;
}
.entry-tip {
  margin-top: 24px;
  color: var(--app-muted);
  font-size: 12px;
}
</style>
