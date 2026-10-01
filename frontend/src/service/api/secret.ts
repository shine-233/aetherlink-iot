/**
 * 文件用途：通用 Secrets Storage（ROADMAP TB-18）前端 API 客户端。
 * 核心逻辑：封装密钥的列表查询、创建、编辑、删除、明文解密与轮换重加密。
 */
import { request } from '../request'
import { createResource } from './resource'

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

const secrets = createResource<
  SecretListParams,
  SecretListResponse,
  SecretItem,
  CreateSecretParams,
  UpdateSecretParams & { id: string },
  { deleted: boolean }
>({ collection: '/secrets', updateStyle: 'item-path' })

/** 分页与条件查询密钥列表（脱敏） */
export const getSecretsList = secrets.list

/** 创建密钥 */
export const createSecret = secrets.create

/** 更新密钥：PUT /secrets/{id}，保持原有 (id, data) 调用签名 */
export const updateSecret = (id: string, data: UpdateSecretParams) => secrets.update({ ...data, id })

/** 删除密钥 */
export const deleteSecret = secrets.remove

/** 明文解密（受审操作） */
export const revealSecret = async (id: string) => {
  return await request.post<RevealSecretResponse>(`/secrets/${encodeURIComponent(id)}/reveal`)
}

/** 在线重加密轮换 */
export const resealSecret = async (id: string) => {
  return await request.post<SecretItem>(`/secrets/${encodeURIComponent(id)}/reseal`)
}
