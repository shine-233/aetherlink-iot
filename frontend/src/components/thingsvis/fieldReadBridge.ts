/**
 * 文件说明：
 * - ThingsVis 读字段桥接里的基础字段分类工具。
 * - 负责把请求字段拆成“当前值字段”“显式历史字段”等集合，供上层桥接复用。
 * - 保持纯函数设计，避免把 Widget/接口细节耦合进来。
 */
export type RequestedFieldGroups = {
  currentFieldIds: string[]
  explicitHistoryFieldIds: string[]
  explicitHistorySourceFieldIds: Set<string>
}

export type RequestedCurrentFieldGroups = {
  alarmFieldIds: string[]
  currentFieldIds: string[]
}

export function normalizeRequestedFieldIds(fieldIds: unknown): string[] {
  return Array.isArray(fieldIds) ? fieldIds.filter((fieldId): fieldId is string => typeof fieldId === 'string') : []
}

export function classifyRequestedFieldIds(fieldIds: string[], historyFieldSuffix: string): RequestedFieldGroups {
  return fieldIds.reduce<RequestedFieldGroups>(
    (acc, fieldId) => {
      if (fieldId.endsWith(historyFieldSuffix)) {
        // __history 字段本质上是在请求“某个实时字段对应的历史序列”。
        acc.explicitHistoryFieldIds.push(fieldId)
        const sourceFieldId = fieldId.slice(0, -historyFieldSuffix.length)
        if (sourceFieldId) acc.explicitHistorySourceFieldIds.add(sourceFieldId)
        return acc
      }

      acc.currentFieldIds.push(fieldId)
      return acc
    },
    {
      currentFieldIds: [],
      explicitHistoryFieldIds: [],
      explicitHistorySourceFieldIds: new Set<string>()
    }
  )
}

export function splitRequestedFieldIds(
  requestedFields: string[],
  options: {
    alarmStatusFieldIds: Set<string>
    historyFieldSuffix: string
  }
): RequestedCurrentFieldGroups {
  // 告警派生字段走单独接口，其余当前值字段仍走实时数据通道。
  return {
    alarmFieldIds: requestedFields.filter((fieldId) => options.alarmStatusFieldIds.has(fieldId)),
    currentFieldIds: requestedFields.filter(
      (fieldId) => !fieldId.endsWith(options.historyFieldSuffix) && !options.alarmStatusFieldIds.has(fieldId)
    )
  }
}
