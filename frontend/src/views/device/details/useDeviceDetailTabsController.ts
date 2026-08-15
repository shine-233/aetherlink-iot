import { nextTick, ref } from 'vue'

import { createDeviceTabPlan, resolveDeviceChartTabResolution } from './device-tab-plan'
import { applyRdiCustomerTabs, createBaseDeviceTabs, type DeviceDetailTabComponent } from './device-tab-registry'
import { bumpDeviceTabRefreshKey } from './device-tab-render-state'

type DeviceDetailData = Record<string, any>

type UseDeviceDetailTabsControllerOptions = {
  startLoading: () => void
  endLoading: () => void
  onDeviceTypeChange: (deviceType: string) => void
  onTabChange?: (tabKey: string) => void
}

export const useDeviceDetailTabsController = ({
  startLoading,
  endLoading,
  onDeviceTypeChange,
  onTabChange
}: UseDeviceDetailTabsControllerOptions) => {
  const components = ref<DeviceDetailTabComponent[]>([])
  const tabsRenderKey = ref(0)
  const tabValue = ref<string>('')

  let lastTabsSig = ''
  let lastConfigId = ''
  let chartResolutionSeq = 0
  let currentHiddenKeys = new Set<string>()
  let currentIsRdi = false
  let pendingRequestedTabKey = ''

  const getTabKeys = (tabs = components.value) => tabs.map((item) => item.key)

  const getPreferredTabKey = (tabs = components.value) => {
    const keys = getTabKeys(tabs)
    if (currentIsRdi && keys.includes('message')) return 'message'
    if (keys.includes('rdi')) return 'rdi'
    if (keys.includes('ready-check')) return 'ready-check'
    if (keys.includes('chart')) return 'chart'
    if (keys.includes('telemetry')) return 'telemetry'
    return tabs[0]?.key || ''
  }

  const ensureActiveTab = () => {
    const preferredKey = getPreferredTabKey()
    if (!preferredKey) {
      tabValue.value = ''
      return
    }

    const exists = components.value.some((item) => item.key === tabValue.value)
    if (!exists) tabValue.value = preferredKey
  }

  const setActiveTabIfVisible = (tabKey: string) => {
    const exists = components.value.some((item) => item.key === tabKey)
    if (!exists) return false
    tabValue.value = tabKey
    return true
  }

  const activatePendingRequestedTabIfVisible = () => {
    if (!pendingRequestedTabKey) return false
    const activated = setActiveTabIfVisible(pendingRequestedTabKey)
    if (activated) pendingRequestedTabKey = ''
    return activated
  }

  const activateTabIfVisible = (tabKey: string | null | undefined) => {
    if (!tabKey) return false
    const normalizedTabKey = String(tabKey)
    const activated = setActiveTabIfVisible(normalizedTabKey)
    pendingRequestedTabKey = activated ? '' : normalizedTabKey
    return activated
  }

  const bumpRefreshKey = (targetKey: string) => {
    bumpDeviceTabRefreshKey(components.value, targetKey)
  }

  const cloneBaseComponents = (isRdi = false) => {
    const tabs = createBaseDeviceTabs().map((item) => ({ ...item }))
    return isRdi ? applyRdiCustomerTabs(tabs) : tabs
  }

  const finishTabLoadingAfterRender = () => {
    void nextTick(() => {
      endLoading()
    })
  }

  const applyActiveTabValue = (nextTabValue: string | number) => {
    tabValue.value = String(nextTabValue)
  }

  const changeTabs = (nextTabValue: string | number) => {
    startLoading()
    applyActiveTabValue(nextTabValue)
    onTabChange?.(tabValue.value)
    finishTabLoadingAfterRender()
  }

  const applyDeviceTabPlan = (hiddenKeys: Set<string>, deviceType: string, isRdi = false) => {
    if (deviceType) {
      onDeviceTypeChange(deviceType)
    }

    currentHiddenKeys = new Set(hiddenKeys)
    currentIsRdi = isRdi
    components.value = cloneBaseComponents(isRdi).filter((item) => !hiddenKeys.has(item.key))
    ensureActiveTab()
  }

  const applyChartTabResolution = async (data: DeviceDetailData, shouldShowChart: boolean) => {
    const nextHiddenKeys = new Set(currentHiddenKeys)
    if (shouldShowChart) {
      nextHiddenKeys.delete('chart')
    } else {
      nextHiddenKeys.add('chart')
    }

    const tabPlan = createDeviceTabPlan(data)
    applyDeviceTabPlan(nextHiddenKeys, tabPlan.deviceType, tabPlan.isRdi)
    activatePendingRequestedTabIfVisible()
    await refreshTabsRenderIfNeeded(data)
  }

  const scheduleChartTabResolution = (data: DeviceDetailData) => {
    const resolveSeq = ++chartResolutionSeq
    void resolveDeviceChartTabResolution(data).then((resolution) => {
      if (resolveSeq !== chartResolutionSeq || !resolution) return

      void applyChartTabResolution(data, resolution.shouldShowChart)
    })
  }

  const buildVisibleTabs = async (data: DeviceDetailData) => {
    const tabPlan = createDeviceTabPlan(data)
    applyDeviceTabPlan(tabPlan.hiddenKeys, tabPlan.deviceType, tabPlan.isRdi)
    scheduleChartTabResolution(data)
  }

  const getTabsSignature = () => getTabKeys().join('|')

  const getDeviceConfigId = (data: DeviceDetailData) => data?.device_config_id || ''

  const shouldRefreshTabsRender = (nextSig: string, currentConfigId: string) =>
    nextSig !== lastTabsSig || (lastConfigId && lastConfigId !== currentConfigId)

  const cacheTabsRenderIdentity = (signature: string, configId: string) => {
    lastTabsSig = signature
    lastConfigId = configId
  }

  const refreshTabsRenderIfNeeded = async (data: DeviceDetailData) => {
    const signature = getTabsSignature()
    const configId = getDeviceConfigId(data)
    const shouldRefresh = shouldRefreshTabsRender(signature, configId)
    const isFirstRender = lastTabsSig === ''

    cacheTabsRenderIdentity(signature, configId)

    if (!shouldRefresh || isFirstRender) return

    await nextTick()
    tabsRenderKey.value += 1
  }

  const syncDeviceDetailTabs = async (data: DeviceDetailData) => {
    await buildVisibleTabs(data)
    await refreshTabsRenderIfNeeded(data)
  }

  const refreshActiveTabIfNeeded = (shouldRefreshActiveTab = false) => {
    if (!shouldRefreshActiveTab) return
    bumpRefreshKey(tabValue.value)
  }

  const remountActiveTabLabel = () => {
    const preservedTabValue = tabValue.value
    tabValue.value = ''
    setTimeout(() => {
      tabValue.value = preservedTabValue
    }, 50)
  }

  return {
    activateTabIfVisible,
    changeTabs,
    components,
    refreshActiveTabIfNeeded,
    remountActiveTabLabel,
    syncDeviceDetailTabs,
    tabValue,
    tabsRenderKey
  }
}
