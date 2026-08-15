import {
  createSelectedTriggerParamState,
  isTriggerParamPathSelected,
  type TriggerParamSelectionState
} from './premise-trigger-param-state'
import { prepareEchoedIfGroups } from './premise-edit-premise-state'
import { loadTriggerParamOptionsForIfItem } from './premise-trigger-param-options'

type LoadTriggerParamOptionsDeps = {
  deviceMetricsConditionMenu: (payload: Record<string, any>) => Promise<{ data?: any[] } | null | undefined>
  configMetricsConditionMenu: (payload: Record<string, any>) => Promise<{ data?: any[] } | null | undefined>
  statusOption: any
  syncSelectedEventParams: (ifItem: any) => void
  onError?: (error: unknown) => void
}

type NormalizeIfItemForEcho = (ifItem: any) => void

type InitialConditionResult = {
  judgeItemData: any
  deviceConfigDisabled: boolean
}

export const loadPremiseTriggerParamOptions = async (ifItem: any, deps: LoadTriggerParamOptionsDeps) => {
  await loadTriggerParamOptionsForIfItem(ifItem, deps)
}

export const handleTriggerParamOptionsShow = async (
  ifItem: any,
  visible: boolean,
  loadTriggerParamOptions: (ifItem: any) => Promise<void>
) => {
  if (visible) {
    await loadTriggerParamOptions(ifItem)
  }
}

export const applyTriggerParamSelectionChange = (
  ifItem: any,
  data: unknown,
  commitSelection: (ifItem: any, selectionState?: TriggerParamSelectionState) => void
) => {
  if (!isTriggerParamPathSelected(data)) {
    commitSelection(ifItem)
    return
  }

  commitSelection(ifItem, createSelectedTriggerParamState(data as any[]))
}

export const applyEchoedConditionData = (
  conditionData: any,
  normalizeIfItemForEcho: NormalizeIfItemForEcho,
  loadTriggerParamOptions: (ifItem: any) => void | Promise<void>
) => {
  const ifGroups = prepareEchoedIfGroups(conditionData, normalizeIfItemForEcho)
  if (!ifGroups) {
    return null
  }

  ifGroups.forEach((ifGroup) => {
    if (Array.isArray(ifGroup)) {
      ifGroup.forEach((ifItem) => {
        loadTriggerParamOptions(ifItem)
      })
    }
  })

  return ifGroups
}

export const buildInitialPremiseCondition = (
  judgeItemTemplate: any,
  props: {
    deviceId?: string
    deviceConfigId?: string
  },
  createInitialConditionFromProps: (
    judgeItemTemplate: any,
    props: { deviceId?: string; deviceConfigId?: string }
  ) => InitialConditionResult
) => {
  return createInitialConditionFromProps(judgeItemTemplate, props)
}

export const applyInitialPremiseCondition = (options: {
  hasConfigId: boolean
  buildInitialCondition: () => InitialConditionResult
  addIfGroupItem: (data: any) => void
  emitConditionChose: (data: any) => void
  setDeviceConfigDisabled: (value: boolean) => void
}) => {
  if (options.hasConfigId) return

  const { judgeItemData, deviceConfigDisabled } = options.buildInitialCondition()
  options.setDeviceConfigDisabled(deviceConfigDisabled)
  options.emitConditionChose(judgeItemData.trigger_conditions_type)
  options.addIfGroupItem(judgeItemData)
}
