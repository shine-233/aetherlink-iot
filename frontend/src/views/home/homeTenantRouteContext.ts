// 文件用途：推导首页的路由 onboarding 状态与租户上下文标识。
// 核心逻辑：从路由 query/hash 解析首设备 onboarding 入口，从用户信息解析首跑租户与 native 租户 ID。
// 关键注意事项：onboarding 判定同时支持 query 与 hash 两种入口链接形式，调整需同步生成方。
import { computed, type ComputedRef } from 'vue'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import { getHomeFirstRunTenantId } from './homeFirstRunWizard'
import { readNativeBoardTenantContext } from '@/service/visualization-provider/native-tenant-context'

type HomeTenantRouteContextOptions = {
  route: Pick<RouteLocationNormalizedLoaded, 'query' | 'hash'>
  userInfo: () => Record<string, unknown> | null | undefined
}

export type HomeTenantRouteContext = {
  isFirstDeviceOnboardingRoute: ComputedRef<boolean>
  homeFirstRunTenantId: ComputedRef<string>
  nativeHomeTenantId: ComputedRef<string>
  hasHomeFirstRunTenantContext: ComputedRef<boolean>
  hasNativeHomeTenantContext: ComputedRef<boolean>
}

export function resolveHomeTenantRouteContext(options: HomeTenantRouteContextOptions): HomeTenantRouteContext {
  const { route, userInfo } = options

  const isFirstDeviceOnboardingRoute = computed(() => {
    const routeHash = String(route.hash || '').replace(/^#/, '')
    return route.query.onboarding === 'first-device' || routeHash.startsWith('first-device')
  })
  const homeFirstRunTenantId = computed(() => getHomeFirstRunTenantId(userInfo()))
  const nativeHomeTenantId = computed(() => homeFirstRunTenantId.value || readNativeBoardTenantContext(userInfo()))
  const hasHomeFirstRunTenantContext = computed(() => Boolean(homeFirstRunTenantId.value))
  const hasNativeHomeTenantContext = computed(() => Boolean(nativeHomeTenantId.value))

  return {
    isFirstDeviceOnboardingRoute,
    homeFirstRunTenantId,
    nativeHomeTenantId,
    hasHomeFirstRunTenantContext,
    hasNativeHomeTenantContext
  }
}
