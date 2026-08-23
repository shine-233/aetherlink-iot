/**
 * 文件用途：DataItemFetcher HTTP request plan 的纯构建逻辑。
 * 核心逻辑：在动态参数已经解析后，按 current schema 与 compatibility alias 规则构建 URL、query、header、body 和缓存 key。
 * 关键注意事项：保持 pathParameter、parameters、raw body 与 request wrapper 的历史行为一致。
 * 重构建议：只在这里扩展 request-plan 规则，动态组件取值仍留在 DataItemFetcher。
 */
import type { HttpParameter } from '@/core/data-architecture/types/http-config'

import type { HttpDataItemConfig } from './DataItemFetcher'

export type HttpParameterSource = 'pathParams' | 'pathParameter' | 'params' | 'parameters'

export interface CollectedHttpParameter {
  source: HttpParameterSource
  param: HttpParameter
  index: number
}

export interface ResolvedHttpParameter extends CollectedHttpParameter {
  resolvedValue: unknown
}

export interface HttpRequestPlan {
  method: string
  finalUrl: string
  requestConfig: {
    timeout: number
    headers?: Record<string, string>
    params?: Record<string, unknown>
  }
  requestBody: unknown
  keyMaterial: string
}

export type HttpRequestConfig = HttpRequestPlan['requestConfig']

export function collectHttpParameters(config: HttpDataItemConfig): CollectedHttpParameter[] {
  const allParams: CollectedHttpParameter[] = []

  config.pathParams?.forEach((param, index) => {
    allParams.push({ source: 'pathParams', param, index })
  })

  if (config.pathParameter) {
    allParams.push({ source: 'pathParameter', param: config.pathParameter as HttpParameter, index: 0 })
  }

  config.params?.forEach((param, index) => {
    allParams.push({ source: 'params', param, index })
  })

  config.parameters?.forEach((param, index) => {
    allParams.push({ source: 'parameters', param, index })
  })

  return allParams
}

export function collectHttpRequestParameterInputs(config: HttpDataItemConfig): CollectedHttpParameter[] {
  return [
    ...collectEnabledPathParameters(config),
    ...collectCurrentQueryParameters(config),
    ...collectEnabledCompatibilityParameters(config).map(({ param, index }) => ({
      source: 'parameters' as const,
      param,
      index
    }))
  ]
}

export function buildHttpRequestPlan(
  config: HttpDataItemConfig,
  resolvedParameters: ResolvedHttpParameter[]
): HttpRequestPlan {
  const requestConfig = createBaseHttpRequestConfig(config)
  const queryParams: Record<string, unknown> = {}
  const currentFinalUrl = applyCurrentPathParameters(config.url, resolvedParameters)

  applyCurrentQueryParameters(queryParams, resolvedParameters)
  const finalUrl = applyCompatibilityParameters(config, currentFinalUrl, queryParams, requestConfig, resolvedParameters)

  if (Object.keys(queryParams).length > 0) {
    requestConfig.params = queryParams
  }

  const method = normalizeHttpMethod(config)
  const requestBody = buildHttpRequestBody(config)

  return {
    method,
    finalUrl,
    requestConfig,
    requestBody,
    keyMaterial: createRequestKeyMaterial(method, finalUrl, requestConfig, requestBody)
  }
}

const FORBIDDEN_REQUEST_KEYS = new Set(['__proto__', 'prototype', 'constructor'])

function isSafeRequestKey(key: unknown): key is string {
  return typeof key === 'string' && key.length > 0 && !FORBIDDEN_REQUEST_KEYS.has(key)
}

function collectEnabledPathParameters(config: HttpDataItemConfig): CollectedHttpParameter[] {
  const currentPathParams =
    config.pathParams
      ?.map((param, index) => ({ source: 'pathParams' as const, param, index }))
      .filter(({ param }) => param.enabled) ?? []

  if (currentPathParams.length > 0) {
    return currentPathParams
  }

  // Saved visual-editor configs may still contain the single pathParameter alias.
  return config.pathParameter
    ? [{ source: 'pathParameter', param: config.pathParameter as HttpParameter, index: 0 }]
    : []
}

function collectCurrentQueryParameters(config: HttpDataItemConfig): CollectedHttpParameter[] {
  return (
    config.params
      ?.map((param, index) => ({ source: 'params' as const, param, index }))
      .filter(({ param }) => param.enabled && isSafeRequestKey(param.key)) ?? []
  )
}

function collectEnabledCompatibilityParameters(
  config: HttpDataItemConfig,
  paramType?: HttpParameter['paramType']
): Array<{ param: HttpParameter; index: number }> {
  return (
    config.parameters
      ?.map((param, index) => ({ param, index }))
      .filter(({ param }) => {
        if (!param.enabled || !param.key) return false
        const effectiveType = param.paramType || 'query'
        if (effectiveType !== 'path' && !isSafeRequestKey(param.key)) return false
        return !paramType || effectiveType === paramType
      }) ?? []
  )
}

function createBaseHttpRequestConfig(config: HttpDataItemConfig): HttpRequestConfig {
  const requestConfig: HttpRequestConfig = {
    timeout: config.timeout || 10000
  }

  if (config.headers) {
    const safeHeaders = Object.fromEntries(Object.entries(config.headers).filter(([key]) => isSafeRequestKey(key)))
    if (Object.keys(safeHeaders).length > 0) {
      requestConfig.headers = safeHeaders
    }
  }

  return requestConfig
}

function applyCurrentPathParameters(initialUrl: string, resolvedParameters: ResolvedHttpParameter[]): string {
  let finalUrl = initialUrl

  for (const { param, resolvedValue } of resolvedParameters.filter(
    ({ source }) => source === 'pathParams' || source === 'pathParameter'
  )) {
    if (hasResolvedPathValue(resolvedValue)) {
      finalUrl = replacePathPlaceholder(finalUrl, param, resolvedValue)
    }
  }

  return finalUrl
}

function applyCurrentQueryParameters(
  queryParams: Record<string, unknown>,
  resolvedParameters: ResolvedHttpParameter[]
): void {
  for (const { param, resolvedValue } of resolvedParameters.filter(({ source }) => source === 'params')) {
    if (resolvedValue !== null) {
      queryParams[param.key] = resolvedValue
    }
  }
}

function applyCompatibilityParameters(
  config: HttpDataItemConfig,
  initialUrl: string,
  queryParams: Record<string, unknown>,
  requestConfig: HttpRequestConfig,
  resolvedParameters: ResolvedHttpParameter[]
): string {
  let finalUrl = initialUrl
  const currentQueryKeys = currentQueryParameterKeys(config)
  const currentHeaderKeys = currentHeaderKeysForConfig(config)

  for (const { param, resolvedValue } of resolvedParameters.filter(({ source }) => source === 'parameters')) {
    if (resolvedValue === null) {
      continue
    }

    switch (param.paramType || 'query') {
      case 'path':
        finalUrl = applyCompatibilityPathParameter(config, finalUrl, param, resolvedValue)
        break
      case 'query':
        applyCompatibilityQueryParameter(queryParams, currentQueryKeys, param, resolvedValue)
        break
      case 'header':
        applyCompatibilityHeaderParameter(requestConfig, currentHeaderKeys, param, resolvedValue)
        break
    }
  }

  return finalUrl
}

function applyCompatibilityPathParameter(
  config: HttpDataItemConfig,
  finalUrl: string,
  param: HttpParameter,
  resolvedValue: unknown
): string {
  if (hasCurrentPathParameters(config) || !hasResolvedPathValue(resolvedValue)) {
    return finalUrl
  }

  const nextUrl = replacePathPlaceholder(finalUrl, param, resolvedValue)
  if (nextUrl !== finalUrl) {
    return nextUrl
  }

  const separator = finalUrl.endsWith('/') ? '' : '/'
  return `${finalUrl}${separator}${String(resolvedValue)}`
}

function applyCompatibilityQueryParameter(
  queryParams: Record<string, unknown>,
  currentQueryKeys: Set<string>,
  param: HttpParameter,
  resolvedValue: unknown
): void {
  if (!currentQueryKeys.has(param.key)) {
    queryParams[param.key] = resolvedValue
  }
}

function applyCompatibilityHeaderParameter(
  requestConfig: HttpRequestConfig,
  currentHeaderKeys: Set<string>,
  param: HttpParameter,
  resolvedValue: unknown
): void {
  requestConfig.headers = requestConfig.headers || {}
  if (!currentHeaderKeys.has(param.key)) {
    requestConfig.headers[param.key] = String(resolvedValue)
  }
}

function hasCurrentPathParameters(config: HttpDataItemConfig): boolean {
  return collectEnabledPathParameters(config).length > 0
}

function currentQueryParameterKeys(config: HttpDataItemConfig): Set<string> {
  return new Set(config.params?.filter(param => param.enabled && param.key).map(param => param.key) ?? [])
}

function currentHeaderKeysForConfig(config: HttpDataItemConfig): Set<string> {
  return new Set(Object.keys(config.headers ?? {}))
}

function replacePathPlaceholder(url: string, param: HttpParameter, resolvedValue: unknown): string {
  let placeholder = param.key ? `{${param.key}}` : null

  if (!placeholder || placeholder === '{}') {
    placeholder = url.match(/\{([^}]+)\}/)?.[0] ?? null
  }

  return placeholder && url.includes(placeholder) ? url.replace(placeholder, String(resolvedValue)) : url
}

function hasResolvedPathValue(resolvedValue: unknown): boolean {
  return resolvedValue !== null && String(resolvedValue).trim() !== ''
}

function normalizeHttpMethod(config: HttpDataItemConfig): string {
  return config.method.toUpperCase()
}

function buildHttpRequestBody(config: HttpDataItemConfig): unknown {
  if (!['POST', 'PUT', 'PATCH', 'DELETE'].includes(config.method) || !config.body) {
    return undefined
  }

  try {
    return typeof config.body === 'string' ? JSON.parse(config.body) : config.body
  } catch {
    return config.body
  }
}

function createRequestKeyMaterial(
  method: string,
  finalUrl: string,
  requestConfig: HttpRequestPlan['requestConfig'],
  requestBody: unknown
): string {
  return stableStringify({
    method,
    url: finalUrl,
    timeout: requestConfig.timeout,
    headers: requestConfig.headers ?? {},
    params: requestConfig.params ?? {},
    body: requestBody
  })
}

function stableStringify(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map(item => stableStringify(item)).join(',')}]`
  }

  if (value && typeof value === 'object') {
    const record = value as Record<string, unknown>
    return `{${Object.keys(record)
      .sort()
      .map(key => `${JSON.stringify(key)}:${stableStringify(record[key])}`)
      .join(',')}}`
  }

  return JSON.stringify(value)
}
