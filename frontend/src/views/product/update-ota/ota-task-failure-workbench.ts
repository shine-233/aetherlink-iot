import { OTA_TASK_DETAIL_STATUS } from './ota-task-actions'
import type { OtaPackageRecord, OtaTaskDetailRecord, OtaTaskRecord, OtaTaskStatisticsItem } from './ota-task-types'

export type OtaFailureGroup = {
  key: string
  reason: string
  count: number
  devices: OtaTaskDetailRecord[]
}

export type OtaRetryRecommendationKey =
  | 'pause_and_check_package'
  | 'needs_device_diagnostics'
  | 'safe_to_retry'

export type OtaRetryRecommendationCard = {
  key: OtaRetryRecommendationKey
  title: string
  description: string
  count: number
  type: 'success' | 'warning' | 'error'
  devices: string[]
}

type RetryRecommendationTranslate = (key: OtaRetryRecommendationKey) => Pick<OtaRetryRecommendationCard, 'title' | 'description'>

const EMPTY_REASON_KEY = 'page.product.update-ota.failureUnknownReason'
const PACKAGE_FAILURE_PATTERN =
  /checksum|crc|hash|md5|sha|signature|sign|verify|verification|package|firmware|file|format|version|compatible|compatibility|mismatch|invalid|corrupt|digest/i
const DEVICE_DIAGNOSTIC_PATTERN =
  /offline|disconnect|unreachable|heartbeat|ack|response|respond|reject|battery|power|storage|space|memory|permission|denied|device/i
const SAFE_RETRY_PATTERN =
  /timeout|timed out|temporary|transient|busy|interrupted|download failed|network error|connection reset|retry/i

const RETRY_RECOMMENDATION_TEXT: Record<
  OtaRetryRecommendationKey,
  Pick<OtaRetryRecommendationCard, 'title' | 'description' | 'type'>
> = {
  pause_and_check_package: {
    title: 'Pause and check the package',
    description:
      'These failures look like version, signature, checksum, or package compatibility issues. ' +
      'Confirm the package, target version, and device profile before retrying.',
    type: 'warning'
  },
  needs_device_diagnostics: {
    title: 'Run device diagnostics first',
    description:
      'These failures need an online, power, storage, response, or diagnostics link check first. ' +
      'Do not retry the whole batch immediately.',
    type: 'error'
  },
  safe_to_retry: {
    title: 'Safe for a small retry',
    description:
      'These failures look more like temporary network issues, timeouts, or interrupted downloads. ' +
      'Retry a small number of devices first, then widen the scope.',
    type: 'success'
  }
}

export const getOtaFailureReason = (row: OtaTaskDetailRecord, fallbackReason: string) =>
  row.status_description?.trim() || fallbackReason

export const getOtaFailedDevices = (rows: OtaTaskDetailRecord[]) =>
  rows.filter((row) => row.status === OTA_TASK_DETAIL_STATUS.failed)

export const buildOtaFailureGroups = (rows: OtaTaskDetailRecord[], fallbackReason: string): OtaFailureGroup[] => {
  const groupMap = new Map<string, OtaTaskDetailRecord[]>()

  getOtaFailedDevices(rows).forEach((row) => {
    const reason = getOtaFailureReason(row, fallbackReason)
    groupMap.set(reason, [...(groupMap.get(reason) || []), row])
  })

  return Array.from(groupMap.entries())
    .map(([reason, devices]) => ({
      key: reason || EMPTY_REASON_KEY,
      reason,
      count: devices.length,
      devices
    }))
    .sort((left, right) => right.count - left.count || left.reason.localeCompare(right.reason))
}

const getDeviceLabel = (row: OtaTaskDetailRecord) => row.name || row.device_number || row.id

const hasSelectedPackageVersionMismatch = (row: OtaTaskDetailRecord, selectedPackage?: OtaPackageRecord | null) => {
  const expectedVersion = selectedPackage?.target_version || selectedPackage?.version
  return Boolean(row.version && expectedVersion && row.version !== expectedVersion)
}

const getRetryRecommendationKey = (
  row: OtaTaskDetailRecord,
  fallbackReason: string,
  selectedPackage?: OtaPackageRecord | null
): OtaRetryRecommendationKey => {
  const reason = getOtaFailureReason(row, fallbackReason)
  const isUnknownReason = !row.status_description?.trim() || reason === fallbackReason

  if (PACKAGE_FAILURE_PATTERN.test(reason) || hasSelectedPackageVersionMismatch(row, selectedPackage)) {
    return 'pause_and_check_package'
  }

  if (!row.device_id || isUnknownReason || DEVICE_DIAGNOSTIC_PATTERN.test(reason)) {
    return 'needs_device_diagnostics'
  }

  if (SAFE_RETRY_PATTERN.test(reason)) {
    return 'safe_to_retry'
  }

  return 'needs_device_diagnostics'
}

export const buildOtaRetryRecommendationCards = (
  rows: OtaTaskDetailRecord[],
  fallbackReason: string,
  selectedPackage?: OtaPackageRecord | null,
  translate?: RetryRecommendationTranslate
): OtaRetryRecommendationCard[] => {
  const groups = new Map<OtaRetryRecommendationKey, OtaTaskDetailRecord[]>()

  getOtaFailedDevices(rows).forEach((row) => {
    const key = getRetryRecommendationKey(row, fallbackReason, selectedPackage)
    groups.set(key, [...(groups.get(key) || []), row])
  })

  return (['pause_and_check_package', 'needs_device_diagnostics', 'safe_to_retry'] as const)
    .map((key) => {
      const devices = groups.get(key) || []
      const text = RETRY_RECOMMENDATION_TEXT[key]
      const translatedText = translate?.(key)

      return {
        key,
        ...text,
        ...translatedText,
        count: devices.length,
        devices: devices.slice(0, 3).map(getDeviceLabel)
      }
    })
    .filter((item) => item.count > 0)
}

const csvCell = (value: unknown) => {
  const text = value === null || value === undefined ? '' : String(value)
  return `"${text.replace(/"/g, '""')}"`
}

export const buildOtaFailureCsv = (rows: OtaTaskDetailRecord[], fallbackReason: string) => {
  const headers = [
    'id',
    'device_number',
    'name',
    'current_version',
    'target_version',
    'progress',
    'updated_at',
    'failure_reason'
  ]
  const csvRows = getOtaFailedDevices(rows).map((row) =>
    [
      row.id,
      row.device_number,
      row.name,
      row.current_version,
      row.version,
      row.steps,
      row.updated_at,
      getOtaFailureReason(row, fallbackReason)
    ]
      .map(csvCell)
      .join(',')
  )

  return [headers.join(','), ...csvRows].join('\r\n')
}

export const buildOtaFailureClipboardText = (rows: OtaTaskDetailRecord[], fallbackReason: string) =>
  getOtaFailedDevices(rows)
    .map((row) =>
      [
        row.name || row.device_number || row.id,
        row.device_number || '-',
        row.current_version || '-',
        row.version || '-',
        getOtaFailureReason(row, fallbackReason)
      ].join(' | ')
    )
    .join('\n')

export type OtaFailureSupportBundleInput = {
  task?: OtaTaskRecord | null
  selectedPackage?: OtaPackageRecord | null
  rows: OtaTaskDetailRecord[]
  statistics?: OtaTaskStatisticsItem[]
  fallbackReason: string
  generatedAt?: string
  lastRefreshLabel?: string
  maxDevices?: number
}

const statusCountLines = (statistics: OtaTaskStatisticsItem[] = []) => {
  if (!statistics.length) return ['statistics=<empty>']

  return statistics.map((item) => `status_${item.status ?? 'unknown'}=${Number(item.count || 0)}`)
}

const taskTargetText = (task?: OtaTaskRecord | null) => {
  if (!task) return '<unknown>'
  if (task.target_mode === 'filter') return `filter expected=${task.preview_total ?? task.selected_count ?? '<unknown>'}`
  return `explicit device_count=${task.device_count ?? task.selected_count ?? '<unknown>'}`
}

const deviceDiagnosticsRouteText = (row: OtaTaskDetailRecord, taskId?: string | number) => {
  if (!row.device_id) return '<missing-device-id>'

  const params = new URLSearchParams({
    d_id: String(row.device_id),
    tab: 'ready-check',
    source: 'ota',
    ota_detail_id: String(row.id)
  })
  if (taskId) {
    params.set('ota_task_id', String(taskId))
  }

  return `/device/details?${params.toString()}`
}

export const buildOtaFailureSupportBundle = ({
  task,
  selectedPackage,
  rows,
  statistics = [],
  fallbackReason,
  generatedAt = new Date().toISOString(),
  lastRefreshLabel,
  maxDevices = 20
}: OtaFailureSupportBundleInput) => {
  const failedDevices = getOtaFailedDevices(rows)
  const groups = buildOtaFailureGroups(rows, fallbackReason)
  const retryRecommendations = buildOtaRetryRecommendationCards(rows, fallbackReason, selectedPackage)
  const deviceLines = failedDevices.slice(0, maxDevices).map((row, index) =>
    [
      `${index + 1}. ${row.name || row.device_number || row.id}`,
      `deviceId=${row.device_id || '-'}`,
      `number=${row.device_number || '-'}`,
      `current=${row.current_version || '-'}`,
      `target=${row.version || selectedPackage?.target_version || selectedPackage?.version || '-'}`,
      `progress=${row.steps ?? '-'}`,
      `updated=${row.updated_at || '-'}`,
      `diagnostics=${deviceDiagnosticsRouteText(row, task?.id)}`,
      `reportedReason=${getOtaFailureReason(row, fallbackReason)}`
    ].join(' | ')
  )

  return [
    '# AetherLink OTA failed-rollout support package',
    '',
    '## Generation info',
    `generatedAt=${generatedAt}`,
    'scope=loaded task detail rows in the current frontend state',
    lastRefreshLabel ? `detailRefresh=${lastRefreshLabel}` : '',
    '',
    '## Task',
    `taskId=${task?.id || '<unknown>'}`,
    `taskName=${task?.name || '<unknown>'}`,
    `target=${taskTargetText(task)}`,
    `createdAt=${task?.created_at || '<unknown>'}`,
    '',
    '## Package',
    `packageId=${selectedPackage?.id || task?.ota_upgrade_package_id || '<unknown>'}`,
    `packageName=${selectedPackage?.name || '<unknown>'}`,
    `packageVersion=${selectedPackage?.version || selectedPackage?.target_version || '<unknown>'}`,
    '',
    '## Rollout statistics',
    ...statusCountLines(statistics),
    `loadedDetailRows=${rows.length}`,
    `failedDevices=${failedDevices.length}`,
    '',
    '## Failure groups',
    ...(groups.length
      ? groups.map((group) => `- ${group.reason || fallbackReason}: ${group.count}`)
      : ['- <none>']),
    '',
    '## Retry recommendations',
    ...(retryRecommendations.length
      ? retryRecommendations.map(
          (item) => `- ${item.key}: ${item.count} device(s). ${item.description}`
        )
      : ['- <none>']),
    '',
    '## Representative failed devices',
    ...(deviceLines.length ? deviceLines : ['<none>']),
    failedDevices.length > maxDevices
      ? `... ${failedDevices.length - maxDevices} more failed device(s) omitted`
      : '',
    '',
    '## Evidence boundary',
    '- This support package comes from the currently loaded task-detail rows and rollout statistics in the browser.',
    '- Device diagnostic links require the backend task-detail row to include deviceId. A missing link means that row cannot be safely deep-linked to the diagnostics page.',
    '- Failure reasons come from the backend status_description field. They are useful triage evidence but are not a proven root cause.',
    '- If you need a complete device archive, export the CSV or reload the detail list.',
    '',
    '## Suggested next steps',
    '- Confirm the failed devices are online and can reach the broker/API.',
    '- Compare the current version, target version, and package compatibility before retrying.',
    '- If many devices share the same reason, fix the shared package, network, or root cause before retrying.',
    '- Record the failure reasons, then retry or cancel the device-level tasks.'
  ]
    .filter((line) => line !== '')
    .join('\n')
}

export const downloadOtaFailureCsv = (
  rows: OtaTaskDetailRecord[],
  fallbackReason: string,
  filename = `ota-failed-devices-${Date.now()}.csv`
) => {
  const blob = new Blob(['\uFEFF', buildOtaFailureCsv(rows, fallbackReason)], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')

  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

export const downloadOtaTaskSupportBundleJson = (
  bundle: unknown,
  filename = `aetherlink-ota-task-support-bundle-${Date.now()}.json`
) => {
  const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: 'application/json;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')

  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}
