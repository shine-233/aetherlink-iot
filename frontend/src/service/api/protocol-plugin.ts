/**
 * 文件用途: 协议插件列表、新增、编辑和删除 API wrapper。
 * 核心逻辑: 为协议插件管理页提供插件 CRUD 请求封装。
 * 关键注意事项: 插件 ID、协议名称和配置字段会影响设备接入解析，删除前需确认后端依赖检查。
 * 重构建议: 补充协议插件请求/响应类型，并增加新增、编辑、删除的契约测试。
 */
import { request } from '../request'

/** 获取协议服务插件列表 */
export const fetchProtocolPluginList = async (params: Record<string, unknown>) => {
  const data = await request.get<Api.ApiApplyManagement.Data | null>('/service/list', {
    params
  })
  return data
}

/** 创建协议服务插件 */
export const addProtocolPlugin = async (params: Record<string, unknown>) => {
  const data = await request.post<Api.BaseApi.Data>('/service', params)
  return data
}

/** 编辑协议服务插件 */
export const editProtocolPlugin = async (params: Record<string, unknown>) => {
  const data = await request.put<Api.BaseApi.Data>('/service', params)
  return data
}

/** 删除协议服务插件 */
export const delProtocolPlugin = async (id: string) => {
  const data = await request.delete<Api.BaseApi.Data>(`/service/${id}`)
  return data
}
