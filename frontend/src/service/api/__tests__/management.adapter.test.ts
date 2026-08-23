/**
 * 文件用途: 菜单路由适配器的结构转换测试。
 * 核心逻辑: 构造后端菜单数据并断言适配后的前端 route、meta、component 和回退结构。
 * 关键注意事项: 该测试不发 HTTP 请求，重点是权限菜单和未知路由的前端兼容边界。
 * 重构建议: 增加单节点 helper 测试，覆盖空子节点、外链、隐藏菜单和异常组件路径。
 */
import { describe, expect, it, vi } from 'vitest'
import { adapterOfFetchUserRouterList } from '../management.adapter'

vi.mock('@/router/elegant/imports', () => ({
  layouts: {
    base: {}
  },
  views: {
    automation_scene_manage: {},
    'automation_scene-manage': {},
    home: {},
    management_setting: {}
  }
}))

vi.mock('@/router/elegant/transform', () => ({
  getRouteName: (path: string) => {
    const map: Record<string, string> = {
      '/automation/space-management': 'automation_space-management',
      '/home': 'home',
      '/management/setting': 'management_setting'
    }

    return map[path] || path.replace(/^\//, '').replace(/\//g, '_')
  }
}))

const baseMenu = {
  id: 'menu-1',
  parent_id: 'home',
  element_type: 2,
  description: 'Menu',
  multilingual: '',
  param2: '',
  param3: '0',
  orders: 1,
  remark: '',
  children: []
}

describe('management route adapter', () => {
  it('maps the automation space route alias to the scene manage view without warning', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined)

    const routes = adapterOfFetchUserRouterList([
      {
        ...baseMenu,
        element_code: 'automation space-management',
        param1: 'automation/space-management',
        route_path: 'view.automation space-management'
      } as any
    ])

    expect(routes).toHaveLength(1)
    expect(routes[0]).toMatchObject({
      name: 'automation_space-management',
      path: '/automation/scene-manage',
      component: 'view.automation_scene-manage'
    })
    expect(warn).toHaveBeenCalledTimes(0)
  })

  it('still warns for unknown invalid backend menu routes', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined)

    const routes = adapterOfFetchUserRouterList([
      {
        ...baseMenu,
        element_code: 'unknown broken-page',
        param1: 'unknown/broken-page',
        route_path: 'view.unknown broken-page'
      } as any
    ])

    expect(routes).toEqual([])
    expect(warn).toHaveBeenCalledWith('[route-adapter] skip invalid menu route:', {
      elementCode: 'unknown broken-page',
      path: 'unknown/broken-page',
      routePath: 'view.unknown broken-page'
    })
  })

  it('forces layout.base on the home root and prepends the default home overview child', () => {
    vi.spyOn(console, 'warn').mockImplementation(() => undefined)

    const routes = adapterOfFetchUserRouterList([
      {
        ...baseMenu,
        element_code: 'home',
        parent_id: '0',
        element_type: 1,
        children: [
          {
            ...baseMenu,
            id: 'menu-home-child',
            parent_id: 'home',
            element_code: 'management setting',
            param1: '/management/setting',
            route_path: 'view.management_setting'
          } as any
        ]
      } as any
    ])

    expect(routes).toHaveLength(1)
    const homeRoute = routes[0]
    expect(homeRoute.component).toBe('layout.base')
    const children = (homeRoute as { children?: Array<{ name?: string; component?: string }> }).children ?? []
    expect(children[0]?.name).toBe('home_overview')
    expect(children[0]?.component).toBe('view.home')
    expect(children[1]?.name).toBe('management_setting')
  })

  it('marks leaf routes coming from the user router list with singleLayout base', () => {
    vi.spyOn(console, 'warn').mockImplementation(() => undefined)

    const routes = adapterOfFetchUserRouterList([
      {
        ...baseMenu,
        element_code: 'automation space-management',
        param1: '/automation/space-management',
        route_path: 'view.automation space-management'
      } as any
    ])

    expect(routes).toHaveLength(1)
    const meta = (routes[0] as { meta?: { singleLayout?: string; hideInMenu?: boolean } }).meta
    expect(meta?.singleLayout).toBe('base')
  })
})
