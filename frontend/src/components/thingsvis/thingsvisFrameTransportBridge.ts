/**
 * 文件说明：
 * - 承接 ThingsVis iframe 宿主侧的 transport 壳，集中 targetOrigin 解析、可信消息判定和 postMessage 出口。
 * - 让 AppFrame 继续聚焦 bridge 装配，避免把 iframe 安全边界与平台数据回推细节重新堆回父组件。
 * 维护提示：
 * - `targetOrigin` 必须来自 ThingsVis Studio URL，不能放宽成 `*`。
 * - `tv:platform-data` 需要保留“带 deviceId 的定向回推 + 通用 fields 广播”双路兼容语义。
 */
import type { Ref } from 'vue'
import { parseTrustedGuestMessage } from '@/components/thingsvis/hostBridge'
import {
  isTrustedThingsVisFrameEvent,
  postToThingsVisFrame,
  resolveThingsVisTargetOrigin
} from '@/components/thingsvis/thingsvisFrameBridge'
import type { TrustedThingsVisFrameMessage } from '@/components/thingsvis/thingsvisFrameMessageDispatcher'
import { getThingsVisStudioBaseUrl } from '@/utils/thingsvis/constants'

type ThingsVisFrameTransportOptions = {
  iframeRef: Ref<HTMLIFrameElement | undefined>
  url: Ref<string>
}

export type ThingsVisFrameTransportBridge = {
  getTargetOrigin: () => string
  getTrustedThingsVisFrameMessage: (event: MessageEvent) => TrustedThingsVisFrameMessage | null
  postToThingsVis: (type: string, payload: Record<string, unknown>) => void
  postPlatformData: (fields: Record<string, unknown>, deviceId?: string, dataSourceId?: string) => void
}

export function createThingsVisFrameTransportBridge(
  options: ThingsVisFrameTransportOptions
): ThingsVisFrameTransportBridge {
  function getThingsVisTargetOrigin(): string {
    return resolveThingsVisTargetOrigin(
      options.url.value || getThingsVisStudioBaseUrl(),
      window.location.href,
      window.location.origin
    )
  }

  function isTrustedThingsVisMessageEvent(event: MessageEvent): boolean {
    return isTrustedThingsVisFrameEvent(
      event,
      options.iframeRef.value?.contentWindow,
      getThingsVisTargetOrigin()
    )
  }

  function getTrustedThingsVisFrameMessage(event: MessageEvent): TrustedThingsVisFrameMessage | null {
    return parseTrustedGuestMessage<Record<string, unknown>>(event, {
      isTrusted: isTrustedThingsVisMessageEvent
    })
  }

  function postToThingsVis(type: string, payload: Record<string, unknown>) {
    postToThingsVisFrame(
      options.iframeRef.value?.contentWindow,
      type,
      payload,
      getThingsVisTargetOrigin()
    )
  }

  function postPlatformData(fields: Record<string, unknown>, deviceId?: string, dataSourceId?: string) {
    if (Object.keys(fields).length === 0) return

    postToThingsVis('tv:platform-data', {
      dataSourceId,
      deviceId,
      fields
    })

    if (deviceId) {
      postToThingsVis('tv:platform-data', { fields })
    }
  }

  return {
    getTargetOrigin: getThingsVisTargetOrigin,
    getTrustedThingsVisFrameMessage,
    postToThingsVis,
    postPlatformData
  }
}
