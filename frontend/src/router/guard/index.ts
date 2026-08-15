/**
 * 文件用途：Router guard 注册入口。
 * 核心逻辑：将进度条、权限和标题守卫按固定顺序挂载到 Vue Router。
 * 关键注意事项：注册顺序会影响导航副作用和失败回滚，新增 guard 前需确认不会重复调用跳转控制。
 * 重构建议：保持注册函数轻量，并为高风险 guard 单独提供可测工厂或纯判断函数。
 */
import type { Router } from 'vue-router'
import { createProgressGuard } from './progress'
import { createDocumentTitleGuard } from './title'
import { createPermissionGuard } from './permission'

/**
 * 注册全局路由守卫。
 *
 * @param router - Vue Router 实例
 */
export function createRouterGuard(router: Router) {
  createProgressGuard(router)
  createPermissionGuard(router)
  createDocumentTitleGuard(router)
}
