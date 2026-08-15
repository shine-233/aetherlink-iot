/**
 * Device data source API wrappers used by selectors and first-run configuration.
 */
import { request } from '../request'

export const getDeviceSourceList = async (params: any) => {
  return await request.get<any>('/device/tenant/list', params)
}

export const getDeviceMetricList = async (deviceId: string) => {
  return await request.get<any>(`/device/metrics/${deviceId}`)
}
