/**
 * 文件说明：
 * - ThingsVis 宿主侧 window message 基础桥接。
 * - 负责消息结构校验、来源信任判断，以及统一的绑定/解绑封装。
 * - 更复杂的业务路由留在 ThingsVisWidget.vue，避免这里承担领域逻辑。
 */
export type TrustedGuestMessage<TPayload extends Record<string, unknown> = Record<string, unknown>> = {
  type: string
  projectId?: unknown
  requestId?: string
  payload: TPayload
  raw: Record<string, unknown>
}

export type GuestMessageTrust = {
  isTrusted(event: MessageEvent): boolean
}

export function parseTrustedGuestMessage<TPayload extends Record<string, unknown> = Record<string, unknown>>(
  event: MessageEvent,
  trust: GuestMessageTrust,
  expectedType?: string
): TrustedGuestMessage<TPayload> | null {
  const data = event.data
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  // 先过来源信任校验，再向下做消息结构解析。
  if (!trust.isTrusted(event)) return null

  const message = data as Record<string, unknown>
  if (typeof message.type !== 'string' || !message.type) return null
  if (expectedType && message.type !== expectedType) return null

  const payload =
    message.payload && typeof message.payload === 'object' && !Array.isArray(message.payload)
      ? (message.payload as TPayload)
      : ({} as TPayload)

  return {
    type: message.type,
    projectId: message.projectId,
    requestId: typeof message.requestId === 'string' ? message.requestId : undefined,
    payload,
    raw: message
  }
}

export function bindWindowGuestMessage(handler: (event: MessageEvent) => void): () => void {
  // 返回解绑函数，便于组件在 onBeforeUnmount 中集中释放监听器。
  window.addEventListener('message', handler)
  return () => window.removeEventListener('message', handler)
}
