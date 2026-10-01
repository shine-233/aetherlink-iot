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
export const editThemeSetting = async (params: Record<string, unknown>) => {
  const data = await request.put<Api.BaseApi.Data>('/logo', params)
  return data
}

/** 获取常规设置 - 数据清理设置列表 */
export const fetchDataClearList = async (params: Record<string, unknown>) => {
  const data = await request.get<Api.GeneralSetting.DataClear | null>('/datapolicy', {
    params
  })
  return data
}

/** 编辑清理设置 */
export const editDataClear = async (params: { list?: unknown } | Record<string, unknown>) => {
  const data = await request.put<Api.BaseApi.Data>('/datapolicy', params)
  return data
}

/** 新增行级（租户/档案粒度）数据清理策略（TB-15R，138.sql；行级仅支持设备数据 data_type=1） */
export const createDataClear = async (params: {
  data_type: string
  tenant_id: string
  device_config_id?: string | null
  retention_days: number
  enabled: string
  remark?: string | null
}) => {
  const data = await request.post<Api.BaseApi.Data>('/datapolicy', params)
  return data
}

/** 删除行级数据清理策略（TB-15R；全局默认行由后端拒绝删除） */
export const deleteDataClear = async (id: string) => {
  const data = await request.delete<Api.BaseApi.Data>(`/datapolicy/${id}`)
  return data
}

/** 租户分页列表（行级数据清理策略的租户选择器数据源） */
export const fetchTenantOptions = async (params?: Record<string, unknown>) => {
  return await request.get<Api.BaseApi.Data | any>('/tenants', { params })
}

/** 设备档案分页列表（行级策略的档案选择器数据源，前端按 tenant_id 过滤） */
export const fetchDeviceConfigOptions = async (params?: Record<string, unknown>) => {
  return await request.get<Api.BaseApi.Data | any>('/device_config', { params })
}

/** 编辑清理设置 */
export const dictQuery = async (params: Record<string, unknown>) => {
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
