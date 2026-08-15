const TEMPLATE_PLATFORM_SOURCE_ID = '__platform___template____'
const RUNTIME_PLATFORM_FIELD_IDS = new Set(['is_online', 'online_text', 'online_status_updated_at'])

export function getRuntimePlatformSourceId(deviceId: string) {
  return `__platform_${deviceId}__`
}

export function isValidRequestedFieldId(fieldId: string, availableFieldIds: Set<string>) {
  if (!fieldId) return false
  if (availableFieldIds.has(fieldId) || RUNTIME_PLATFORM_FIELD_IDS.has(fieldId)) return true
  if (fieldId.endsWith('__history')) {
    const root = fieldId.slice(0, -'__history'.length)
    return availableFieldIds.has(root)
  }
  return false
}

export function rewriteTemplateBindingExpression(expression: unknown, runtimeDataSourceId: string): unknown {
  if (typeof expression !== 'string') return expression

  const directFieldMatch = expression.match(/^\{\{\s*ds\.__platform___template____\.data\.([A-Za-z0-9_-]+)\s*\}\}$/)
  if (directFieldMatch?.[1]) {
    return `{{ ds.${runtimeDataSourceId}.data.${directFieldMatch[1]} }}`
  }

  const booleanSelectMatch = expression.match(
    /^\{\{\s*ds\.__platform___template____\.data\.([A-Za-z0-9_-]+)\s*\?\s*'1'\s*:\s*'0'\s*\}\}$/
  )
  if (booleanSelectMatch?.[1]) {
    return `{{ ds.${runtimeDataSourceId}.data.${booleanSelectMatch[1]} }}`
  }

  return expression.split(TEMPLATE_PLATFORM_SOURCE_ID).join(runtimeDataSourceId)
}

function normalizeTemplateDataSource(
  dataSource: any,
  deviceId: string,
  runtimeDataSourceId: string,
  availableFieldIds: Set<string>
) {
  if (dataSource?.type !== 'PLATFORM_FIELD') return dataSource

  const requestedFields = Array.isArray(dataSource?.config?.requestedFields)
    ? dataSource.config.requestedFields.filter(
        (fieldId: unknown): fieldId is string =>
          typeof fieldId === 'string' && isValidRequestedFieldId(fieldId, availableFieldIds)
      )
    : []

  return {
    ...dataSource,
    id: dataSource?.id === TEMPLATE_PLATFORM_SOURCE_ID ? runtimeDataSourceId : dataSource?.id,
    config: {
      ...(dataSource?.config || {}),
      deviceId,
      requestedFields
    }
  }
}

function hasTemplateBooleanSelectExpression(expression: unknown) {
  return typeof expression === 'string'
    ? expression.match(/^\{\{\s*ds\.__platform___template____\.data\.([A-Za-z0-9_-]+)\s*\?\s*'1'\s*:\s*'0'\s*\}\}$/)
    : null
}

function normalizeTemplateBinding(binding: any, runtimeDataSourceId: string) {
  const nextBinding = { ...binding }
  nextBinding.expression = rewriteTemplateBindingExpression(nextBinding?.expression, runtimeDataSourceId)
  if (hasTemplateBooleanSelectExpression(binding?.expression)?.[1]) {
    nextBinding.transform = `value ? '1' : '0'`
  }
  return nextBinding
}

function normalizeTemplateAction(action: any, runtimeDataSourceId: string) {
  const nextAction = { ...action }
  if (nextAction?.dataSourceId === TEMPLATE_PLATFORM_SOURCE_ID) {
    nextAction.dataSourceId = runtimeDataSourceId
  }
  if (typeof nextAction?.payload === 'string') {
    nextAction.payload = nextAction.payload.replace(/"([A-Za-z0-9_-]+)\s*\?\s*'1'\s*:\s*'0'"/g, '"$1"')
  }
  return nextAction
}

function normalizeTemplateNode(node: any, runtimeDataSourceId: string) {
  const nextNode = { ...node }

  if (Array.isArray(nextNode.data)) {
    nextNode.data = nextNode.data.map((binding: any) => normalizeTemplateBinding(binding, runtimeDataSourceId))
  }

  if (nextNode.props && typeof nextNode.props === 'object') {
    Object.entries(nextNode.props).forEach(([key, value]) => {
      nextNode.props[key] = rewriteTemplateBindingExpression(value, runtimeDataSourceId)
    })
  }

  if (Array.isArray(nextNode.events)) {
    nextNode.events = nextNode.events.map((handler: any) => ({
      ...handler,
      actions: Array.isArray(handler?.actions)
        ? handler.actions.map((action: any) => normalizeTemplateAction(action, runtimeDataSourceId))
        : handler?.actions
    }))
  }

  return nextNode
}

export function normalizeTemplateChartConfig(rawConfig: any, deviceId: string, availableFieldIds: Set<string>) {
  if (!rawConfig || typeof rawConfig !== 'object') return rawConfig

  const runtimeDataSourceId = getRuntimePlatformSourceId(deviceId)
  const config = JSON.parse(JSON.stringify(rawConfig))

  if (Array.isArray(config.dataSources)) {
    config.dataSources = config.dataSources.map((dataSource: any) =>
      normalizeTemplateDataSource(dataSource, deviceId, runtimeDataSourceId, availableFieldIds)
    )
  }

  if (Array.isArray(config.nodes)) {
    config.nodes = config.nodes.map((node: any) => normalizeTemplateNode(node, runtimeDataSourceId))
  }

  return config
}
