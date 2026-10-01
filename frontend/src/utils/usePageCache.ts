/*
 * 文件用途：提供页面缓存控制 Hook。
 * 核心逻辑：封装当前路由相关的 keep-alive/cache 操作入口。
 * 关键注意事项：缓存状态会影响页面刷新、标签关闭和路由返回体验。
 * 重构建议：可与全局 tab 缓存逻辑统一，减少重复缓存控制。
 */
/** 用于恢复页面参数 */
import { useRoute } from 'vue-router'

/**
 * 页面参数缓存上限。
 *
 * key 是 route.path，含动态段（如 /device/details/:id）时组合数无上限，
 * 长会话下会一直堆积；超出后按插入顺序淘汰最旧的条目。
 */
const PAGE_CACHE_MAX_ENTRIES = 100

const queryCache = new Map<string, Record<string, any>>()

/** 淘汰最旧条目，保证缓存不会无上限增长。 */
function evictOldestPageCache() {
  while (queryCache.size > PAGE_CACHE_MAX_ENTRIES) {
    const oldestKey = queryCache.keys().next().value as string | undefined
    if (oldestKey === undefined) break
    queryCache.delete(oldestKey)
  }
}

export const usePageCache = () => {
  const route = useRoute()
  return {
    cache: {
      ...queryCache.get(route.path)
    } as Record<string, any>,
    setCache: (data: Record<string, any>) => {
      queryCache.set(route.path, {
        ...queryCache.get(route.path),
        ...data
      })
      evictOldestPageCache()
    }
  }
}
