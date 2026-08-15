/**
 * 文件说明：
 * - 封装 ThingsVis AppFrame 的平台设备目录编排逻辑，包括设备配置到物模型映射、分组缓存、按组加载、分页搜索和字段加载。
 * - 对 AppFrame 暴露小接口，让 iframe 消息处理只关心“加载什么”，不再关心平台 API、物模型资产和缓存细节。
 * 维护提示：
 * - 设备 ID、分组 ID、deviceConfigId、templateId 和分页响应字段属于 ThingsVis guest 侧可见契约，改动要保持兼容。
 * - 后续如果继续拆目录逻辑，优先在本模块内部拆私有 helper，不要把缓存和 API 装配重新散回 AppFrame。
 * 审查建议：
 * - 当前模块仍依赖真实平台 API 适配器；若要补测试，可从 createThingsVisPlatformDeviceCatalogOrchestrator 的依赖注入接口构造内存 adapter。
 */
import {
  asRecord,
  firstString,
  flattenDeviceGroupTree,
  loadDeviceFilterOptions,
  loadPlatformDeviceGroups,
  normalizeEditorGroupId,
  normalizeEditorGroupName,
  type DeviceFilterOption,
  type PlatformDeviceGroupEntry
} from '@/components/thingsvis/thingsvisDeviceCatalogBridge'
import { findPlatformDeviceById, loadPlatformDevicesByGroup } from '@/components/thingsvis/thingsvisDeviceLookupBridge'
import { attachPlatformDeviceTemplateAssets } from '@/components/thingsvis/thingsvisDeviceTemplateBridge'
import { createThingsVisDeviceConfigTemplateMapCache } from '@/components/thingsvis/thingsvisDeviceConfigTemplateMapCacheBridge'
import { createThingsVisTemplateAssetCacheBridge } from '@/components/thingsvis/thingsvisTemplateAssetCacheBridge'
import {
  normalizePlatformDeviceRow,
  type NormalizedPlatformDeviceRow,
  type PlatformDeviceRowContext
} from '@/components/thingsvis/thingsvisDeviceRowBridge'
import {
  buildSearchDevicesPagedParams,
  normalizeSearchDevicesPagedPayload,
  resolveSearchDevicesGroupContext,
  type SearchDevicesPagedRequest
} from '@/components/thingsvis/searchDevicesPagedBridge'
import type { PlatformDeviceField } from '@/components/thingsvis/thingsvisDeviceWsBridge'

type CatalogLogger = {
  error: (...args: any[]) => void
}

type DeviceListParams = Record<string, unknown>

type PlatformDeviceCatalogDependencies = {
  loadDeviceConfigs: (params: DeviceListParams) => Promise<any>
  loadServiceCatalog: (params: DeviceListParams) => Promise<any>
  loadGroupTree: (params: DeviceListParams) => Promise<any>
  listDevices: (params: DeviceListParams) => Promise<any>
  listDevicesByGroup: (params: DeviceListParams) => Promise<any>
  loadTemplate: (templateId: string | number) => Promise<any>
  loadTelemetry: (params: DeviceListParams) => Promise<any>
  loadAttributes: (params: DeviceListParams) => Promise<any>
  loadCommands: (params: DeviceListParams) => Promise<any>
  loadEvents: (params: DeviceListParams) => Promise<any>
}

type PlatformDeviceCatalogLabels = {
  allGroups: string
  protocolPrefix: string
  servicePrefix: string
}

type PlatformDeviceCatalogPageSizes = {
  templateField: number
  deviceConfig: number
  groupDevice: number
}

export type PlatformDeviceEntry = {
  deviceId: string
  deviceName: string
  groupId: string
  groupName: string
  deviceConfigId?: string
  deviceConfigName?: string
  isOnline?: number | string | boolean
  warnStatus?: string
  deviceType?: string
  accessWay?: string
  protocolType?: string
  lastPushTime?: string
  templateId?: string
  fields: PlatformDeviceField[]
  presets: any[]
}

export type SearchDevicesPagedResponse = {
  devices: PlatformDeviceEntry[]
  total: unknown
}

export type DeviceFieldsResponse = {
  deviceId: string
  templateId: string
  fields: PlatformDeviceField[]
}

export type PlatformDeviceCatalogOrchestrator = {
  loadGroups: () => Promise<PlatformDeviceGroupEntry[]>
  loadFilterOptions: () => Promise<{ deviceConfigs: DeviceFilterOption[]; serviceOptions: DeviceFilterOption[] }>
  loadDeviceById: (deviceId: string) => Promise<PlatformDeviceEntry | null>
  loadDevicesByGroup: (groupId: string) => Promise<PlatformDeviceEntry[]>
  searchDevicesPaged: (request: SearchDevicesPagedRequest) => Promise<SearchDevicesPagedResponse>
  normalizeSearchPayload: (payload: Record<string, unknown>) => SearchDevicesPagedRequest
  loadDeviceFields: (params: {
    deviceId: string
    templateId?: string
    deviceConfigId?: string
  }) => Promise<DeviceFieldsResponse>
  reset: () => void
}

export function createThingsVisPlatformDeviceCatalogOrchestrator(options: {
  dependencies: PlatformDeviceCatalogDependencies
  labels: PlatformDeviceCatalogLabels
  pageSizes: PlatformDeviceCatalogPageSizes
  getLanguage: () => string | undefined
  logger: CatalogLogger
}): PlatformDeviceCatalogOrchestrator {
  let platformDeviceGroupsCache: PlatformDeviceGroupEntry[] | null = null
  let platformDeviceGroupsCachePromise: Promise<PlatformDeviceGroupEntry[]> | null = null
  const platformDevicesByGroupCache = new Map<string, PlatformDeviceEntry[]>()
  const platformDevicesByGroupPromise = new Map<string, Promise<PlatformDeviceEntry[]>>()

  function unwrapList(payload: any): any[] {
    if (Array.isArray(payload?.list)) return payload.list
    if (Array.isArray(payload)) return payload
    if (Array.isArray(payload?.data)) return payload.data
    return []
  }

  const templateAssetCache = createThingsVisTemplateAssetCacheBridge({
    dependencies: options.dependencies,
    templateFieldPageSize: options.pageSizes.templateField,
    unwrapList,
    logger: options.logger
  })

  const deviceConfigTemplateMapCache = createThingsVisDeviceConfigTemplateMapCache({
    loadDeviceConfigs: options.dependencies.loadDeviceConfigs,
    deviceConfigPageSize: options.pageSizes.deviceConfig,
    unwrapList,
    logger: options.logger
  })

  async function loadFilterOptions() {
    return loadDeviceFilterOptions({
      loadDeviceConfigs: () =>
        options.dependencies.loadDeviceConfigs({ page: 1, page_size: options.pageSizes.deviceConfig }),
      loadServiceCatalog: () => options.dependencies.loadServiceCatalog({ language_code: options.getLanguage() }),
      unwrapList,
      firstString,
      asRecord,
      protocolLabelPrefix: options.labels.protocolPrefix,
      serviceLabelPrefix: options.labels.servicePrefix
    })
  }

  async function loadGroups(): Promise<PlatformDeviceGroupEntry[]> {
    if (platformDeviceGroupsCache) {
      return platformDeviceGroupsCache
    }
    if (platformDeviceGroupsCachePromise) {
      return platformDeviceGroupsCachePromise
    }
    platformDeviceGroupsCachePromise = (async () => {
      const groups = await loadPlatformDeviceGroups({
        loadGroupTree: () => options.dependencies.loadGroupTree({}),
        flattenDeviceGroupTree,
        allGroupLabel: options.labels.allGroups,
        onError: (err) => {
          options.logger.error('[AppFrame] Failed to load platform device groups', err)
        }
      })
      platformDeviceGroupsCache = groups
      return groups
    })().finally(() => {
      platformDeviceGroupsCachePromise = null
    })
    return platformDeviceGroupsCachePromise
  }

  function getCachedGroupsForSearch(): PlatformDeviceGroupEntry[] {
    if (platformDeviceGroupsCache) {
      return platformDeviceGroupsCache
    }

    void loadGroups().catch((err) => {
      options.logger.error('[AppFrame] Failed to warm platform device groups for search', err)
    })
    return []
  }

  function buildPlatformDeviceEntry(device: NormalizedPlatformDeviceRow): PlatformDeviceEntry {
    return {
      ...device,
      fields: [],
      presets: []
    }
  }

  function mapPlatformDeviceRowForGroup(
    row: any,
    fallbackGroupId: string,
    fallbackGroupName: string,
    groupNameById: Map<string, string>,
    configTemplateMap: Map<string, string>
  ): PlatformDeviceEntry | null {
    const normalized = normalizePlatformDeviceRow(row, {
      fallbackGroupId,
      fallbackGroupName,
      groupNameById,
      configTemplateMap
    } satisfies PlatformDeviceRowContext)
    return normalized ? buildPlatformDeviceEntry(normalized) : null
  }

  async function buildPlatformDeviceTemplateAssets(devices: PlatformDeviceEntry[]) {
    return templateAssetCache.loadTemplateAssetsForDevices<PlatformDeviceField>(devices)
  }

  async function mapPlatformDevicesForGroup(
    rawDevices: any[],
    fallbackGroupId = '',
    fallbackGroupName = '',
    groups: PlatformDeviceGroupEntry[] = []
  ): Promise<PlatformDeviceEntry[]> {
    const groupNameById = new Map(groups.map((group) => [group.groupId, group.groupName]))
    const configTemplateMap = await deviceConfigTemplateMapCache.load()
    const devices = rawDevices
      .map((row: any) =>
        mapPlatformDeviceRowForGroup(row, fallbackGroupId, fallbackGroupName, groupNameById, configTemplateMap)
      )
      .filter((item): item is PlatformDeviceEntry => Boolean(item))

    const templateAssets = await buildPlatformDeviceTemplateAssets(devices)
    return devices.map((device) => attachPlatformDeviceTemplateAssets(device, templateAssets))
  }

  async function buildFallbackPlatformDevicesForDefaultGroup(
    normalizedGroupId: string,
    groupName: string,
    groups: PlatformDeviceGroupEntry[]
  ): Promise<PlatformDeviceEntry[]> {
    const rootGroups = groups.filter((group) => !group.parentId || String(group.parentId) === '0')
    if (rootGroups.length !== 1 || rootGroups[0]?.groupId !== normalizedGroupId) {
      return []
    }

    const deviceRes = await options.dependencies.listDevices({ page: 1, page_size: options.pageSizes.groupDevice })
    const rawDevices = unwrapList(deviceRes?.data)
    return mapPlatformDevicesForGroup(rawDevices, normalizedGroupId, groupName, groups)
  }

  async function loadDeviceById(deviceId: string): Promise<PlatformDeviceEntry | null> {
    return findPlatformDeviceById({
      deviceId,
      loadGroups,
      searchDevices: async (normalizedDeviceId) => {
        const res = await options.dependencies.listDevices({ page: 1, page_size: 20, search: normalizedDeviceId })
        return unwrapList(res?.data)
      },
      listDevices: async () => {
        const res = await options.dependencies.listDevices({ page: 1, page_size: options.pageSizes.groupDevice })
        return unwrapList(res?.data)
      },
      mapDevicesForGroup: mapPlatformDevicesForGroup,
      loadDevicesByGroup
    })
  }

  async function loadDevicesByGroup(groupId: string): Promise<PlatformDeviceEntry[]> {
    const normalizedGroupId = normalizeEditorGroupId(groupId)

    const cached = platformDevicesByGroupCache.get(normalizedGroupId)
    if (cached) {
      return cached
    }

    const pending = platformDevicesByGroupPromise.get(normalizedGroupId)
    if (pending) {
      return pending
    }

    const promise = (async () => {
      try {
        const devices = await loadPlatformDevicesByGroup({
          groupId: normalizedGroupId,
          loadGroups,
          loadRelatedDevices: async (nextGroupId) => {
            const res = await options.dependencies.listDevicesByGroup({
              group_id: nextGroupId,
              page: 1,
              page_size: options.pageSizes.groupDevice
            })
            return unwrapList(res?.data)
          },
          mapDevicesForGroup: mapPlatformDevicesForGroup,
          buildFallbackDevices: buildFallbackPlatformDevicesForDefaultGroup,
          normalizeGroupId: normalizeEditorGroupId,
          normalizeGroupName: normalizeEditorGroupName,
          onError: (failedGroupId, err) => {
            options.logger.error('[AppFrame] Failed to assemble platformDevices for group', failedGroupId, err)
          }
        })
        platformDevicesByGroupCache.set(normalizedGroupId, devices)
        return devices
      } finally {
        platformDevicesByGroupPromise.delete(normalizedGroupId)
      }
    })()

    platformDevicesByGroupPromise.set(normalizedGroupId, promise)
    return promise
  }

  async function searchDevicesPaged(request: SearchDevicesPagedRequest): Promise<SearchDevicesPagedResponse> {
    const res = await options.dependencies.listDevices(buildSearchDevicesPagedParams(request))
    const groups = getCachedGroupsForSearch()
    const { groupId, fallbackGroupName } = resolveSearchDevicesGroupContext(request, groups)
    const devices = await mapPlatformDevicesForGroup(unwrapList(res?.data), groupId, fallbackGroupName, groups)

    return {
      devices,
      total: res?.data?.total
    }
  }

  async function loadDeviceFields(params: {
    deviceId: string
    templateId?: string
    deviceConfigId?: string
  }): Promise<DeviceFieldsResponse> {
    let templateId = params.templateId
    if (!templateId && params.deviceConfigId) {
      templateId = (await deviceConfigTemplateMapCache.load()).get(params.deviceConfigId)
    }
    if (!templateId) {
      return {
        deviceId: params.deviceId,
        templateId: '',
        fields: []
      }
    }

    const entry = await templateAssetCache.loadTemplateEntry(templateId)
    if (entry.loadError) {
      throw entry.loadError
    }
    return {
      deviceId: params.deviceId,
      templateId,
      fields: Array.isArray(entry.fields) ? (entry.fields as PlatformDeviceField[]) : []
    }
  }

  function reset() {
    templateAssetCache.reset()
    platformDeviceGroupsCache = null
    platformDeviceGroupsCachePromise = null
    deviceConfigTemplateMapCache.reset()
    platformDevicesByGroupCache.clear()
    platformDevicesByGroupPromise.clear()
  }

  return {
    loadGroups,
    loadFilterOptions,
    loadDeviceById,
    loadDevicesByGroup,
    searchDevicesPaged,
    normalizeSearchPayload: normalizeSearchDevicesPagedPayload,
    loadDeviceFields,
    reset
  }
}
