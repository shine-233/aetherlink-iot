/**
 * 文件用途：验证 全局状态单元测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia } from 'pinia'

const hoisted = vi.hoisted(() => ({
  routerPush: vi.fn(),
  getAllTabs: vi.fn(),
  getDefaultHomeTab: vi.fn(),
  getTabByRoute: vi.fn(),
  isTabInTabs: vi.fn(),
  filterTabsById: vi.fn(),
  filterTabsByIds: vi.fn(),
  getFixedTabIds: vi.fn(),
  findTabByRouteName: vi.fn(),
  updateTabByI18nKey: vi.fn(),
  updateTabsByI18nKey: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    getRoutes: vi.fn(() => []),
    currentRoute: { value: { name: 'home', path: '/home', meta: {}, query: {} } }
  })
}))

vi.mock('@vueuse/core', () => ({
  useEventListener: vi.fn()
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({
    routerPush: hoisted.routerPush,
    route: { value: { meta: { constant: false } } }
  })
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    set: vi.fn(),
    get: vi.fn(),
    remove: vi.fn()
  }
}))

vi.mock('../modules/theme', () => ({
  useThemeStore: () => ({
    tab: { cache: false }
  })
}))

vi.mock('../modules/tab/shared', () => ({
  getAllTabs: hoisted.getAllTabs,
  getDefaultHomeTab: hoisted.getDefaultHomeTab,
  getTabByRoute: hoisted.getTabByRoute,
  isTabInTabs: hoisted.isTabInTabs,
  filterTabsById: hoisted.filterTabsById,
  filterTabsByIds: hoisted.filterTabsByIds,
  getFixedTabIds: hoisted.getFixedTabIds,
  findTabByRouteName: hoisted.findTabByRouteName,
  updateTabByI18nKey: hoisted.updateTabByI18nKey,
  updateTabsByI18nKey: hoisted.updateTabsByI18nKey
}))

import { useTabStore } from '../modules/tab'

const mockTab = (id: string, overrides: Record<string, any> = {}): App.Global.Tab => ({
  id,
  label: `Tab ${id}`,
  routeKey: id as any,
  routePath: `/${id}`,
  fullPath: `/${id}`,
  ...overrides
})

describe('tab store', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    vi.clearAllMocks()
    pinia = createPinia()

    hoisted.getDefaultHomeTab.mockReturnValue(mockTab('home'))
    hoisted.getAllTabs.mockImplementation((tabs, homeTab) => homeTab ? [homeTab, ...tabs] : [])
    hoisted.getTabByRoute.mockImplementation((route: any) => mockTab(route.name || route.path || 'unknown'))
    hoisted.isTabInTabs.mockImplementation((id, tabs) => tabs.some((t: any) => t.id === id))
    hoisted.filterTabsById.mockImplementation((id, tabs) => tabs.filter((t: any) => t.id !== id))
    hoisted.filterTabsByIds.mockImplementation((ids, tabs) => tabs.filter((t: any) => !ids.includes(t.id)))
    hoisted.getFixedTabIds.mockImplementation((tabs) => tabs.filter((t: any) => t.fixedIndex !== undefined).map((t: any) => t.id))
    hoisted.findTabByRouteName.mockReturnValue(undefined)
    hoisted.updateTabByI18nKey.mockImplementation(tab => tab)
    hoisted.updateTabsByI18nKey.mockImplementation(tabs => tabs)
  })

  describe('initial state', () => {
    it('has empty tabs on init', () => {
      const store = useTabStore(pinia)
      expect(store.tabs).toEqual([])
    })

    it('activeTabId is empty string on init', () => {
      const store = useTabStore(pinia)
      expect(store.activeTabId).toBe('')
    })
  })

  describe('initHomeTab', () => {
    it('calls getDefaultHomeTab', () => {
      const store = useTabStore(pinia)
      store.initHomeTab()
      expect(hoisted.getDefaultHomeTab).toHaveBeenCalledTimes(1)
    })
  })

  describe('addTab', () => {
    it('adds a new tab and sets it as active', () => {
      const store = useTabStore(pinia)
      store.initHomeTab()

      const route = { name: 'dashboard', path: '/dashboard', meta: {}, query: {} } as any
      store.addTab(route)

      expect(hoisted.getTabByRoute).toHaveBeenCalledWith(route)
      expect(store.activeTabId).toBe('dashboard')
    })

    it('does not add duplicate tabs', () => {
      const store = useTabStore(pinia)
      store.initHomeTab()

      const route = { name: 'dashboard', path: '/dashboard', meta: {}, query: {} } as any
      store.addTab(route)

      hoisted.isTabInTabs.mockReturnValue(true)
      store.addTab(route)

      expect(hoisted.getTabByRoute).toHaveBeenCalledTimes(2)
    })

    it('does not add home tab to tabs list', () => {
      const store = useTabStore(pinia)
      store.initHomeTab()

      const homeRoute = { name: 'home', path: '/home', meta: {}, query: {} } as any
      hoisted.getTabByRoute.mockReturnValue(mockTab('home'))
      store.addTab(homeRoute)

      expect(store.activeTabId).toBe('home')
    })

    it('does not set active when active=false', () => {
      const store = useTabStore(pinia)
      store.initHomeTab()
      store.activeTabId = 'existing-tab'

      const route = { name: 'new-tab', path: '/new-tab', meta: {}, query: {} } as any
      hoisted.getTabByRoute.mockReturnValue(mockTab('new-tab'))
      store.addTab(route, false)

      expect(store.activeTabId).toBe('existing-tab')
    })
  })

  describe('setActiveTabId', () => {
    it('exposes activeTabId as writable public state', () => {
      const store = useTabStore(pinia)
      store.activeTabId = 'tab-1'
      expect(store.activeTabId).toBe('tab-1')
    })
  })

  describe('removeTab', () => {
    it('removes a non-active tab without switching route', async () => {
      const store = useTabStore(pinia)
      store.initHomeTab()
      store.activeTabId = 'active-tab'

      hoisted.filterTabsById.mockReturnValue([])
      await store.removeTab('other-tab')

      expect(hoisted.filterTabsById).toHaveBeenCalledWith('other-tab', expect.any(Array))
      expect(hoisted.routerPush).toHaveBeenCalledTimes(0)
    })
  })

  describe('removeActiveTab', () => {
    it('calls removeTab with activeTabId', async () => {
      const store = useTabStore(pinia)
      store.initHomeTab()
      store.activeTabId = 'tab-to-remove'

      hoisted.filterTabsById.mockReturnValue([])
      await store.removeActiveTab()

      expect(hoisted.filterTabsById).toHaveBeenCalledWith('tab-to-remove', expect.any(Array))
    })
  })

  describe('clearTabs', () => {
    it('clears non-fixed tabs', async () => {
      const store = useTabStore(pinia)
      store.initHomeTab()
      store.activeTabId = 'home'

      hoisted.getFixedTabIds.mockReturnValue([])
      hoisted.filterTabsByIds.mockReturnValue([])

      await store.clearTabs()

      expect(hoisted.getFixedTabIds).toHaveBeenCalledTimes(1)
    })
  })

  describe('isTabRetain', () => {
    it('returns true for home tab', () => {
      const store = useTabStore(pinia)
      store.initHomeTab()

      expect(store.isTabRetain('home')).toBe(true)
    })

    it('returns true for fixed tabs', () => {
      const store = useTabStore(pinia)
      store.initHomeTab()

      hoisted.getFixedTabIds.mockReturnValue(['fixed-tab'])
      expect(store.isTabRetain('fixed-tab')).toBe(true)
    })

    it('returns false for non-fixed, non-home tabs', () => {
      const store = useTabStore(pinia)
      store.initHomeTab()

      hoisted.getFixedTabIds.mockReturnValue([])
      expect(store.isTabRetain('normal-tab')).toBe(false)
    })
  })

  describe('updateTabsByLocale', () => {
    it('calls updateTabsByI18nKey for tabs and homeTab', () => {
      const store = useTabStore(pinia)
      store.initHomeTab()
      store.updateTabsByLocale()

      expect(hoisted.updateTabsByI18nKey).toHaveBeenCalledTimes(1)
    })
  })
})
