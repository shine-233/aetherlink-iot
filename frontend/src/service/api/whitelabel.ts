/**
 * 文件用途：白标（TB-47）租户翻译覆盖与自定义 CSS 的前端 API 客户端。
 * 核心逻辑：封装覆盖获取（登录后可读）、翻译覆盖批量 UPSERT/删除/列表与自定义 CSS 读写。
 * 关键注意事项：lang 取值受后端白名单限制（zh-cn/en-us/es-es/fr-fr）；DELETE /whitelabel/translations
 *   请求体携带 items（走 request.delete2）；css 空串语义为"清除自定义样式"。
 * 重构建议：若后续开放匿名登录页覆盖获取，新增独立公开端点而非放开本组接口的鉴权。
 */
import { request } from '../request'

/** 翻译覆盖条目（UPSERT 键 = lang + key） */
export interface WhitelabelTranslationItem {
  lang: string
  key: string
  value: string
}

/** 翻译覆盖删除定位键（不携带 value） */
export interface WhitelabelTranslationKey {
  lang: string
  key: string
}

/** 翻译覆盖列表响应（后端 {total, list} 形态） */
export interface WhitelabelTranslationListResponse {
  total: number
  list: Array<{
    id: string
    tenant_id: string
    lang: string
    key: string
    value: string
    created_at?: string
    updated_at?: string
  }>
}

/** 批量写入响应 */
export interface WhitelabelUpsertResponse {
  count: number
}

/** 批量删除响应 */
export interface WhitelabelDeleteResponse {
  deleted: number
}

/** 自定义 CSS 响应（管理面回显） */
export interface WhitelabelCustomCSSResponse {
  css: string
  updated_at?: string | null
}

/** 登录后可读的覆盖获取响应：translations 按 lang 分组为 key→value 平铺映射 */
export interface WhitelabelOverrides {
  tenant_id: string
  translations: Record<string, Record<string, string>>
  css: string
}

/** 登录后可读：获取本租户（或 SYS_ADMIN 全局）的翻译覆盖与自定义 CSS */
export const fetchWhitelabelOverrides = async () => {
  return await request.get<WhitelabelOverrides>('/whitelabel/overrides')
}

/** 管理面：翻译覆盖列表（lang 可选过滤，需先按白名单规范化） */
export const fetchTenantTranslations = async (lang?: string) => {
  const params = lang ? { lang } : undefined
  return await request.get<WhitelabelTranslationListResponse>('/whitelabel/translations', { params })
}

/** 管理面：批量 UPSERT 翻译覆盖（同键覆盖 value） */
export const upsertTenantTranslations = async (items: WhitelabelTranslationItem[]) => {
  return await request.put<WhitelabelUpsertResponse>('/whitelabel/translations', { items })
}

/** 管理面：批量删除翻译覆盖（请求体携带 items，走 delete2） */
export const deleteTenantTranslations = async (items: WhitelabelTranslationKey[]) => {
  return await request.delete2<WhitelabelDeleteResponse>('/whitelabel/translations', { items })
}

/** 管理面：读取自定义 CSS（回显编辑框） */
export const fetchTenantCustomCSS = async () => {
  return await request.get<WhitelabelCustomCSSResponse>('/whitelabel/custom-css')
}

/** 管理面：写入自定义 CSS（空串=清除；后端拒绝 '</style' 序列与超长） */
export const upsertTenantCustomCSS = async (css: string) => {
  return await request.put<null>('/whitelabel/custom-css', { css })
}
