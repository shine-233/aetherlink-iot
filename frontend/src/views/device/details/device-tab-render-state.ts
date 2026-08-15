import type { DeviceDetailTabComponent } from './device-tab-registry'

export type TabsRenderIdentity = {
  signature: string
  configId: string
}

export type TabsRenderRefreshDecision = {
  identity: TabsRenderIdentity
  shouldRefresh: boolean
  isFirstRender: boolean
}

export function bumpDeviceTabRefreshKey(tabs: DeviceDetailTabComponent[], targetKey: string) {
  const current = tabs.find((item) => item.key === targetKey)
  if (current) current.refreshKey += 1
}
