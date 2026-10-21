import axios from 'axios'

/**
 * 统一 HTTP 客户端：
 * - 同域 Cookie Session，withCredentials=true；
 * - axios 自动把 XSRF-TOKEN cookie 写入 X-XSRF-TOKEN 请求头；
 * - 响应直接返回 data，错误统一抛 Error（message 为后端中文提示）。
 */
export const http = axios.create({
  baseURL: '/api',
  timeout: 60000,
  withCredentials: true,
})

http.interceptors.response.use(
  (response) => response.data,
  (error) => {
    const status = error.response?.status ?? 0
    const data = error.response?.data

    // 登录失效：通知群布局切回登录卡片（由 layout 监听，避免循环依赖）
    if (status === 401) {
      window.dispatchEvent(new CustomEvent('lol:unauthorized'))
    }

    let message =
      (typeof data === 'string' && data) ||
      data?.message ||
      (error.code === 'ECONNABORTED' ? '请求超时，请稍后重试' : '网络异常，请稍后重试')
    if (status === 401 && (!data || !data.message)) message = '登录状态已失效，请重新登录'
    if (status === 419) message = '页面已过期，请刷新后重试'

    const wrapped = new Error(message)
    wrapped.status = status
    wrapped.errors = data?.errors ?? {}
    wrapped.raw = data
    return Promise.reject(wrapped)
  }
)

export const api = {
  logout: () => axios.post('/auth/logout', null, { withCredentials: true }),
  health: () => http.get('/health'),
  meta: () => http.get('/meta'),
  calculatePower: (params) => http.get('/power/calculate', { params }),
  me: () => http.get('/me'),

  resolveGroup: (groupNumber) => http.post('/groups/resolve', { groupNumber }),
  group: (group) => http.get(`/groups/${encodeURIComponent(group)}`),
  updateProfile: (group, payload) => http.put(`/groups/${encodeURIComponent(group)}/profile`, payload),

  events: (group, params) => http.get(`/groups/${encodeURIComponent(group)}/events`, { params }),
  event: (group, id) => http.get(`/groups/${encodeURIComponent(group)}/events/${id}`),
  createEvent: (group, payload) => http.post(`/groups/${encodeURIComponent(group)}/events`, payload),
  updateEvent: (group, id, payload) => http.put(`/groups/${encodeURIComponent(group)}/events/${id}`, payload),

  register: (group, id, payload) => http.post(`/groups/${encodeURIComponent(group)}/events/${id}/registrations`, payload),
  updateRegistration: (group, id, memberId, payload) =>
    http.put(`/groups/${encodeURIComponent(group)}/events/${id}/registrations/${memberId}`, payload),
  unregister: (group, id, memberId) =>
    http.delete(`/groups/${encodeURIComponent(group)}/events/${id}/registrations/${memberId}`),

  autoMatch: (group, id) => http.post(`/groups/${encodeURIComponent(group)}/events/${id}/matches/auto`),
  applyRoster: (group, id, payload) => http.post(`/groups/${encodeURIComponent(group)}/events/${id}/roster`, payload),
  endEvent: (group, id) => http.post(`/groups/${encodeURIComponent(group)}/events/${id}/end`),
  toggleSpectator: (group, id, memberId, spectator) =>
    http.post(`/groups/${encodeURIComponent(group)}/events/${id}/registrations/${memberId}/spectator`, { spectator }),
  toggleEventSpectator: (group, id, watching) =>
    http.post(`/groups/${encodeURIComponent(group)}/events/${id}/spectators`, { watching }),
  deleteMatch: (group, id, gameNo) =>
    http.delete(`/groups/${encodeURIComponent(group)}/events/${id}/matches/${gameNo}`),
  rateMatch: (group, id, gameNo, targetMemberId, score) =>
    http.post(`/groups/${encodeURIComponent(group)}/events/${id}/matches/${gameNo}/ratings`, { targetMemberId, score }),

  createMatch: (group, id) => http.post(`/groups/${encodeURIComponent(group)}/events/${id}/matches`),
  match: (group, id, gameNo) => http.get(`/groups/${encodeURIComponent(group)}/events/${id}/matches/${gameNo}`),
  saveMatch: (group, id, gameNo, payload) =>
    http.put(`/groups/${encodeURIComponent(group)}/events/${id}/matches/${gameNo}`, payload),
  uploadRofl: (group, id, gameNo, file, onProgress) => {
    const form = new FormData()
    form.append('file', file)
    return http.post(`/groups/${encodeURIComponent(group)}/events/${id}/matches/${gameNo}/rofl`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 300000,
      onUploadProgress: onProgress,
    })
  },

  ranking: (group, metric) => http.get(`/groups/${encodeURIComponent(group)}/ranking`, { params: { metric } }),
  player: (group, memberId) => http.get(`/groups/${encodeURIComponent(group)}/players/${memberId}`),

  champions: (params) => http.get('/champions/library', { params }),
  championNames: (version) => http.get('/champions', { params: { version } }),
}
