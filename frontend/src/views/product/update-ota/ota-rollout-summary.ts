import type {
  OtaTaskStatisticsItem,
  RolloutGuidanceItem,
  RolloutSummaryItem,
  RolloutSummaryTagType
} from './ota-task-types'

type Translate = (key: string) => string

type OtaTaskStatusDefinition = {
  status: number
  labelKey: string
  tagType: RolloutSummaryTagType
  summaryKey: string
}

const OTA_TASK_STATUSES: OtaTaskStatusDefinition[] = [
  {
    status: 1,
    labelKey: 'page.product.update-ota.pendingTask',
    tagType: 'info',
    summaryKey: 'pending'
  },
  {
    status: 2,
    labelKey: 'page.product.update-ota.pushTask',
    tagType: 'info',
    summaryKey: 'pushed'
  },
  {
    status: 3,
    labelKey: 'page.product.update-ota.upgradingTask',
    tagType: 'warning',
    summaryKey: 'upgrading'
  },
  {
    status: 4,
    labelKey: 'page.product.update-ota.completeTask',
    tagType: 'success',
    summaryKey: 'success'
  },
  {
    status: 5,
    labelKey: 'page.product.update-ota.failTask',
    tagType: 'error',
    summaryKey: 'failed'
  },
  {
    status: 6,
    labelKey: 'page.product.update-ota.cancelTask',
    tagType: 'default',
    summaryKey: 'canceled'
  }
]

export function buildOtaTaskStatusOptions(t: Translate) {
  return [
    { label: t('page.product.update-ota.allStatus'), value: 0 },
    ...OTA_TASK_STATUSES.map((item) => ({
      label: t(item.labelKey),
      value: item.status
    }))
  ]
}

export function getOtaTaskStatusLabel(status: number | undefined, t: Translate) {
  if (!status) return '-'
  const definition = OTA_TASK_STATUSES.find((item) => item.status === status)
  return definition ? t(definition.labelKey) : String(status)
}

export function getOtaTaskStatusTagType(status?: number): RolloutSummaryTagType {
  return OTA_TASK_STATUSES.find((item) => item.status === status)?.tagType || 'info'
}

export function countOtaTaskStatus(statistics: OtaTaskStatisticsItem[], status: number) {
  const item = statistics.find((statistic) => Number(statistic.status) === status)
  return Number(item?.count || 0)
}

export function buildOtaRolloutSummary(statistics: OtaTaskStatisticsItem[], t: Translate) {
  const total = statistics.reduce((sum, item) => sum + Number(item.count || 0), 0)
  const successCount = countOtaTaskStatus(statistics, 4)
  const failedCount = countOtaTaskStatus(statistics, 5)
  const successRate = total ? `${Math.round((successCount / total) * 100)}%` : '0%'

  const statusItems = OTA_TASK_STATUSES.map<RolloutSummaryItem>((item) => ({
    key: item.summaryKey,
    label: t(item.labelKey),
    value: countOtaTaskStatus(statistics, item.status),
    type: item.tagType
  }))

  return {
    total,
    successCount,
    failedCount,
    successRate,
    items: [
      {
        key: 'total',
        label: t('page.product.update-ota.rolloutTotal'),
        value: total,
        type: 'default' as RolloutSummaryTagType
      },
      ...statusItems
    ]
  }
}

export function buildOtaRolloutGuidance(statistics: OtaTaskStatisticsItem[], t: Translate): RolloutGuidanceItem[] {
  const total = statistics.reduce((sum, item) => sum + Number(item.count || 0), 0)
  const pendingCount = countOtaTaskStatus(statistics, 1)
  const pushedCount = countOtaTaskStatus(statistics, 2)
  const upgradingCount = countOtaTaskStatus(statistics, 3)
  const successCount = countOtaTaskStatus(statistics, 4)
  const failedCount = countOtaTaskStatus(statistics, 5)
  const canceledCount = countOtaTaskStatus(statistics, 6)
  const activeCount = pendingCount + pushedCount + upgradingCount

  if (!total) {
    return [
      {
        key: 'no-data',
        title: t('page.product.update-ota.rolloutGuidanceNoDataTitle'),
        description: t('page.product.update-ota.rolloutGuidanceNoDataDesc'),
        value: 0,
        type: 'info'
      }
    ]
  }

  const guidance: RolloutGuidanceItem[] = []

  if (failedCount) {
    guidance.push({
      key: 'retry-failed',
      title: t('page.product.update-ota.rolloutGuidanceRetryTitle'),
      description: t('page.product.update-ota.rolloutGuidanceRetryDesc'),
      value: failedCount,
      type: 'error'
    })
  }

  if (activeCount) {
    guidance.push({
      key: 'monitor-active',
      title: t('page.product.update-ota.rolloutGuidanceMonitorTitle'),
      description: t('page.product.update-ota.rolloutGuidanceMonitorDesc'),
      value: activeCount,
      type: 'warning'
    })
  }

  if (canceledCount) {
    guidance.push({
      key: 'review-canceled',
      title: t('page.product.update-ota.rolloutGuidanceCanceledTitle'),
      description: t('page.product.update-ota.rolloutGuidanceCanceledDesc'),
      value: canceledCount,
      type: 'default'
    })
  }

  if (!guidance.length && successCount === total) {
    guidance.push({
      key: 'completed',
      title: t('page.product.update-ota.rolloutGuidanceCompleteTitle'),
      description: t('page.product.update-ota.rolloutGuidanceCompleteDesc'),
      value: successCount,
      type: 'success'
    })
  }

  return guidance
}
