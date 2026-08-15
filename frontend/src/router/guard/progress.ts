/**
 * 文件用途：路由导航进度条守卫。
 * 核心逻辑：路由进入前启动 NProgress，路由完成后关闭 NProgress。
 * 关键注意事项：`window.NProgress` 可能不存在，调用时必须保持可选链容错。
 * 重构建议：如果后续支持更多加载状态，可抽出统一导航反馈服务。
 */
import type { Router } from 'vue-router'

export function createProgressGuard(router: Router) {
  router.beforeEach((_to, _from, next) => {
    window.NProgress?.start?.()
    next()
  })
  router.afterEach(_to => {
    window.NProgress?.done?.()
  })
}
