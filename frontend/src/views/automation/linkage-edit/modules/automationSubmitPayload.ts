import dayjs from 'dayjs'

export function cloneAutomationEditorData<T>(value: T): T {
  return JSON.parse(JSON.stringify(value || []))
}

export function buildEventParamConditionValue(condition: any) {
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

export function applyEventTriggerValue(ifItem: any) {
  ifItem.trigger_operator = '='
  ifItem.trigger_value = JSON.stringify({
    match_mode: 'field',
    conditions: (ifItem.eventParamConditions || []).map(buildEventParamConditionValue)
  })
}

export function applyTimeTriggerValue(ifItem: any) {
  if (ifItem.trigger_conditions_type === '22') {
    let triggerValue = ''
    ;(ifItem.weekChoseValue || []).forEach((item: any) => {
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
      ;(ifItem.weekChoseValue || []).forEach((item: any) => {
        params += item
      })
      ifItem.params = `${params}|${dayjs(ifItem.weekTimeValue).format('HH:mm:00Z')}`
    }
    if (ifItem.task_type === 'MONTH') {
      ifItem.params = `${ifItem.monthChoseValue}T${dayjs(ifItem.monthTimeValue).format('HH:mm:00Z')}`
    }
  }
}

export function normalizeSubmitConditionItem(ifItem: any) {
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

export function buildSubmitConditionGroups(ifGroupsData: any[]) {
  const ifGroups = cloneAutomationEditorData(ifGroupsData)
  ifGroups.forEach((ifGroupItem: any[]) => {
    ifGroupItem.forEach(normalizeSubmitConditionItem)
  })
  return ifGroups
}

export function normalizeSubmitActionItem(instructItem: any) {
  if (
    instructItem.action_param_type === 'c_telemetry' ||
    instructItem.action_param_type === 'c_attribute' ||
    instructItem.action_param_type === 'c_command'
  ) {
    instructItem.action_value = instructItem.actionValue
  }
  if (instructItem.action_param_type === 'telemetry' || instructItem.action_param_type === 'attributes') {
    instructItem.action_value = JSON.stringify({
      [instructItem.action_param]: instructItem.actionValue
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

export function buildSubmitActions(actionGroupsData: any[]) {
  const actionGroups = cloneAutomationEditorData(actionGroupsData)
  const actionsData: any[] = []

  actionGroups.forEach((item: any) => {
    if (item.actionType === '1') {
      ;(item.actionInstructList || []).forEach((instructItem: any) => {
        actionsData.push(normalizeSubmitActionItem(instructItem))
      })
    } else {
      item.action_type = item.actionType
      actionsData.push(item)
    }
  })

  return actionsData
}

export function hasOnlyTimeRangeConditionGroup(conditionGroups: any[][]) {
  return conditionGroups.some((group: any[]) =>
    group.every((condition: any) => condition.trigger_conditions_type === '22')
  )
}

export function hasScheduleConditionWithAlarmAction(conditionGroups: any[][], actions: any[]) {
  return (
    conditionGroups.some((group: any[]) => group.some((condition: any) => condition.ifType === '2')) &&
    actions.some((action: any) => action.actionType === '30' || action.action_type === '30')
  )
}

export function hasEmptyEventParamMatchCondition(conditionGroups: any[][] = []) {
  return conditionGroups.some((group: any[]) =>
    group.some((condition: any) => {
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
        return triggerValue.conditions.some((eventCondition: any) => !isValidEventParamConditionValue(eventCondition))
      } catch {
        return true
      }
    })
  )
}

function isValidEventParamConditionValue(condition: any) {
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
