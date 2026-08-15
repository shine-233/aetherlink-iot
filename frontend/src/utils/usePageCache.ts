/*
 * 文件用途：提供页面缓存控制 Hook。
 * 核心逻辑：封装当前路由相关的 keep-alive/cache 操作入口。
 * 关键注意事项：缓存状态会影响页面刷新、标签关闭和路由返回体验。
 * 重构建议：可与全局 tab 缓存逻辑统一，减少重复缓存控制。
 */
/** 用于恢复页面参数 */
import { useRoute } from 'vue-router'

const queryCache = new Map<string, Record<string, any>>()
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
    }
  }
}
