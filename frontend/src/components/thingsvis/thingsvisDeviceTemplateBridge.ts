import { extractPlatformFields } from '@/utils/thingsvis/platform-fields'
import type { PlatformField } from '@/utils/thingsvis/types'

export type TemplateEntry = {
  fields: PlatformField[]
  loadError?: unknown
}

type FetchTemplateEntryOptions = {
  templateId: string | number
  pageSize: number
  loadTelemetry: (params: any) => Promise<any>
  loadAttributes: (params: any) => Promise<any>
  loadCommands: (params: any) => Promise<any>
  loadEvents: (params: any) => Promise<any>
  unwrapList: (payload: any) => any[]
}

type FetchTemplatePresetsOptions = {
  templateId: string | number
  loadTemplate: (templateId: string | number) => Promise<any>
  onError?: (templateId: string | number, error: unknown) => void
}

type DeviceWithTemplateAssets<TField = PlatformField> = {
  templateId?: string
  fields: TField[]
  presets: any[]
}

const TEMPLATE_ASSET_LOAD_CONCURRENCY = 3

async function mapWithConcurrency<T>(
  items: T[],
  concurrency: number,
  mapper: (item: T) => Promise<void>
) {
  let nextIndex = 0
  const workerCount = Math.min(Math.max(concurrency, 1), items.length)

  await Promise.all(
    Array.from({ length: workerCount }, async () => {
      while (nextIndex < items.length) {
        const currentIndex = nextIndex
        nextIndex += 1
        await mapper(items[currentIndex])
      }
    })
  )
}

function parseTemplateChartConfig(rawConfig: unknown): Record<string, unknown> | null {
  if (typeof rawConfig === 'string') {
    if (!rawConfig.trim()) return null
    try {
      const parsed = JSON.parse(rawConfig)
      return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? (parsed as Record<string, unknown>) : null
    } catch {
      return null
    }
  }

  if (rawConfig && typeof rawConfig === 'object' && !Array.isArray(rawConfig)) {
    return rawConfig as Record<string, unknown>
  }

  return null
}

function resolveNodePresetName(node: Record<string, unknown>, index: number) {
  const props =
    node.props && typeof node.props === 'object' && !Array.isArray(node.props)
      ? (node.props as Record<string, unknown>)
      : {}

  return String(
    node.name || props.title || props.label || props.text || props.placeholder || node.type || `挂件 ${index + 1}`
  )
}

export function buildDeviceWidgetPresets(templateId: string, rawConfig: unknown): any[] {
  const config = parseTemplateChartConfig(rawConfig)
  const nodes = Array.isArray(config?.nodes)
    ? config.nodes.filter(
        (node): node is Record<string, unknown> => Boolean(node) && typeof node === 'object' && !Array.isArray(node)
      )
    : config?.nodesById && typeof config.nodesById === 'object' && !Array.isArray(config.nodesById)
      ? Object.values(config.nodesById).filter(
          (node): node is Record<string, unknown> => Boolean(node) && typeof node === 'object' && !Array.isArray(node)
        )
      : []

  const nodePresets = nodes.map((node, index) => ({
    id: `${templateId}-web-node-${String(node.id || index)}`,
    name: resolveNodePresetName(node, index),
    widget: node,
    ...(typeof node.thumbnail === 'string' ? { thumbnail: node.thumbnail } : {})
  }))

  const presetMap = config?.device_widget_presets
  if (!presetMap || typeof presetMap !== 'object' || Array.isArray(presetMap)) return nodePresets

  const storedPresets = Object.entries(presetMap).flatMap(([presetKey, entries]) => {
    if (!Array.isArray(entries)) return []

    return entries.flatMap((entry: any, index) => {
      if (!entry?.widget || typeof entry.widget !== 'object' || Array.isArray(entry.widget)) return []

      return [
        {
          id: `${templateId}-stored-${String(entry.id || `${presetKey}-${index}`)}`,
          name: String(entry.name || '挂件预设'),
          widget: entry.widget,
          ...(typeof entry.thumbnail === 'string' ? { thumbnail: entry.thumbnail } : {})
        }
      ]
    })
  })

  return [...nodePresets, ...storedPresets]
}

export async function fetchTemplatePresets(options: FetchTemplatePresetsOptions): Promise<any[]> {
  try {
    const response = await options.loadTemplate(options.templateId)
    const template = response?.data || {}
    return buildDeviceWidgetPresets(String(options.templateId), template?.web_chart_config)
  } catch (error) {
    options.onError?.(options.templateId, error)
    return []
  }
}

export async function fetchTemplateEntry(options: FetchTemplateEntryOptions): Promise<TemplateEntry> {
  const [telemetryResult, attributesResult, commandsResult, eventsResult] = await Promise.allSettled([
    options.loadTelemetry({ page: 1, page_size: options.pageSize, device_template_id: options.templateId }),
    options.loadAttributes({ page: 1, page_size: options.pageSize, device_template_id: options.templateId }),
    options.loadCommands({ page: 1, page_size: options.pageSize, device_template_id: options.templateId }),
    options.loadEvents({ page: 1, page_size: options.pageSize, device_template_id: options.templateId })
  ])

  const telemetryRes = telemetryResult.status === 'fulfilled' ? telemetryResult.value : null
  const attributesRes = attributesResult.status === 'fulfilled' ? attributesResult.value : null
  const commandsRes = commandsResult.status === 'fulfilled' ? commandsResult.value : null
  const eventsRes = eventsResult.status === 'fulfilled' ? eventsResult.value : null
  const firstLoadError =
    telemetryResult.status === 'rejected'
      ? telemetryResult.reason
      : attributesResult.status === 'rejected'
        ? attributesResult.reason
        : commandsResult.status === 'rejected'
          ? commandsResult.reason
          : eventsResult.status === 'rejected'
            ? eventsResult.reason
            : undefined

  return {
    fields: extractPlatformFields({
      telemetry: options.unwrapList(telemetryRes?.data),
      attributes: options.unwrapList(attributesRes?.data),
      commands: options.unwrapList(commandsRes?.data),
      events: options.unwrapList(eventsRes?.data)
    }),
    ...(firstLoadError ? { loadError: firstLoadError } : {})
  }
}

export async function loadPlatformDeviceTemplateAssets<TField = PlatformField>(options: {
  devices: Array<{ templateId?: string }>
  loadTemplatePresets: (templateId: string) => Promise<any[]>
  loadTemplateEntry: (templateId: string) => Promise<{ fields: TField[] }>
}): Promise<{ fieldsByTemplateId: Map<string, TField[]>; presetsByTemplateId: Map<string, any[]> }> {
  const templateIds = Array.from(
    new Set(
      options.devices
        .map((device) => device.templateId)
        .filter((templateId): templateId is string => Boolean(templateId))
    )
  )
  const presetsByTemplateId = new Map<string, any[]>()
  const fieldsByTemplateId = new Map<string, TField[]>()

  await mapWithConcurrency(templateIds, TEMPLATE_ASSET_LOAD_CONCURRENCY, async (templateId) => {
    const [presets, entry] = await Promise.all([
      options.loadTemplatePresets(templateId),
      options.loadTemplateEntry(templateId)
    ])
    presetsByTemplateId.set(templateId, Array.isArray(presets) ? presets : [])
    fieldsByTemplateId.set(templateId, Array.isArray(entry.fields) ? entry.fields : [])
  })

  return { fieldsByTemplateId, presetsByTemplateId }
}

export function attachPlatformDeviceTemplateAssets<
  TDevice extends DeviceWithTemplateAssets<TField>,
  TField = PlatformField
>(
  device: TDevice,
  assets: { fieldsByTemplateId: Map<string, TField[]>; presetsByTemplateId: Map<string, any[]> }
): TDevice {
  return {
    ...device,
    fields: device.templateId ? assets.fieldsByTemplateId.get(device.templateId) || device.fields : device.fields,
    presets: device.templateId ? assets.presetsByTemplateId.get(device.templateId) || [] : []
  }
}
