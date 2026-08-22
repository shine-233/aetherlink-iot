import type { Ref } from 'vue'
import { ref } from 'vue'
import { deviceDictProtocolServiceFirstLevel, deviceDictProtocolServiceSecondLevel } from '@/service/api/device'
import type { SearchConfig } from '@/components/data-table-page/types'
import { $t } from '@/locales'
import { localStg } from '@/utils/storage'

interface ServiceIds {
  service_identifier: string
  service_plugin_id: string
}

interface ServiceAccessFilterOptions {
  searchConfigs: Ref<SearchConfig[]>
  tablePageRef: Ref<any>
  initialServiceIdentifier: unknown
  initialServiceAccessId: unknown
}

const SERVICE_ACCESS_FILTER_KEY = 'service_access_id'

const normalizeServiceIdentifier = (value: unknown) => String(value || '')

const createServiceAccessSearchConfig = (): SearchConfig => ({
  key: SERVICE_ACCESS_FILTER_KEY,
  label: 'custom.devicePage.selectSecondLevelService',
  type: 'select',
  options: []
})

const setSelectOptions = (item: SearchConfig, options: any[]) => {
  if (item.type === 'select') {
    item.options = options
  }
}

export const useDeviceManageServiceAccessFilters = ({
  searchConfigs,
  tablePageRef,
  initialServiceIdentifier,
  initialServiceAccessId
}: ServiceAccessFilterOptions) => {
  const secondLevelOptions = ref<DeviceManagement.ServiceData[]>([])
  const selectedFirstLevel = ref<string | null>(null)
  const serviceIds = ref<ServiceIds[]>([])
  const queryOfServiceIdentifier = ref(initialServiceIdentifier)
  const queryOfServiceAccessId = ref(initialServiceAccessId)
  const serviceFiltersReady = ref(false)
  let secondLevelRequestId = 0

  const getSearchConfigIndex = (key: string) => searchConfigs.value.findIndex((item) => item.key === key)

  const updateSearchConfigsByKey = (key: string, update: (item: SearchConfig) => void) => {
    searchConfigs.value.forEach((item) => {
      if (item.key === key) {
        update(item)
      }
    })
  }

  const getServiceFilterIndexes = () => ({
    identifierIndex: getSearchConfigIndex('service_identifier'),
    accessIndex: getSearchConfigIndex(SERVICE_ACCESS_FILTER_KEY)
  })

  const isServiceIdentifier = (identifier: string) =>
    serviceIds.value.some((item) => item.service_identifier === identifier)

  const getServicePluginId = (identifier: string) =>
    serviceIds.value.find((item) => item.service_identifier === identifier)?.service_plugin_id

  const clearServiceAccessOptions = () => {
    updateSearchConfigsByKey(SERVICE_ACCESS_FILTER_KEY, (item) => {
      setSelectOptions(item, [])
    })
  }

  const applySecondLevelOptions = () => {
    updateSearchConfigsByKey(SERVICE_ACCESS_FILTER_KEY, (item) => {
      setSelectOptions(
        item,
        secondLevelOptions.value.map((item2) => ({
          label: item2.name,
          value: item2.id
        }))
      )
    })
  }

  const resetServiceAccessQueryParam = () => {
    tablePageRef.value?.forceChangeParamsByKey({
      [SERVICE_ACCESS_FILTER_KEY]: null
    })
  }

  const ensureServiceAccessFilter = (identifierIndex: number, accessIndex: number) => {
    if (identifierIndex === -1) return
    if (accessIndex === -1) {
      searchConfigs.value.splice(identifierIndex + 1, 0, createServiceAccessSearchConfig())
      return
    }

    resetServiceAccessQueryParam()
  }

  const removeServiceAccessFilter = (accessIndex: number) => {
    searchConfigs.value.splice(accessIndex, 1)
    resetServiceAccessQueryParam()
  }

  const buildRouteServiceParams = () => ({
    service_identifier: queryOfServiceIdentifier.value,
    [SERVICE_ACCESS_FILTER_KEY]: queryOfServiceAccessId.value
  })

  const hasInitialServiceAccessRoute = () =>
    Boolean(normalizeServiceIdentifier(queryOfServiceIdentifier.value) || queryOfServiceAccessId.value)

  const primeInitialServiceAccessFilter = () => {
    if (!queryOfServiceAccessId.value) return
    const { identifierIndex, accessIndex } = getServiceFilterIndexes()
    ensureServiceAccessFilter(identifierIndex, accessIndex)
  }

  const initializeServiceAccessFilters = async () => {
    const { data } = await deviceDictProtocolServiceFirstLevel({
      language_code: localStg.get('lang')
    })
    if (!data) return

    serviceIds.value = []
    const protocolOptions = data.protocol.map((item) => ({
      label: item.name,
      value: item.service_identifier,
      type: 'protocol'
    }))

    const serviceOptions = data.service
      ? data.service.map((item) => {
          serviceIds.value.push({
            service_identifier: item.service_identifier,
            service_plugin_id: item.service_plugin_id
          })

          return {
            label: item.name,
            value: item.service_identifier,
            type: 'service'
          }
        })
      : []

    updateSearchConfigsByKey('service_identifier', (item) => {
      setSelectOptions(item, [
        { label: $t('card.anyProtocolService'), value: '' },
        {
          type: 'group',
          label: $t('common.protocol'),
          key: 'protocol',
          children: [...protocolOptions]
        },
        {
          type: 'group',
          label: $t('common.service'),
          key: 'service',
          children: [...serviceOptions]
        }
      ])
    })
    serviceFiltersReady.value = true
  }

  const fetchSecondLevelOptionsPage = async (firstLevelValue: string, page: number, requestId: number) => {
    if (!firstLevelValue) return
    if (page === 1) {
      secondLevelOptions.value = []
      clearServiceAccessOptions()
    }

    const pluginId = getServicePluginId(firstLevelValue)
    const { data } = await deviceDictProtocolServiceSecondLevel({
      params: {
        service_plugin_id: pluginId,
        page,
        page_size: 100
      }
    })

    if (!data) return
    const { list, total } = data
    if (requestId !== secondLevelRequestId || selectedFirstLevel.value !== firstLevelValue) return

    if (page === 1) {
      secondLevelOptions.value = list
    } else {
      secondLevelOptions.value = [...secondLevelOptions.value, ...list]
    }
    applySecondLevelOptions()

    if (total > secondLevelOptions.value.length) {
      void fetchSecondLevelOptionsPage(firstLevelValue, page + 1, requestId)
    }
  }

  const fetchSecondLevelOptions = async (firstLevelValue: string) => {
    secondLevelRequestId += 1
    await fetchSecondLevelOptionsPage(firstLevelValue, 1, secondLevelRequestId)
  }

  const paramsUpdateHandle = async (params: Record<string, unknown>) => {
    if (!serviceFiltersReady.value) return
    const firstSelected = normalizeServiceIdentifier(params.service_identifier)
    if (firstSelected && selectedFirstLevel.value !== firstSelected) {
      selectedFirstLevel.value = firstSelected
      const { identifierIndex, accessIndex } = getServiceFilterIndexes()
      if (isServiceIdentifier(firstSelected)) {
        ensureServiceAccessFilter(identifierIndex, accessIndex)
        await fetchSecondLevelOptions(firstSelected)
      } else if (accessIndex > -1) {
        removeServiceAccessFilter(accessIndex)
      }
    }
  }

  const setServiceParams = () => {
    tablePageRef.value?.forceChangeParamsByKey(buildRouteServiceParams())
  }

  const initializeServiceAccessFiltersInBackground = async () => {
    try {
      await initializeServiceAccessFilters()
      if (hasInitialServiceAccessRoute()) {
        setServiceParams()
      }
    } catch {
      serviceFiltersReady.value = false
    }
  }

  return {
    initializeServiceAccessFilters,
    initializeServiceAccessFiltersInBackground,
    primeInitialServiceAccessFilter,
    paramsUpdateHandle,
    setServiceParams
  }
}
