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

  // 英雄头像全部保存在 frontend/public/champions/{key}.png，本地优先；
  // 仅当本地资源缺失时才回退 CDN，避免国内网络下头像频繁加载失败。
  const urls = [`/champions/${fileKey}.png`]
  if (version) {
    urls.push(`https://ddragon.leagueoflegends.com/cdn/${version}/img/champion/${fileKey}.png`)
  }
  urls.push(
    `https://raw.communitydragon.org/latest/plugins/rcp-be-lol-game-data/global/default/assets/characters/${lower}/hud/${lower}_square.png`
  )
  return urls
}
