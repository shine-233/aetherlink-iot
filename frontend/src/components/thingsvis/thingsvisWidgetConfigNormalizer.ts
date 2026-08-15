import {
  THINGSVIS_WIDGET_CARTESIAN_CHART_TYPES,
  THINGSVIS_WIDGET_GAUGE_CHART_TYPES,
  THINGSVIS_WIDGET_MODEL_3D_ACCEPTED_EXTENSIONS,
  THINGSVIS_WIDGET_MODEL_3D_MAX_UPLOAD_SIZE_MB,
  THINGSVIS_WIDGET_MODEL_3D_TYPES,
  THINGSVIS_WIDGET_PIE_CHART_TYPES
} from './thingsvisWidgetRuntimeCapabilities'

const FIELD_BINDING_EXPR_RE = /^\{\{\s*ds\.([^.\s]+)\.data(?:\.(.+?))?\s*\}\}$/
const DEFAULT_WRITE_EVENT_BY_COMPONENT: Record<string, string> = {
  'interaction/basic-switch': 'change',
  'interaction/basic-slider': 'change',
  'interaction/basic-select': 'change',
  'interaction/basic-input': 'submit'
}
const EZUIKIT_PLAYBACK_COMMAND = 'playback'
const EZUIKIT_COMPAT_DEFAULT_SPACE_ID = '361254'
const EZUIKIT_COMPAT_DEFAULT_BUS_TYPE = '7'
const AUTO_WRITE_MARKER = 'field-binding'
const VIEWER_INFINITE_CANVAS_PADDING = 48

type ChartFontSizeConfig = Partial<{
  title: number
  legend: number
  axisLabel: number
  axisName: number
  seriesLabel: number
  tooltip: number
  value: number
  gaugeDetail: number
  pieLabel: number
}>

const CARTESIAN_CHART_TYPES = new Set(THINGSVIS_WIDGET_CARTESIAN_CHART_TYPES)
const PIE_CHART_TYPES = new Set(THINGSVIS_WIDGET_PIE_CHART_TYPES)
const GAUGE_CHART_TYPES = new Set(THINGSVIS_WIDGET_GAUGE_CHART_TYPES)
const MODEL_3D_TYPES = new Set(THINGSVIS_WIDGET_MODEL_3D_TYPES)

const DEFAULT_CARTESIAN_CHART_FONT_SIZES: ChartFontSizeConfig = {
  title: 16,
  legend: 12,
  axisLabel: 12,
  axisName: 12,
  seriesLabel: 12,
  tooltip: 12
}

const DEFAULT_PIE_CHART_FONT_SIZES: ChartFontSizeConfig = {
  title: 16,
  legend: 12,
  pieLabel: 12,
  tooltip: 12
}

const DEFAULT_GAUGE_CHART_FONT_SIZES: ChartFontSizeConfig = {
  title: 16,
  axisLabel: 11,
  value: 24,
  gaugeDetail: 16,
  tooltip: 12
}

const DEFAULT_MODEL_3D_PROPS = {
  acceptedExtensions: THINGSVIS_WIDGET_MODEL_3D_ACCEPTED_EXTENSIONS,
  maxUploadSizeMb: THINGSVIS_WIDGET_MODEL_3D_MAX_UPLOAD_SIZE_MB,
  cameraControls: true,
  autoRotate: false,
  backgroundColor: 'transparent'
}

type ThingsVisWidgetMode = 'viewer' | 'editor'

type NormalizeThingsVisWidgetLoadConfigOptions = {
  mode?: ThingsVisWidgetMode
  previewDeviceId?: string
  fieldValueTypes: Record<string, string>
}

export const parseThingsVisFieldBindingExpression = (expression: unknown) => {
  if (typeof expression !== 'string') return null
  const match = FIELD_BINDING_EXPR_RE.exec(expression.trim())
  if (!match?.[1] || !match?.[2]) return null

  return {
    dataSourceId: match[1],
    fieldPath: match[2]
  }
}

export const getThingsVisFieldRoot = (fieldPath?: string) => {
  if (!fieldPath) return ''
  return fieldPath.split(/[.[\]]/).filter(Boolean)[0] || ''
}

const getEzuikitPlaybackDefaults = () => {
  const spaceId = String(import.meta.env.VITE_EZUIKIT_DEFAULT_SPACE_ID ?? '').trim()
  const busType = String(import.meta.env.VITE_EZUIKIT_DEFAULT_BUS_TYPE ?? '').trim()
  const compatDefaultsEnabled = import.meta.env.VITE_ENABLE_EZUIKIT_COMPAT_DEFAULTS === 'Y'

  return {
    spaceId: spaceId || (compatDefaultsEnabled ? EZUIKIT_COMPAT_DEFAULT_SPACE_ID : ''),
    busType: busType || (compatDefaultsEnabled ? EZUIKIT_COMPAT_DEFAULT_BUS_TYPE : '')
  }
}

const isPlatformFieldDataSource = (dataSource: any) => {
  const type = typeof dataSource?.type === 'string' ? dataSource.type.toUpperCase() : ''
  return type === 'PLATFORM_FIELD' || type === 'PLATFORM'
}

const normalizeViewerPlatformDataSources = (
  config: any,
  options: NormalizeThingsVisWidgetLoadConfigOptions
) => {
  if (!config || options.mode !== 'viewer' || !Array.isArray(config.dataSources)) return config

  const previewDeviceId = options.previewDeviceId
  if (!previewDeviceId) return config

  return {
    ...config,
    dataSources: config.dataSources.map((dataSource: any) => {
      if (!isPlatformFieldDataSource(dataSource)) return dataSource
      if (dataSource?.config?.deviceId) return dataSource

      return {
        ...dataSource,
        config: {
          ...(dataSource.config || {}),
          deviceId: previewDeviceId
        }
      }
    })
  }
}

const resolveInteractiveWriteBinding = (
  node: any,
  options: NormalizeThingsVisWidgetLoadConfigOptions
) => {
  const bindings = Array.isArray(node?.data) ? node.data : []
  const valueBinding = bindings.find((binding: any) => binding?.targetProp === 'value')
  const parsed =
    parseThingsVisFieldBindingExpression(valueBinding?.expression) ||
    parseThingsVisFieldBindingExpression(node?.props?.value)
  if (!parsed?.dataSourceId || !parsed.fieldPath) return null

  const fieldId = getThingsVisFieldRoot(parsed.fieldPath)
  if (!fieldId) return null

  return {
    dataSourceId: parsed.dataSourceId,
    fieldId,
    fieldValueType: options.fieldValueTypes[fieldId]
  }
}

const buildInteractiveAutoWriteAction = (dataSourceId: string, fieldId: string, fieldValueType?: string) => ({
  type: 'callWrite',
  dataSourceId,
  payload: `({ ${JSON.stringify(fieldId)}: ${fieldValueType === 'number' ? 'payload ? 1 : 0' : 'payload'} })`,
  __thingsvisAutoWrite: AUTO_WRITE_MARKER,
  ...(fieldValueType === 'number' || fieldValueType === 'boolean'
    ? { __thingsvisAutoWriteValueType: fieldValueType }
    : {})
})

const upsertNodeEventActions = (events: any[], eventName: string, resolveActions: (actions: any[]) => any[] | null) => {
  const nextEvents = [...events]
  const index = nextEvents.findIndex((handler) => handler?.event === eventName)

  if (index >= 0) {
    const existing = nextEvents[index]
    const actions = Array.isArray(existing?.actions) ? existing.actions : []
    const nextActions = resolveActions(actions)
    if (nextActions) {
      nextEvents[index] = {
        ...existing,
        actions: nextActions
      }
    }
    return nextEvents
  }

  const nextActions = resolveActions([])
  if (nextActions) {
    nextEvents.push({
      event: eventName,
      actions: nextActions
    })
  }

  return nextEvents
}

const upsertInteractiveAutoWriteEvent = (
  events: any[],
  eventName: string,
  autoAction: ReturnType<typeof buildInteractiveAutoWriteAction>
) =>
  upsertNodeEventActions(events, eventName, (actions: any[]) => {
    const manualActions = actions.filter((action: any) => action?.__thingsvisAutoWrite !== AUTO_WRITE_MARKER)
    return [...manualActions, autoAction]
  })

const normalizeInteractiveWriteNode = (
  node: any,
  options: NormalizeThingsVisWidgetLoadConfigOptions
) => {
  const eventName = DEFAULT_WRITE_EVENT_BY_COMPONENT[node?.type]
  if (!eventName) return node

  const binding = resolveInteractiveWriteBinding(node, options)
  if (!binding) return node

  const events = Array.isArray(node?.events) ? node.events : []
  const autoAction = buildInteractiveAutoWriteAction(binding.dataSourceId, binding.fieldId, binding.fieldValueType)

  return {
    ...node,
    events: upsertInteractiveAutoWriteEvent(events, eventName, autoAction)
  }
}

const ensureInteractiveWriteEvents = (config: any, options: NormalizeThingsVisWidgetLoadConfigOptions) => {
  if (!config || !Array.isArray(config.nodes)) return config

  return {
    ...config,
    nodes: config.nodes.map((node: any) => normalizeInteractiveWriteNode(node, options))
  }
}

const resolvePlatformDataSourceId = (config: any): string | undefined => {
  const dataSources = Array.isArray(config?.dataSources) ? config.dataSources : []
  const platformDs = dataSources.find((dataSource: any) => isPlatformFieldDataSource(dataSource))
  return typeof platformDs?.id === 'string' ? platformDs.id : undefined
}

const buildEzuikitWriteAction = (dataSourceId: string, payload: string) => ({
  type: 'callWrite',
  dataSourceId,
  payload
})

const upsertEzuikitWriteEvent = (
  events: any[],
  eventName: string,
  action: ReturnType<typeof buildEzuikitWriteAction>
) =>
  upsertNodeEventActions(events, eventName, (actions: any[]) => {
    if (actions.length === 0) return [action]
    return null
  })

const ensureEzuikitNodeEvents = (node: any, dataSourceId: string) => {
  const events = Array.isArray(node?.events) ? node.events : []
  const playbackAction = buildEzuikitWriteAction(
    dataSourceId,
    `({ ${JSON.stringify(EZUIKIT_PLAYBACK_COMMAND)}: payload })`
  )
  const liveAction = buildEzuikitWriteAction(
    dataSourceId,
    `({ ${JSON.stringify(EZUIKIT_PLAYBACK_COMMAND)}: { type: "live" } })`
  )

  return upsertEzuikitWriteEvent(
    upsertEzuikitWriteEvent(events, 'playbackRequest', playbackAction),
    'liveRequest',
    liveAction
  )
}

const normalizeEzuikitNodeProps = (nodeProps: any) => {
  const nextProps = { ...(nodeProps || {}) }
  delete nextProps.ezopenUrl
  delete nextProps.playbackBegin
  delete nextProps.playbackEnd
  delete nextProps.streamSuffix
  delete nextProps.playbackParamsUrl

  const playbackDefaults = getEzuikitPlaybackDefaults()
  if (!String(nextProps.spaceId ?? '').trim() && playbackDefaults.spaceId) {
    nextProps.spaceId = playbackDefaults.spaceId
  }
  if (!String(nextProps.busType ?? '').trim() && playbackDefaults.busType) {
    nextProps.busType = playbackDefaults.busType
  }

  return nextProps
}

const buildEzuikitBinding = (dataSourceId: string, targetProp: string, fieldId: string) => ({
  targetProp,
  expression: `{{ ds.${dataSourceId}.data.${fieldId} }}`
})

const ensureEzuikitBinding = (bindings: any[], dataSourceId: string, targetProp: string, fieldId: string) => {
  if (bindings.some((binding: any) => binding?.targetProp === targetProp)) return bindings
  return [...bindings, buildEzuikitBinding(dataSourceId, targetProp, fieldId)]
}

const normalizeEzuikitNodeDataBindings = (node: any, dataSourceId: string) => {
  const data = (Array.isArray(node?.data) ? node.data : []).filter(
    (binding: any) => !['ezopenUrl', 'playbackParamsUrl', 'spaceId', 'busType'].includes(binding?.targetProp)
  )
  const defaultBindings: Array<[string, string]> = [
    ['accessToken', 'ys7_playback_access_token'],
    ['deviceSerial', 'ys7_device_serial'],
    ['channelNo', 'ys7_channel_no']
  ]

  return defaultBindings.reduce((bindings, [targetProp, fieldId]) => {
    return ensureEzuikitBinding(bindings, dataSourceId, targetProp, fieldId)
  }, data)
}

const normalizeEzuikitPlaybackNode = (node: any, dataSourceId: string) => {
  if (node?.type !== 'media/ezuikit-player') return node

  return {
    ...node,
    props: normalizeEzuikitNodeProps(node.props),
    data: normalizeEzuikitNodeDataBindings(node, dataSourceId),
    events: ensureEzuikitNodeEvents(node, dataSourceId)
  }
}

const ensureEzuikitPlaybackEvents = (config: any) => {
  if (!config || !Array.isArray(config.nodes)) return config

  const dataSourceId = resolvePlatformDataSourceId(config)
  if (!dataSourceId) return config

  return {
    ...config,
    nodes: config.nodes.map((node: any) => normalizeEzuikitPlaybackNode(node, dataSourceId))
  }
}

const chartFontSizeDefaultsForType = (nodeType: unknown): ChartFontSizeConfig | null => {
  if (typeof nodeType !== 'string') return null
  if (CARTESIAN_CHART_TYPES.has(nodeType)) return DEFAULT_CARTESIAN_CHART_FONT_SIZES
  if (PIE_CHART_TYPES.has(nodeType)) return DEFAULT_PIE_CHART_FONT_SIZES
  if (GAUGE_CHART_TYPES.has(nodeType)) return DEFAULT_GAUGE_CHART_FONT_SIZES
  return null
}

const normalizeChartFontSizeNode = (node: any) => {
  const defaults = chartFontSizeDefaultsForType(node?.type)
  if (!defaults) return node

  const props = node?.props && typeof node.props === 'object' ? node.props : {}
  const existingFontSizes =
    props.fontSizes && typeof props.fontSizes === 'object' && !Array.isArray(props.fontSizes)
      ? props.fontSizes
      : {}

  return {
    ...node,
    props: {
      ...props,
      fontSizes: {
        ...defaults,
        ...existingFontSizes
      }
    }
  }
}

const normalizeModel3DNode = (node: any) => {
  if (typeof node?.type !== 'string' || !MODEL_3D_TYPES.has(node.type)) return node

  const props = node?.props && typeof node.props === 'object' ? node.props : {}
  const acceptedExtensions = Array.isArray(props.acceptedExtensions)
    ? props.acceptedExtensions
    : DEFAULT_MODEL_3D_PROPS.acceptedExtensions
  const maxUploadSizeMb =
    typeof props.maxUploadSizeMb === 'number' && props.maxUploadSizeMb > 0
      ? props.maxUploadSizeMb
      : DEFAULT_MODEL_3D_PROPS.maxUploadSizeMb

  return {
    ...node,
    props: {
      ...DEFAULT_MODEL_3D_PROPS,
      ...props,
      acceptedExtensions,
      maxUploadSizeMb
    }
  }
}

const normalizeThingsVisRuntimeDefaultNode = (node: any) => normalizeModel3DNode(normalizeChartFontSizeNode(node))

const ensureChartFontSizeDefaults = (config: any) => {
  if (!config || !Array.isArray(config.nodes)) return config

  return {
    ...config,
    nodes: config.nodes.map((node: any) => normalizeThingsVisRuntimeDefaultNode(node))
  }
}

const hasPositionedNodeBounds = (node: any) => {
  return [node?.x, node?.y, node?.width, node?.height].every((value) => typeof value === 'number')
}

const getPositionedNodeBounds = (nodes: any[]) => {
  const positionedNodes = nodes.filter(hasPositionedNodeBounds)
  if (positionedNodes.length === 0) return null

  const minX = Math.min(...positionedNodes.map((node: any) => node.x))
  const minY = Math.min(...positionedNodes.map((node: any) => node.y))
  const maxX = Math.max(...positionedNodes.map((node: any) => node.x + node.width))
  const maxY = Math.max(...positionedNodes.map((node: any) => node.y + node.height))

  return {
    minX,
    minY,
    width: Math.ceil(maxX - minX + VIEWER_INFINITE_CANVAS_PADDING * 2),
    height: Math.ceil(maxY - minY + VIEWER_INFINITE_CANVAS_PADDING * 2)
  }
}

const offsetPositionedNodes = (nodes: any[], offsetX: number, offsetY: number) => {
  return nodes.map((node: any) => {
    if ([node?.x, node?.y].every((value) => typeof value === 'number')) {
      return {
        ...node,
        x: node.x + offsetX,
        y: node.y + offsetY
      }
    }
    return node
  })
}

const normalizeInfiniteCanvasConfig = (config: any) => {
  const canvas = config.canvas || config.canvasConfig
  if (!canvas || canvas.mode !== 'infinite') return config

  const nodes = Array.isArray(config.nodes) ? config.nodes : []
  const bounds = getPositionedNodeBounds(nodes)

  if (!bounds) {
    return {
      ...config,
      canvas: {
        ...canvas,
        scaleMode: canvas.scaleMode || 'fit-min'
      }
    }
  }

  const offsetX = VIEWER_INFINITE_CANVAS_PADDING - bounds.minX
  const offsetY = VIEWER_INFINITE_CANVAS_PADDING - bounds.minY

  return {
    ...config,
    canvas: {
      ...canvas,
      width: Math.max(canvas.width || 0, bounds.width),
      height: Math.max(canvas.height || 0, bounds.height),
      scaleMode: canvas.scaleMode || 'fit-min'
    },
    nodes: offsetPositionedNodes(nodes, offsetX, offsetY)
  }
}

function normalizeViewerConfig(config: any, options: NormalizeThingsVisWidgetLoadConfigOptions) {
  if (!config || options.mode !== 'viewer') return config

  const platformNormalizedConfig = normalizeViewerPlatformDataSources(
    ensureChartFontSizeDefaults(ensureEzuikitPlaybackEvents(ensureInteractiveWriteEvents(config, options))),
    options
  )

  return normalizeInfiniteCanvasConfig(platformNormalizedConfig)
}

export function normalizeThingsVisWidgetLoadConfig(
  config: any,
  options: NormalizeThingsVisWidgetLoadConfigOptions
) {
  const writeNormalizedConfig = ensureChartFontSizeDefaults(
    ensureInteractiveWriteEvents(ensureEzuikitPlaybackEvents(config), options)
  )
  return normalizeViewerConfig(writeNormalizedConfig, options)
}
