/*
 * 文件用途：构建 ThingsVis Studio 嵌入 URL。
 * 核心逻辑：组合模式、token、平台字段、宿主 API 地址和显示参数，生成 iframe 可加载地址。
 * 关键注意事项：URL query/hash 拼接和 token 注入会影响嵌入可用性与安全性。
 * 重构建议：建议继续补齐特殊字符、已有 query/hash、缺 token 和 viewer/editor 模式测试。
 */
/**
 * ThingsVis URL 构建工具。
 * 用于生成嵌入式编辑器或预览器的访问 URL。
 */

import type { PlatformField, ThingsVisProject } from './types'
import { createLogger } from '@/utils/logger'
import { getPlatformApiBase, getThingsVisApiBase, getThingsVisStudioBaseUrl } from './constants'

const logger = createLogger('ThingsVisUrlBuilder')

/** URL 构建选项。 */
export interface ThingsVisUrlOptions {
  /** 模式：editor 为编辑模式，viewer 为预览模式。 */
  mode: 'editor' | 'viewer'
  /** 初始配置，会被编码后传入嵌入页。 */
  config?: ThingsVisProject
  /** 平台字段列表。 */
  platformFields?: PlatformField[]
  /** 保存目标：self 表示编辑器自身，host 表示宿主平台。 */
  saveTarget?: 'self' | 'host'
  /** 是否显示组件库。 */
  showLibrary?: boolean
  /** 是否显示属性面板。 */
  showProps?: boolean
  /** 是否显示工具栏。 */
  showToolbar?: boolean
  /** 是否显示左上角区域。 */
  showTopLeft?: boolean
  /** 是否显示右上角区域。 */
  showTopRight?: boolean
}

/**
 * 构建 ThingsVis 编辑器或预览器 URL，支持 SSO token 注入。
 */
export async function buildThingsVisUrl(options: ThingsVisUrlOptions): Promise<string> {
  const baseUrl = getThingsVisStudioBaseUrl()
  const integration = resolveThingsVisIntegration(options)
  const params = createThingsVisUrlParams(options, integration)

  await attachThingsVisToken(params)
  applyThingsVisDisplayOptions(params, options, integration)
  applyThingsVisPlatformFields(params, options.platformFields)

  return `${baseUrl}#${thingsVisRouteForMode(options.mode)}?${params.toString()}`
}

function resolveThingsVisIntegration(options: ThingsVisUrlOptions) {
  return options.mode === 'editor' ? 'full' : 'minimal'
}

function createThingsVisUrlParams(options: ThingsVisUrlOptions, integration: string) {
  const params = new URLSearchParams({
    mode: 'embedded',
    integration,
    saveTarget: options.saveTarget || 'host'
  })
  params.set('thingsvisApiBaseUrl', getThingsVisApiBase())
  params.set('platformApiBaseUrl', getPlatformApiBase())
  return params
}

async function attachThingsVisToken(params: URLSearchParams) {
  // 安全边界：SSO 失败时绝不回退为把平台 JWT 塞进 iframe URL hash——
  // 该 URL 会进入浏览器历史/引用页并交给第三方页面承载，等价于泄露长期凭据。
  // SSO 失败属于显式配置/服务故障，由上层生命周期逻辑报告 disabled / configuration-required。
  try {
    const { getThingsVisToken } = await import('./thingsvis-auth')
    const thingsvisToken = await getThingsVisToken()
    if (thingsvisToken) {
      params.set('token', thingsvisToken)
      return
    }
    logger.warn('SSO token exchange returned no token; generating ThingsVis URL without token')
  } catch (error) {
    logger.error('SSO token exchange failed:', error)
  }
}

function applyThingsVisDisplayOptions(params: URLSearchParams, options: ThingsVisUrlOptions, integration: string) {
  if (options.mode === 'viewer') {
    hideThingsVisDisplayRegions(params)
    return
  }

  if (integration !== 'full') return

  const isEditor = options.mode === 'editor'
  const flags = {
    showLibrary: options.showLibrary ?? isEditor,
    showProps: options.showProps ?? isEditor,
    showToolbar: options.showToolbar ?? isEditor,
    showTopLeft: options.showTopLeft ?? false,
    showTopRight: options.showTopRight ?? false
  }

  Object.entries(flags).forEach(([key, enabled]) => {
    if (!enabled) params.set(key, '0')
  })
}

function hideThingsVisDisplayRegions(params: URLSearchParams) {
  ;['showLibrary', 'showProps', 'showToolbar', 'showTopLeft', 'showTopRight'].forEach((key) => {
    params.set(key, '0')
  })
}

function applyThingsVisPlatformFields(params: URLSearchParams, platformFields?: PlatformField[]) {
  if (platformFields && platformFields.length > 0) {
    params.set('platformFields', JSON.stringify(platformFields))
  }
}

function thingsVisRouteForMode(mode: ThingsVisUrlOptions['mode']) {
  return mode === 'viewer' ? '/embed' : '/editor'
}
