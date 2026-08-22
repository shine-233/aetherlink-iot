/**
 * File purpose: snapshot filter-panel composable for the RDI Overview page.
 * Owns the client-side keyword/status/level/group filter state, the advanced
 * panel visibility, group-tree options, active-filter chips, and the visible
 * snapshot list derived from the server page. The matching predicate and chip
 * assembly are exported as pure helpers so they stay testable without page
 * lifecycle side effects.
 */
import { computed, ref, toValue, type MaybeRefOrGetter } from 'vue'
import type { TreeSelectOption } from 'naive-ui/es/tree-select/src/interface'
import { deviceGroupTree } from '@/service/api/device'
import { $t } from '@/locales'
import { type DeviceSnapshot, type Translate } from './rdiOverviewState'

export interface SnapshotFilterCriteria {
  keyword: string
  status: 'all' | 'online' | 'offline' | 'alarm'
  alarmLevel: 'all' | 'H' | 'M' | 'L' | 'N'
  groupId: string | null
}

// matchSnapshotFilters narrows a single snapshot against the user-selected
// criteria. The keyword is trimmed and lowercased here so callers can pass the
// raw input binding; empty keyword means the field search is a no-op.
export function matchSnapshotFilters(device: DeviceSnapshot, criteria: SnapshotFilterCriteria): boolean {
  if (criteria.status === 'online' && !device.online) return false
  if (criteria.status === 'offline' && device.online) return false
  if (criteria.status === 'alarm' && device.alarm !== true) return false
  if (criteria.alarmLevel !== 'all') {
    const level = device.alarmLevel || (device.alarm === true ? 'H' : device.alarm === false ? 'N' : '')
    if (criteria.alarmLevel === 'N') {
      if (level && level !== 'N') return false
    } else if (level !== criteria.alarmLevel) {
      return false
    }
  }
  if (criteria.groupId && device.groupId !== criteria.groupId) return false
  const keyword = criteria.keyword.trim().toLowerCase()
  if (!keyword) return true
  const haystack = [
    device.name,
    device.id,
    device.pid,
    device.serialNumber,
    device.installLocation,
    device.installAddress,
    device.installerName,
    device.adminName
  ]
    .filter((value) => value && value !== '--')
    .join(' ')
    .toLowerCase()
  return haystack.includes(keyword)
}

export function buildSnapshotFilterChips(
  criteria: SnapshotFilterCriteria,
  groupNameById: Record<string, string>,
  t: Translate
): Array<{ key: string; label: string }> {
  const chips: Array<{ key: string; label: string }> = []
  const keyword = criteria.keyword.trim()
  if (keyword) {
    chips.push({ key: 'keyword', label: `${t('common.search')}: ${keyword}` })
  }
  if (criteria.status !== 'all') {
    const statusLabels: Record<string, string> = {
      online: t('rdi.overview.snapshotStatusOnline'),
      offline: t('rdi.overview.snapshotStatusOffline'),
      alarm: t('rdi.overview.snapshotStatusAlarm')
    }
    chips.push({ key: 'status', label: statusLabels[criteria.status] })
  }
  if (criteria.alarmLevel !== 'all') {
    const alarmLevelLabels: Record<string, string> = {
      H: t('rdi.overview.high'),
      M: t('rdi.overview.medium'),
      L: t('rdi.overview.low'),
      N: t('rdi.overview.normal')
    }
    chips.push({
      key: 'alarmLevel',
      label: `${t('common.alarm_level')}: ${alarmLevelLabels[criteria.alarmLevel]}`
    })
  }
  if (criteria.groupId) {
    const groupName = groupNameById[criteria.groupId] || criteria.groupId
    chips.push({ key: 'group', label: `${t('rdi.overview.snapshotFilterGroup')}: ${groupName}` })
  }
  return chips
}

export function useRdiSnapshotFilters(options: {
  deviceSnapshots: MaybeRefOrGetter<DeviceSnapshot[]>
}) {
  const snapshotFilterKeyword = ref('')
  const snapshotFilterStatus = ref<'all' | 'online' | 'offline' | 'alarm'>('all')
  const snapshotFilterAlarmLevel = ref<'all' | 'H' | 'M' | 'L' | 'N'>('all')
  const snapshotFilterGroupId = ref<string | null>(null)
  const snapshotFilterAdvancedVisible = ref(false)
  const snapshotGroupOptions = ref<TreeSelectOption[]>([])
  const snapshotGroupNameById = ref<Record<string, string>>({})

  const snapshotStatusOptions = computed(() => [
    { label: $t('rdi.overview.snapshotStatusAll'), value: 'all' },
    { label: $t('rdi.overview.snapshotStatusOnline'), value: 'online' },
    { label: $t('rdi.overview.snapshotStatusOffline'), value: 'offline' },
    { label: $t('rdi.overview.snapshotStatusAlarm'), value: 'alarm' }
  ])

  const snapshotAlarmLevelOptions = computed(() => [
    { label: $t('rdi.overview.snapshotAlarmLevelAll'), value: 'all' },
    { label: $t('rdi.overview.high'), value: 'H' },
    { label: $t('rdi.overview.medium'), value: 'M' },
    { label: $t('rdi.overview.low'), value: 'L' },
    { label: $t('rdi.overview.normal'), value: 'N' }
  ])

  // snapshotHasActiveFilters reports whether at least one user-selected filter
  // narrows the visible snapshot list beyond the raw server page.
  const snapshotHasActiveFilters = computed(() => {
    return (
      snapshotFilterKeyword.value.trim() !== '' ||
      snapshotFilterStatus.value !== 'all' ||
      snapshotFilterAlarmLevel.value !== 'all' ||
      !!snapshotFilterGroupId.value
    )
  })

  const snapshotActiveFilterChips = computed(() =>
    buildSnapshotFilterChips(
      {
        keyword: snapshotFilterKeyword.value,
        status: snapshotFilterStatus.value,
        alarmLevel: snapshotFilterAlarmLevel.value,
        groupId: snapshotFilterGroupId.value
      },
      snapshotGroupNameById.value,
      $t
    )
  )

  // visibleDeviceSnapshots filters the currently loaded snapshot page client-side.
  // Server-side pagination for the base list stays in fetchDeviceSnapshots so the
  // filter narrows the visible items without hiding pagination controls.
  const visibleDeviceSnapshots = computed(() => {
    const criteria: SnapshotFilterCriteria = {
      keyword: snapshotFilterKeyword.value,
      status: snapshotFilterStatus.value,
      alarmLevel: snapshotFilterAlarmLevel.value,
      groupId: snapshotFilterGroupId.value
    }
    return toValue(options.deviceSnapshots).filter((device) => matchSnapshotFilters(device, criteria))
  })

  function resetSnapshotFilters() {
    snapshotFilterKeyword.value = ''
    snapshotFilterStatus.value = 'all'
    snapshotFilterAlarmLevel.value = 'all'
    snapshotFilterGroupId.value = null
  }

  function removeSnapshotFilter(key: string) {
    if (key === 'keyword') snapshotFilterKeyword.value = ''
    else if (key === 'status') snapshotFilterStatus.value = 'all'
    else if (key === 'alarmLevel') snapshotFilterAlarmLevel.value = 'all'
    else if (key === 'group') snapshotFilterGroupId.value = null
  }

  function toggleSnapshotAdvancedFilters() {
    snapshotFilterAdvancedVisible.value = !snapshotFilterAdvancedVisible.value
  }

  // loadSnapshotGroupOptions fetches the device-group tree once and flattens the
  // labels for filter-chip lookup. Missing groups fall back to the raw id so a
  // stale filter still renders meaningfully.
  async function loadSnapshotGroupOptions() {
    try {
      const res = await deviceGroupTree({})
      const rootNodes = Array.isArray(res?.data) ? res.data : []
      const nameMap: Record<string, string> = {}
      function convert(node: any): TreeSelectOption {
        const group = node?.group || {}
        const children = Array.isArray(node?.children) ? node.children : []
        const id = String(group.id ?? '')
        const label = String(group.name ?? id ?? '')
        if (id) nameMap[id] = label
        const option: TreeSelectOption = { key: id, label }
        if (children.length > 0) option.children = children.map(convert)
        return option
      }
      snapshotGroupOptions.value = rootNodes.map(convert)
      snapshotGroupNameById.value = nameMap
    } catch {
      // A missing group tree only degrades the optional filter; keep the rest of
      // the overview page usable when the backend is unavailable.
      snapshotGroupOptions.value = []
      snapshotGroupNameById.value = {}
    }
  }

  return {
    snapshotFilterKeyword,
    snapshotFilterStatus,
    snapshotFilterAlarmLevel,
    snapshotFilterGroupId,
    snapshotFilterAdvancedVisible,
    snapshotGroupOptions,
    snapshotGroupNameById,
    snapshotStatusOptions,
    snapshotAlarmLevelOptions,
    snapshotHasActiveFilters,
    snapshotActiveFilterChips,
    visibleDeviceSnapshots,
    resetSnapshotFilters,
    removeSnapshotFilter,
    toggleSnapshotAdvancedFilters,
    loadSnapshotGroupOptions
  }
}
