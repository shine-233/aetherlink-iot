import dayjs from 'dayjs'

export function decodeEventParamConditions(triggerValue: string) {
  try {
    const eventMatchConfig = JSON.parse(triggerValue || '{}')
    if (eventMatchConfig.match_mode !== 'field') return []

    return (eventMatchConfig.conditions || []).map((condition: any) => ({
      field: condition.field,
      operator: condition.operator || '=',
      value: condition.operator === 'in' && Array.isArray(condition.value) ? condition.value.join(',') : condition.value,
      minValue: condition.operator === 'between' && Array.isArray(condition.value) ? condition.value[0] : null,
      maxValue: condition.operator === 'between' && Array.isArray(condition.value) ? condition.value[1] : null
    }))
  } catch {
    return []
  }
}

export function normalizeEchoConditionItem(ifItem: any) {
  if (ifItem.trigger_conditions_type === '10' || ifItem.trigger_conditions_type === '11') {
    ifItem.ifType = '1'
    if (ifItem.trigger_param_type === 'event') {
      ifItem.eventParamConditions = decodeEventParamConditions(ifItem.trigger_value)
    } else if (ifItem.trigger_operator === 'between') {
      ifItem.minValue = ifItem.trigger_value.split('-')[0]
      ifItem.maxValue = ifItem.trigger_value.split('-')[1]
    }
    ifItem.trigger_param_key = `${ifItem.trigger_param_type}/${ifItem.trigger_param}`
  }
  if (ifItem.trigger_conditions_type === '22') {
    ifItem.ifType = '2'
    const weekChoseValue = ifItem.trigger_value.split('|')[0]
    ifItem.weekChoseValue = weekChoseValue.split('')
    const startTimeValue = `${String(dayjs().format('YYYY-MM-DD'))} ${ifItem.trigger_value.split('|')[1]}`
    const endTimeValue = `${String(dayjs().format('YYYY-MM-DD'))} ${ifItem.trigger_value.split('|')[2]}`
    ifItem.startTimeValue = dayjs(startTimeValue).valueOf()
    ifItem.endTimeValue = dayjs(endTimeValue).valueOf()
  }
  if (ifItem.trigger_conditions_type === '20') {
    ifItem.ifType = '2'
    ifItem.onceTimeValue = dayjs(ifItem.execution_time).valueOf()
  }
  if (ifItem.trigger_conditions_type === '21') {
    ifItem.ifType = '2'
    if (ifItem.task_type === 'HOUR') {
      const hourTimeValue = `${String(dayjs().format('YYYY-MM-DD HH'))}:${ifItem.params}`
      ifItem.hourTimeValue = dayjs(hourTimeValue).valueOf()
    }
    if (ifItem.task_type === 'DAY') {
      const dayTimeValue = `${String(dayjs().format('YYYY-MM-DD'))} ${ifItem.params}`
      ifItem.dayTimeValue = dayjs(dayTimeValue).valueOf()
    }
    if (ifItem.task_type === 'WEEK') {
      const weekStr = ifItem.params.split('|')[0] || null
      const timeStr = ifItem.params.split('|')[1] || null
      ifItem.weekChoseValue = weekStr.split('')
      const weekTimeValue = `${String(dayjs().format('YYYY-MM-DD'))} ${timeStr}`
      ifItem.weekTimeValue = dayjs(weekTimeValue).valueOf()
    }
    if (ifItem.task_type === 'MONTH') {
      ifItem.monthChoseValue = ifItem.params.split('T')[0] || null
      const monthTimeStr = ifItem.params.split('T')[1] || null
      const monthTimeValue = `${String(dayjs().format('YYYY-MM-DD'))} ${monthTimeStr}`
      ifItem.monthTimeValue = dayjs(monthTimeValue).valueOf()
    }
  }

  return ifItem
}

export function echoConditionGroups(ifData: any[][]) {
  return ifData.map((group: any[]) => group.map(normalizeEchoConditionItem))
}

export function parseActionValue(actionValue: string) {
  try {
    return JSON.parse(actionValue || '{}')
  } catch {
    return {}
  }
}

export function normalizeEchoActionItem(item: any) {
  item.actionParamOptions = []
  const actionValueObj = parseActionValue(item.action_value)
  if (
    item.action_param_type === 'c_telemetry' ||
    item.action_param_type === 'c_attribute' ||
    item.action_param_type === 'c_command'
  ) {
    item.actionValue = item.action_value
  }
  if (item.action_param_type === 'telemetry' || item.action_param_type === 'attributes') {
    item.actionValue = actionValueObj[item.action_param]
  }
  if (item.action_param_type === 'command') {
    item.actionValue = actionValueObj.params
  }

  return item
}

export function echoActionGroups(actionsData: any[]) {
  const actionGroupsData: any[] = []
  const actionInstructList: any[] = []

  actionsData.forEach((item: any) => {
    if (item.action_type === '10' || item.action_type === '11') {
      actionInstructList.push(normalizeEchoActionItem(item))
    } else {
      item.actionType = item.action_type
      actionGroupsData.push(item)
    }
  })
  if (actionInstructList.length > 0) {
    actionGroupsData.push({
      actionType: '1',
      actionInstructList
    })
  }

  return actionGroupsData
}
