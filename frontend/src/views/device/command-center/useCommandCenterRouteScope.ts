import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  buildCommandCenterFilterSummaryItems,
  normalizeQueryValue,
  parseCommandCenterScopeContext
} from './commandCenterState'
import type { QueryValue } from './commandCenterState'
import { parseCommandCenterRouteDraft } from './commandCenterRouteDraft'
import { buildActiveCommandJobQuery } from './commandCenterRouteQuery'

export function useCommandCenterRouteScope() {
  const route = useRoute()
  const router = useRouter()

  const scopeContext = computed(() => parseCommandCenterScopeContext(route.query as Record<string, any>))
  const selectedDeviceIds = computed(() => scopeContext.value.deviceIds)

  const selectedCount = computed(() => selectedDeviceIds.value.length)
  const requestedTotal = computed(() => scopeContext.value.requestedTotal)
  const currentPageCount = computed(() => scopeContext.value.currentPageCount)
  const scope = computed(() => scopeContext.value.scopeType)
  const routeScope = computed(() => scopeContext.value.routeScope)
  const deviceFilter = computed(() => scopeContext.value.deviceFilter)
  const filterSummaryItems = computed(() => buildCommandCenterFilterSummaryItems(deviceFilter.value))
  const hasDeviceFilter = computed(() => filterSummaryItems.value.length > 0)
  const isDeviceFilterScope = computed(() => scope.value === 'device_filter')
  const hasSelectedDevices = computed(() => selectedCount.value > 0)
  const hasCommandJobScope = computed(() =>
    isDeviceFilterScope.value ? hasDeviceFilter.value : hasSelectedDevices.value
  )
  const activeCommandJobId = computed(
    () =>
      normalizeQueryValue(route.query.command_job_id as QueryValue) ||
      normalizeQueryValue(route.query.job_id as QueryValue)
  )
  const routeCommandDraft = computed(() => parseCommandCenterRouteDraft(route.query))

  const setActiveCommandJobQuery = async (jobId: string) => {
    await router.replace({ query: buildActiveCommandJobQuery(route.query, jobId) })
  }

  return {
    activeCommandJobId,
    currentPageCount,
    deviceFilter,
    filterSummaryItems,
    hasCommandJobScope,
    hasDeviceFilter,
    hasSelectedDevices,
    isDeviceFilterScope,
    requestedTotal,
    routeCommandDraft,
    routeScope,
    scope,
    scopeContext,
    selectedCount,
    selectedDeviceIds,
    setActiveCommandJobQuery
  }
}
