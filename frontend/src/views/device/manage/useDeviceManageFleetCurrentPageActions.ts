import { ref, type Ref } from 'vue'
import type { Router } from 'vue-router'
import {
  buildFleetCurrentPageActionRoute,
  buildFleetCurrentPageHandoffQuery,
  type FleetCurrentPageAction
} from './device-fleet-handoff-routes'

type TablePageRef = Ref<
  | {
      dataList?: any[]
    }
  | undefined
>

type MessageApi = {
  warning?: (message: string) => void
}

type UseDeviceManageFleetCurrentPageActionsOptions = {
  tablePageRef: TablePageRef
  router: Router
  t: (key: string) => string
  message?: MessageApi
  lastDeviceQueryParams: Ref<Record<string, unknown>>
  targetPreviewTotal: Ref<number | null>
}

export function useDeviceManageFleetCurrentPageActions(
  options: UseDeviceManageFleetCurrentPageActionsOptions
) {
  const fleetScopeConfirmVisible = ref(false)
  const pendingFleetScopeAction = ref<FleetCurrentPageAction | null>(null)

  const getCurrentPageDeviceRows = () => options.tablePageRef.value?.dataList || []

  const buildFleetCurrentPageQuery = () => {
    const rows = getCurrentPageDeviceRows()
    if (rows.length === 0) {
      options.message?.warning?.(options.t('custom.devicePage.noFleetDevicesToOperate'))
      return null
    }

    return buildFleetCurrentPageHandoffQuery(
      rows,
      options.lastDeviceQueryParams.value,
      options.targetPreviewTotal.value
    )
  }

  const requestFleetCurrentPageAction = (action: FleetCurrentPageAction) => {
    if (!buildFleetCurrentPageQuery()) return

    pendingFleetScopeAction.value = action
    fleetScopeConfirmVisible.value = true
  }

  const confirmFleetCurrentPageAction = () => {
    if (!pendingFleetScopeAction.value) return

    const query = buildFleetCurrentPageQuery()
    if (!query) return

    options.router.push(buildFleetCurrentPageActionRoute(pendingFleetScopeAction.value, query))
    fleetScopeConfirmVisible.value = false
    pendingFleetScopeAction.value = null
  }

  const cancelFleetCurrentPageAction = () => {
    fleetScopeConfirmVisible.value = false
    pendingFleetScopeAction.value = null
  }

  const openFleetOtaContext = () =>
    requestFleetCurrentPageAction({ path: '/product/update-ota', labelKey: 'custom.devicePage.openFleetOta' })
  const openFleetAlarmContext = () =>
    requestFleetCurrentPageAction({ path: '/alarm/warning-message', labelKey: 'custom.devicePage.openFleetAlarms' })

  const openFleetAuditContext = () =>
    requestFleetCurrentPageAction({ path: '/alarm/warning-message', labelKey: 'custom.devicePage.openFleetAudit' })

  return {
    fleetScopeConfirmVisible,
    pendingFleetScopeAction,
    confirmFleetCurrentPageAction,
    cancelFleetCurrentPageAction,
    openFleetOtaContext,
    openFleetAlarmContext,
    openFleetAuditContext
  }
}
