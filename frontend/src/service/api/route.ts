/**
 * 文件用途: 用户路由、菜单元素和 UI 元素权限 API wrapper。
 * 核心逻辑: 获取后端菜单后交给路由适配器转换，并封装元素权限的增删改查。
 * 关键注意事项: 路由适配影响页面可见性，元素权限影响按钮级控制，后端字段变化要同步 adapter 测试。
 * 重构建议: 将路由获取、元素管理和 UI 权限查询拆分，并收紧 `any` 参数类型。
 */
import { adapterOfFetchRouterList, adapterOfFetchUserRouterList } from '@/service/api/management.adapter'
import { request } from '../request'

/** get user routes */
export async function fetchGetUserRoutes() {
  const data = await request<Api.Route.UserRoute>({ url: '/ui_elements/menu' })
  if (data?.data) {
    data.data.list = adapterOfFetchUserRouterList(data.data.list)
  }
  return data
}

/** 获取路由列表 */
export const fetchElementList = async (params: any = {}) => {
  const data = await request.get<Api.Route.Data>('/ui_elements', {
    params
  })
  if (data?.data) {
    data.data.list = adapterOfFetchRouterList(data.data)
  }
  return data
}

/** 添加路由 */
export const addElement = async (params: any) => {
  const data = await request.post<Api.BaseApi.Data>('/ui_elements', params)
  return data
}
/** 编辑路由 */
export const editElement = async (params: any) => {
  const data = await request.put<Api.BaseApi.Data>('/ui_elements', params)
  return data
}

/** 删除路由 */
export const delElement = async (id: string) => {
  const data = await request.delete<Api.BaseApi.Data>(`/ui_elements/${id}`)
  return data
}

/** Get UI Element List */
export const fetchUIElementList = async () => {
  const data = await request.get<Api.Route.Data>('/ui_elements/select/form')
  return data?.data?.list || []
}
