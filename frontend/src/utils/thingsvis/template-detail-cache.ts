/*
 * 文件用途：管理物模型详情的本地缓存。
 * 核心逻辑：归一化物模型 ID 后读写和清理缓存，减少重复详情请求。
 * 关键注意事项：
 *   - 物模型 ID 空值或类型转换错误会导致缓存串用，因此入口先归一化。
 *   - 并发同 ID 请求共享同一个 in-flight promise（去重），但每个调用方拿到的是
 *     **独立副本**，避免调用方 A 修改详情对象后污染调用方 B。
 *   - 缓存带 TTL 与容量上限：原实现是无上限 Map，长会话下持续堆积且永远读不到新数据。
 * 重构建议：可加入按租户隔离策略。
 */
import { deviceTemplateDetail } from '@/service/api/device'
import { cloneResponseValue } from '@/service/request/dedupe'

const TEMPLATE_DETAIL_TTL_MS = 5 * 60_000
/** 容量上限：物模型 ID 基数有限，超过阈值按插入顺序淘汰最旧条目。 */
const TEMPLATE_DETAIL_MAX_ENTRIES = 200

interface CacheEntry {
  promise: Promise<any>
  expiresAt: number
}

const templateDetailCache = new Map<string, CacheEntry>()

function normalizeTemplateId(templateId?: string | number) {
  return String(templateId || '').trim()
}

/** 命中前必须做过期判定，否则 TTL 形同虚设。 */
function isExpired(entry: CacheEntry) {
  return entry.expiresAt <= Date.now()
}

/** 先清过期条目，再按插入顺序淘汰最旧的，保证 Map 不会无上限增长。 */
function evictIfNeeded() {
  templateDetailCache.forEach((entry, key) => {
    if (isExpired(entry)) templateDetailCache.delete(key)
  })

  while (templateDetailCache.size > TEMPLATE_DETAIL_MAX_ENTRIES) {
    const oldestKey = templateDetailCache.keys().next().value as string | undefined
    if (oldestKey === undefined) break
    templateDetailCache.delete(oldestKey)
  }
}

export function getCachedDeviceTemplateDetail(templateId?: string | number) {
  const normalizedTemplateId = normalizeTemplateId(templateId)
  if (!normalizedTemplateId) {
    return Promise.resolve(null)
  }

  const cached = templateDetailCache.get(normalizedTemplateId)
  if (cached && !isExpired(cached)) {
    // 命中缓存也返回副本，保持"共享的是结果、不是对象"的语义。
    return cached.promise.then((value) => cloneResponseValue(value))
  }

  if (cached) {
    templateDetailCache.delete(normalizedTemplateId)
  }

  const request = deviceTemplateDetail({ id: normalizedTemplateId }).catch((error) => {
    // 失败不留缓存，否则一次网络抖动会被永久记住。
    const current = templateDetailCache.get(normalizedTemplateId)
    if (current?.promise === request) {
      templateDetailCache.delete(normalizedTemplateId)
    }
    throw error
  })

  templateDetailCache.set(normalizedTemplateId, {
    promise: request,
    expiresAt: Date.now() + TEMPLATE_DETAIL_TTL_MS
  })
  evictIfNeeded()

  return request.then((value) => cloneResponseValue(value))
}
