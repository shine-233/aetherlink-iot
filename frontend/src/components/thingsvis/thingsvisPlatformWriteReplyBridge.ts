/**
 * 文件说明：
 * - 编排 ThingsVis iframe 发起的平台写入请求，并将执行结果回推给 iframe。
 * - 该模块复用纯解析/发布规则，只负责把宿主运行时字段、平台 API、postMessage 回调和日志策略串起来。
 * 维护提示：
 * - requestId 为空时保持不回推结果，避免改变旧版 iframe 的匿名写入行为。
 * - `PlatformWriteValidationError` 属于可预期业务错误，不写 error 日志；其他异常仍需要记录，方便排查 API 或网络问题。
 */
import type { PlatformField } from '@/utils/thingsvis/types'
import {
  PlatformWriteValidationError,
  publishPlatformWrite,
  resolvePlatformWriteErrorMessage,
  resolvePlatformWriteRequest,
  type PlatformWritePublishDependencies
} from '@/components/thingsvis/thingsvisPlatformWriteBridge'

type PlatformWriteLogger = {
  error: (...args: unknown[]) => void
}

export type PlatformWriteResultPayload = {
  success: boolean
  echo?: unknown
  error?: string
}

export type ThingsVisPlatformWriteReplyBridgeOptions = {
  resolveDeviceId: (payload: Record<string, unknown>) => string | undefined
  getDeviceFields: (deviceId: string) => Array<Pick<PlatformField, 'id' | 'name' | 'dataType'>>
  postResult: (requestId: string | undefined, payload: PlatformWriteResultPayload) => void
  dependencies: PlatformWritePublishDependencies
  logger: PlatformWriteLogger
}

export function createThingsVisPlatformWriteReplyBridge(options: ThingsVisPlatformWriteReplyBridgeOptions) {
  async function handlePlatformWrite(payload: Record<string, unknown>, requestId?: string) {
    const request = resolvePlatformWriteRequest(payload, { resolveDeviceId: options.resolveDeviceId })
    if (!request) {
      options.postResult(requestId, {
        success: false,
        error: 'Missing deviceId or data'
      })
      return
    }

    try {
      const result = await publishPlatformWrite({
        deviceId: request.deviceId,
        data: request.data,
        fields: options.getDeviceFields(request.deviceId),
        dependencies: options.dependencies
      })

      options.postResult(requestId, {
        success: true,
        echo: result?.data ?? request.data
      })
    } catch (error) {
      if (!(error instanceof PlatformWriteValidationError)) {
        options.logger.error('[AppFrame] Failed to publish platform write:', error)
      }
      options.postResult(requestId, {
        success: false,
        error: resolvePlatformWriteErrorMessage(error)
      })
    }
  }

  return {
    handlePlatformWrite
  }
}
