import {
  hasEmptyEventParamMatchCondition,
  hasOnlyTimeRangeConditionGroup,
  hasScheduleConditionWithAlarmAction
} from '@/views/automation/linkage-edit/modules/automationSubmitPayload'

type Translate = (key: string) => string

type AutomationSubmitPayload = {
  trigger_condition_groups?: any[]
  actions: any[]
}

type AutomationDryRunService = (payload: any) => Promise<any>
type AutomationDryRunSaveGatePayload = Record<string, any>

type AutomationSaveApi = (payload: any) => Promise<{
  error?: unknown
}>

export type AutomationReturnContext = {
  device_id?: unknown
  device_config_id?: unknown
}

export const getAutomationSubmitBlocker = (payload: AutomationSubmitPayload, t: Translate) => {
  if (hasOnlyTimeRangeConditionGroup((payload.trigger_condition_groups ?? []) as any[][])) {
    return t('generate.timeRangeWarning')
  }

  if (hasScheduleConditionWithAlarmAction((payload.trigger_condition_groups ?? []) as any[][], payload.actions)) {
    return t('generate.timeTypeWarning')
  }

  if (hasEmptyEventParamMatchCondition(payload.trigger_condition_groups)) {
    return t('generate.eventParamConditionRequired')
  }

  return ''
}

export const resolveAutomationPostSaveRoute = (backType: unknown, context: AutomationReturnContext) => {
  if (backType === 'device') {
    return { path: '/device/details', query: { d_id: context.device_id } }
  }

  if (backType === 'config') {
    return { path: '/device/config-detail', query: { id: context.device_config_id } }
  }

  return { path: '/automation/scene-linkage' }
}

export const saveAutomationDefinition = async (options: {
  isEdit: boolean
  payload: AutomationSubmitPayload
  addAutomation: AutomationSaveApi
  editAutomation: AutomationSaveApi
}) => {
  const response = options.isEdit
    ? await options.editAutomation(options.payload)
    : await options.addAutomation(options.payload)

  return Boolean(response && !response.error)
}

export const normalizeAutomationDryRunBlockers = (dryRunData: any) => {
  const blockers = dryRunData?.blocking_errors || dryRunData?.blockers || []
  if (!Array.isArray(blockers)) return []

  return blockers.filter(Boolean).map((item: any) => {
    if (typeof item === 'string') return item
    if (item?.message) return item.message
    if (item?.msg) return item.msg
    if (item?.detail) return item.detail
    if (item?.code) return String(item.code)

    try {
      return JSON.stringify(item)
    } catch {
      return String(item)
    }
  })
}

export const getAutomationDryRunSaveBlocker = (dryRunData: any, fallbackMessage: string) => {
  const blockers = normalizeAutomationDryRunBlockers(dryRunData)
  if (dryRunData?.can_save === false || dryRunData?.canSave === false || blockers.length > 0) {
    return blockers[0] || fallbackMessage
  }

  return ''
}

export const runAutomationDryRunSaveGate = async (options: {
  payload: AutomationDryRunSaveGatePayload
  runBackendDryRunForPayload: AutomationDryRunService
  backendUnavailableMessage: string
  saveBlockedMessage: string
}) => {
  const dryRunData = await options.runBackendDryRunForPayload(options.payload)
  if (!dryRunData) {
    return {
      canSave: false,
      message: options.backendUnavailableMessage
    }
  }

  const blocker = getAutomationDryRunSaveBlocker(dryRunData, options.saveBlockedMessage)
  if (blocker) {
    return {
      canSave: false,
      message: blocker
    }
  }

  return {
    canSave: true,
    message: ''
  }
}
