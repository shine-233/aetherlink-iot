/**
 * 文件用途: data-architecture 运行时数据项获取器。
 * 核心逻辑: 解析 HTTP 参数、绑定路径、缓存键、请求 payload、脚本执行和 fallback，再返回组件运行时数据。
 * 关键注意事项: cache key、绑定路径恢复、参数别名和脚本前后处理是 persisted 配置兼容热点。
 * 重构建议: 按 json/http/websocket/script fetch strategy 拆分，并先锁定缓存键与绑定路径测试。
 */
import { defaultScriptEngine } from '@/core/script-engine'
import type { HttpConfig, HttpParameter, PathParameter } from '@/core/data-architecture/types/http-config'
import { request } from '@/service/request'
import { useEditorStore } from '@/components/visual-editor/store/editor'
import {
  buildHttpRequestPlan as buildResolvedHttpRequestPlan,
  collectHttpRequestParameterInputs,
  type HttpRequestPlan,
  type ResolvedHttpParameter
} from './DataItemFetcherRequestPlan'
import {
  logHttpParametersLifecycle,
  resolveHttpParameterValue,
  validateParameterBindingPaths
} from './DataItemFetcherParameterResolution'

type HttpExecutionContext = {
  preparedConfig: HttpDataItemConfig
  requestPlan: HttpRequestPlan
}

type HttpRequestDispatcher = () => Promise<unknown>

/** 组件绑定读取时使用的可视化编辑器节点形状（仅依赖 id 与 properties 字段） */
type EditorStoreNodeLike = {
  id: string
  properties?: unknown
}

/** 配置集成桥的只读视图：DataItemFetcher 仅消费 getConfiguration 结果 */
type ConfigurationBridgeView = {
  getConfiguration(componentId: string): WidgetConfigurationView | null
}

/** 组件配置的最小结构视图（base/component 双轨兼容） */
type WidgetConfigurationView = {
  base?: unknown
  component?: unknown
}

export type DataItem =
  | {
      type: 'json'
      config: JsonDataItemConfig
    }
  | {
      type: 'http'
      config: HttpDataItemConfig
    }
  | {
      type: 'websocket'
      config: WebSocketDataItemConfig
    }
  | {
      type: 'script'
      config: ScriptDataItemConfig
    }

export interface JsonDataItemConfig {
  jsonString: string
}

// Runtime HTTP configs intentionally accept both current editor fields and saved aliases.
export interface HttpDataItemConfig {
  url: string
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
  headers?: Record<string, string>
  body?: unknown
  timeout?: number

  addressType?: 'internal' | 'external'
  selectedInternalAddress?: string
  enableParams?: boolean

  pathParameter?: PathParameter
  pathParams?: HttpParameter[]
  params?: HttpParameter[]
  parameters?: HttpParameter[]

  preRequestScript?: string
  postResponseScript?: string
}

export type HttpDataItemConfigV2 = HttpConfig

export interface WebSocketDataItemConfig {
  url: string
  protocols?: string[]
  reconnectInterval?: number
}

export interface ScriptDataItemConfig {
  script: string
  context?: Record<string, unknown>
}

export interface UnsupportedDataSourceResult {
  success: false
  unsupported: true
  error: {
    code: 'UNSUPPORTED_DATA_SOURCE'
    message: string
    type: DataItem['type']
  }
}

export interface IDataItemFetcher {
  fetchData(item: DataItem): Promise<unknown>
  setCurrentComponentId(componentId: string): void
}

export class DataItemFetcher implements IDataItemFetcher {
  private requestCache = new Map<string, Promise<unknown>>()
  private currentComponentId?: string

  setCurrentComponentId(componentId: string): void {
    this.currentComponentId = componentId
  }

  private async getComponentPropertyValue(bindingPath: string): Promise<unknown> {
    try {
      const resolvedBinding = this.resolveComponentBinding(bindingPath)
      if (!resolvedBinding) {
        return undefined
      }

      const { componentId, propertyPath } = resolvedBinding
      const configurationValue = await this.readComponentValueFromConfiguration(componentId, propertyPath)
      if (configurationValue !== undefined) {
        return configurationValue
      }

      const targetComponent = this.findEditorStoreComponent(componentId)
      return targetComponent ? this.getNestedProperty(targetComponent.properties, propertyPath) : undefined
    } catch (error) {
      console.error('[DataItemFetcher] Component property binding failed:', error)
      return undefined
    }
  }

  private resolveComponentBinding(bindingPath: string): { componentId: string; propertyPath: string } | null {
    if (!bindingPath || typeof bindingPath !== 'string' || !bindingPath.includes('.')) {
      return null
    }

    const parts = bindingPath.split('.')
    const propertyPath = parts.slice(1).join('.')
    const componentId = this.resolveBindingComponentId(parts[0])
    if (!componentId || !propertyPath) {
      return null
    }

    return { componentId, propertyPath }
  }

  private resolveBindingComponentId(componentId: string): string | undefined {
    if (componentId !== '__CURRENT_COMPONENT__') {
      return componentId
    }
    return this.currentComponentId
  }

  private async readComponentValueFromConfiguration(componentId: string, propertyPath: string): Promise<unknown> {
    try {
      const { configurationIntegrationBridge } =
        await import('@/components/visual-editor/configuration/ConfigurationIntegrationBridge')
      const latestConfig = this.getPreferredConfiguration(componentId, configurationIntegrationBridge)
      if (!latestConfig) {
        return undefined
      }
      return this.resolveFromConfiguration(latestConfig, propertyPath)
    } catch {
      return undefined
    }
  }

  private getPreferredConfiguration(
    componentId: string,
    configurationIntegrationBridge: ConfigurationBridgeView
  ): WidgetConfigurationView | null {
    const directConfig = configurationIntegrationBridge.getConfiguration(componentId)
    if (directConfig || !this.currentComponentId || this.currentComponentId === componentId) {
      return directConfig
    }
    return configurationIntegrationBridge.getConfiguration(this.currentComponentId)
  }

  private findEditorStoreComponent(componentId: string): EditorStoreNodeLike | undefined {
    const editorStore = useEditorStore()
    const exactMatch = editorStore.nodes?.find((node) => node.id === componentId)
    if (exactMatch) {
      return exactMatch
    }

    const fuzzyMatch = editorStore.nodes?.find((node) => node.id.includes(componentId) || componentId.includes(node.id))
    if (fuzzyMatch) {
      return fuzzyMatch
    }

    if (!this.currentComponentId) {
      return undefined
    }
    return editorStore.nodes?.find((node) => node.id === this.currentComponentId)
  }

  private resolveFromConfiguration(latestConfig: WidgetConfigurationView, propertyPath: string): unknown {
    if (propertyPath.startsWith('customize.')) {
      const actualPath = propertyPath.replace('customize.', '')
      return this.firstDefined(
        this.getNestedProperty(latestConfig.component, actualPath),
        this.getNestedProperty(latestConfig.base, actualPath)
      )
    }

    if (propertyPath.startsWith('base.')) {
      const actualPath = propertyPath.replace('base.', '')
      return this.firstDefined(
        this.getNestedProperty(latestConfig.base, actualPath),
        this.getNestedProperty(latestConfig.component, actualPath)
      )
    }

    if (propertyPath.startsWith('component.')) {
      const actualPath = propertyPath.replace('component.', '')
      return this.firstDefined(
        this.getNestedProperty(latestConfig.component, actualPath),
        this.getNestedProperty(latestConfig.base, actualPath)
      )
    }

    return this.firstDefined(
      this.getNestedProperty(latestConfig.base, propertyPath),
      this.getNestedProperty(latestConfig.component, propertyPath)
    )
  }

  private firstDefined(...values: unknown[]): unknown {
    return values.find(
      (value) => value !== undefined && value !== null && !(typeof value === 'string' && value.trim() === '')
    )
  }

  private formatError(error: unknown): string {
    return error instanceof Error ? error.message : String(error)
  }

  private toLoggableError(error: unknown): unknown {
    return error instanceof Error ? error.message : error
  }

  private logFetcherError(scope: string, details: Record<string, unknown>): void {
    console.error(`[DataItemFetcher] ${scope}:`, details)
  }

  private getNestedProperty(obj: unknown, path: string): unknown {
    if (!obj || !path) return undefined

    const keys = path.split('.')
    let current: unknown = obj

    for (const key of keys) {
      if (current && typeof current === 'object' && key in current) {
        current = (current as Record<string, unknown>)[key]
      } else {
        return undefined
      }
    }

    return current
  }

  private async resolveParameterValue(param: HttpParameter): Promise<unknown> {
    return resolveHttpParameterValue(param, (bindingPath) => this.getComponentPropertyValue(bindingPath))
  }

  async fetchData(item: DataItem): Promise<unknown> {
    try {
      return await this.fetchByDataItemType(item)
    } catch (error) {
      this.logFetcherError('fetchData failed', {
        type: item.type,
        error: this.toLoggableError(error)
      })
      return {}
    }
  }

  private async fetchByDataItemType(item: DataItem): Promise<unknown> {
    switch (item.type) {
      case 'json':
        return await this.fetchJsonData(item.config)
      case 'http':
        return await this.fetchHttpData(item.config)
      case 'websocket':
        return await this.fetchWebSocketData(item.config)
      case 'script':
        return await this.fetchScriptData(item.config)
      default:
        return {}
    }
  }

  private async fetchJsonData(config: JsonDataItemConfig): Promise<unknown> {
    try {
      return JSON.parse(config.jsonString)
    } catch (error) {
      this.logFetcherError('JSON data source parse failed', {
        error: this.formatError(error)
      })
      return {}
    }
  }

  private async fetchHttpData(config: HttpDataItemConfig): Promise<unknown> {
    const executionContext = await this.buildExecutionContext(config)
    return await this.fetchHttpDataWithCache(executionContext)
  }

  private async buildExecutionContext(config: HttpDataItemConfig): Promise<HttpExecutionContext> {
    const preparedConfig = { ...config }
    await this.prepareHttpConfigForExecution(preparedConfig)

    return {
      preparedConfig,
      requestPlan: await this.buildHttpRequestPlan(preparedConfig)
    }
  }

  private async fetchHttpDataWithCache(executionContext: HttpExecutionContext): Promise<unknown> {
    const { preparedConfig, requestPlan } = executionContext
    const requestPromise = this.getOrCreateHttpRequestPromise(executionContext)
    return await this.normalizeHttpResponse(preparedConfig, await requestPromise)
  }

  private getOrCreateHttpRequestPromise(executionContext: HttpExecutionContext): Promise<unknown> {
    // requestCache only de-duplicates in-flight requests; it is not a durable response cache.
    // The key must stay aligned with the fully resolved request plan after compatibility aliases and scripts run.
    const requestKey = this.generateRequestKey(executionContext.requestPlan)
    const existingRequest = this.requestCache.get(requestKey)
    if (existingRequest) {
      return existingRequest
    }

    const requestPromise = this.executeHttpRequest(executionContext.preparedConfig, executionContext.requestPlan)
    const trackedPromise = requestPromise.finally(() => {
      if (this.requestCache.get(requestKey) === trackedPromise) {
        this.requestCache.delete(requestKey)
      }
    })
    this.requestCache.set(requestKey, trackedPromise)
    return trackedPromise
  }

  private async prepareHttpConfigForExecution(config: HttpDataItemConfig): Promise<void> {
    logHttpParametersLifecycle(config, 'before request')
    validateParameterBindingPaths(config)
    logHttpParametersLifecycle(config, 'after validation')
    await this.applyPreRequestScript(config)
    logHttpParametersLifecycle(config, 'before parameter handling')
  }

  private async buildHttpRequestPlan(config: HttpDataItemConfig): Promise<HttpRequestPlan> {
    const resolvedParameters = await this.resolveHttpRequestParameters(config)
    return buildResolvedHttpRequestPlan(config, resolvedParameters)
  }

  private async resolveHttpRequestParameters(config: HttpDataItemConfig): Promise<ResolvedHttpParameter[]> {
    const resolvedParameters: ResolvedHttpParameter[] = []
    for (const parameterInput of collectHttpRequestParameterInputs(config)) {
      resolvedParameters.push({
        ...parameterInput,
        resolvedValue: await this.resolveParameterValue(parameterInput.param)
      })
    }
    return resolvedParameters
  }

  private async applyPreRequestScript(config: HttpDataItemConfig): Promise<void> {
    if (!config.preRequestScript) {
      return
    }

    try {
      const scriptResult = await defaultScriptEngine.execute(config.preRequestScript, {
        config
      })
      if (scriptResult.success && scriptResult.data) {
        Object.assign(config, scriptResult.data)
      }
    } catch (error) {
      console.error('[DataItemFetcher] Pre-request script failed:', error)
    }
  }

  private async executeHttpRequest(config: HttpDataItemConfig, requestPlan: HttpRequestPlan): Promise<unknown> {
    try {
      logHttpParametersLifecycle(config, 'before send')
      return await this.dispatchHttpRequest(config, requestPlan)
    } catch (error) {
      this.logFetcherError('fetchHttpData failed', {
        url: config.url,
        method: config.method,
        error: this.toLoggableError(error)
      })
      return {}
    }
  }

  private async dispatchHttpRequest(config: HttpDataItemConfig, requestPlan: HttpRequestPlan): Promise<unknown> {
    const dispatcher = this.getHttpRequestDispatcher(config, requestPlan)
    return await dispatcher()
  }

  private getHttpRequestDispatcher(config: HttpDataItemConfig, requestPlan: HttpRequestPlan): HttpRequestDispatcher {
    const dispatchers: Record<HttpRequestPlan['method'], HttpRequestDispatcher> = {
      GET: () => request.get(requestPlan.finalUrl, requestPlan.requestConfig),
      POST: () => request.post(requestPlan.finalUrl, requestPlan.requestBody, requestPlan.requestConfig),
      PUT: () => request.put(requestPlan.finalUrl, requestPlan.requestBody, requestPlan.requestConfig),
      PATCH: () => (request as unknown as { patch: typeof request.post }).patch(requestPlan.finalUrl, requestPlan.requestBody, requestPlan.requestConfig),
      DELETE: () => request.delete(requestPlan.finalUrl, requestPlan.requestConfig)
    }

    const dispatcher = dispatchers[requestPlan.method]
    if (!dispatcher) {
      throw new Error(`Unsupported HTTP method: ${config.method}`)
    }

    return dispatcher
  }

  private async normalizeHttpResponse(config: HttpDataItemConfig, response: unknown): Promise<unknown> {
    return await this.applyPostResponseScript(config, response)
  }

  private async applyPostResponseScript(config: HttpDataItemConfig, response: unknown): Promise<unknown> {
    if (!config.postResponseScript) {
      return response
    }

    try {
      const scriptResult = await defaultScriptEngine.execute(config.postResponseScript, { response })
      if (scriptResult.success) {
        return scriptResult.data !== undefined ? scriptResult.data : response
      }
    } catch (error) {
      console.error('[DataItemFetcher] Post-response script failed:', error)
    }

    return response
  }

  private generateRequestKey(requestPlan: HttpRequestPlan): string {
    return `http_${requestPlan.keyMaterial}`
  }

  private async fetchWebSocketData(config: WebSocketDataItemConfig): Promise<UnsupportedDataSourceResult> {
    const message = 'WebSocket data sources are not supported by DataItemFetcher fetchData.'
    this.logFetcherError('Unsupported data source', {
      type: 'websocket',
      url: config.url,
      message
    })
    return this.buildUnsupportedDataSourceResult('websocket', message)
  }

  private buildUnsupportedDataSourceResult(type: DataItem['type'], message: string): UnsupportedDataSourceResult {
    return {
      success: false,
      unsupported: true,
      error: {
        code: 'UNSUPPORTED_DATA_SOURCE',
        message,
        type
      }
    }
  }

  private async fetchScriptData(config: ScriptDataItemConfig): Promise<unknown> {
    try {
      const result = await defaultScriptEngine.execute(config.script, config.context || {})
      if (!result.success) {
        this.logFetcherError('Script data source failed', {
          error: result.error ?? 'Script execution returned an unsuccessful result.'
        })
        return {}
      }

      // 保留 0、false 和空字符串等有效脚本结果，仅空值回退为空对象。
      return result.data !== null && result.data !== undefined ? result.data : {}
    } catch (error) {
      this.logFetcherError('Script data source failed', {
        error: this.formatError(error)
      })
      return {}
    }
  }
}
