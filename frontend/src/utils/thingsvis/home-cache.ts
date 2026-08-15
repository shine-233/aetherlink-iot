/*
 * 文件用途：管理 ThingsVis 首页选择缓存。
 * 核心逻辑：按当前用户生成缓存 key，读写首页 dashboard 状态并支持清理。
 * 关键注意事项：用户隔离和 sysadmin/setup 状态会影响首页落点，不能跨账号复用缓存。
 * 重构建议：可补充多用户、空用户和缓存过期策略测试。
 */
import type { VisualizationHomeDashboard } from '@/service/visualization-provider/home-dashboard'
import { getCurrentUserInfo, resolveThingsVisSpaceId } from './space'

const THINGSVIS_HOME_CACHE_PREFIX = 'thingsvis-home-cache'
const THINGSVIS_HOME_CACHE_TTL_MS = 60_000

export type ThingsVisHomeCacheState = 'thingsvis' | 'classic' | 'sysadmin-setup'

interface ThingsVisHomeCacheEntry {
  state: ThingsVisHomeCacheState
  dashboard?: VisualizationHomeDashboard | null
  expiresAt: number
}

function getCacheKey(): string | null {
  const userInfo = getCurrentUserInfo()
  if (!userInfo) return null

  return `${THINGSVIS_HOME_CACHE_PREFIX}:${resolveThingsVisSpaceId(userInfo)}`
}

export function readThingsVisHomeCache(): ThingsVisHomeCacheEntry | null {
  const key = getCacheKey()
  if (!key) return null

  try {
    const raw = sessionStorage.getItem(key)
    if (!raw) return null

    const parsed = JSON.parse(raw) as ThingsVisHomeCacheEntry
    if (!parsed?.expiresAt || parsed.expiresAt <= Date.now()) {
      sessionStorage.removeItem(key)
      return null
    }

    return parsed
  } catch {
    return null
  }
}

export function writeThingsVisHomeCache(
  state: ThingsVisHomeCacheState,
  dashboard?: VisualizationHomeDashboard | null
): void {
  const key = getCacheKey()
  if (!key) return

  sessionStorage.setItem(
    key,
    JSON.stringify({
      state,
      dashboard: dashboard || null,
      expiresAt: Date.now() + THINGSVIS_HOME_CACHE_TTL_MS
    } satisfies ThingsVisHomeCacheEntry)
  )
}

export function clearThingsVisHomeCache(): void {
  const key = getCacheKey()
  if (!key) return

  sessionStorage.removeItem(key)
}
