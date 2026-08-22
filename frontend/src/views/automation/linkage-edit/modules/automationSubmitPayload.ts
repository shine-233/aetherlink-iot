import dayjs from 'dayjs'

/** dayjs 可接受的输入形态 */
type DayjsInput = string | number | Date | null | undefined

/** 事件参数条件行（编辑器数据，字段宽松） */
type EventParamConditionLike = {
  field?: unknown
  operator?: unknown
  value?: unknown
  minValue?: unknown
  maxValue?: unknown
  [key: string]: unknown
}

/** 事件参数匹配的 trigger_value 结构（字符串 JSON 或已解析对象） */
type EventParamMatchValue = {
  match_mode?: unknown
  conditions?: EventParamConditionLike[] | null
  [key: string]: unknown
}

/** 联动规则条件行（编辑器数据，字段宽松，运行时逐个校验） */
type SubmitConditionItem = {
  trigger_conditions_type?: unknown
  trigger_param_type?: unknown
  trigger_operator?: unknown
  trigger_value?: string | EventParamMatchValue | null
  execution_time?: unknown
  task_type?: unknown
  params?: unknown
  eventParamConditions?: EventParamConditionLike[] | null
  weekChoseValue?: unknown[]
  startTimeValue?: DayjsInput
  endTimeValue?: DayjsInput
  onceTimeValue?: DayjsInput
  hourTimeValue?: DayjsInput
  dayTimeValue?: DayjsInput
  weekTimeValue?: DayjsInput
  monthTimeValue?: DayjsInput
  monthChoseValue?: unknown
  minValue?: unknown
  maxValue?: unknown
  field?: unknown
  operator?: unknown
  value?: unknown
  ifType?: unknown
  [key: string]: unknown
}

/** 动作指令行（编辑器数据，字段宽松） */
type SubmitActionItem = {
  action_param_type?: unknown
  action_param?: string
  actionValue?: unknown
  action_value?: unknown
  [key: string]: unknown
}

/** 动作组行（编辑器数据，字段宽松） */
type SubmitActionGroupItem = {
  actionType?: unknown
  actionInstructList?: SubmitActionItem[] | null
  action_type?: unknown
  [key: string]: unknown
}

export function cloneAutomationEditorData<T>(value: T): T {
  return JSON.parse(JSON.stringify(value || []))
}

export function buildEventParamConditionValue(condition: EventParamConditionLike) {
  const operator = condition.operator || '='
  const field = typeof condition.field === 'string' ? condition.field.trim() : condition.field
  let value = condition.value
  if (operator === 'between') {
    value = [condition.minValue, condition.maxValue]
  } else if (operator === 'in') {
    value = Array.isArray(condition.value)
      ? condition.value
      : String(condition.value || '')
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean)
  } else if (operator === 'exists') {
    value = condition.value !== false
  }

  return {
    field,
    operator,
    value
  }
}

export function applyEventTriggerValue(ifItem: SubmitConditionItem) {
  ifItem.trigger_operator = '='
  ifItem.trigger_value = JSON.stringify({
    match_mode: 'field',
    conditions: (ifItem.eventParamConditions || []).map(buildEventParamConditionValue)
  })
}

export function applyTimeTriggerValue(ifItem: SubmitConditionItem) {
  if (ifItem.trigger_conditions_type === '22') {
    let triggerValue = ''
    ;(ifItem.weekChoseValue || []).forEach(item => {
      triggerValue += item
    })
    triggerValue += `|${dayjs(ifItem.startTimeValue).format('HH:mm:ssZ')}`
    triggerValue += `|${dayjs(ifItem.endTimeValue).format('HH:mm:ssZ')}`
    ifItem.trigger_value = triggerValue
  }
  if (ifItem.trigger_conditions_type === '20') {
    ifItem.execution_time = dayjs(ifItem.onceTimeValue).format()
  }
  if (ifItem.trigger_conditions_type === '21') {
    if (ifItem.task_type === 'HOUR') {
      ifItem.params = dayjs(ifItem.hourTimeValue).format('mm:00Z')
    }
    if (ifItem.task_type === 'DAY') {
      ifItem.params = dayjs(ifItem.dayTimeValue).format('HH:mm:00Z')
    }
    if (ifItem.task_type === 'WEEK') {
      let params = ''
      ;(ifItem.weekChoseValue || []).forEach(item => {
        params += item
      })
      ifItem.params = `${params}|${dayjs(ifItem.weekTimeValue).format('HH:mm:00Z')}`
    }
    if (ifItem.task_type === 'MONTH') {
      ifItem.params = `${ifItem.monthChoseValue}T${dayjs(ifItem.monthTimeValue).format('HH:mm:00Z')}`
    }
  }
}

export function normalizeSubmitConditionItem(ifItem: SubmitConditionItem) {
  if (ifItem.trigger_conditions_type === '10' || ifItem.trigger_conditions_type === '11') {
    if (ifItem.trigger_param_type === 'event') {
      applyEventTriggerValue(ifItem)
    } else if (ifItem.trigger_operator === 'between') {
      ifItem.trigger_value = `${ifItem.minValue}-${ifItem.maxValue}`
    }
  }

  applyTimeTriggerValue(ifItem)
  return ifItem
}

export function buildSubmitConditionGroups(ifGroupsData: SubmitConditionItem[][]) {
  const ifGroups = cloneAutomationEditorData(ifGroupsData)
  ifGroups.forEach(ifGroupItem => {
    ifGroupItem.forEach(normalizeSubmitConditionItem)
  })
  return ifGroups
}

export function normalizeSubmitActionItem(instructItem: SubmitActionItem): SubmitActionItem {
  if (
    instructItem.action_param_type === 'c_telemetry' ||
    instructItem.action_param_type === 'c_attribute' ||
    instructItem.action_param_type === 'c_command'
  ) {
    instructItem.action_value = instructItem.actionValue
  }
  if (instructItem.action_param_type === 'telemetry' || instructItem.action_param_type === 'attributes') {
    instructItem.action_value = JSON.stringify({
      [String(instructItem.action_param)]: instructItem.actionValue
    })
  }
  if (instructItem.action_param_type === 'command') {
    instructItem.action_value = JSON.stringify({
      method: instructItem.action_param,
      params: instructItem.actionValue
    })
  }

  return instructItem
}

export function buildSubmitActions(actionGroupsData: SubmitActionGroupItem[]) {
  const actionGroups = cloneAutomationEditorData(actionGroupsData)
  const actionsData: SubmitActionItem[] = []

  actionGroups.forEach(item => {
    if (item.actionType === '1') {
      ;(item.actionInstructList || []).forEach(instructItem => {
        actionsData.push(normalizeSubmitActionItem(instructItem))
      })
    } else {
      item.action_type = item.actionType
      actionsData.push(item)
    }
  })

  return actionsData
}

export function hasOnlyTimeRangeConditionGroup(conditionGroups: SubmitConditionItem[][]) {
  return conditionGroups.some(group =>
    group.every(condition => condition.trigger_conditions_type === '22')
  )
}

export function hasScheduleConditionWithAlarmAction(
  conditionGroups: SubmitConditionItem[][],
  actions: SubmitActionItem[]
) {
  return (
    conditionGroups.some(group => group.some(condition => condition.ifType === '2')) &&
    actions.some(action => action.actionType === '30' || action.action_type === '30')
  )
}

export function hasEmptyEventParamMatchCondition(conditionGroups: SubmitConditionItem[][] = []) {
  return conditionGroups.some(group =>
    group.some(condition => {
      const triggerParamType = String(condition.trigger_param_type || '').toUpperCase()
      if (triggerParamType !== 'EVENT' && triggerParamType !== 'EVT') {
        return false
      }

      try {
        const triggerValue =
          typeof condition.trigger_value === 'string' ? JSON.parse(condition.trigger_value || '{}') : condition.trigger_value
        if (triggerValue?.match_mode !== 'field') {
          return true
        }
        if (!Array.isArray(triggerValue.conditions) || triggerValue.conditions.length === 0) {
          return true
        }
        return triggerValue.conditions.some(eventCondition => !isValidEventParamConditionValue(eventCondition))
      } catch {
        return true
      }
    })
  )
}

function isValidEventParamConditionValue(condition: EventParamConditionLike) {
  const field = typeof condition?.field === 'string' ? condition.field.trim() : ''
  const operator = typeof condition?.operator === 'string' && condition.operator ? condition.operator : '='
  const value = condition?.value

  if (!field) {
    return false
  }

  if (operator === 'exists') {
    return typeof value === 'boolean'
  }

  if (operator === 'between') {
    return Array.isArray(value) && value.length === 2 && value.every((item) => !isBlankEventParamValue(item))
  }

  if (operator === 'in') {
    return Array.isArray(value) && value.length > 0 && value.every((item) => !isBlankEventParamValue(item))
  }

  return !isBlankEventParamValue(value)
}

function isBlankEventParamValue(value: unknown) {
  return value == null || (typeof value === 'string' && value.trim() === '')
}
