/**
 * 文件说明：
 * - 将 guest 侧发来的字段读取消息规范化为宿主侧可执行的请求对象。
 * - 核心职责是做字段数组清洗和目标设备解析，不参与实际数据获取。
 */
import { normalizeRequestedFieldIds } from '@/components/thingsvis/fieldReadBridge'

export type PlatformFieldReadRequestInput<TPayload extends { fieldIds?: unknown; deviceId?: string } | undefined> = {
  payload: TPayload
  resolveTargetDeviceId: (payload: TPayload) => string | null | undefined
}

export type PlatformFieldReadRequest<TPayload> = {
  payload: TPayload
  fieldIds: string[]
  targetDeviceId?: string
}

export function resolvePlatformFieldReadRequest<
  TPayload extends { fieldIds?: unknown; deviceId?: string } | undefined
>(input: PlatformFieldReadRequestInput<TPayload>): PlatformFieldReadRequest<TPayload> | null {
  const fieldIds = normalizeRequestedFieldIds(input.payload?.fieldIds)
  if (fieldIds.length === 0) return null

  const targetDeviceId = input.resolveTargetDeviceId(input.payload)
  // 允许 undefined，表示继续走宿主默认设备；显式 null 则代表本次请求应被拒绝。
  if (targetDeviceId === null) return null

  return {
    payload: input.payload,
    fieldIds,
    targetDeviceId: targetDeviceId || undefined
  }
}
