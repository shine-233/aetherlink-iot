/**
 * 文件用途：通用 Secrets Storage（ROADMAP TB-18）前端 API 客户端。
 * 核心逻辑：封装密钥的列表查询、创建、编辑、删除、明文解密与轮换重加密。
 */
import { request } from '../request'

export type SecretType = 'GENERIC' | 'API_KEY' | 'TOKEN' | 'PASSWORD' | 'CERTIFICATE' | 'OAUTH2'

export interface SecretItem {
  id: string
  tenant_id: string
  key: string
  name: string
  secret_type: SecretType
  description: string
  mask_preview: string
  key_id: string
  needs_reseal: boolean
  created_at: string
  updated_at: string
}

export interface SecretListParams {
  page?: number
  page_size?: number
  query?: string
  secret_type?: string
}

export interface SecretListResponse {
  list: SecretItem[]
  total: number
}

export interface CreateSecretParams {
  key: string
  name: string
  secret_type?: SecretType | string
  description?: string
  value: string
}

export interface UpdateSecretParams {
  name?: string
  secret_type?: SecretType | string
  description?: string
  value?: string
}

export interface RevealSecretResponse {
  id: string
  key: string
  value: string
}

/** 分页与条件查询密钥列表（脱敏） */
export const getSecretsList = async (params?: SecretListParams) => {
  return await request.get<SecretListResponse>('/secrets', { params })
}

/** 获取单条密钥详情（脱敏） */
export const getSecretDetail = async (id: string) => {
  return await request.get<SecretItem>(`/secrets/${encodeURIComponent(id)}`)
}

/** 创建密钥 */
export const createSecret = async (data: CreateSecretParams) => {
  return await request.post<SecretItem>('/secrets', data)
}

/** 更新密钥 */
export const updateSecret = async (id: string, data: UpdateSecretParams) => {
  return await request.put<SecretItem>(`/secrets/${encodeURIComponent(id)}`, data)
}

/** 删除密钥 */
export const deleteSecret = async (id: string) => {
  return await request.delete<{ deleted: boolean }>(`/secrets/${encodeURIComponent(id)}`)
}

/** 明文解密（受审操作） */
export const revealSecret = async (id: string) => {
  return await request.post<RevealSecretResponse>(`/secrets/${encodeURIComponent(id)}/reveal`)
}

/** 在线重加密轮换 */
export const resealSecret = async (id: string) => {
  return await request.post<SecretItem>(`/secrets/${encodeURIComponent(id)}/reseal`)
}
