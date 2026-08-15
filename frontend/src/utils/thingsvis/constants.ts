/*
 * 文件用途：定义 ThingsVis 集成常量、兼容 alias、代理路径和 Studio 地址解析。
 * 核心逻辑：根据运行时环境归一化 API proxy、平台 base path、host source 前缀和 Studio base URL。
 * 关键注意事项：这些常量属于嵌入契约，改动会影响 SSO、iframe 和 host 数据源兼容。
 * 重构建议：建议把环境覆盖、默认值和兼容 alias 纳入测试。
 */
/**
 * ThingsVis shared constants.
 * Single source of truth for the API proxy path used across the app.
 */

/**
 * Vite proxy path prefix for ThingsVis backend requests.
 *
 * This MUST match the key registered in build/config/proxy.ts — always '/thingsvis-api'.
 * Do NOT read VITE_THINGSVIS_API_URL here: that env var holds the proxy TARGET (a full
 * http address used by Vite at build-time), not the frontend path prefix.
 */
export const THINGSVIS_API_PROXY_PATH = '/thingsvis-api'
export const PLATFORM_API_BASE_PATH = '/api/v1'
export const THINGSVIS_COMPAT_ALIAS = 'aetherlink' as const
export type ThingsVisCompatAlias = typeof THINGSVIS_COMPAT_ALIAS

export const THINGSVIS_COMPAT_PROVIDER = THINGSVIS_COMPAT_ALIAS
export const THINGSVIS_COMPAT_PLATFORM = THINGSVIS_COMPAT_ALIAS
export const THINGSVIS_HOST_DATA_SOURCE_ID_PREFIX = 'aetherlink_' as const
export type ThingsVisCompatPlatform = ThingsVisCompatAlias
const DEFAULT_ORIGIN = 'http://localhost'
const DEFAULT_STUDIO_ENTRY = '/main.html'
const DEV_STUDIO_BASE_URL = 'http://localhost:3000/main.html'

if (typeof window === 'undefined') {
  Object.defineProperty(globalThis, 'window', {
    value: {
      location: {
        origin: globalThis.location?.origin ?? DEFAULT_ORIGIN
      }
    },
    configurable: true
  })
}

function getCurrentOrigin(): string {
  if (typeof window !== 'undefined' && window.location?.origin) {
    return window.location.origin
  }

  if (typeof globalThis.location !== 'undefined' && globalThis.location?.origin) {
    return globalThis.location.origin
  }

  return DEFAULT_ORIGIN
}

function normalizeStudioBaseUrl(url: string): string {
  const hashIndex = url.indexOf('#')
  const cleanUrl = hashIndex === -1 ? url : url.substring(0, hashIndex)

  if (cleanUrl.endsWith('/main') && !cleanUrl.endsWith('.html')) {
    return `${cleanUrl}.html`
  }

  return cleanUrl
}

/**
 * Returns the ThingsVis Studio HTML entry used by embedded editor/viewer iframes.
 * Production defaults to the current site origin instead of a browser-localhost URL.
 */
export function getThingsVisStudioBaseUrl(): string {
  const configuredUrl = import.meta.env.VITE_THINGSVIS_STUDIO_URL?.trim()
  if (configuredUrl) {
    return normalizeStudioBaseUrl(configuredUrl)
  }

  const fallbackUrl = import.meta.env.DEV ? DEV_STUDIO_BASE_URL : getCurrentOrigin() + DEFAULT_STUDIO_ENTRY
  return normalizeStudioBaseUrl(fallbackUrl)
}

/**
 * Returns the absolute ThingsVis API base URL suitable for cross-origin
 * postMessage payloads (e.g. inside iframe init messages).
 * Uses the current page origin so it always matches the running host.
 */
export function getThingsVisApiBase(): string {
  return getCurrentOrigin() + THINGSVIS_API_PROXY_PATH
}

/**
 * Returns the absolute platform API base URL used by ThingsVis runtime REST
 * data sources. Keep this pinned to the current host origin so embedded
 * dashboards follow the deployed system instead of a build-time fallback target.
 */
export function getPlatformApiBase(): string {
  return getCurrentOrigin() + PLATFORM_API_BASE_PATH
}
