/**
 * 文件说明：
 * - 封装 ThingsVis 平台写入请求的纯解析逻辑，包括设备 ID、字段 ID、字段类型、命令参数和错误文案。
 * - AppFrame 保留实际 API 发布调用，本模块只负责把 iframe payload 转成可执行的写入意图。
 * 维护提示：
 * - telemetry、attribute、command 三类写入的字段类型语义会影响设备控制链路，新增类型时要同步宿主和 ThingsVis runtime。
 * - command 写入要求 payload 中只有一个命令字段，避免多个命令被错误合并到同一次下发。
 */
import type { PlatformField } from '@/utils/thingsvis/types'

export class PlatformWriteValidationError extends Error {}

export type PlatformWriteRequest = {
  deviceId: string
  data: unknown
}

type ResolvePlatformWriteRequestOptions = {
  resolveDeviceId: (payload: Record<string, unknown>) => string | undefined
}

export type PlatformWritePublishDependencies = {
  telemetryDataPub: (params: { device_id: string; value: string }) => Promise<any>
  attributeDataPub: (params: { device_id: string; value: string }) => Promise<any>
  commandDataPub: (params: { device_id: string; identify: string; value: string }) => Promise<any>
}

export function resolveWriteFieldId(data: unknown): string | undefined {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return undefined
  const keys = Object.keys(data as Record<string, unknown>)
  return keys.length === 1 ? keys[0] : undefined
}

export function resolveWriteFieldType(
  fields: Array<Pick<PlatformField, 'id' | 'name' | 'dataType'>>,
  fieldId?: string
): PlatformField['dataType'] | undefined {
  if (!fieldId) return undefined
  const field = fields.find((item) => item.id === fieldId || item.name === fieldId)
  return field?.dataType
}

export function resolveCommandWrite(data: unknown, fieldId?: string): { identify: string; value: string } | null {
  if (!fieldId || !data || typeof data !== 'object' || Array.isArray(data)) return null
  const params = (data as Record<string, unknown>)[fieldId]
  return {
    identify: fieldId,
    value: JSON.stringify(params ?? {})
  }
}

export function buildPlatformWriteValue(data: unknown): string {
  return typeof data === 'string' ? data : JSON.stringify(data)
}

export async function publishPlatformWriteByFieldType(options: {
  deviceId: string
  data: unknown
  value: string
  fieldType: PlatformField['dataType'] | undefined
  fieldId?: string
  dependencies: PlatformWritePublishDependencies
}): Promise<any> {
  if (options.fieldType === 'attribute') {
    return options.dependencies.attributeDataPub({ device_id: options.deviceId, value: options.value })
  }

  if (options.fieldType === 'command') {
    const commandWrite = resolveCommandWrite(options.data, options.fieldId)
    if (!commandWrite) {
      throw new PlatformWriteValidationError('Command write requires a single command field payload')
    }
    return options.dependencies.commandDataPub({
      device_id: options.deviceId,
      identify: commandWrite.identify,
      value: commandWrite.value
    })
  }

  if (options.fieldType === 'telemetry' || options.fieldType === undefined) {
    return options.dependencies.telemetryDataPub({ device_id: options.deviceId, value: options.value })
  }

  throw new PlatformWriteValidationError(`Unsupported write field type '${options.fieldType}'`)
}

export async function publishPlatformWrite(options: {
  deviceId: string
  data: unknown
  fields: Array<Pick<PlatformField, 'id' | 'name' | 'dataType'>>
  dependencies: PlatformWritePublishDependencies
}): Promise<any> {
  const value = buildPlatformWriteValue(options.data)
  const fieldId = resolveWriteFieldId(options.data)
  const fieldType = resolveWriteFieldType(options.fields, fieldId)

  return publishPlatformWriteByFieldType({
    deviceId: options.deviceId,
    data: options.data,
    value,
    fieldType,
    fieldId,
    dependencies: options.dependencies
  })
}

export function resolvePlatformWriteRequest(
  payload: Record<string, unknown>,
  options: ResolvePlatformWriteRequestOptions
): PlatformWriteRequest | null {
  const deviceId = options.resolveDeviceId(payload)
  const data = payload.data
  if (!deviceId || data === undefined) return null

  return { deviceId, data }
}

export function resolvePlatformWriteErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error || 'Platform write failed')
}
