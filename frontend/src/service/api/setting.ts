/**
 * 文件用途: 系统设置、主题、数据清理、字典和功能开关相关 API wrapper。
 * 核心逻辑: 封装系统配置读取与保存、字典查询、数据清理配置和功能开关修改请求。
 * 关键注意事项: 功能开关和数据清理配置会影响全局行为，字段或默认值变更需有后端证据。
 * 重构建议: 按主题、数据清理、字典、功能开关拆分模块，并补充默认值与失败分支测试。
 */
import { request } from '../request'

/** 获取常规设置 - 主题设置 */
export const fetchThemeSetting = async () => {
  const data = await request.get<Api.GeneralSetting.Theme | null>('/logo')
  return data
}

/** 获取常规设置 - 主题编辑 */
export const editThemeSetting = async (params: any) => {
  const data = await request.put<Api.BaseApi.Data>('/logo', params)
  return data
}

/** 获取常规设置 - 数据清理设置列表 */
export const fetchDataClearList = async (params: any) => {
  const data = await request.get<Api.GeneralSetting.DataClear | null>('/datapolicy', {
    params
  })
  return data
}

/** 编辑清理设置 */
export const editDataClear = async (params: any) => {
  const data = await request.put<Api.BaseApi.Data>('/datapolicy', params)
  return data
}

/** 编辑清理设置 */
export const dictQuery = async (params: any) => {
  return await request.get<Api.BaseApi.Data | any>('/dict/enum', { params })
}
/** 编辑清理设置 */
export const getFunction = async () => {
  return await request.get<Api.BaseApi.Data | any>('/sys_function')
}
/** 编辑清理设置 */
export const editFunction = async (param: { function_id: string }) => {
  return await request.put<Api.BaseApi.Data | any>(`/sys_function/${param.function_id}`)
}
