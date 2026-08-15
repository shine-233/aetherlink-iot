/*
 * 文件用途：管理物模型详情的本地缓存。
 * 核心逻辑：归一化物模型 ID 后读写和清理缓存，减少重复详情请求。
 * 关键注意事项：物模型 ID 空值或类型转换错误会导致缓存串用。
 * 重构建议：可加入缓存过期和按租户隔离策略。
 */
import { deviceTemplateDetail } from '@/service/api/device'

const templateDetailCache = new Map<string, Promise<any>>()

function normalizeTemplateId(templateId?: string | number) {
  return String(templateId || '').trim()
}

export function clearCachedDeviceTemplateDetail(templateId?: string | number) {
  const normalizedTemplateId = normalizeTemplateId(templateId)
  if (!normalizedTemplateId) return
  templateDetailCache.delete(normalizedTemplateId)
}

export function getCachedDeviceTemplateDetail(templateId?: string | number) {
  const normalizedTemplateId = normalizeTemplateId(templateId)
  if (!normalizedTemplateId) {
    return Promise.resolve(null)
  }

  const cached = templateDetailCache.get(normalizedTemplateId)
  if (cached) return cached

  const request = deviceTemplateDetail({ id: normalizedTemplateId }).catch(error => {
    templateDetailCache.delete(normalizedTemplateId)
    throw error
  })

  templateDetailCache.set(normalizedTemplateId, request)
  return request
}
