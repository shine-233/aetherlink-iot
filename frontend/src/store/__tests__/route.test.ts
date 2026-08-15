/**
 * 文件用途：验证 全局状态单元测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia } from 'pinia'

const hoisted = vi.hoisted(() => ({
  fetchGetUserRoutes: vi.fn(),
  createRoutes: vi.fn(),
  authRoles: ['SYS_ADMIN'] as string[],
  getAuthVueRoutes: vi.fn(),
  filterAuthRoutesByRoles: vi.fn(),
  getGlobalMenusByAuthRoutes: vi.fn(),
  getCacheRouteNames: vi.fn(),
  sortRoutesByOrder: vi.fn(),
  getBreadcrumbsByRoute: vi.fn(),
  getSelectedMenuKeyPathByKey: vi.fn(),
  isRouteExistByRouteName: vi.fn(),
  updateLocaleOfGlobalMenus: vi.fn(),
  initHomeTab: vi.fn(),
  reloadPage: vi.fn(),
  addRoute: vi.fn(),
  removeRoute: vi.fn(),
  getRouteName: vi.fn((path: string) => path),
  getRoutePath: vi.fn((key: string) => `/${key}`),
  removeAddedRoute: vi.fn()
}))

vi.mock('@/service/api', () => ({
  fetchGetUserRoutes: hoisted.fetchGetUserRoutes
}))

vi.mock('@/router/routes', () => ({
  ROOT_ROUTE: { name: 'root', path: '/', redirect: '/home' },
  createRoutes: hoisted.createRoutes,
  getAuthVueRoutes: hoisted.getAuthVueRoutes
}))

vi.mock('@/router/elegant/transform', () => ({
  getRouteName: hoisted.getRouteName,
  getRoutePath: hoisted.getRoutePath
}))

vi.mock('@/router', () => ({
  router: {
    addRoute: hoisted.addRoute,
    removeRoute: hoisted.removeRoute,
    getRoutes: vi.fn(() => []),
    currentRoute: { value: { name: 'home', path: '/home', meta: {} } }
  }
}))

vi.mock('../modules/app', () => ({
  useAppStore: () => ({
    locale: 'zh-CN',
    reloadPage: hoisted.reloadPage
  })
}))

vi.mock('../modules/auth', () => ({
  useAuthStore: () => ({
    userInfo: { roles: hoisted.authRoles }
  })
}))

vi.mock('../modules/tab', () => ({
  useTabStore: () => ({
    initHomeTab: hoisted.initHomeTab
  })
}))

vi.mock('../modules/route/shared', () => ({
  filterAuthRoutesByRoles: hoisted.filterAuthRoutesByRoles,
  getBreadcrumbsByRoute: hoisted.getBreadcrumbsByRoute,
  getCacheRouteNames: hoisted.getCacheRouteNames,
  getGlobalMenusByAuthRoutes: hoisted.getGlobalMenusByAuthRoutes,
  getSelectedMenuKeyPathByKey: hoisted.getSelectedMenuKeyPathByKey,
  isRouteExistByRouteName: hoisted.isRouteExistByRouteName,
  sortRoutesByOrder: hoisted.sortRoutesByOrder,
  updateLocaleOfGlobalMenus: hoisted.updateLocaleOfGlobalMenus
}))

vi.mock('@aetherlink/hooks', () => ({
  useBoolean: () => {
    const bool = ref(false)
    return {
      bool,
      setBool: (value: boolean) => {
        bool.value = value
      }
    }
  },
  useLoading: () => ({
    loading: { value: false },
    startLoading: vi.fn(),
    endLoading: vi.fn()
  }),
  useContext: vi.fn(() => ({
    setupStore: vi.fn(),
    useStore: vi.fn()
  }))
}))

import { useRouteStore } from '../modules/route'

const readBoolean = (value: unknown) => {
  if (typeof value === 'object' && value && 'value' in value) {
    return Boolean((value as { value: unknown }).value)
  }
  return Boolean(value)
}

describe('route store', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    vi.clearAllMocks()
    vi.unstubAllEnvs()
    hoisted.authRoles = ['SYS_ADMIN']
    vi.stubEnv('VITE_AUTH_ROUTE_MODE', 'dynamic')
    vi.stubEnv('VITE_ROUTE_HOME', 'dashboard_workbench')
    pinia = createPinia()

    hoisted.addRoute.mockImplementation(() => hoisted.removeAddedRoute)
    hoisted.createRoutes.mockReturnValue({
      authRoutes: [{ name: 'home', path: '/home', meta: { order: 1 }, children: [] }],
      constantVueRoutes: [{ name: 'root', path: '/', meta: {} }]
    })
    hoisted.filterAuthRoutesByRoles.mockImplementation(routes => routes)
    hoisted.sortRoutesByOrder.mockImplementation(routes => routes)
    hoisted.getAuthVueRoutes.mockImplementation(routes =>
      routes.map((route: any) => ({ ...route, component: { name: `${route.name}-component` } }))
    )
    hoisted.getGlobalMenusByAuthRoutes.mockReturnValue([
      { key: 'home', label: 'Home', routeKey: 'home', routePath: '/home' }
    ])
    hoisted.getCacheRouteNames.mockReturnValue(['home'])
    hoisted.getBreadcrumbsByRoute.mockReturnValue([{ key: 'home', label: 'Home' }])
    hoisted.getSelectedMenuKeyPathByKey.mockReturnValue(['root', 'home'])
    hoisted.isRouteExistByRouteName.mockReturnValue(false)
    hoisted.updateLocaleOfGlobalMenus.mockImplementation(menus =>
      menus.map((menu: any) => ({ ...menu, label: `${menu.label}-localized` }))
    )
    hoisted.fetchGetUserRoutes.mockResolvedValue({
      data: { list: [{ name: 'home', path: '/home', meta: { order: 1 }, children: [] }] },
      error: null
    })
  })

  it('starts with empty public state', () => {
    const store = useRouteStore(pinia)
    expect(store.menus).toEqual([])
    expect(store.cacheRoutes).toEqual([])
    expect(readBoolean(store.isInitAuthRoute)).toBe(false)
    expect(store.routeHome).toBe('dashboard_workbench')
  })

  it('initializes dynamic auth routes through the public API', async () => {
    const store = useRouteStore(pinia)
    const result = await store.initAuthRoute()

    expect(result).toBe(true)
    expect(hoisted.fetchGetUserRoutes).toHaveBeenCalledTimes(1)
    expect(hoisted.sortRoutesByOrder).toHaveBeenCalledTimes(1)
    expect(hoisted.getAuthVueRoutes).toHaveBeenCalledTimes(2)
    expect(hoisted.getGlobalMenusByAuthRoutes).toHaveBeenCalledTimes(1)
    expect(hoisted.getCacheRouteNames).toHaveBeenCalledTimes(1)
    expect(hoisted.initHomeTab).toHaveBeenCalledTimes(1)
    expect(readBoolean(store.isInitAuthRoute)).toBe(true)
    expect(store.routeHome).toBe('home')
    expect(store.menus).toHaveLength(1)
    expect(store.cacheRoutes).toEqual(['home'])
  })

  it('supplements hidden visualization children from the local route tree', async () => {
    vi.stubEnv('VITE_ENABLE_THINGSVIS_COMPAT', 'Y')
    const childNames = [
      'visualization_thingsvis-dashboards',
      'visualization_thingsvis-editor',
      'visualization_thingsvis-menu-dashboard',
      'visualization_thingsvis-preview',
      'visualization_native-boards',
      'visualization_native-board',
      'visualization_native-board-editor'
    ]
    const children = childNames.map(name => ({ name, path: `/${name}`, meta: {} }))
    const visualization = { name: 'visualization', path: '/visualization', children }
    hoisted.createRoutes.mockReturnValueOnce({
      authRoutes: [visualization],
      constantVueRoutes: [{ name: 'root', path: '/', meta: {} }]
    })
    hoisted.fetchGetUserRoutes.mockResolvedValueOnce({
      data: {
        list: [{ name: 'visualization', path: '/visualization', children: [children[0]] }]
      },
      error: null
    })

    const store = useRouteStore(pinia)
    await store.initAuthRoute()

    const mergedRoutes = hoisted.sortRoutesByOrder.mock.calls[0][0] as Array<{ name: string; children?: Array<{ name: string }> }>
    expect(mergedRoutes[0].children?.map(child => child.name)).toEqual(expect.arrayContaining(childNames))
  })

  it('keeps hybrid visualization routes resolvable while the external provider is disabled', async () => {
    vi.stubEnv('VITE_ENABLE_THINGSVIS_COMPAT', 'N')
    const compatChildNames = [
      'visualization_thingsvis-dashboards',
      'visualization_thingsvis-editor',
      'visualization_thingsvis-menu-dashboard',
      'visualization_thingsvis-preview'
    ]
    const nativeChildNames = [
      'visualization_native-boards',
      'visualization_native-board',
      'visualization_native-board-editor'
    ]
    const localChildren = nativeChildNames.map(name => ({ name, path: `/${name}`, meta: {} }))
    const apiChildren = [...compatChildNames, ...nativeChildNames].map(name => ({ name, path: `/${name}`, meta: {} }))

    hoisted.createRoutes.mockReturnValueOnce({
      authRoutes: [{ name: 'visualization', path: '/visualization', children: localChildren }],
      constantVueRoutes: [{ name: 'root', path: '/', meta: {} }]
    })
    hoisted.fetchGetUserRoutes.mockResolvedValueOnce({
      data: {
        list: [{ name: 'visualization', path: '/visualization', children: apiChildren }]
      },
      error: null
    })

    const store = useRouteStore(pinia)
    await store.initAuthRoute()

    const mergedRoutes = hoisted.sortRoutesByOrder.mock.calls[0][0] as Array<{
      name: string
      children?: Array<{ name: string }>
    }>
    expect(mergedRoutes[0].children?.map(child => child.name)).toEqual([
      ...compatChildNames,
      ...nativeChildNames
    ])
  })

  it('supplements hybrid visualization routes when the dynamic API only returns the parent', async () => {
    vi.stubEnv('VITE_ENABLE_THINGSVIS_COMPAT', 'N')
    const childNames = [
      'visualization_thingsvis',
      'visualization_thingsvis-dashboards',
      'visualization_thingsvis-editor',
      'visualization_thingsvis-menu-dashboard',
      'visualization_thingsvis-preview',
      'visualization_native-boards',
      'visualization_native-board',
      'visualization_native-board-editor'
    ]
    const children = childNames.map(name => ({ name, path: `/${name}`, meta: {} }))
    hoisted.createRoutes.mockReturnValueOnce({
      authRoutes: [{ name: 'visualization', path: '/visualization', children }],
      constantVueRoutes: [{ name: 'root', path: '/', meta: {} }]
    })
    hoisted.fetchGetUserRoutes.mockResolvedValueOnce({
      data: { list: [{ name: 'visualization', path: '/visualization', children: [] }] },
      error: null
    })

    const store = useRouteStore(pinia)
    await store.initAuthRoute()

    const mergedRoutes = hoisted.sortRoutesByOrder.mock.calls[0][0] as Array<{
      name: string
      children?: Array<{ name: string }>
    }>
    expect(mergedRoutes[0].children?.map(child => child.name)).toEqual(childNames)
  })

  it('does not supplement admin-only native board management routes for tenant users', async () => {
    vi.stubEnv('VITE_ENABLE_THINGSVIS_COMPAT', 'Y')
    hoisted.authRoles = ['TENANT_USER']
    const localChildren = [
      {
        name: 'visualization_native-boards',
        path: '/visualization/native-boards',
        meta: { roles: ['SYS_ADMIN', 'TENANT_ADMIN'] }
      },
      { name: 'visualization_native-board', path: '/visualization/native-board', meta: {} },
      {
        name: 'visualization_native-board-editor',
        path: '/visualization/native-board-editor',
        meta: { roles: ['SYS_ADMIN', 'TENANT_ADMIN'] }
      }
    ]
    hoisted.createRoutes.mockReturnValueOnce({
      authRoutes: [{ name: 'visualization', path: '/visualization', children: localChildren }],
      constantVueRoutes: [{ name: 'root', path: '/', meta: {} }]
    })
    hoisted.fetchGetUserRoutes.mockResolvedValueOnce({
      data: { list: [{ name: 'visualization', path: '/visualization', children: [] }] },
      error: null
    })

    const store = useRouteStore(pinia)
    await store.initAuthRoute()

    const mergedRoutes = hoisted.sortRoutesByOrder.mock.calls[0][0] as Array<{
      name: string
      children?: Array<{ name: string }>
    }>
    expect(mergedRoutes[0].children?.map(child => child.name)).toEqual(['visualization_native-board'])
  })

  it('returns false when dynamic route fetch fails', async () => {
    hoisted.fetchGetUserRoutes.mockResolvedValueOnce({
      data: null,
      error: { message: 'Unauthorized' }
    })

    const store = useRouteStore(pinia)
    const result = await store.initAuthRoute()

    expect(result).toBe(false)
    expect(hoisted.initHomeTab).toHaveBeenCalledTimes(0)
    expect(readBoolean(store.isInitAuthRoute)).toBe(false)
  })

  it('updates localized menus through the public locale updater', async () => {
    const store = useRouteStore(pinia)
    await store.initAuthRoute()

    store.updateGlobalMenusByLocale()

    expect(hoisted.updateLocaleOfGlobalMenus).toHaveBeenCalledWith([
      { key: 'home', label: 'Home', routeKey: 'home', routePath: '/home' }
    ])
    expect(store.menus[0].label).toBe('Home-localized')
  })

  it('re-caches a route through reloadPage and keeps the public cache list stable', async () => {
    const store = useRouteStore(pinia)
    await store.initAuthRoute()

    await store.reCacheRoutesByKey('home' as any)

    expect(hoisted.reloadPage).toHaveBeenCalledTimes(1)
    expect(store.cacheRoutes).toContain('home')
  })

  it('returns false when route name cannot be derived', async () => {
    hoisted.getRouteName.mockReturnValueOnce(undefined)

    const store = useRouteStore(pinia)
    const result = await store.getIsAuthRouteExist('/nonexistent' as any)

    expect(result).toBe(false)
  })

  it('checks static route existence through the route helper', async () => {
    vi.stubEnv('VITE_AUTH_ROUTE_MODE', 'static')
    hoisted.isRouteExistByRouteName.mockReturnValueOnce(true)

    const store = useRouteStore(pinia)
    const result = await store.getIsAuthRouteExist('/home' as any)

    expect(result).toBe(true)
    expect(hoisted.isRouteExistByRouteName).toHaveBeenCalledWith('/home', [
      { name: 'home', path: '/home', meta: { order: 1 }, children: [] }
    ])
  })

  it('treats locally defined routes as existing in dynamic mode before fallback', async () => {
    hoisted.isRouteExistByRouteName.mockReturnValueOnce(true)

    const store = useRouteStore(pinia)
    const result = await store.getIsAuthRouteExist('/management/setting' as any)

    expect(result).toBe(true)
    expect(hoisted.isRouteExistByRouteName).toHaveBeenCalledWith('/management/setting', [
      { name: 'home', path: '/home', meta: { order: 1 }, children: [] }
    ])
  })

  it('computes selected menu key path from current menus', async () => {
    const store = useRouteStore(pinia)
    await store.initAuthRoute()

    const result = store.getSelectedMenuKeyPath('home')

    expect(hoisted.getSelectedMenuKeyPathByKey).toHaveBeenCalledWith('home', store.menus)
    expect(result).toEqual(['root', 'home'])
  })

  it('resets public state and removes dynamically added routes', async () => {
    const store = useRouteStore(pinia)
    await store.initAuthRoute()

    await store.resetStore()

    expect(store.menus).toEqual([])
    expect(store.cacheRoutes).toEqual([])
    expect(readBoolean(store.isInitAuthRoute)).toBe(false)
    expect(hoisted.removeAddedRoute).toHaveBeenCalledTimes(1)
  })
})
