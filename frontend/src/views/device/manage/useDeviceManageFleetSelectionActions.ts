import { computed, ref, type Ref } from 'vue'
import type { Router } from 'vue-router'
import type { TreeSelectOption } from 'naive-ui/es/tree-select/src/interface'
import {
  buildFleetSelectedDeviceIdentifiers,
  buildFleetSelectionSummary
} from './device-fleet-operations'
import { buildSelectedDeviceCommandCenterRoute } from './device-fleet-handoff-routes'

type TablePageRef = Ref<
  | {
      clearSelection?: () => void
      handleSearch?: () => void
    }
  | undefined
>

type MessageApi = {
  success?: (message: string) => void
  warning?: (message: string) => void
}

type UseDeviceManageFleetSelectionActionsOptions = {
  tablePageRef: TablePageRef
  router: Router
  t: (key: string) => string
  message?: MessageApi
  getGroupOptions: () => Promise<TreeSelectOption[]>
  assignDevicesToGroup: (payload: {
    group_id: string | number
    device_id_list: string[]
  }) => Promise<{ error?: unknown }>
}

export function useDeviceManageFleetSelectionActions(options: UseDeviceManageFleetSelectionActionsOptions) {
  const selectedFleetDeviceRows = ref<any[]>([])
  const selectedFleetSummaryVisible = ref(false)
  const bulkGroupModalVisible = ref(false)
  const bulkGroupOptions = ref<TreeSelectOption[]>([])
  const bulkGroupId = ref<string | number | null>(null)
  const bulkGroupAssigning = ref(false)

  const selectedFleetDeviceIds = computed(() =>
    selectedFleetDeviceRows.value.map((row) => row?.id).filter((id): id is string => Boolean(id))
  )
  const selectedFleetSummary = computed(() => buildFleetSelectionSummary(selectedFleetDeviceRows.value))
  const selectedFleetDeviceIdentifiers = computed(() =>
    buildFleetSelectedDeviceIdentifiers(selectedFleetDeviceRows.value)
  )

  const openSelectedDeviceGroupDialog = async () => {
    if (selectedFleetDeviceIds.value.length === 0) {
      options.message?.warning?.(options.t('custom.devicePage.selectFleetDevicesFirst'))
      return
    }

    bulkGroupId.value = null
    bulkGroupOptions.value = await options.getGroupOptions()
    bulkGroupModalVisible.value = true
  }

  const assignSelectedDevicesToGroup = async () => {
    if (!bulkGroupId.value) {
      options.message?.warning?.(options.t('custom.devicePage.selectTargetGroupFirst'))
      return
    }

    const deviceIds = selectedFleetDeviceIds.value
    if (deviceIds.length === 0) {
      options.message?.warning?.(options.t('custom.devicePage.selectFleetDevicesFirst'))
      return
    }

    bulkGroupAssigning.value = true
    try {
      const { error } = await options.assignDevicesToGroup({
        group_id: bulkGroupId.value,
        device_id_list: deviceIds
      })

      if (error) return

      options.message?.success?.(options.t('custom.devicePage.selectedDevicesAddedToGroup'))
      bulkGroupModalVisible.value = false
      selectedFleetDeviceRows.value = []
      options.tablePageRef.value?.clearSelection?.()
      options.tablePageRef.value?.handleSearch?.()
    } finally {
      bulkGroupAssigning.value = false
    }
  }

  const handleFleetSelectionUpdate = (rows: any[]) => {
    selectedFleetDeviceRows.value = rows
    if (rows.length === 0) {
      selectedFleetSummaryVisible.value = false
    }
  }

  const openSelectedFleetSummary = () => {
    if (selectedFleetDeviceRows.value.length === 0) {
      options.message?.warning?.(options.t('custom.devicePage.selectFleetDevicesFirst'))
      return
    }
    selectedFleetSummaryVisible.value = true
  }

  const copySelectedFleetDeviceIdentifiers = async () => {
    if (!selectedFleetDeviceIdentifiers.value) {
      options.message?.warning?.(options.t('custom.devicePage.selectFleetDevicesFirst'))
      return
    }
    try {
      await navigator.clipboard.writeText(selectedFleetDeviceIdentifiers.value)
      options.message?.success?.(options.t('custom.devicePage.selectedDeviceIdentifiersCopied'))
    } catch {
      options.message?.warning?.(options.t('custom.devicePage.selectedDeviceIdentifiersCopyFailed'))
    }
  }

  const openSelectedDeviceCommandContext = () => {
    const route = buildSelectedDeviceCommandCenterRoute(selectedFleetDeviceIds.value)
    if (!route) {
      options.message?.warning?.(options.t('custom.devicePage.selectFleetDevicesFirst'))
      return
    }

    options.router.push(route)
  }

  const openFleetConfigContext = () => {
    if (selectedFleetDeviceIds.value.length === 0) {
      options.message?.warning?.(options.t('custom.devicePage.selectFleetDevicesFirst'))
      return
    }

    options.message?.warning?.(options.t('custom.devicePage.fleetConfigTagUseGroupAction'))
    openSelectedDeviceGroupDialog()
  }

  return {
    selectedFleetDeviceRows,
    selectedFleetDeviceIds,
    selectedFleetSummary,
    selectedFleetSummaryVisible,
    selectedFleetDeviceIdentifiers,
    bulkGroupModalVisible,
    bulkGroupOptions,
    bulkGroupId,
    bulkGroupAssigning,
    openSelectedDeviceGroupDialog,
    assignSelectedDevicesToGroup,
    handleFleetSelectionUpdate,
    openSelectedFleetSummary,
    copySelectedFleetDeviceIdentifiers,
    openSelectedDeviceCommandContext,
    openFleetConfigContext
  }
}
