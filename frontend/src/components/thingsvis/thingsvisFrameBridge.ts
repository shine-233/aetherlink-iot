/**
 * 文件说明：
 * - 封装 ThingsVis iframe 的 URL 构造、targetOrigin 解析和宿主到 iframe 的 postMessage 发送。
 * - 将 iframe 安全边界从 AppFrame 中拆出，避免后续业务改动误放宽 origin 校验。
 * 维护提示：
 * - `provider`、`saveTarget`、route 与 query 参数是 ThingsVis 宿主协议，调整时要同步验证 editor/viewer 两种模式。
 * - `targetOrigin` 必须来自 Studio URL，不能改成 `*`。
 */
type ThingsVisFrameRoute = 'embed' | 'editor'

type BuildThingsVisFrameUrlOptions = {
  token: string
  mode?: string
  studioBaseUrl: string
  provider: string
  saveTarget: string
  thingsVisApiBaseUrl: string
  platformApiBaseUrl: string
}

export function resolveThingsVisTargetOrigin(rawUrl: string, currentHref: string, fallbackOrigin: string): string {
  try {
    return new URL(rawUrl, currentHref).origin
  } catch {
    return fallbackOrigin
  }
}

export function getThingsVisFrameRoute(mode?: string): ThingsVisFrameRoute {
  return mode === 'viewer' ? 'embed' : 'editor'
}

export function buildThingsVisFrameQuery(options: BuildThingsVisFrameUrlOptions): string {
  return [
    'mode=embedded',
    `provider=${options.provider}`,
    'context=dashboard',
    `saveTarget=${options.saveTarget}`,
    `token=${encodeURIComponent(options.token)}`,
    `thingsvisApiBaseUrl=${encodeURIComponent(options.thingsVisApiBaseUrl)}`,
    `platformApiBaseUrl=${encodeURIComponent(options.platformApiBaseUrl)}`
  ].join('&')
}

export function buildThingsVisFrameUrl(options: BuildThingsVisFrameUrlOptions): string {
  return `${options.studioBaseUrl}#/${getThingsVisFrameRoute(options.mode)}?${buildThingsVisFrameQuery(options)}`
}

export function isTrustedThingsVisFrameEvent(
  event: MessageEvent,
  iframeWindow: Window | null | undefined,
  targetOrigin: string
): boolean {
  return !!iframeWindow && event.source === iframeWindow && event.origin === targetOrigin
}

export function postToThingsVisFrame(
  iframeWindow: Window | null | undefined,
  type: string,
  payload: Record<string, unknown>,
  targetOrigin: string
) {
  postRawThingsVisFrameMessage(iframeWindow, { type, payload }, targetOrigin)
}

export function postRawThingsVisFrameMessage(
  iframeWindow: Window | null | undefined,
  message: Record<string, unknown>,
  targetOrigin: string
) {
  if (!iframeWindow) return
  iframeWindow.postMessage(message, targetOrigin)
}
