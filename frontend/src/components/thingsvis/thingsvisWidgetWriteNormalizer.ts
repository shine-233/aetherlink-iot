/**
 * 文件说明：
 * - 承接 ThingsVisWidget 中平台字段写入前的字段推断与类型归一化逻辑。
 * - 负责从 widget config 反推 dataSource -> field 绑定，并将 guest 写入值按平台字段类型整理。
 * 维护提示：
 * - 这里只处理纯数据归一化，不负责消息可信校验、平台 API 发布或 guest 回包。
 * - 自动写回字段的推断依赖 ThingsVis 节点 value 绑定与 requestedFields 约定，修改前要先确认 guest 契约。
 */

type ParsedThingsVisFieldBinding = {
  dataSourceId: string
  fieldPath: string
}

type CreateThingsVisWidgetWriteNormalizerOptions = {
  getConfig: () => any
  historyFieldSuffix: string
  getFieldValueTypeMap: () => Record<string, string>
  parseFieldBindingExpression: (expression: unknown) => ParsedThingsVisFieldBinding | null
}

const collectConfiguredWriteFields = ({
  getConfig,
  parseFieldBindingExpression
}: Pick<CreateThingsVisWidgetWriteNormalizerOptions, 'getConfig' | 'parseFieldBindingExpression'>) => {
  const fieldIdsByDataSourceId = new Map<string, Set<string>>()
  const config = getConfig()
  const nodes = Array.isArray(config?.nodes) ? config.nodes : []

  nodes.forEach((node: any) => {
    const bindings = Array.isArray(node?.data) ? node.data : []
    const valueBinding = bindings.find((binding: any) => binding?.targetProp === 'value')
    const parsed =
      parseFieldBindingExpression(valueBinding?.expression) || parseFieldBindingExpression(node?.props?.value)

    if (!parsed) return

    const fieldSet = fieldIdsByDataSourceId.get(parsed.dataSourceId) || new Set<string>()
    fieldSet.add(parsed.fieldPath)
    fieldIdsByDataSourceId.set(parsed.dataSourceId, fieldSet)
  })

  const dataSources = Array.isArray(config?.dataSources) ? config.dataSources : []
  dataSources.forEach((dataSource: any) => {
    const dataSourceId = typeof dataSource?.id === 'string' ? dataSource.id : ''
    if (!dataSourceId) return

    const configuredFields = Array.isArray(dataSource?.config?.requestedFields)
      ? dataSource.config.requestedFields.filter(
          (fieldId: unknown): fieldId is string => typeof fieldId === 'string' && !!fieldId
        )
      : []

    if (configuredFields.length !== 1) return

    const fieldSet = fieldIdsByDataSourceId.get(dataSourceId) || new Set<string>()
    fieldSet.add(configuredFields[0]!)
    fieldIdsByDataSourceId.set(dataSourceId, fieldSet)
  })

  return fieldIdsByDataSourceId
}

const normalizeBooleanValue = (value: unknown) => {
  if (typeof value === 'boolean') return value
  if (value === 'true' || value === '1' || value === 1) return true
  if (value === 'false' || value === '0' || value === 0) return false
  return value
}

const normalizeNumberValue = (value: unknown) => {
  if (typeof value === 'boolean') return value ? 1 : 0
  if (typeof value === 'number') return Number.isFinite(value) ? value : value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return value
}

export const createThingsVisWidgetWriteNormalizer = ({
  getConfig,
  historyFieldSuffix,
  getFieldValueTypeMap,
  parseFieldBindingExpression
}: CreateThingsVisWidgetWriteNormalizerOptions) => {
  const normalizeFieldWriteValue = (fieldId: string, value: unknown) => {
    const fieldValueType = getFieldValueTypeMap()[fieldId]
    if (!fieldValueType) return value

    if (fieldValueType === 'boolean') return normalizeBooleanValue(value)
    if (fieldValueType === 'number') return normalizeNumberValue(value)
    return value
  }

  const normalizeWriteDataObject = (data: Record<string, unknown>) => {
    return Object.entries(data).reduce<Record<string, unknown>>((acc, [fieldId, value]) => {
      acc[fieldId] = normalizeFieldWriteValue(fieldId, value)
      return acc
    }, {})
  }

  const normalizeSingleFieldWriteData = (dataSourceId: string, data: unknown) => {
    const configuredFields = collectConfiguredWriteFields({
      getConfig,
      parseFieldBindingExpression
    }).get(dataSourceId)
    const writableFields = Array.from(configuredFields || []).filter((fieldId) => !fieldId.endsWith(historyFieldSuffix))
    if (writableFields.length !== 1) return data

    const fieldId = writableFields[0]
    if (!fieldId) return data

    return {
      [fieldId]: normalizeFieldWriteValue(fieldId, data)
    }
  }

  return (dataSourceId: string, data: unknown) => {
    if (data !== null && typeof data === 'object') {
      return normalizeWriteDataObject(data as Record<string, unknown>)
    }

    return normalizeSingleFieldWriteData(dataSourceId, data)
  }
}
