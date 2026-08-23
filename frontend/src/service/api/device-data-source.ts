/**
 * Device data source API wrappers used by selectors and first-run configuration.
 */
import { request } from '../request'

/** /device/tenant/list 返回的设备选项条目：选择器组件按 id+name 消费，其余字段保持开放。 */
export interface DeviceTenantListEntry {
  id: string
  name: string
  [key: string]: unknown
}

export const getDeviceSourceList = async (params?: Record<string, unknown>) => {
  return await request.get<DeviceTenantListEntry[]>('/device/tenant/list', params)
}

export const getDeviceMetricList = async (deviceId: string) => {
  return await request.get<Record<string, unknown>>(`/device/metrics/${deviceId}`)
}
