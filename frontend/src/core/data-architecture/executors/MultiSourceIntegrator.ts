/**
 * 文件用途: 数据源执行链第四层多源整合器。
 * 核心逻辑: 将多个 DataSourceResult 按 key 整合为组件最终 ComponentData。
 * 关键注意事项: sourceId、success/error 和 lastUpdated 字段会影响组件数据可见性与错误展示。
 * 重构建议: 抽出成功/失败结果 normalize，并补重复 sourceId、失败源和空源列表测试。
 */

import { smartDeepClone } from '@/utils/deep-clone'

const FORBIDDEN_SOURCE_IDS = new Set(['__proto__', 'prototype', 'constructor'])

function isSafeSourceId(sourceId: unknown): sourceId is string {
  return typeof sourceId === 'string' && sourceId.length > 0 && !FORBIDDEN_SOURCE_IDS.has(sourceId)
}

export interface ComponentData {
  [dataSourceKey: string]: {
    /** 数据源类型 */
    type: string
    /** 解析后的数据 */
    data: any
    /** 最后更新时间 */
    lastUpdated: number
    /** 元数据 */
    metadata?: any
  }
}

export interface DataSourceResult {
  /** 数据源ID */
  sourceId: string
  /** 数据源类型 */
  type: string
  /** 合并后的数据 */
  data: any
  /** 是否成功 */
  success: boolean
  /** 错误信息 */
  error?: string
  /** 稳定错误代码，供上层区分外部阻断、配置错误和执行失败。 */
  errorCode?: string
}

/**
 * 多源整合器接口
 */
export interface IMultiSourceIntegrator {
  /**
   * 按key整合多数据源
   * @param sources 数据源结果列表
   * @param componentId 组件ID
   * @returns 组件最终数据，出错时返回空ComponentData
   */
  integrateDataSources(sources: DataSourceResult[], componentId: string): Promise<ComponentData>
}

/**
 * 多源整合器实现类
 */
export class MultiSourceIntegrator implements IMultiSourceIntegrator {
  /**
   * 多数据源整合主方法
   */
  async integrateDataSources(sources: DataSourceResult[], componentId: string): Promise<ComponentData> {
    try {
      const result: ComponentData = {}
      const timestamp = Date.now()
      const processedAt = new Date(timestamp).toISOString()

      for (const source of sources) {
        if (!this.validateDataSourceResult(source)) {
          continue
        }

        result[source.sourceId] = {
          type: source.type || 'unknown',
          // 整合结果拥有独立数据快照，避免调用方修改结果后反向污染执行层输入。
          data: source.success ? smartDeepClone(source.data) : null,
          lastUpdated: timestamp,
          metadata: {
            componentId,
            success: source.success,
            error: source.error,
            errorCode: source.errorCode,
            processedAt
          }
        }
      }

      return Object.keys(result).length === 0 ? {} : result
    } catch (_error) {
      return {} // 统一错误处理：返回空ComponentData
    }
  }

  /**
   * 验证数据源结果的有效性，并拒绝会影响普通对象原型的保留键。
   */
  validateDataSourceResult(source: DataSourceResult): boolean {
    return !!(source && isSafeSourceId(source.sourceId) && source.type !== undefined)
  }

  /**
   * 获取组件数据统计信息
   */
  getDataStatistics(componentData: ComponentData): {
    totalSources: number
    successfulSources: number
    failedSources: number
    lastUpdated: number
  } {
    const sources = Object.entries(componentData)
    const successful = sources.filter(([_, data]) => data.metadata?.success !== false)
    const failed = sources.filter(([_, data]) => data.metadata?.success === false)
    const lastUpdated = Math.max(...sources.map(([_, data]) => data.lastUpdated), 0)

    return {
      totalSources: sources.length,
      successfulSources: successful.length,
      failedSources: failed.length,
      lastUpdated
    }
  }

  /**
   * 检查组件数据是否有效
   */
  isValidComponentData(componentData: ComponentData): boolean {
    if (!componentData || typeof componentData !== 'object') {
      return false
    }

    // 至少要有一个数据源
    const sourceKeys = Object.keys(componentData)
    if (sourceKeys.length === 0) {
      return false
    }

    // 检查每个数据源的结构
    return sourceKeys.every(key => {
      const source = componentData[key]
      return (
        source && typeof source.type === 'string' && typeof source.lastUpdated === 'number' && source.data !== undefined
      )
    })
  }

  /**
   * 合并多个 ComponentData，用时间戳选择更新，并隔离输入对象引用。
   */
  mergeComponentData(existing: ComponentData, updates: ComponentData): ComponentData {
    const result: ComponentData = {}

    for (const [sourceId, sourceData] of Object.entries(existing)) {
      if (isSafeSourceId(sourceId)) {
        result[sourceId] = smartDeepClone(sourceData)
      }
    }

    for (const [sourceId, sourceData] of Object.entries(updates)) {
      if (!isSafeSourceId(sourceId)) {
        continue
      }

      const existingData = result[sourceId]
      if (!existingData || existingData.lastUpdated < sourceData.lastUpdated) {
        result[sourceId] = smartDeepClone(sourceData)
      }
    }

    return result
  }

  /**
   * 清理过期或危险键的数据源，并返回与输入隔离的快照。
   */
  cleanupExpiredData(componentData: ComponentData, maxAge: number = 5 * 60 * 1000): ComponentData {
    const now = Date.now()
    const result: ComponentData = {}

    for (const [sourceId, sourceData] of Object.entries(componentData)) {
      if (isSafeSourceId(sourceId) && now - sourceData.lastUpdated <= maxAge) {
        result[sourceId] = smartDeepClone(sourceData)
      }
    }

    return result
  }

  /**
   * 转换为 Visual Editor 兼容格式，并返回安全键组成的独立 payload 快照。
   */
  toVisualEditorFormat(componentData: ComponentData): Record<string, any> {
    const result: Record<string, any> = {}

    for (const [sourceId, sourceData] of Object.entries(componentData)) {
      if (isSafeSourceId(sourceId)) {
        result[sourceId] = smartDeepClone(sourceData.data)
      }
    }

    return result
  }

  /**
   * 转换为 Card 2.1 兼容格式；危险键或无法 JSON 序列化的数据源会被隔离跳过。
   */
  toCard21Format(componentData: ComponentData): any {
    const dataSourceBindings: Record<string, { rawData: string }> = {}

    for (const [sourceId, sourceData] of Object.entries(componentData)) {
      if (!isSafeSourceId(sourceId)) {
        continue
      }

      try {
        dataSourceBindings[sourceId] = { rawData: JSON.stringify(sourceData.data) }
      } catch (_error) {
        // 单个外部数据源不可序列化时，不阻断其他本地数据源输出。
      }
    }

    return {
      rawDataSources: {
        dataSourceBindings
      }
    }
  }
}
