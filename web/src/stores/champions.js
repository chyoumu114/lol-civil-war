import { reactive } from 'vue'
import { api } from '../api'

/**
 * 英雄资料库（本地静态数据）与头像版本。
 * - 列表来自 /api/champions/library（173 名，含中文名/称号/外号/位置）
 * - 头像优先使用本地静态资源 /champions/{key}.png；CDN 仅作缺失兜底
 * - 本地资源由 tools/fetch_champion_icons.py 统一更新
 */
export const champions = reactive({
  list: [],
  map: {},
  latestVersion: '',
  loaded: false,
  loading: false,
  error: '',
})

let loadingPromise = null

export async function loadChampions(force = false) {
  if (champions.loaded && !force) return champions.list
  if (loadingPromise) return loadingPromise

  champions.loading = true
  champions.error = ''
  loadingPromise = (async () => {
    try {
      const data = await api.champions({})
      champions.list = data.items || []
      const map = {}
      for (const item of champions.list) {
        map[item.key] = item
      }
      champions.map = map
      champions.loaded = true
    } catch (error) {
      champions.error = error.message
      console.error('[champions] 加载英雄资料库失败', error)
    } finally {
      champions.loading = false
      loadingPromise = null
    }
    // 版本只影响头像，失败不影响选择器
    try {
      const data = await api.championNames('latest')
      champions.latestVersion = data.latestVersion || data.version || ''
    } catch (error) {
      // 离线时忽略
    }
    return champions.list
  })()

  return loadingPromise
}

export function championInfo(key) {
  if (!key) return null
  return champions.map[key] || champions.map[key?.toLowerCase?.()] || null
}

export function championName(key) {
  return championInfo(key)?.cn || key || ''
}

export function championTitle(key) {
  return championInfo(key)?.title || ''
}

export function championIconUrls(key, version) {
  if (!key) return []
  const rawKey = String(key)
  const lower = rawKey.toLowerCase()
  // 解析成 DataDragon 的精确 key（大小写统一），本地文件按 key 命名。
  const info = champions.map[rawKey]
    || champions.map[lower]
    || Object.values(champions.map).find((item) => String(item.key || '').toLowerCase() === lower)
  const fileKey = info?.key || rawKey

  // 顺序：DataDragon CDN → 服务器本地缓存 → 组件用英雄名首字兜底。
  // 本地文件由 update-champions 命令下载到 LCW_CHAMPION_ICON_DIR。
  const iconVersion = version || champions.latestVersion
  const urls = []
  if (iconVersion) {
    urls.push(`https://ddragon.leagueoflegends.com/cdn/${iconVersion}/img/champion/${fileKey}.png`)
  }
  urls.push(`/champions/${fileKey}.png`)
  return urls
}
