import {
  fetchTemplateEntry,
  fetchTemplatePresets,
  loadPlatformDeviceTemplateAssets,
  type TemplateEntry
} from '@/components/thingsvis/thingsvisDeviceTemplateBridge'

type DeviceListParams = Record<string, unknown>

type TemplateAssetCacheDependencies = {
  loadTemplate: (templateId: string | number) => Promise<any>
  loadTelemetry: (params: DeviceListParams) => Promise<any>
  loadAttributes: (params: DeviceListParams) => Promise<any>
  loadCommands: (params: DeviceListParams) => Promise<any>
  loadEvents: (params: DeviceListParams) => Promise<any>
}

type TemplateAssetCacheLogger = {
  error: (...args: any[]) => void
}

export type ThingsVisTemplateAssetCacheBridgeOptions = {
  dependencies: TemplateAssetCacheDependencies
  templateFieldPageSize: number
  unwrapList: (payload: any) => any[]
  logger: TemplateAssetCacheLogger
}

export function createThingsVisTemplateAssetCacheBridge(options: ThingsVisTemplateAssetCacheBridgeOptions) {
  const templateEntryCache = new Map<string, TemplateEntry>()
  const templateEntryPromise = new Map<string, Promise<TemplateEntry>>()
  const templatePresetCache = new Map<string, any[]>()
  const templatePresetPromise = new Map<string, Promise<any[]>>()

  async function loadTemplatePresets(templateId: string | number): Promise<any[]> {
    const cacheKey = String(templateId)
    if (templatePresetCache.has(cacheKey)) {
      return templatePresetCache.get(cacheKey) || []
    }
    if (templatePresetPromise.has(cacheKey)) {
      return templatePresetPromise.get(cacheKey) as Promise<any[]>
    }

    const promise = (async () => {
      const presets = await fetchTemplatePresets({
        templateId,
        loadTemplate: options.dependencies.loadTemplate,
        onError: (failedTemplateId, error) => {
          options.logger.error('[AppFrame] Failed to load thing-model presets', failedTemplateId, error)
        }
      })
      templatePresetCache.set(cacheKey, presets)
      return presets
    })().finally(() => {
      templatePresetPromise.delete(cacheKey)
    })

    templatePresetPromise.set(cacheKey, promise)
    return promise
  }

  async function loadTemplateEntry(templateId: string | number): Promise<TemplateEntry> {
    const cacheKey = String(templateId)
    if (templateEntryCache.has(cacheKey)) {
      return templateEntryCache.get(cacheKey) as TemplateEntry
    }
    if (templateEntryPromise.has(cacheKey)) {
      return templateEntryPromise.get(cacheKey) as Promise<TemplateEntry>
    }

    const promise = (async () => {
      const entry = await fetchTemplateEntry({
        templateId,
        pageSize: options.templateFieldPageSize,
        loadTelemetry: options.dependencies.loadTelemetry,
        loadAttributes: options.dependencies.loadAttributes,
        loadCommands: options.dependencies.loadCommands,
        loadEvents: options.dependencies.loadEvents,
        unwrapList: options.unwrapList
      })
      if (!entry.loadError) {
        templateEntryCache.set(cacheKey, entry)
      }
      return entry
    })().finally(() => {
      templateEntryPromise.delete(cacheKey)
    })

    templateEntryPromise.set(cacheKey, promise)
    return promise
  }

  async function loadTemplateAssetsForDevices<TField>(devices: Array<{ templateId?: string }>) {
    return loadPlatformDeviceTemplateAssets<TField>({
      devices,
      loadTemplatePresets: async (templateId) => loadTemplatePresets(templateId),
      loadTemplateEntry: async (templateId) => {
        const entry = await loadTemplateEntry(templateId)
        return {
          fields: Array.isArray(entry.fields) ? (entry.fields as TField[]) : []
        }
      }
    })
  }

  function reset() {
    templateEntryCache.clear()
    templateEntryPromise.clear()
    templatePresetCache.clear()
    templatePresetPromise.clear()
  }

  return {
    loadTemplateEntry,
    loadTemplatePresets,
    loadTemplateAssetsForDevices,
    reset
  }
}
