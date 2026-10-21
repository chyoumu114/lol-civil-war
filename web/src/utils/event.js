/** 组织展示标题：自定义标题 · 比赛大区 · 赛制 */
export function eventDisplayTitle(event) {
  if (!event) return ''
  const suffix = `${event.region || ''} · BO${event.bestOf || 3}`
  return event.title ? `${event.title} · ${suffix}` : suffix
}
