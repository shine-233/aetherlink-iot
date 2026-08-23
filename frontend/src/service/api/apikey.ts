/**
 * 文件用途: API Key 管理相关 API wrapper。
 * 核心逻辑: 封装密钥列表、新增、更新和删除请求，供系统管理页面维护外部访问凭证。
 * 关键注意事项: 密钥权限、租户边界和删除操作属于敏感合同，不能只依赖前端展示状态判断安全性。
 * 重构建议: 增加明确类型替代 `any`，并补充创建、更新、删除参数的契约测试。
 */
import { request } from '../request'

/** 获取keys列表 */
export const fetchKeyList = async (params: Record<string, unknown>) => {
  const data = await request.get<Api.UserManagement.KeyData | null>('/open/keys', {
    params
  })
  return data
}

/** 添加key */
export const addKey = async (params: Record<string, unknown>) => {
  const data = await request.post<Api.BaseApi.Data>('/open/keys', params)
  return data
}

/** 更新key */
export const updateKey = async (params: Record<string, unknown>) => {
  const data = await request.put<Api.BaseApi.Data>('/open/keys', params)
  return data
}
/** 删除key */
export const apiKeyDel = async (id: string) => {
  return await request.delete<Api.BaseApi.Data>(`/open/keys/${id}`)
}
