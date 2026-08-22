import {
  createSelectedTriggerParamState,
  isTriggerParamPathSelected,
  type TriggerParamSelectionState
} from './premise-trigger-param-state'
import { prepareEchoedIfGroups } from './premise-edit-premise-state'
import {
  loadTriggerParamOptionsForIfItem,
  type TriggerParamOptionsLoadDeps
} from './premise-trigger-param-options'

type LoadTriggerParamOptionsDeps = Pick<
  TriggerParamOptionsLoadDeps,
  'deviceMetricsConditionMenu' | 'configMetricsConditionMenu' | 'statusOption' | 'syncSelectedEventParams'
> & {
  onError?: (error: unknown) => void
}

/** 触发条件行（编辑器表单数据，字段宽松） */
type TriggerIfItemLike = Record<string, unknown>

type NormalizeIfItemForEcho = (ifItem: TriggerIfItemLike) => void

type InitialConditionResult = {
  judgeItemData: Record<string, unknown>
  deviceConfigDisabled: boolean
}

export const loadPremiseTriggerParamOptions = async (ifItem: TriggerIfItemLike, deps: LoadTriggerParamOptionsDeps) => {
  await loadTriggerParamOptionsForIfItem(ifItem, deps)
}

export const handleTriggerParamOptionsShow = async (
  ifItem: TriggerIfItemLike,
  visible: boolean,
  loadTriggerParamOptions: (ifItem: TriggerIfItemLike) => Promise<void>
) => {
  if (visible) {
    await loadTriggerParamOptions(ifItem)
  }
}

export const applyTriggerParamSelectionChange = (
  ifItem: TriggerIfItemLike,
  data: unknown,
  commitSelection: (ifItem: TriggerIfItemLike, selectionState?: TriggerParamSelectionState) => void
) => {
  if (!isTriggerParamPathSelected(data)) {
    commitSelection(ifItem)
    return
  }

  commitSelection(ifItem, createSelectedTriggerParamState(data as unknown[]))
}

export const applyEchoedConditionData = (
  conditionData: unknown,
  normalizeIfItemForEcho: NormalizeIfItemForEcho,
  loadTriggerParamOptions: (ifItem: TriggerIfItemLike) => void | Promise<void>
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
  judgeItemTemplate: Record<string, unknown>,
  props: {
    deviceId?: string
    deviceConfigId?: string
  },
  createInitialConditionFromProps: (
    judgeItemTemplate: Record<string, unknown>,
    props: { deviceId?: string; deviceConfigId?: string }
  ) => InitialConditionResult
) => {
  return createInitialConditionFromProps(judgeItemTemplate, props)
}

export const applyInitialPremiseCondition = (options: {
  hasConfigId: boolean
  buildInitialCondition: () => InitialConditionResult
  addIfGroupItem: (data: Record<string, unknown>) => void
  emitConditionChose: (data: unknown) => void
  setDeviceConfigDisabled: (value: boolean) => void
}) => {
  if (options.hasConfigId) return

  const { judgeItemData, deviceConfigDisabled } = options.buildInitialCondition()
  options.setDeviceConfigDisabled(deviceConfigDisabled)
  options.emitConditionChose(judgeItemData.trigger_conditions_type)
  options.addIfGroupItem(judgeItemData)
}
