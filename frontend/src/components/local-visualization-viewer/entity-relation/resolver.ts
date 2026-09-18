/**
 * 文件用途：看板端通用实体关系动态解析与遥测聚合引擎（ROADMAP P1.1）。
 * 核心逻辑：对标 ThingsBoard 关系图谱查询过滤、多实体拓扑边解析与多策略遥测聚合。
 */

import type {
  EntityRelationAggregation,
  EntityRelationEdge,
  EntityRelationSourceConfig,
  ResolveEntityRelationResult,
  ResolvedTargetValue
} from './types'

/**
 * 判断实体关系数据源配置是否完整且有效。
 */
export function isEntityRelationConfigured(config?: EntityRelationSourceConfig | null): boolean {
  if (!config || !config.enabled) return false
  return Boolean(
    config.rootId?.trim() &&
    config.relationType?.trim() &&
    config.targetKey?.trim() &&
    (config.direction === 'from' || config.direction === 'to')
  )
}

/**
 * 生成动态字段在看板 fields 中的唯一且确定性命名。
 */
export function generateRelationFieldKey(config: EntityRelationSourceConfig): string {
  const rootType = config.rootType || 'device'
  const rootId = (config.rootId || '').trim()
  const dir = config.direction || 'from'
  const relType = (config.relationType || '').trim().replace(/\s+/g, '_')
  const targetKey = (config.targetKey || '').trim().replace(/\s+/g, '_')
  return `__rel_${rootType}_${rootId}_${dir}_${relType}_${targetKey}`
}

/**
 * 根据关系边和配置过滤出匹配的目标实体 ID 列表（去重保序）。
 */
export function filterRelationTargetIds(
  config: EntityRelationSourceConfig,
  edges: readonly EntityRelationEdge[]
): string[] {
  if (!isEntityRelationConfigured(config) || !Array.isArray(edges)) {
    return []
  }

  const rootId = config.rootId.trim()
  const relType = config.relationType.trim()
  const targetType = config.targetType || 'device'
  const matchedIds: string[] = []
  const seen = new Set<string>()

  for (const edge of edges) {
    if (!edge) continue

    if (config.direction === 'from') {
      // 关系从起点指向目标：edge.from 为起点，edge.to 为目标
      if (
        edge.from_type === config.rootType &&
        edge.from_id === rootId &&
        edge.relation_type === relType &&
        edge.to_type === targetType
      ) {
        const id = edge.to_id?.trim()
        if (id && !seen.has(id)) {
          seen.add(id)
          matchedIds.push(id)
        }
      }
    } else {
      // 关系从目标指向起点：edge.to 为起点，edge.from 为目标
      if (
        edge.to_type === config.rootType &&
        edge.to_id === rootId &&
        edge.relation_type === relType &&
        edge.from_type === targetType
      ) {
        const id = edge.from_id?.trim()
        if (id && !seen.has(id)) {
          seen.add(id)
          matchedIds.push(id)
        }
      }
    }
  }

  return matchedIds
}

/**
 * 对提取出的目标数值集合执行聚合运算。
 */
export function aggregateNumericValues(
  values: readonly (number | null | undefined)[],
  aggregation: EntityRelationAggregation = 'latest'
): number | null {
  const validNumbers = values.filter((n): n is number => typeof n === 'number' && !Number.isNaN(n))

  if (validNumbers.length === 0) {
    return aggregation === 'count' ? 0 : null
  }

  switch (aggregation) {
    case 'count':
      return validNumbers.length
    case 'sum':
      return Number(validNumbers.reduce((acc, curr) => acc + curr, 0).toFixed(4))
    case 'avg': {
      const sum = validNumbers.reduce((acc, curr) => acc + curr, 0)
      return Number((sum / validNumbers.length).toFixed(4))
    }
    case 'max':
      return Math.max(...validNumbers)
    case 'min':
      return Math.min(...validNumbers)
    case 'latest':
    default:
      return validNumbers[0]
  }
}

/**
 * 核心综合解析函数：结合边数据与遥测实体映射表，计算最终小部件展示值。
 *
 * @param config 实体关系数据源配置
 * @param edges 关系图谱边列表
 * @param telemetryMap 实体遥测映射表，形如 { [targetId]: { [key]: rawValue } }
 */
export function resolveEntityRelationValue(
  config: EntityRelationSourceConfig,
  edges: readonly EntityRelationEdge[],
  telemetryMap: Record<string, Record<string, unknown>> = {}
): ResolveEntityRelationResult {
  const matchedTargetIds = filterRelationTargetIds(config, edges)
  const targetValues: ResolvedTargetValue[] = []
  const numericList: (number | null)[] = []
  let firstValidRaw: string | number | null = null

  const key = config.targetKey.trim()

  for (const targetId of matchedTargetIds) {
    const entityData = telemetryMap[targetId] || {}
    const rawVal = entityData[key]
    let numVal: number | undefined

    if (typeof rawVal === 'number' && !Number.isNaN(rawVal)) {
      numVal = rawVal
      numericList.push(numVal)
    } else if (typeof rawVal === 'string' && rawVal.trim() !== '' && !Number.isNaN(Number(rawVal))) {
      numVal = Number(rawVal)
      numericList.push(numVal)
    } else {
      numericList.push(null)
    }

    if (firstValidRaw === null && rawVal !== undefined && rawVal !== null) {
      if (typeof rawVal === 'string' || typeof rawVal === 'number') {
        firstValidRaw = rawVal
      }
    }

    targetValues.push({
      targetId,
      value: rawVal,
      numericValue: numVal
    })
  }

  const aggregation = config.aggregation || 'latest'
  let resolvedValue: string | number | null = null

  if (aggregation === 'count') {
    resolvedValue = targetValues.filter(t => t.value !== undefined && t.value !== null).length
  } else if (numericList.some(n => n !== null)) {
    resolvedValue = aggregateNumericValues(numericList, aggregation)
  } else {
    // 非数值型字段回退首个有效文本
    resolvedValue = firstValidRaw
  }

  return {
    matchedTargetIds,
    resolvedValue,
    targetValues
  }
}