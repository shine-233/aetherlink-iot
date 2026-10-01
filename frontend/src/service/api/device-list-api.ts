/**
 * 文件用途: 设备域 API wrapper —— 设备列表、分组与基础生命周期（增删改查、位置、子设备、状态历史、诊断）。
 * 核心逻辑: 对接后端 `/device`、`/device/group` 等接口，复用全局 request 实例。
 * 关键注意事项: 参数名、响应 envelope 是前后端契约面，变更前需核对后端路由与自动化测试。
 * 来源: 从 device.ts 按“设备列表”域拆分而来（见该文件头注的重构建议），签名与行为保持不变。
 */
import type { CustomAxiosRequestConfig } from '@aetherlink/axios'
import { request } from '../request'

/** 获取设备分组 */
export const getDeviceGroup = async (params: object) => {
  return await request.get('/device/group', { params })
}

/** 接入方式下拉菜单：原始协议服务字典接口 */
export const deviceDictProtocolService = async (params: CustomAxiosRequestConfig & Record<string, unknown>) => {
  return await request.get<DeviceManagement.TreeStructure>('/dict/protocol/service', params)
}
/** 接入方式下拉一级菜单 */
export const deviceDictProtocolServiceFirstLevel = async (
  params: CustomAxiosRequestConfig & Record<string, unknown>
) => {
  return await request.get<DeviceManagement.ProtocolAndService>('/service/plugin/select', params)
}
/** 接入方式下拉二级菜单 */
export const deviceDictProtocolServiceSecondLevel = async (
  params: CustomAxiosRequestConfig & Record<string, unknown>
) => {
  return await request.get<DeviceManagement.ServiceList>('/service/access/list', params)
}

/** 获取设备分组树 */
export const deviceGroupTree = async (params: CustomAxiosRequestConfig & Record<string, unknown>) => {
  return await request.get<DeviceManagement.TreeStructure>('/device/group/tree', params)
}
/** 新增设备分组 */
export const deviceGroup = async (params: { id: string; parent_id: string; name: string; description: string }) => {
  return await request.post<Api.BaseApi.Data>('/device/group', params)
}

/** 修改设备分组 */
export const putDeviceGroup = async (params: { id: string; parent_id: string; name: string; description: string }) => {
  return await request.put<Api.BaseApi.Data>('/device/group', params)
}

/** 激活设备 */
export const putDeviceActive = async (params: object) => {
  return await request.put<Api.BaseApi.Data>('/device/active', params)
}

/** 删除设备分组 */
export const deleteDeviceGroup = async (params: { id: string }) => {
  return await request.delete<Api.BaseApi.Data>(`/device/group/${params.id}`)
}

/** 获取设备分详情 */
export const deviceGroupDetail = async (params: { id: string }) => {
  return await request.get<DeviceManagement.DetailData>(`/device/group/detail/${params.id}`)
}

/** 获取设备列表 */
export const deviceList = async (params: object) => {
  return await request.get<DeviceManagement.DeviceDatas>(`/device`, {
    params
  })
}

/** 获取设备列表 */
export const deviceListByGroup = async (params: object) => {
  return await request.get<DeviceManagement.DeviceDatas>(`/device/group/relation/list`, {
    params
  })
}

/** 获取设备详情；可选透传 CustomAxiosRequestConfig（如 signal，用于条件变更后取消未归请求） */
export const deviceDetail = async (id: string, config?: CustomAxiosRequestConfig) => {
  const url = `/device/detail/${id}`
  return await request.get<DeviceManagement.DeviceDetail>(url, config)
}

/** 获取设备分组关系 */
export const deviceGroupRelation = async (params: object) => {
  return await request.post<Api.BaseApi.Data>(`/device/group/relation`, params)
}

export const getDeviceGroupRelation = async (params: object) => {
  return await request.get(`/device/group/relation`, { params })
}

/** 获取设备列表 */
export const deleteDeviceGroupRelation = async (params: object) => {
  return await request.delete2<Api.BaseApi.Data>(`/device/group/relation`, params)
}

/** 获取设备连接信息 */
export const getDeviceConnectInfo = async (params: object) => {
  return await request.get<Record<string, unknown>>(`/device/connect/info`, {
    params
  })
}

/** 获取设备连接信息 */
export const getPlugininfoByService = async (params: object) => {
  return await request.get<Api.BaseApi.Data>(`/service/plugin/info`, {
    params
  })
}

export const deviceAdd = async (params: object) => {
  return await request.post(`/device`, params)
}

export const deviceConnectForm = async (params: object) => {
  return await request.get(`/device/connect/form`, { params })
}

export const checkDevice = async (deviceNumber: string) => {
  const url = `/device/check/${encodeURIComponent(deviceNumber)}`
  return await request.get(url)
}
export const deleteDevice = async (params: { id: string }) => {
  return await request.delete<Api.BaseApi.Data>(`/device/${params.id}`)
}

// 保存设备位置
export const deviceLocation = async (params: object) => {
  return await request.put(`/device`, params)
}
/** 修改设备名称 */
export const deviceUpdate = async (params: object) => {
  return await request.put<Api.BaseApi.Data>('/device', params)
}
/** 网关下子设备列表 */
export const childDeviceTableList = async (params: { id: string; page?: number; page_size?: number }) => {
  return await request.get(`/device/sub-list/${params.id}`, {
    params
  })
}
/** 添加子设备选择列表 */
export const childDeviceSelectList = async () => {
  return await request.get(`/device/list`, {})
}
/** 添加子设备 */
export const addChildDevice = async (params: object) => {
  return await request.post(`/device/son/add`, params)
}
/** 移除子设备 */
export const removeChildDevice = async (params: object) => {
  return await request.put(`/device/sub-remove`, params)
}
// 根据设备id查自定义命令列表
export const deviceCustomCommandsIdList = async (paramsId: string) => {
  return await request.get(`/device/model/custom/commands/${paramsId}`)
}

export const deviceProtocolServiceList = async (params: object) => {
  return await request.get(`/service/plugin/select`, { params })
}

/** 获取设备状态历史记录 */
export const deviceStatusHistory = async (params: {
  device_id: string
  page: number
  page_size: number
  start_time?: number
  end_time?: number
  status?: number
}) => {
  return await request.get(`/device/status/history`, { params })
}

/** 获取设备诊断信息 */
export const deviceDiagnostics = async (deviceId: string) => {
  return await request.get(`/devices/${deviceId}/diagnostics`)
}

/** 设备在线状态查询 */
export const getDeviceOnlineStatus = async (deviceId: string) => {
  return await request.get(`/device/online/status/${deviceId}`)
}
