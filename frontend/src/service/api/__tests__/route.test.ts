/**
 * 文件用途: 路由和元素权限 wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证用户路由、元素权限和 UI 元素列表请求。
 * 关键注意事项: 菜单可见性还依赖 `management.adapter.ts` 和路由 guard，不能只看 HTTP 调用。
 * 重构建议: 分离路由获取和元素权限测试，并补未知路由与空菜单边界。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => {
  const requestFn = vi.fn()
  return {
    requestFn,
    mockGet: vi.fn(),
    mockPost: vi.fn(),
    mockPut: vi.fn(),
    mockDelete: vi.fn(),
    adapterOfFetchUserRouterList: vi.fn(),
    adapterOfFetchRouterList: vi.fn()
  }
})

vi.mock('@/service/request', () => ({
  request: Object.assign(hoisted.requestFn, {
    get: hoisted.mockGet,
    post: hoisted.mockPost,
    put: hoisted.mockPut,
    delete: hoisted.mockDelete
  })
}))

vi.mock('@/service/api/management.adapter', () => ({
  adapterOfFetchUserRouterList: hoisted.adapterOfFetchUserRouterList,
  adapterOfFetchRouterList: hoisted.adapterOfFetchRouterList
}))

import {
  addElement,
  delElement,
  editElement,
  fetchElementList,
  fetchGetUserRoutes,
  fetchUIElementList
} from '../route'

describe('route API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.adapterOfFetchUserRouterList.mockImplementation(list => [{ adapted: 'user', list }])
    hoisted.adapterOfFetchRouterList.mockImplementation(data => [{ adapted: 'admin', total: data.total }])
  })

  it('fetches user routes and adapts backend menu list in-place', async () => {
    const backendList = [{ element_code: 'home' }]
    hoisted.requestFn.mockResolvedValue({
      data: {
        list: backendList
      }
    })

    const result = await fetchGetUserRoutes()

    expect(hoisted.requestFn).toHaveBeenCalledWith({ url: '/ui_elements/menu' })
    expect(hoisted.adapterOfFetchUserRouterList).toHaveBeenCalledWith(backendList)
    expect(result.data.list).toEqual([{ adapted: 'user', list: backendList }])
  })

  it('keeps user route response untouched when data or list is missing', async () => {
    hoisted.requestFn.mockResolvedValueOnce(null)
    expect(await fetchGetUserRoutes()).toBeNull()

    hoisted.requestFn.mockResolvedValueOnce({ data: null })
    expect(await fetchGetUserRoutes()).toEqual({ data: null })

    expect(hoisted.adapterOfFetchUserRouterList).toHaveBeenCalledTimes(0)
  })

  it('fetches admin UI element list with params and adapts paged data', async () => {
    const backendData = {
      total: 2,
      list: [{ element_code: 'dashboard' }]
    }
    hoisted.mockGet.mockResolvedValue({
      data: backendData
    })

    const params = { page: 1, page_size: 10, element_code: 'dashboard' }
    const result = await fetchElementList(params)

    expect(hoisted.mockGet).toHaveBeenCalledWith('/ui_elements', { params })
    expect(hoisted.adapterOfFetchRouterList).toHaveBeenCalledWith(backendData)
    expect(result.data.list).toEqual([{ adapted: 'admin', total: 2 }])
  })

  it('uses empty params by default when fetching admin UI elements', async () => {
    hoisted.mockGet.mockResolvedValue({ data: { total: 0, list: [] } })

    await fetchElementList()

    expect(hoisted.mockGet).toHaveBeenCalledWith('/ui_elements', { params: {} })
  })

  it('creates, edits, and deletes UI elements with expected endpoints', async () => {
    hoisted.mockPost.mockResolvedValue({ data: { id: 'route-1' } })
    hoisted.mockPut.mockResolvedValue({ data: { id: 'route-1' } })
    hoisted.mockDelete.mockResolvedValue({ data: null })

    const createPayload = { element_code: 'new_menu', param1: '/new/menu' }
    const editPayload = { id: 'route-1', element_code: 'edited_menu' }

    await addElement(createPayload)
    await editElement(editPayload)
    await delElement('route-1')

    expect(hoisted.mockPost).toHaveBeenCalledWith('/ui_elements', createPayload)
    expect(hoisted.mockPut).toHaveBeenCalledWith('/ui_elements', editPayload)
    expect(hoisted.mockDelete).toHaveBeenCalledWith('/ui_elements/route-1')
  })

  it('fetches selectable UI element form list and falls back to an empty array', async () => {
    hoisted.mockGet.mockResolvedValueOnce({
      data: {
        list: [{ element_code: 'home' }]
      }
    })
    expect(await fetchUIElementList()).toEqual([{ element_code: 'home' }])
    expect(hoisted.mockGet).toHaveBeenCalledWith('/ui_elements/select/form')

    hoisted.mockGet.mockResolvedValueOnce({ data: null })
    expect(await fetchUIElementList()).toEqual([])
  })
})
