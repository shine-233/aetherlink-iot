/*
 * 文件用途：刷新授权路由并在需要时回到目标路径。
 * 核心逻辑：重新拉取/生成权限路由后，根据传入 fullPath 触发路由替换。
 * 关键注意事项：该工具影响登录态恢复和动态菜单刷新，需防止重复注册路由。
 * 重构建议：可把路由刷新和跳转恢复拆成两个可测步骤。
 */
import { router } from '@/router'
import { useRouteStore } from '@/store/modules/route'

export async function refreshAuthRoutes(fullPath?: string) {
  const routeStore = useRouteStore()
  const targetPath = fullPath || router.currentRoute.value.fullPath

  await routeStore.resetStore()
  const success = await routeStore.initAuthRoute()

  if (success) {
    await router.replace(targetPath)
  }

  return success
}
