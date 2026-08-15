/**
 * 文件说明：
 * - 抽离联动条件编辑里“触发参数选择”的纯状态逻辑。
 * - 负责 event 参数条件的初始化、回显归一化、路径键生成与 JSON 值校验。
 * - 这里不直接依赖 Vue，目的是让 edit-premise.vue 只保留视图装配职责。
 */
export type EventParamCondition = {
  field: string | null
  operator: string
  value: unknown
  minValue: unknown
  maxValue: unknown
}

export type TriggerParamSelectionState = {
  triggerParamType?: string | null
  triggerParam?: string | null
  params?: unknown
  eventParamConditions?: EventParamCondition[]
}

export const createTriggerParamKey = (triggerParamType: string | null, triggerParam: string | null) => {
  return triggerParamType && triggerParam ? `${triggerParamType}/${triggerParam}` : null
}

export const createEventParamCondition = (): EventParamCondition => ({
  field: null,
  operator: '=',
  value: null,
  minValue: null,
  maxValue: null
})

export const parseEventParamOptions = (params: unknown) => {
  if (!params) {
    return []
  }

  try {
    // 事件参数来源既可能是字符串化 JSON，也可能已经是数组对象，这里统一兜底。
    const list = typeof params === 'string' ? JSON.parse(params) : params
    if (!Array.isArray(list)) {
      return []
    }

    return list.map((item: any) => ({
      label: `${item.data_identifier}${item.data_name ? `(${item.data_name})` : ''}`,
      value: item.data_identifier,
      dataType: item.read_write_flag || item.param_type || item.data_type || 'String'
    }))
  } catch {
    return []
  }
}

export const createEventParamUiState = (
  triggerParamType: string | null,
  params: unknown,
  eventParamConditions: EventParamCondition[] = []
) => {
  if (triggerParamType !== 'event') {
    return {
      eventParamsRaw: null,
      eventParamOptions: [],
      eventParamConditions: []
    }
  }

  return {
    eventParamsRaw: params || null,
    eventParamOptions: parseEventParamOptions(params),
    eventParamConditions
  }
}

export const applyTriggerParamSelectionState = (
  ifItem: Record<string, any>,
  {
    triggerParamType = null,
    triggerParam = null,
    params = null,
    eventParamConditions = []
  }: TriggerParamSelectionState,
  options: {
    resetComparatorState?: boolean
  } = {}
) => {
  const { resetComparatorState = false } = options

  ifItem.trigger_param_type = triggerParamType
  ifItem.trigger_param = triggerParam
  ifItem.trigger_param_key = createTriggerParamKey(triggerParamType, triggerParam)

  if (resetComparatorState) {
    // 切换触发参数后，旧比较符与旧输入值通常不再成立，需要一起清空。
    ifItem.trigger_operator = null
    ifItem.trigger_value = null
    ifItem.minValue = null
    ifItem.maxValue = null
  }

  Object.assign(ifItem, createEventParamUiState(triggerParamType, params, eventParamConditions))
}

export const isTriggerParamPathSelected = (data: unknown) => {
  return Array.isArray(data) && data.length >= 2
}

export const createSelectedTriggerParamState = (data: any[]) => {
  const triggerParamType = data[0].value
  return {
    triggerParamType,
    triggerParam: data[1].key,
    params: data[1].params || null,
    eventParamConditions: triggerParamType === 'event' ? [createEventParamCondition()] : []
  }
}

export const commitSelectedTriggerParam = (ifItem: Record<string, any>, selectionState: TriggerParamSelectionState = {}) => {
  applyTriggerParamSelectionState(ifItem, selectionState, { resetComparatorState: true })
}

export const normalizeEventConditionUiState = (ifItem: Record<string, any>) => {
  Object.assign(
    ifItem,
    createEventParamUiState(
      ifItem.trigger_param_type,
      ifItem.eventParamsRaw,
      Array.isArray(ifItem.eventParamConditions) ? ifItem.eventParamConditions : []
    )
  )
}

export const normalizeTriggerParamRule = (ifItem: Record<string, any>) => {
  ifItem.trigger_param_key =
    ifItem.trigger_conditions_type === '10' || ifItem.trigger_conditions_type === '11'
      ? createTriggerParamKey(ifItem.trigger_param_type, ifItem.trigger_param)
      : null
}

export const ensureTriggerParamOptionsState = (ifItem: Record<string, any>) => {
  if (!Array.isArray(ifItem.triggerParamOptions)) {
    ifItem.triggerParamOptions = []
  }
}

export const normalizeIfItemForEcho = (ifItem: Record<string, any>) => {
  // 编辑回显时补齐运行期 UI 字段，避免老数据直接渲染时报空。
  ensureTriggerParamOptionsState(ifItem)
  normalizeEventConditionUiState(ifItem)
  normalizeTriggerParamRule(ifItem)
}

export const validateEventTriggerJsonValue = (value: unknown) => {
  try {
    const parsedValue = JSON.parse(String(value))
    return !!parsedValue && typeof parsedValue === 'object'
  } catch {
    return false
  }
}
