/**
 * 文件用途：Customer（ThingsBoard 对标客户实体）前端 API 客户端。
 * 核心逻辑：封装客户档案的增删改查与客户名下设备的分配/解绑/查询。
 */
import { request } from '../request'
import { createResource } from './resource'

export interface CustomerItem {
  id: string
  name: string
  tenant_id: string
  country?: string
  state?: string
  city?: string
  address?: string
  address2?: string
  zip?: string
  phone?: string
  email?: string
  additional_info?: string
  created_at?: string
  updated_at?: string
}

export interface CustomerListParams {
  page?: number
  page_size?: number
  search?: string
}

export interface CustomerListResponse {
  list: CustomerItem[]
  total: number
  page?: number
  page_size?: number
}

export interface SaveCustomerParams {
  id?: string
  name: string
  country?: string
  state?: string
  city?: string
  address?: string
  address2?: string
  zip?: string
  phone?: string
  email?: string
  additional_info?: string
}

const customers = createResource<
  CustomerListParams,
  CustomerListResponse,
  CustomerItem,
  SaveCustomerParams,
  SaveCustomerParams & { id: string },
  boolean
>({
  // 与用户组同理：列表 /customers 复数、单体 /customer 单数。
  collection: '/customer',
  listPath: '/customers'
})

/** 分页与条件查询客户列表 */
export const getCustomersList = customers.list

/** 获取客户详情 */
export const getCustomerDetail = customers.detail

/** 创建或更新客户（后端按 id 是否存在区分） */
export const saveCustomer = customers.create

/** 删除客户（同时解除其名下设备分配） */
export const deleteCustomer = customers.remove

/** 分配设备到客户（分配即移动：设备只属一个客户） */
export const assignCustomerDevices = async (customerId: string, device_ids: string[]) => {
  return await request.post<boolean>(`/customer/${encodeURIComponent(customerId)}/devices`, { device_ids })
}

/** 从客户解绑设备 */
export const unassignCustomerDevice = async (customerId: string, deviceId: string) => {
  return await request.delete<boolean>(
    `/customer/${encodeURIComponent(customerId)}/device/${encodeURIComponent(deviceId)}`
  )
}

/** 查询客户名下设备 ID 列表 */
export const getCustomerDevices = async (customerId: string) => {
  return await request.get<{ device_ids: string[] }>(`/customer/${encodeURIComponent(customerId)}/devices`)
}
