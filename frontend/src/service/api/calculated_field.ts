/**
 * 文件用途：计算字段（遥测派生指标）API 封装。
 * 核心逻辑：封装 /calculated_fields 的分页查询、详情、创建、更新、启停开关与删除。
 * 关键注意事项：enabled 开关走 PUT :id/toggle，body 可省略表示按当前值取反；
 *   错误码契约：非法 expression 返回 100002；资源不存在返回 100404。
 */
import { request } from '../request'

export interface CalculatedFieldRow {
  id: string
  tenant_id: string
  name: string
  device_template_id: string
  output_key: string
  expression: string
  enabled: boolean
  remark?: string | null
  created_at: string
  updated_at: string
}

export interface CalculatedFieldListParams {
  page: number
  page_size: number
  device_template_id?: string
  name?: string
}

export interface CalculatedFieldListResult {
  total: number
  list: CalculatedFieldRow[]
}

export interface CalculatedFieldUpsertParams {
  name: string
  device_template_id: string
  output_key: string
  expression: string
  remark?: string | null
}

export interface CalculatedFieldCreateParams extends CalculatedFieldUpsertParams {
  enabled?: boolean
}

export const getCalculatedFields = async (params: CalculatedFieldListParams) => {
  return await request.get<CalculatedFieldListResult>('/calculated_fields', { params })
}

export const getCalculatedField = async (id: string) => {
  return await request.get<CalculatedFieldRow>(`/calculated_fields/${id}`)
}

export const createCalculatedField = async (params: CalculatedFieldCreateParams) => {
  return await request.post<CalculatedFieldRow>('/calculated_fields', params)
}

export const updateCalculatedField = async (id: string, params: CalculatedFieldUpsertParams) => {
  return await request.put<CalculatedFieldRow>(`/calculated_fields/${id}`, params)
}

/** enabled 省略时后端按当前值取反。 */
export const toggleCalculatedField = async (id: string, enabled?: boolean) => {
  const data = enabled === undefined ? {} : { enabled }
  return await request.put<CalculatedFieldRow>(`/calculated_fields/${id}/toggle`, data)
}

export const deleteCalculatedField = async (id: string) => {
  return await request.delete<null>(`/calculated_fields/${id}`)
}
