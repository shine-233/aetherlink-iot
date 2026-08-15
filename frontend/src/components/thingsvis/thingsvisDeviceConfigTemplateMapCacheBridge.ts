import { firstString } from '@/components/thingsvis/thingsvisDeviceCatalogBridge'

type DeviceListParams = Record<string, unknown>

export type ThingsVisDeviceConfigTemplateMapCacheOptions = {
  loadDeviceConfigs: (params: DeviceListParams) => Promise<any>
  deviceConfigPageSize: number
  unwrapList: (payload: any) => any[]
  logger: { error: (...args: any[]) => void }
}

export function createThingsVisDeviceConfigTemplateMapCache(
  options: ThingsVisDeviceConfigTemplateMapCacheOptions
) {
  let deviceConfigTemplateMapCache: Map<string, string> | null = null
  let deviceConfigTemplateMapPromise: Promise<Map<string, string>> | null = null

  async function load(): Promise<Map<string, string>> {
    if (deviceConfigTemplateMapCache) {
      return deviceConfigTemplateMapCache
    }

    if (deviceConfigTemplateMapPromise) {
      return deviceConfigTemplateMapPromise
    }

    deviceConfigTemplateMapPromise = (async () => {
      try {
        const confRes = await options.loadDeviceConfigs({
          page: 1,
          page_size: options.deviceConfigPageSize
        })
        const configs = options.unwrapList(confRes?.data)
        const configTemplateMap = new Map<string, string>()

        configs.forEach((config: any) => {
          const configId = firstString(config?.id, config?.device_config_id, config?.deviceConfigId)
          const configName = firstString(config?.name, config?.device_config_name, config?.deviceConfigName)
          const templateId = firstString(
            config?.device_template_id,
            config?.deviceTemplateId,
            config?.template_id,
            config?.templateId
          )
          if (configId && templateId) {
            configTemplateMap.set(configId, templateId)
          }
          if (configName && templateId) {
            configTemplateMap.set(configName, templateId)
          }
        })

        deviceConfigTemplateMapCache = configTemplateMap
        return configTemplateMap
      } catch (err) {
        options.logger.error('[AppFrame] Failed to load device config thing-model map', err)
        deviceConfigTemplateMapCache = new Map()
        return deviceConfigTemplateMapCache
      } finally {
        deviceConfigTemplateMapPromise = null
      }
    })()

    return deviceConfigTemplateMapPromise
  }

  function reset() {
    deviceConfigTemplateMapCache = null
    deviceConfigTemplateMapPromise = null
  }

  return {
    load,
    reset
  }
}
