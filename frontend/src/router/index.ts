/**
 * 文件用途：Vue Router 实例创建与安装入口。
 * 核心逻辑：根据环境选择 history 模式，加载常量路由，注册全局 guard，并等待 router ready。
 * 关键注意事项：history mode、base URL 和 guard 注册顺序会影响部署路径、登录跳转和首屏可达性。
 * 重构建议：将 history 选择与 route 创建保持可测，路由兼容变更需同步更新 route contract 测试。
 */
import type { App } from 'vue'
import {
  type RouterHistory,
  createMemoryHistory,
  createRouter,
  createWebHashHistory,
  createWebHistory
} from 'vue-router'
import { createRoutes } from './routes'
import { createRouterGuard } from './guard'

const { VITE_ROUTER_HISTORY_MODE = 'history', VITE_BASE_URL = '/' } = import.meta.env

const historyCreatorMap: Record<Env.RouterHistoryMode, (base?: string) => RouterHistory> = {
  hash: createWebHashHistory,
  history: createWebHistory,
  memory: createMemoryHistory
}

const { constantVueRoutes } = createRoutes()

export const router = createRouter({
  history: historyCreatorMap[VITE_ROUTER_HISTORY_MODE](VITE_BASE_URL),
  routes: constantVueRoutes
})

/** 安装 Vue Router。 */
export async function setupRouter(app: App) {
  app.use(router)
  createRouterGuard(router)
  await router.isReady()
}
