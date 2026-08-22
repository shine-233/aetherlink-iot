/**
 * 文件用途: 连接 Visual Editor 保存态配置与数据架构执行入口。
 * 核心职责: 负责配置归一化、基础参数注入、绑定表达式替换，以及向 SimpleDataBridge 下发执行请求。
 * 桥接边界: 这里只做“编辑器配置 -> 数据需求”的单向转换，不直接反查 Visual Editor store，也不维护编辑器内部状态。
 * 循环依赖说明: 若在此层直接引入编辑器配置桥或节点状态查询，会把 data-architecture 与 visual-editor 再次耦合，形成循环依赖。
 * 兼容性注意: 历史字段别名、whitelist 持久化别名与默认值注入策略都会影响旧组件配置回放结果。
 */

import {
  simpleDataBridge,
  type ComponentDataRequirement,
  type DataResult,
  type SimpleDataSourceConfig
} from '@/core/data-architecture/SimpleDataBridge'
import type { DataSourceConfiguration } from '@/core/data-architecture/executors/MultiLayerExecutorChain'
import { dataSourceBindingConfig, type AutoBindConfig } from '@/core/data-architecture/DataSourceBindingConfig'
import { createLogger } from '@/utils/logger'

const bridgeLogger = createLogger('VisualEditorBridge')

type StandardDataSource = DataSourceConfiguration['dataSources'][number]
type BridgeDataSource = SimpleDataSourceConfig | StandardDataSource

// 保持这个可选导入处于禁用状态，避免桥接层反向依赖编辑器配置模块并形成循环依赖。
// import { configurationIntegrationBridge } from '@/components/visual-editor/configuration/ConfigurationIntegrationBridge'

/** 组件数据载荷（与 SimpleDataBridge 缓存结构一致，字段宽松） */
type ComponentDataPayload = Record<string, unknown>

/** 编辑器保存态配置（历史格式宽松，运行时逐个校验） */
type EditorComponentConfig = {
  base?: Record<string, unknown> | null
  dataSource?: Record<string, unknown> | null
  componentType?: unknown
  [key: string]: unknown
}

/** 编辑器数据源声明（历史格式宽松，字段运行时逐个校验） */
type EditorDataSourceLike = {
  type?: unknown
  enabled?: unknown
  config?: (Record<string, unknown> & { params?: Record<string, unknown> | null }) | null
  autoBind?: AutoBindConfig | null
  componentType?: string
  deviceId?: unknown
  metricsList?: unknown
  filterPath?: string
  processScript?: string
  [key: string]: unknown
}

type ResolvedBridgeConfig = {
  resolvedConfig: EditorDataSourceLike | null
  baseConfig: Record<string, unknown> | null
}

/** 编辑器标准数据项行（sourceId 槽位内的 dataItem） */
type EditorStandardDataItem = {
  item?: { type?: unknown; config?: Record<string, unknown> | null } | null
  processing?: { filterPath?: unknown; customScript?: unknown } | null
}

/**
 * Visual Editor 专用的数据桥接器
 * 封装 SimpleDataBridge，提供编辑器运行时数据接口
 */
export class VisualEditorBridge {
  private dataUpdateCallbacks = new Map<number, (componentId: string, data: ComponentDataPayload) => void>()
  private nextDataUpdateCallbackId = 1

  private normalizeDataSourceType(type: unknown, sourceId: string): SimpleDataSourceConfig['type'] | null {
    if (type === 'api') {
      return 'http'
    }

    if (['static', 'http', 'json', 'websocket', 'file', 'data-source-bindings'].includes(String(type))) {
      return type as SimpleDataSourceConfig['type']
    }

    bridgeLogger.error('[VisualEditorBridge] UNSUPPORTED_DATA_SOURCE_TYPE', {
      sourceId,
      type: String(type)
    })
    return null
  }

  /**
   * 更新组件执行器
   * @param componentId 组件ID
   * @param componentType 组件类型
   * @param config 数据源配置
   */
  async updateComponentExecutor(
    componentId: string,
    componentType: string,
    config: EditorComponentConfig | null
  ): Promise<DataResult> {
    // 将编辑器保存态配置转换为当前数据架构可执行的数据需求。
    const requirement = this.convertConfigToRequirement(componentId, componentType, config)

    const result = await simpleDataBridge.executeComponent(requirement)

    // 通知数据更新回调
    this.notifyDataUpdate(componentId, result.data)

    return result
  }

  /**
   * 监听数据更新。
   * 返回的退订函数可重复调用，不依赖随机数生成订阅标识。
   * @param callback 数据更新回调函数
   */
  onDataUpdate(callback: (componentId: string, data: ComponentDataPayload) => void): () => void {
    const callbackId = this.nextDataUpdateCallbackId++
    this.dataUpdateCallbacks.set(callbackId, callback)

    return () => {
      this.dataUpdateCallbacks.delete(callbackId)
    }
  }

  /**
   * 释放当前桥接器持有的本地订阅。
   * 数据执行与缓存由共享的 SimpleDataBridge 管理，不在这里越权清空。
   */
  dispose(): void {
    this.dataUpdateCallbacks.clear()
  }

  /**
   * 获取组件当前数据
   * @param componentId 组件ID
   */
  getComponentData(componentId: string): Record<string, unknown> | null {
    return simpleDataBridge.getComponentData(componentId)
  }

  /**
   * 清除组件数据缓存
   * @param componentId 组件ID
   */
  clearComponentCache(componentId: string): void {
    simpleDataBridge.clearComponentCache(componentId)
  }

  /**
   * 通知数据更新
   * @param componentId 组件ID
   * @param data 数据
   */
  private notifyDataUpdate(componentId: string, data: ComponentDataPayload): void {
    this.dataUpdateCallbacks.forEach((callback) => {
      try {
        callback(componentId, data)
      } catch (error) {
        bridgeLogger.error('[VisualEditorBridge] 数据更新回调执行失败:', {
          componentId,
          error: error instanceof Error ? error.message : error
        })
      }
    })
  }

  /**
   * 将旧的配置格式转换为新的数据需求格式
   * @param componentId 组件ID
   * @param componentType 组件类型
   * @param config 配置对象
   */
  private convertConfigToRequirement(
    componentId: string,
    componentType: string,
    config: EditorComponentConfig | null
  ): ComponentDataRequirement {
    const { resolvedConfig, baseConfig } = this.resolveBridgeConfig(config)
    const dataSources =
      resolvedConfig && typeof resolvedConfig === 'object'
        ? this.resolveDataSourcesInPriorityOrder(resolvedConfig, baseConfig)
        : []

    return {
      componentId,
      componentType,
      dataSources,
      enabled: true
    }
  }

  private resolveDataSourcesInPriorityOrder(
    resolvedConfig: EditorDataSourceLike,
    baseConfig: Record<string, unknown> | null
  ): BridgeDataSource[] {
    const dataSources: BridgeDataSource[] = []
    const collectors = [
      () => this.collectStandardDataSources(dataSources, resolvedConfig),
      () => this.collectRawDataListSources(dataSources, resolvedConfig),
      () => this.collectNamedDataSources(dataSources, resolvedConfig, baseConfig),
      () => this.collectSingleDataSource(dataSources, resolvedConfig, baseConfig)
    ]

    for (const collect of collectors) {
      collect()
      if (dataSources.length > 0) {
        break
      }
    }

    return dataSources
  }

  private resolveBridgeConfig(config: EditorComponentConfig | null): ResolvedBridgeConfig {
    let resolvedConfig: EditorDataSourceLike | null = config
    let baseConfig: Record<string, unknown> | null = null

    if (config && typeof config === 'object' && (config.base || config.dataSource)) {
      baseConfig = config.base || {}
      resolvedConfig = {
        ...config.dataSource,
        deviceId: baseConfig.deviceId,
        metricsList: baseConfig.metricsList,
        ...(config.dataSource || {})
      }
      resolvedConfig = this.injectBaseConfigToDataSource(resolvedConfig, baseConfig)
    }

    return { resolvedConfig, baseConfig }
  }

  private injectBaseConfigToDataSource<T>(dataSourceConfig: T, baseConfig: Record<string, unknown> | null): T {
    if (!baseConfig) {
      return dataSourceConfig
    }

    // 绑定替换会原地修改嵌套配置，这里先克隆，避免污染编辑器原始配置对象。
    const enhanced = JSON.parse(JSON.stringify(dataSourceConfig)) as T

    this.processBindingReplacements(enhanced as Record<string, unknown>, baseConfig)

    return enhanced
  }

  private collectStandardDataSources(dataSources: BridgeDataSource[], resolvedConfig: EditorDataSourceLike): void {
    if (!resolvedConfig.dataSources || !Array.isArray(resolvedConfig.dataSources)) {
      return
    }

    resolvedConfig.dataSources.forEach(dataSource => {
      if (!dataSource.sourceId || !Array.isArray(dataSource.dataItems)) {
        return
      }

      dataSources.push({
        sourceId: dataSource.sourceId,
        dataItems: dataSource.dataItems.map(dataItem => this.convertStandardDataItem(dataItem)).filter(Boolean),
        mergeStrategy: dataSource.mergeStrategy || { type: 'object' }
      })
    })
  }

  private convertStandardDataItem(dataItem: EditorStandardDataItem | null | undefined) {
    if (!dataItem || !dataItem.item) {
      return null
    }

    return {
      item: {
        type: dataItem.item.type,
        config: this.convertItemConfig(dataItem.item)
      },
      processing: {
        filterPath: dataItem.processing?.filterPath || '$',
        customScript: dataItem.processing?.customScript,
        defaultValue: {}
      }
    }
  }

  private collectRawDataListSources(dataSources: BridgeDataSource[], resolvedConfig: EditorDataSourceLike): void {
    if (dataSources.length > 0 || !Array.isArray(resolvedConfig.rawDataList)) {
      return
    }

    resolvedConfig.rawDataList.forEach((item, index) => {
      if (!item || !item.type || item.enabled === false) {
        return
      }

      const sourceId = item.id || item.sourceId || `dataSource${index + 1}`
      const normalizedType = this.normalizeDataSourceType(item.type, sourceId)
      if (!normalizedType) {
        return
      }

      dataSources.push({
        id: sourceId,
        type: normalizedType,
        config: item.config || {},
        filterPath: item.filterPath,
        processScript: item.processScript
      })
    })
  }

  private collectNamedDataSources(
    dataSources: BridgeDataSource[],
    resolvedConfig: EditorDataSourceLike,
    baseConfig: Record<string, unknown> | null
  ): void {
    if (dataSources.length > 0) {
      return
    }

    for (const [key, value] of Object.entries(resolvedConfig)) {
      if (!key.startsWith('dataSource') || !value || typeof value !== 'object') {
        continue
      }

      const enhancedDataSourceConfig = this.injectBaseConfigToDataSource(value as EditorDataSourceLike, baseConfig)
      if (!enhancedDataSourceConfig.type || enhancedDataSourceConfig.enabled === false) {
        continue
      }

      const normalizedType = this.normalizeDataSourceType(enhancedDataSourceConfig.type, key)
      if (!normalizedType) {
        continue
      }

      dataSources.push({
        id: key,
        type: normalizedType,
        config: enhancedDataSourceConfig.config || {},
        filterPath: enhancedDataSourceConfig.filterPath,
        processScript: enhancedDataSourceConfig.processScript
      })
    }
  }

  private collectSingleDataSource(
    dataSources: BridgeDataSource[],
    resolvedConfig: EditorDataSourceLike,
    baseConfig: Record<string, unknown> | null
  ): void {
    if (dataSources.length > 0 || !resolvedConfig.type || resolvedConfig.enabled === false) {
      return
    }

    if (resolvedConfig.type === 'data-source-bindings') {
      this.collectBindingAliasSources(dataSources, resolvedConfig)
      return
    }

    const enhancedConfig = this.injectBaseConfigToDataSource(resolvedConfig, baseConfig)
    const sourceId = 'dataSource1'
    const normalizedType = this.normalizeDataSourceType(enhancedConfig.type, sourceId)
    if (!normalizedType) {
      return
    }

    dataSources.push({
      id: sourceId,
      type: normalizedType,
      config: enhancedConfig.config || enhancedConfig,
      filterPath: enhancedConfig.filterPath,
      processScript: enhancedConfig.processScript
    })
  }

  private collectBindingAliasSources(dataSources: BridgeDataSource[], resolvedConfig: EditorDataSourceLike): void {
    for (const [key, value] of Object.entries(resolvedConfig)) {
      if (!key.startsWith('dataSource') || !value || typeof value !== 'object') {
        continue
      }

      dataSources.push({
        id: key,
        type: 'data-source-bindings',
        config: { dataSourceBindings: { [key]: value } },
        filterPath: undefined,
        processScript: undefined
      })
    }
  }

  /**
   * 在克隆后的数据源配置上替换绑定表达式。
   * 这里允许原地修改，因为调用方已经先做了深拷贝。
   */
  private processBindingReplacements(config: EditorDataSourceLike, baseConfig: Record<string, unknown>): void {
    const autoBindConfig = this.getAutoBindConfigFromDataSource(config)

    if (autoBindConfig && autoBindConfig.enabled) {
      // 使用autoBind配置处理参数绑定（同步版本）
      this.processAutoBindParamsSync(config, baseConfig, autoBindConfig)
    } else {
      // 使用传统方式处理参数绑定
      this.processTraditionalBinding(config, baseConfig)
    }
  }

  /**
   * 将 autoBind 规则落到请求参数中。
   */
  private processAutoBindParamsSync(
    config: EditorDataSourceLike,
    baseConfig: Record<string, unknown>,
    autoBindConfig: AutoBindConfig
  ): void {
    // 构建完整配置对象
    const fullConfig = {
      base: baseConfig,
      dataSource: config,
      componentType: config.componentType || 'widget'
    }

    // 使用autoBind生成HTTP参数
    const autoBindParams = dataSourceBindingConfig.buildAutoBindParams(fullConfig, autoBindConfig, config.componentType)

    // 将autoBind参数注入到HTTP配置中
    if (config.type === 'http' && config.config) {
      config.config.params = {
        ...config.config.params,
        ...autoBindParams
      }
    } else if (config.config) {
      config.config = {
        ...config.config,
        ...autoBindParams
      }
    }

    bridgeLogger.debug('[VisualEditorBridge] AutoBind参数注入完成:', {
      mode: autoBindConfig.mode,
      autoBindParams,
      finalConfig: config.config
    })
  }

  /**
   * 传统方式处理参数绑定
   */
  private processTraditionalBinding(config: EditorDataSourceLike, baseConfig: Record<string, unknown>): void {
    // 1. 首先处理基础配置注入（原有逻辑，模拟设备ID的硬编码机制）
    if (config.config && typeof config.config === 'object') {
      config.config = {
        ...config.config,
        // 注入基础配置中的设备属性（模拟设备ID硬编码逻辑）
        ...(baseConfig.deviceId && { deviceId: baseConfig.deviceId }),
        ...(baseConfig.metricsList && { metricsList: baseConfig.metricsList })
      }
    } else {
      // 如果没有 config 对象，直接在顶层注入
      config.deviceId = config.deviceId || baseConfig.deviceId
      config.metricsList = config.metricsList || baseConfig.metricsList
    }

    // 2. 递归替换嵌套配置中的 component/base 绑定表达式。
    this.recursivelyReplaceBindings(config)
  }

  /**
   * 从数据源配置中提取 autoBind 设置。
   * @param dataSourceConfig 数据源配置
   * @returns autoBind配置或null
   */
  private getAutoBindConfigFromDataSource(dataSourceConfig: EditorDataSourceLike): AutoBindConfig | null {
    // 检查数据源配置中的autoBind设置
    if (dataSourceConfig.autoBind) {
      return dataSourceConfig.autoBind
    }

    // 检查config层级的autoBind设置
    if (dataSourceConfig.config?.autoBind) {
      return dataSourceConfig.config.autoBind
    }

    return null
  }

  /**
   * 递归替换绑定表达式。
   * 支持 component、base，以及历史遗留 whitelist 持久化别名。
   */
  private recursivelyReplaceBindings(obj: Record<string, unknown>): void {
    if (!obj || typeof obj !== 'object') {
      return
    }

    for (const key in obj) {
      if (Object.prototype.hasOwnProperty.call(obj, key)) {
        const val = obj[key]

        if (typeof val === 'string') {
          // 识别当前桥接层支持的绑定表达式格式。
          // 格式1: componentId.component.propertyName （标准组件属性绑定）
          const componentBindingMatch = val.match(/^([^.]+)\.component\.(.+)$/)

          // 格式2: componentId.base.propertyName （基础配置绑定）
          const baseBindingMatch = val.match(/^([^.]+)\.base\.(.+)$/)

          // whitelist 是历史持久化别名，读取时继续兼容，但新配置应统一写成 component 绑定格式。
          const whitelistBindingMatch = val.match(/^([^.]+)\.whitelist\.(.+)$/)

          if (componentBindingMatch) {
            const [, componentId, propertyName] = componentBindingMatch

            const actualValue = this.getComponentPropertyValueFixed(componentId, propertyName)
            if (actualValue !== undefined) {
              obj[key] = String(actualValue)
            }
            // actualValue 为 undefined 时保留原绑定表达式，等待组件属性就绪后再替换
          } else if (baseBindingMatch) {
            const [, componentId, propertyName] = baseBindingMatch

            // 尝试获取基础配置值（使用已有的获取逻辑）
            const actualValue = this.getBaseConfigPropertyValue(componentId, propertyName)
            if (actualValue !== undefined) {
              obj[key] = String(actualValue)
            }
            // actualValue 为 undefined 时保留原绑定表达式
          } else if (whitelistBindingMatch) {
            // 兼容历史 whitelist 别名，并按组件属性绑定处理。
            const [, componentId, propertyName] = whitelistBindingMatch

            const actualValue = this.getComponentPropertyValueFixed(componentId, propertyName)
            if (actualValue !== undefined) {
              obj[key] = String(actualValue)
            }
            // actualValue 为 undefined 时保留原绑定表达式
          }
          // 不是绑定表达式的字符串值无需处理
        } else if (typeof val === 'object' && val !== null) {
          // 递归处理嵌套对象
          this.recursivelyReplaceBindings(val as Record<string, unknown>)
        }
      }
    }
  }

  /**
   * 桥接边界说明:
   * 这里暂时不负责反向读取 component/base 绑定值。
   * 只有当 Visual Editor 上层提供稳定的配置查询入口后，才能安全接入；
   * 否则会把桥接层重新耦合回编辑器实现，破坏当前的单向依赖边界。
   */
  private logUnsupportedPropertyRead(scope: 'base' | 'component', componentId: string, propertyName: string): void {
    bridgeLogger.error('[VisualEditorBridge] Unsupported property binding read:', {
      scope,
      componentId,
      propertyName,
      reason: 'No reliable Visual Editor store/configuration read path is wired for this bridge.'
    })
  }

  private getBaseConfigPropertyValue(componentId: string, propertyName: string): undefined {
    this.logUnsupportedPropertyRead('base', componentId, propertyName)
    return undefined
  }

  /**
   * 组件属性反向读取占位点。
   * 理想优先级应为: 最新配置 > 编辑器节点 > DOM。
   * 当前仍返回 undefined，表示桥接层只保留绑定表达式，不在这里擅自补齐编辑器读路径。
   */
  private getComponentPropertyValueFixed(componentId: string, propertyName: string): undefined {
    this.logUnsupportedPropertyRead('component', componentId, propertyName)
    return undefined
  }

  /**
   * 转换数据项配置，处理字段映射
   */
  private convertItemConfig(item: NonNullable<EditorStandardDataItem['item']>) {
    const { type, config } = item

    switch (type) {
      case 'json':
        // JSON类型：jsonString → jsonContent
        return {
          ...config,
          jsonContent: config.jsonString || config.jsonContent
        }

      case 'http':
        // HTTP类型：保持原有字段
        return config

      case 'script':
        // Script类型：script → scriptContent
        return {
          ...config,
          scriptContent: config.script || config.scriptContent
        }

      default:
        return config
    }
  }
}

// 端口隔离的VisualEditorBridge实例管理
const bridgeInstances = new Map<string, VisualEditorBridge>()

/**
 * 获取端口ID（用于多端口开发环境的实例隔离）
 */
function getPortId(): string {
  if (typeof window !== 'undefined') {
    return window.location.port || 'default'
  }
  return 'default'
}

/**
 * 获取当前端口的VisualEditorBridge实例
 * 确保不同端口使用独立的桥接器实例，避免数据回调干扰
 */
export function getVisualEditorBridge(): VisualEditorBridge {
  const portId = getPortId()

  if (!bridgeInstances.has(portId)) {
    bridgeInstances.set(portId, new VisualEditorBridge())
  }

  return bridgeInstances.get(portId)!
}

/**
 * 释放当前端口的桥接实例及其本地订阅。
 * 可重复调用；下一次获取时会创建全新的隔离实例。
 */
export function disposeVisualEditorBridge(): void {
  const portId = getPortId()
  const bridge = bridgeInstances.get(portId)
  if (!bridge) {
    return
  }

  bridge.dispose()
  bridgeInstances.delete(portId)
}
