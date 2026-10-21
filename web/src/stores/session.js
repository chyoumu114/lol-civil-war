import { reactive } from 'vue'
import { api } from '../api'

/**
 * 全局会话/群上下文（组合式单例，不引入 Pinia）。
 * 页面切换时复用，避免重复请求。
 */
export const session = reactive({
  me: null,
  meta: null,
  group: null,
  member: null,
  profileComplete: false,
  loading: false,
  error: '',
})

export function loginUrl(redirect = window.location.pathname + window.location.search) {
  return `/auth/qq?redirect=${encodeURIComponent(redirect)}`
}

export function devLoginUrl(qq, nickname, redirect = window.location.pathname + window.location.search) {
  const params = new URLSearchParams({ qq, redirect })
  if (nickname) params.set('nickname', nickname)
  return `/auth/dev?${params.toString()}`
}

export async function loadMe() {
  session.me = await api.me()
  return session.me
}

export async function loadMeta() {
  if (session.meta) return session.meta
  session.meta = await api.meta()
  return session.meta
}

/** 加载群上下文；未登录时由后端返回 401，交由页面展示登录引导 */
export async function loadGroup(groupNumber) {
  session.loading = true
  session.error = ''
  try {
    const data = await api.group(groupNumber)
    session.group = data.group
    session.member = data.member
    session.profileComplete = data.profileComplete
    if (data.meta) session.meta = data.meta
    return data
  } catch (error) {
    session.error = error.message
    session.group = null
    session.member = null
    throw error
  } finally {
    session.loading = false
  }
}

export async function refreshMember(groupNumber) {
  const data = await api.group(groupNumber)
  session.member = data.member
  session.profileComplete = data.profileComplete
  return data.member
}

export async function logout() {
  await api.logout?.()
  window.location.href = '/'
}
