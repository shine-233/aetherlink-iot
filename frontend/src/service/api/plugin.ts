/**
 * 文件用途: 接入服务、服务访问、插件服务和协议相关 API wrapper。
 * 核心逻辑: 封装服务注册、访问配置、插件配置、协议服务和脚本调试等请求。
 * 关键注意事项: 服务 ID、协议类型、访问表单和调试接口会影响设备接入链路，删除和调试操作要谨慎验证。
 * 重构建议: 按服务注册、访问配置、协议插件、调试接口拆分大文件，并补充关键 payload 测试。
 */
import { request } from '../request'

type ServiceApiData = {
  list?: unknown[]
  total?: number
  [key: string]: unknown
}

// 获取服务列表数据
export const getServices = async (params: any) => {
  return await request.get<ServiceApiData>('/service/list', { params })
}

// 注册服务
export const registerService = async (params: any) => {
  return await request.post<ServiceApiData>('/service', params)
}

// 更新服务
export const putRegisterService = async (params: any) => {
  return await request.put<ServiceApiData>('/service', params)
}

// 删除服务
export const delRegisterService = async (id: any) => {
  return await request.delete<ServiceApiData>(`/service/${id}`)
}

// 获取租户三方接入点列表
export const getServiceAccess = async (params: any) => {
  return await request.get<ServiceApiData>('/service/access/list', { params })
}

// 获取租户三方接入点表单
export const getServiceAccessForm = async (params: any) => {
  return await request.get<ServiceApiData>('/service/access/voucher/form', {
    params
  })
}

// 删除租户三方接入点
export const delServiceAccess = async (id: any) => {
  return await request.delete<ServiceApiData>(`/service/access/${id}`)
}

// 创建三方接入点
export const createServiceDrop = async (params: any) => {
  return await request.post<ServiceApiData>('/service/access', params)
}

// 更新三方接入点
export const putServiceDrop = async (params: any) => {
  return await request.put<ServiceApiData>('/service/access', params)
}

// 三方服务设备列表查询
export const getServiceListDrop = async (params: any) => {
  return await request.get<ServiceApiData>('/service/access/device/list', {
    params
  })
}

// 设备配置下拉菜单✅
export const getSelectServiceMenuList = async (params: any) => {
  return await request.get<ServiceApiData>('/device_config/menu', {
    params
  })
}

// 批量添加服务
export const batchAddServiceMenuList = async (params: any) => {
  return await request.post<ServiceApiData>('/device/service/access/batch', params)
}
