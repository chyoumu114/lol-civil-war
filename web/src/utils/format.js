/** 通用格式化工具：时间、时长、数字 */

export function formatDateTime(value) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

export function formatShortDateTime(value) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

export function formatDuration(ms) {
  if (!ms || ms < 0) return '—'
  const total = Math.floor(ms / 1000)
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  return `${minutes}分${String(seconds).padStart(2, '0')}秒`
}

export function formatNumber(value) {
  return (Number(value) || 0).toLocaleString('en-US')
}

export function relativeTime(value) {
  if (!value) return '—'
  const diff = new Date(value).getTime() - Date.now()
  const abs = Math.abs(diff)
  const minute = 60 * 1000
  const hour = 60 * minute
  const day = 24 * hour
  let text
  if (abs < minute) text = '刚刚'
  else if (abs < hour) text = `${Math.floor(abs / minute)} 分钟`
  else if (abs < day) text = `${Math.floor(abs / hour)} 小时`
  else text = `${Math.floor(abs / day)} 天`
  return diff >= 0 ? `${text}后` : `${text}前`
}
