/**
 * 文件说明：
 * - 封装 ThingsVis dashboard 配置归一化、canvas 背景兼容和 401 后重取 token 的请求重试。
 * - AppFrame、保存桥接和初始化桥接都可以复用这里的纯辅助逻辑，避免在宿主组件中堆积配置细节。
 * 维护提示：
 * - dashboard config 会被保存、预览、初始化共用，改动时要确认 nodes、dataSources、variables 和 canvas 字段仍保持兼容。
 * - 401 重试只负责清除 ThingsVis token 并重发同一个请求，不应在这里加入 UI 跳转或全局登出副作用。
 */
import { clearThingsVisToken } from '@/utils/thingsvis'

type ThingsVisApiResult = {
  error?: {
    status?: number
  } | null
}

export async function retryThingsVisRequestAfterUnauthorized<T extends ThingsVisApiResult>(
  request: () => Promise<T>
): Promise<T> {
  let result = await request()

  if (result.error?.status === 401) {
    clearThingsVisToken()
    result = await request()
  }

  return result
}

export function cloneDashboardConfig<T>(config: T): T {
  if (!config || typeof config !== 'object') return config
  return JSON.parse(JSON.stringify(config))
}

export function normalizeDashboardConfig<T>(config: T): T {
  return cloneDashboardConfig(config)
}

export function normalizeCanvasBackground(background: unknown): Record<string, unknown> {
  if (background && typeof background === 'object' && !Array.isArray(background)) {
    return background as Record<string, unknown>
  }

  const color = typeof background === 'string' && background.trim().length > 0 ? background : 'transparent'

  return { color }
}
