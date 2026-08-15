/*
 * 文件用途：ThingsVis 嵌入式 iframe SDK 客户端，负责宿主页与 Guest 页之间的 PostMessage 协议收发。
 * 核心职责：管理 iframe 生命周期、拼装嵌入参数、维护 ready/loaded 阶段队列，并兼容 Widget/App 两种接入模式。
 * 协议边界：`targetOrigin`、`contentWindow`、`event.source` 与 `event.origin` 校验属于安全边界，不能为兼容旧消息而放宽。
 * 维护约束：消息类型常量和 Host/Guest 收发方向需要与 Guest 端 embed 协议保持一致，修改前必须同步核对两端实现。
 */

import { getPlatformApiBase } from '@/utils/thingsvis/constants'
import { createLogger } from '@/utils/logger'

const sdkLogger = createLogger('ThingsVisSDK')

// 与 ThingsVis Guest 端协议保持一致的消息类型常量。
// 这些值一旦改名，Guest 端 MSG_TYPES、嵌入协议适配层和 Host 保存事件别名都要同步更新。
const TV_MSG = {
  // 宿主 -> Guest
  INIT: 'tv:init',
  TRIGGER_SAVE: 'tv:trigger-save',
  REQUEST_SAVE: 'tv:request-save',
  EVENT: 'tv:event',
  // Guest -> 宿主
  SAVE: 'tv:save',
  READY: 'tv:ready',
  REQUEST_INIT: 'tv:request-init',
  // SDK 内部事件名（仅 Host 内部使用）
  SAVE_CONFIG: 'tv:save-config'
} as const

export interface ThingsVisOptions {
  /** iframe 挂载容器。 */
  container: HTMLElement
  /**
   * 接入模式。
   * - widget: 产品模型挂件模式，由宿主提供数据，`saveTarget='host'`
   * - app: 完整编辑器模式，由 Guest 自管数据，`saveTarget='self'`
   */
  mode: 'widget' | 'app'
  /** iframe 地址，例如 `http://localhost:3000/#/embed` 或 `#/editor`。 */
  url: string
  /** 可选的 iframe 样式覆盖。 */
  style?: Partial<CSSStyleDeclaration>
}

export interface WidgetLoadOptions {
  platformBufferSize?: number
  platformDevices?: any[]
  deviceId?: string
  thingsvisApiBaseUrl?: string
  platformApiBaseUrl?: string
  platformToken?: string
  runtimeCapabilities?: any
}

export type MessageHandler = (payload: any) => void

export class ThingsVisClient {
  private iframe: HTMLIFrameElement
  private container: HTMLElement
  private options: ThingsVisOptions
  private targetOrigin: string
  public ready: boolean = false
  /**
   * Guest 完成 `registerAndLoad` 并发出 `LOADED` 后置为 true。
   * `tv:platform-data` / `tv:platform-history` 必须等到这一阶段再发送，
   * 否则消息会早于 PlatformFieldAdapter 挂载而被直接丢弃。
   */
  private loaded: boolean = false
  private messageHandlers: Map<string, MessageHandler[]> = new Map()
  private pendingQueue: Array<() => void> = []
  /** 需要等到 `LOADED` 之后再发送的消息队列。 */
  private postLoadQueue: Array<() => void> = []
  /** 按作用域缓存最近一次实时值，便于 Guest 重新 init 后回放。 */
  private latestPlatformDataByScope: Map<string, { fields: Record<string, unknown>; deviceId?: string }> = new Map()
  /** 按作用域和字段缓存最近一次历史值，便于 Guest 重新 init 后回放。 */
  private latestPlatformHistoryByScope: Map<
    string,
    { fieldId: string; history: Array<{ value: unknown; ts: number }>; deviceId?: string }
  > = new Map()
  private lastInitPayload: any = null
  private platformPushCount = 0

  constructor(options: ThingsVisOptions) {
    this.options = options
    this.container = options.container
    this.targetOrigin = this.resolveTargetOrigin(options.url)
    this.iframe = document.createElement('iframe')
    this.initIframe()
    this.setupMessageListener()
  }

  private resolveTargetOrigin(url: string): string {
    try {
      return new URL(url, window.location.href).origin
    } catch (error) {
      sdkLogger.warn('Failed to resolve ThingsVis iframe target origin:', error)
      return window.location.origin
    }
  }

  private initIframe() {
    // 追加嵌入参数，确保 Guest 页进入 embed 模式并隐藏默认角标按钮。
    const separator = this.resolveEmbedParamSeparator(this.options.url)
    const modeParam = 'mode=embedded&showTopLeft=0&showTopRight=0'
    const finalUrl = `${this.options.url}${separator}${modeParam}`

    this.targetOrigin = this.resolveTargetOrigin(finalUrl)
    this.iframe.src = finalUrl
    this.iframe.allowFullscreen = true
    this.iframe.allow = 'fullscreen; autoplay; clipboard-write; camera; microphone; encrypted-media; picture-in-picture'
    // 默认 iframe 样式。
    this.iframe.style.width = '100%'
    this.iframe.style.height = '100%'
    this.iframe.style.border = 'none'
    this.iframe.style.display = 'block'

    // 合并宿主传入的样式覆盖。
    if (this.options.style) {
      Object.assign(this.iframe.style, this.options.style)
    }

    this.container.appendChild(this.iframe)

    // 保留 onload 钩子，真实可通信时机仍以 Guest 主动发 READY 为准。
    this.iframe.onload = () => {
      // iframe onload 不代表内部 React 已经 hydrate 完成。
      // 当前协议仍以 Guest 发回 READY 作为可发送初始化消息的标志。
    }
  }

  private resolveEmbedParamSeparator(url: string): '?' | '&' {
    const hashIndex = url.indexOf('#')
    const routePart = hashIndex === -1 ? url : url.slice(hashIndex)
    return routePart.includes('?') ? '&' : '?'
  }

  private setupMessageListener() {
    window.addEventListener('message', this.handleMessage)
  }

  private handleMessage = (event: MessageEvent) => {
    if (!this.isTrustedMessageEvent(event)) return

    const { type, payload } = event.data || {}
    if (!type) return

    // 协议适配层：只处理 SDK 关心的消息，其余仍向外透传。

    // 1. 保存回调（Guest -> Host）
    if (type === TV_MSG.SAVE) {
      this.emit(TV_MSG.SAVE_CONFIG, payload) // 转成统一 SDK 内部事件，方便宿主订阅。
    }

    // 2. READY（Guest -> Host）：Guest 已可接收基础协议消息。
    if (type === 'READY' || type === TV_MSG.READY) {
      if (!this.ready) {
        this.ready = true
        // 必须先派发 `ready`，让宿主有机会在 flush 前调用 loadWidgetConfig()
        // 发送 `tv:init`，避免排队中的平台数据先到达，而后又被后续 init 重置。
        this.emit('ready', {})
        this.flushPendingQueue()
        // 注意：这里不能 flush postLoadQueue；平台数据必须等到 LOADED 后再发。
      }
    }

    // 3. LOADED：Guest 完成 registerAndLoad，平台字段适配器已经接好。
    if (type === 'LOADED') {
      if (!this.loaded) {
        this.loaded = true
        this.flushPostLoadQueue()
        this.replayLatestPlatformData()
      }
      this.emit('loaded', payload)
    }

    // 4. Guest 主动请求重新 init，例如内部 bootstrap 重跑之后。
    if (type === TV_MSG.REQUEST_INIT) {
      if (this.lastInitPayload) {
        this.loaded = false
        this.send(TV_MSG.INIT, this.lastInitPayload)
      } else {
        this.emit('ready', {})
      }
    }

    // 5. 其他消息保持透传，交给外部订阅者自行处理。
    this.emit(type, payload)
  }

  /**
   * 触发 SDK 内部事件总线。
   */
  private emit(type: string, payload: any) {
    const handlers = this.messageHandlers.get(type)
    if (handlers) {
      handlers.forEach(handler => handler(payload))
    }
  }

  /**
   * 直接向 iframe 发消息。
   * 这里不做 ready/loaded 判断，调用方自行决定是否排队。
   */
  private postToIframe(message: unknown): void {
    if (!this.iframe.contentWindow) return
    this.iframe.contentWindow.postMessage(message, this.targetOrigin)
  }

  /**
   * 发送基础协议消息；若 Guest 尚未 READY，则先进入 pendingQueue。
   */
  private send(type: string, payload: any = {}) {
    // 当前协议约定 Guest 接收 `{ type, payload }` 结构，不要改成扁平字段。
    const message = { type, payload }
    const action = () => this.postToIframe(message)

    if (this.ready) {
      action()
    } else {
      this.pendingQueue.push(action)
    }
  }

  private flushPendingQueue() {
    while (this.pendingQueue.length > 0) {
      const action = this.pendingQueue.shift()
      if (action) action()
    }
  }

  /**
   * 发送必须等到 Guest 适配器接好后才能消费的消息。
   * 在 `LOADED` 之前调用时会先进入 `postLoadQueue`，待 Guest 就绪后自动冲刷。
   */
  private sendWhenLoaded(type: string, payload: any = {}) {
    const message = { type, payload }
    const action = () => this.postToIframe(message)
    if (this.loaded) {
      action()
    } else {
      this.postLoadQueue.push(action)
    }
  }

  private flushPostLoadQueue() {
    while (this.postLoadQueue.length > 0) {
      const action = this.postLoadQueue.shift()
      if (action) action()
    }
  }

  private replayLatestPlatformData() {
    for (const { fields, deviceId } of this.latestPlatformDataByScope.values()) {
      this.sendWhenLoaded('tv:platform-data', { fields, deviceId })
    }

    for (const { fieldId, history, deviceId } of this.latestPlatformHistoryByScope.values()) {
      this.sendWhenLoaded('tv:platform-history', { fieldId, history, deviceId })
    }
  }

  // ===========================
  // 对外 API：Widget 模式
  // ===========================

  /**
   * [Widget Mode] 加载或更新挂件配置。
   * 方向：Host -> Guest
   * 协议：`tv:init`
   */
  public loadWidgetConfig(config: any, platformFields?: any[], options?: WidgetLoadOptions) {
    this.loaded = false

    // 容忍空配置或损坏配置，至少保证编辑器/预览器可以先以空白画布启动。
    const safeConfig = config || {}
    const safeCanvas = safeConfig.canvas || {
      mode: 'grid',
      width: 1920,
      height: 1080,
      gridCols: 24,
      gridRowHeight: 50,
      gridGap: 5
    }
    const safeNodes = safeConfig.nodes || []
    const safeVariables = Array.isArray(safeConfig.variables) ? safeConfig.variables : []

    // 将平台数据缓冲区配置合并到保存态数据源上，避免 Guest 侧遗漏 bufferSize。
    const existingDataSources: any[] = safeConfig.dataSources ?? []
    const mergedDataSources = existingDataSources.map((ds: any) => {
      const typeStr = typeof ds.type === 'string' ? ds.type.toUpperCase() : ''
      if (typeStr === 'PLATFORM_FIELD' || typeStr === 'PLATFORM') {
        const config = ds.config || {}
        return {
          ...ds,
          config: {
            ...config,
            bufferSize: Math.max(config.bufferSize ?? 0, options?.platformBufferSize ?? 0)
          }
        }
      }
      return ds
    })

    const payload = {
      // `platformDevices` 必须放在顶层，Guest 的 EmbedPage.tsx 直接从 `msg.platformDevices` 读取。
      platformDevices: options?.platformDevices ?? [],
      data: {
        meta: safeConfig.meta || { id: 'widget', name: '挂件' },
        canvas: safeCanvas,
        nodes: safeNodes,
        dataSources: mergedDataSources,
        variables: safeVariables,
        platformFields: platformFields
      },
      config: {
        saveTarget: 'host',
        thingsvisApiBaseUrl: options?.thingsvisApiBaseUrl ?? `${window.location.origin}/thingsvis-api`,
        platformApiBaseUrl: options?.platformApiBaseUrl ?? getPlatformApiBase(),
        ...(options?.platformToken ? { platformToken: options.platformToken } : {}),
        ...(options?.deviceId ? { deviceId: options.deviceId } : {})
      }
    }

    this.lastInitPayload = payload
    this.send(TV_MSG.INIT, payload)
  }

  /**
   * [Widget Mode] 更新平台字段定义，供 Guest 数据源选择器使用。
   * 方向：Host -> Guest
   *
   * 说明：Guest 目前没有专门的 `update-schema` 监听器。
   * 现在仍以 `tv:init` 携带 `platformFields` 为主，这里只是预留一个可选扩展事件。
   */
  public updateSchema(fields: any[]) {
    // 这里发送通用 editor 事件，是否生效取决于 Guest 是否实现对应适配。
    this.send(TV_MSG.EVENT, { event: 'updateSchema', payload: fields })
  }

  /**
   * [Widget Mode] 推送实时平台字段值到嵌入挂件。
   * 协议：`tv:platform-data`，由 Guest 侧 PlatformFieldAdapter 直接消费。
   *
   * @param fields 字段 ID 到当前值的映射，例如 `{ temperature: 25.3 }`
   */
  public pushPlatformFieldData(fields: Record<string, unknown>, deviceId?: string): void {
    this.platformPushCount += 1
    const scopeKey = deviceId ?? '__global__'
    this.latestPlatformDataByScope.set(scopeKey, {
      fields: { ...fields },
      ...(deviceId ? { deviceId } : {})
    })
    // 必须等到 LOADED：PlatformFieldAdapter 的 messageListener 在 registerAndLoad
    // 结束后才会挂上，而 registerAndLoad 又发生在 tv:init 处理之后。
    this.sendWhenLoaded('tv:platform-data', { fields, deviceId })
  }

  /**
   * [Widget Mode] 推送单字段历史值到嵌入挂件。
   * 协议：`tv:platform-history`，同样要求 Guest 已经进入 `LOADED` 阶段。
   */
  public pushPlatformFieldHistory(
    fieldId: string,
    history: Array<{ value: unknown; ts: number }>,
    deviceId?: string
  ): void {
    if (!fieldId || !Array.isArray(history)) return

    const scopeKey = `${deviceId ?? '__global__'}:${fieldId}`
    this.latestPlatformHistoryByScope.set(scopeKey, {
      fieldId,
      history: history.map(item => ({ value: item.value, ts: item.ts })),
      ...(deviceId ? { deviceId } : {})
    })
    this.sendWhenLoaded('tv:platform-history', { fieldId, history, deviceId })
  }

  /**
   * [Widget Mode] 宿主主动触发保存。
   * 方向：Host -> Guest -> Host
   * 协议：`tv:trigger-save` -> `tv:save`
   */
  public triggerSave() {
    this.send(TV_MSG.TRIGGER_SAVE)
  }

  /**
   * [Widget Mode] 监听 Guest 返回的保存结果。
   * 方向：Guest -> Host
   */
  public onWidgetSave(callback: (config: any) => void) {
    // `handleMessage` 会先把 `tv:save` 统一转发为 `tv:save-config`。
    this.on(TV_MSG.SAVE_CONFIG, payload => {
      // 典型 payload 结构为 `{ canvas, nodes, dataBindings }`，这里保持原样透传。
      callback(payload)
    })
  }

  // ===========================
  // 对外 API：App 模式
  // ===========================

  /**
   * [App Mode] 请求编辑器立即执行一次保存。
   * Guest 完成 `saveNow()` 后，宿主仍通过 `onWidgetSave()` 接收返回数据。
   */
  public requestSave() {
    sdkLogger.debug('[SDK] Client requesting save from Editor...')
    // `tv:request-save` 目前属于扩展协议消息，不在通用 EmbedMessage 类型声明里。
    this.send(TV_MSG.REQUEST_SAVE)
  }

  /**
   * [App Mode] 更新 Guest 侧使用的 token。
   * 方向：Host -> Guest
   *
   * App Mode 主流程通常通过 URL 传 token，这里保留 PostMessage 热更新能力。
   */
  public updateToken(token: string) {
    this.send(TV_MSG.EVENT, { event: 'updateToken', payload: { token } })
  }

  /** 校验消息是否来自当前 iframe 且 origin 与目标域完全一致。 */
  public isTrustedMessageEvent(event: MessageEvent): boolean {
    const iframeWindow = this.iframe.contentWindow
    return !!iframeWindow && event.source === iframeWindow && event.origin === this.targetOrigin
  }

  public postMessageToGuest(message: unknown): void {
    this.postToIframe(message)
  }

  // ===========================
  // 消息总线
  // ===========================

  public on(type: string, handler: MessageHandler) {
    if (!this.messageHandlers.has(type)) {
      this.messageHandlers.set(type, [])
    }
    this.messageHandlers.get(type)?.push(handler)
  }

  public off(type: string, handler: MessageHandler) {
    const handlers = this.messageHandlers.get(type)
    if (handlers) {
      const index = handlers.indexOf(handler)
      if (index !== -1) {
        handlers.splice(index, 1)
      }
    }
  }

  public destroy() {
    window.removeEventListener('message', this.handleMessage)
    if (this.iframe.parentNode && this.iframe) {
      this.iframe.parentNode.removeChild(this.iframe)
    }
    this.messageHandlers.clear()
    this.pendingQueue = []
    this.postLoadQueue = []
    this.latestPlatformDataByScope.clear()
    this.latestPlatformHistoryByScope.clear()
    this.lastInitPayload = null
    this.ready = false
    this.loaded = false
  }
}
