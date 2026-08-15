import { nextTick, type Ref } from 'vue'
import type { Router } from 'vue-router'
import type { FleetCommandJobPreviewRow } from '@/service/api/device'
import { FLEET_CURRENT_PAGE_SCOPE, FLEET_FILTER_RESULT_SCOPE } from '../modules/fleet-rollout-context'
import { buildSavedFilterCommandCenterRoute } from '../manage/device-fleet-handoff-routes'
import type { DeviceFilterPayload } from './commandCenterState'
import type { SavedFleetFilter } from '../manage/device-fleet-saved-filters'

type PreviewSnapshot = {
  rows: FleetCommandJobPreviewRow[]
  requested_count?: number
} | null

type CommandCenterNavigationDeps = {
  router: Router
  selectedDeviceIds: () => string[]
  selectedCount: () => number
  requestedTotal: () => number | null
  isDeviceFilterScope: () => boolean
  deviceFilter: () => DeviceFilterPayload
  previewResult: () => PreviewSnapshot
  savedFleetFilters: () => SavedFleetFilter[]
  selectedSavedFleetFilterId: Ref<string | null>
  commandIdentify: () => string
  resetCommandJobDraft: () => void
  previewCommandJob: () => unknown
  t: (key: string) => string
}

function collectOtaDeviceIds(input: {
  isDeviceFilterScope: boolean
  previewResult: PreviewSnapshot
  selectedDeviceIds: string[]
}) {
  if (!input.isDeviceFilterScope) return input.selectedDeviceIds
  return Array.from(new Set((input.previewResult?.rows ?? []).map((row) => row.device_id).filter(Boolean)))
}

const hasDeviceFilterPayload = (filter: DeviceFilterPayload) =>
  Object.values(filter).some((value) => {
    if (value === null || value === undefined || value === '') return false
    return !(Array.isArray(value) && value.length === 0)
  })

export function useCommandCenterNavigation(deps: CommandCenterNavigationDeps) {
  const openImmediateCommand = () => {
    const firstDeviceId = deps.selectedDeviceIds()[0]
    if (!firstDeviceId) return

    deps.router.push({
      path: '/device/details',
      query: {
        d_id: firstDeviceId,
        tab: 'command-delivery',
        command_center: 'immediate',
        selected_count: deps.selectedCount()
      }
    })
  }

  const openOtaJobs = () => {
    const preview = deps.previewResult()
    const isFilterScope = deps.isDeviceFilterScope()
    const deviceFilter = deps.deviceFilter()
    if (isFilterScope && !preview) {
      window.$message?.warning(deps.t('custom.commandCenter.previewBeforeOta'))
      return
    }

    const otaDeviceIds = collectOtaDeviceIds({
      isDeviceFilterScope: isFilterScope,
      previewResult: preview,
      selectedDeviceIds: deps.selectedDeviceIds()
    })

    if (isFilterScope && !hasDeviceFilterPayload(deviceFilter)) {
      window.$message?.warning(deps.t('custom.commandCenter.noSelection'))
      return
    }

    if (!isFilterScope && !otaDeviceIds.length) {
      window.$message?.warning(deps.t('custom.commandCenter.noSelection'))
      return
    }

    const query: Record<string, string | number> = {
      fleet_source: 'device_manage',
      fleet_scope: isFilterScope ? FLEET_FILTER_RESULT_SCOPE : FLEET_CURRENT_PAGE_SCOPE,
      fleet_current_page_count: otaDeviceIds.length,
      fleet_requested_total: isFilterScope
        ? preview?.requested_count || deps.requestedTotal() || otaDeviceIds.length
        : deps.requestedTotal() || otaDeviceIds.length,
      ...deviceFilter
    }

    if (isFilterScope) {
      query.device_filter = JSON.stringify(deviceFilter)
      if (otaDeviceIds.length) query.preview_sample_device_ids = otaDeviceIds.join(',')
    } else {
      query.device_ids = otaDeviceIds.join(',')
    }

    deps.router.push({
      path: '/product/update-ota',
      query
    })
  }

  const openFleet = () => {
    deps.router.push('/device/manage')
  }

  const applySavedFleetFilterInCommandCenter = async (filterId: string | number | null) => {
    deps.selectedSavedFleetFilterId.value = typeof filterId === 'string' ? filterId : null
    const savedFilter = deps
      .savedFleetFilters()
      .find((filter) => filter.id === deps.selectedSavedFleetFilterId.value)
    if (!savedFilter) return

    const nextRoute = buildSavedFilterCommandCenterRoute(savedFilter.params, savedFilter.previewTotal, savedFilter)
    const query = typeof nextRoute === 'object' && nextRoute && 'query' in nextRoute ? nextRoute.query : null
    if (!query) {
      window.$message?.warning(deps.t('custom.commandCenter.savedFilterEmpty'))
      return
    }

    deps.resetCommandJobDraft()
    await deps.router.replace({
      path: '/device/command-center',
      query
    })
    await nextTick()

    if (deps.commandIdentify().trim()) {
      void deps.previewCommandJob()
    }
  }

  return {
    applySavedFleetFilterInCommandCenter,
    openFleet,
    openImmediateCommand,
    openOtaJobs
  }
}
