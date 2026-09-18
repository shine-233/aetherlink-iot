/**
 * 文件用途：看板端实体关系图谱动态数据装载器 Composable（ROADMAP P1.1）。
 * 核心逻辑：
 * 1. 扫描看板所有启用了 entityRelation 的小部件配置；
 * 2. 批量拉取起点实体的拓扑关联边（listEntityRelations）；
 * 3. 提取关联的目标实体并批量获取实时遥测（telemetryDataCurrent）；
 * 4. 通过 resolveEntityRelationValue 计算各小部件的聚合数值或序列，并注入到响应式 fields 中。
 */

import { computed, getCurrentInstance, onBeforeUnmount, ref, watch, type Ref } from 'vue'
import type { LocalFieldValue, NormalizedLocalDashboard, NormalizedLocalWidget } from '@/components/local-visualization-viewer'
import {
  generateRelationFieldKey,
  isEntityRelationConfigured,
  resolveEntityRelationValue
} from '@/components/local-visualization-viewer/entity-relation'
import type {
  EntityRelationEdge,
  EntityRelationSourceConfig
} from '@/components/local-visualization-viewer/entity-relation/types'
import { listEntityRelations } from '@/service/api/entity-relation'
import { telemetryDataCurrent } from '@/service/api/device-telemetry-twin-api'

export interface EntityRelationDataLoaderOptions {
  refreshIntervalMs?: number
}

export function useEntityRelationDataLoader(
  dashboardRef: Ref<unknown | null>,
  options: EntityRelationDataLoaderOptions = {}
) {
  const fields = ref<Record<string, LocalFieldValue>>({})
  const loading = ref(false)
  const error = ref<string | null>(null)
  let timer: ReturnType<typeof setInterval> | null = null
  let loadSequence = 0

  function extractEntityRelationConfigs(dashboard: unknown): Array<{
    widgetId: string
    widgetType: string
    config: EntityRelationSourceConfig
  }> {
    if (!dashboard || typeof dashboard !== 'object') return []
    const d = dashboard as NormalizedLocalDashboard
    if (!Array.isArray(d.widgets)) return []

    const results: Array<{
      widgetId: string
      widgetType: string
      config: EntityRelationSourceConfig
    }> = []

    for (const w of d.widgets as NormalizedLocalWidget[]) {
      const cfg = (w.config as any)?.entityRelation || w.entityRelation
      if (isEntityRelationConfigured(cfg)) {
        results.push({
          widgetId: w.id,
          widgetType: w.type,
          config: cfg
        })
      }
    }

    return results
  }

  async function loadData() {
    const currentSeq = ++loadSequence
    const relationWidgets = extractEntityRelationConfigs(dashboardRef.value)

    if (relationWidgets.length === 0) {
      fields.value = {}
      loading.value = false
      error.value = null
      return
    }

    loading.value = true
    error.value = null

    try {
      // 1. 按 (rootType, rootId, direction, relationType) 分组批处理关系边查询
      const edgeQueryMap = new Map<string, {
        rootType: string
        rootId: string
        direction: 'from' | 'to'
        relationType: string
      }>()

      for (const item of relationWidgets) {
        const key = `${item.config.rootType}:${item.config.rootId}:${item.config.direction}:${item.config.relationType}`
        if (!edgeQueryMap.has(key)) {
          edgeQueryMap.set(key, {
            rootType: item.config.rootType,
            rootId: item.config.rootId,
            direction: item.config.direction,
            relationType: item.config.relationType
          })
        }
      }

      const allEdges: EntityRelationEdge[] = []
      const edgePromises = Array.from(edgeQueryMap.values()).map(async query => {
        try {
          const params: Record<string, any> = {
            relation_type: query.relationType,
            direction: query.direction,
            limit: 100
          }
          if (query.direction === 'from') {
            params.from_type = query.rootType
            params.from_id = query.rootId
          } else {
            params.to_type = query.rootType
            params.to_id = query.rootId
          }
          const res = await listEntityRelations(params)
          if (res && res.data && Array.isArray(res.data.list)) {
            allEdges.push(...(res.data.list as EntityRelationEdge[]))
          }
        } catch (err) {
          console.warn('[useEntityRelationDataLoader] Failed to query relations for:', query, err)
        }
      })

      await Promise.all(edgePromises)
      if (currentSeq !== loadSequence) return

      // 2. 收集所有可能的目标实体 ID 并拉取遥测
      const targetDeviceIds = new Set<string>()
      for (const edge of allEdges) {
        if (edge.to_type === 'device' && edge.to_id) {
          targetDeviceIds.add(edge.to_id)
        }
        if (edge.from_type === 'device' && edge.from_id) {
          targetDeviceIds.add(edge.from_id)
        }
      }

      const telemetryMap: Record<string, Record<string, unknown>> = {}
      const telemetryPromises = Array.from(targetDeviceIds).map(async deviceId => {
        try {
          const res = await telemetryDataCurrent(deviceId)
          const dataMap: Record<string, unknown> = {}
          if (res && res.data && Array.isArray(res.data)) {
            for (const row of res.data) {
              if (row && typeof row.key === 'string') {
                dataMap[row.key] = row.value
              }
            }
          }
          telemetryMap[deviceId] = dataMap
        } catch (err) {
          console.warn(`[useEntityRelationDataLoader] Failed to fetch telemetry for device ${deviceId}:`, err)
          telemetryMap[deviceId] = {}
        }
      })

      await Promise.all(telemetryPromises)
      if (currentSeq !== loadSequence) return

      // 3. 计算每个小部件的解析值并组装 fields
      const newFields: Record<string, LocalFieldValue> = {}

      for (const item of relationWidgets) {
        const cfg = item.config
        const fieldKey = generateRelationFieldKey(cfg)
        const result = resolveEntityRelationValue(cfg, allEdges, telemetryMap)

        if (item.widgetType === 'line-chart' || item.widgetType === 'bar-chart') {
          // 图表小部件：提取匹配的目标实体列表与各实体的数值列表
          const categories: string[] = []
          const values: number[] = []
          for (const tv of result.targetValues) {
            categories.push(tv.targetId)
            values.push(typeof tv.numericValue === 'number' ? tv.numericValue : 0)
          }
          newFields[`${fieldKey}_cats`] = Object.freeze(categories)
          newFields[fieldKey] = Object.freeze(values)
        } else {
          // 文本与指标小部件：直接绑定解析出的标量聚合值
          newFields[fieldKey] = result.resolvedValue
        }
      }

      fields.value = Object.freeze(newFields)
    } catch (err) {
      if (currentSeq !== loadSequence) return
      error.value = err instanceof Error ? err.message : '加载实体关系数据失败'
    } finally {
      if (currentSeq === loadSequence) {
        loading.value = false
      }
    }
  }

  watch(
    () => dashboardRef.value,
    () => {
      loadData()
    },
    { immediate: true, deep: true }
  )

  if (options.refreshIntervalMs && options.refreshIntervalMs > 0) {
    timer = setInterval(loadData, options.refreshIntervalMs)
  }

  if (getCurrentInstance()) {
    onBeforeUnmount(() => {
      loadSequence += 1
      if (timer) {
        clearInterval(timer)
        timer = null
      }
    })
  }

  return {
    fields,
    loading,
    error,
    refresh: loadData
  }
}
