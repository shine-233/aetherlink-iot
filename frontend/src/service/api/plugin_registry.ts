/** 插件框架 gRPC 网关——注册表管理 API（PHASE-D-D9） */
import { request } from '@/service/request'

export interface PluginRegistryRow {
  id: string
  name: string
  version?: string
  transport: string
  status: string
  last_heartbeat?: string
  description?: string
  created_at?: string
  updated_at?: string
}

export interface PluginCreateResp {
  plugin: PluginRegistryRow
  token: string
}

/** 登记插件（返回一次性接入凭证） */
export const createPluginRegistry = async (data: { name: string; version?: string; description?: string }) => {
  return await request.post<PluginCreateResp>('/plugins', data)
}

/** 登记列表 */
export const listPluginRegistries = async () => {
  return await request.get<PluginRegistryRow[]>('/plugins')
}

/** 删除登记 */
export const deletePluginRegistry = async (id: string) => {
  return await request.delete(`/plugins/${id}`)
}

/** 启用/禁用 */
export const setPluginRegistryEnabled = async (id: string, enabled: boolean) => {
  return await request.put(`/plugins/${id}/${enabled ? 'enable' : 'disable'}`)
}

/** 凭证轮换（返回一次性新凭证） */
export const rotatePluginToken = async (id: string) => {
  return await request.put<PluginCreateResp>(`/plugins/${id}/token`)
}

/** 下行命令下发 */
export const sendPluginDownlink = async (id: string, data: { device_number: string; identify: string; params?: Record<string, unknown> }) => {
  return await request.post(`/plugins/${id}/downlink`, data)
}
