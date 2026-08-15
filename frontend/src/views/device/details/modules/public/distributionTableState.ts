import type { CommandSubmitTracking } from './useDistributionSubmitFlow'

export type Translate = (key: string) => string

export type DistributionListView = {
  rows: any[]
  pageCount: number
}

export function normalizeDistributionListView(data: any, pageSize = 4): DistributionListView {
  const rows = data?.value || data?.list || (Array.isArray(data) ? data : []) || []
  const total = data?.count || data?.total || 0

  return {
    rows,
    pageCount: total ? Math.ceil(total / pageSize) : 0
  }
}

export function shouldDisableDistributionSubmit(options: {
  isCommand?: boolean
  commandValue?: string
  textValue?: string
  isValidJson: (value: string) => boolean
}) {
  if (options.isCommand && !options.commandValue) return true
  if (options.textValue && !options.isValidJson(options.textValue)) return true
  return false
}

export function createDeliveryModeView(isExpected: boolean, waitForResponse: boolean, t: Translate) {
  if (isExpected) {
    return {
      title: t('generate.deliveryModeExpectedTitle'),
      hint: t('generate.deliveryModeExpectedHint')
    }
  }
  if (waitForResponse) {
    return {
      title: t('generate.deliveryModeDirectTitle'),
      hint: t('generate.deliveryModeDirectHint')
    }
  }
  return {
    title: t('generate.deliveryModeImmediateTitle'),
    hint: t('generate.deliveryModeImmediateHint')
  }
}

export function createSubmitTrackingView(tracking: CommandSubmitTracking | null, t: Translate) {
  if (!tracking?.messageId) {
    return {
      visible: false,
      type: 'success' as const,
      text: ''
    }
  }

  const key =
    tracking.logRecorded === false ? 'generate.commandSubmittedLogUnavailable' : 'generate.commandSubmittedWithMessageId'

  return {
    visible: true,
    type: tracking.logRecorded === false ? ('warning' as const) : ('success' as const),
    text: `${t(key)} ${tracking.messageId}`
  }
}
