/**
 * 文件说明：
 * - 承接联动前提中“事件参数条件行”的轻量状态 helper。
 * - 负责参数选项读取、操作符列表切换，以及条件行的增删与值类型联动。
 * 维护提示：
 * - 这里只处理事件参数条件的局部状态，不应重新吸收父组件的设备来源、生命周期或整表单编排逻辑。
 */
import {
  createEventParamCondition,
  createEventParamUiState,
  parseEventParamOptions
} from './premise-trigger-param-state'
import { $t } from '@/locales'

type OperatorOption = {
  label: string
  value: string
}

type Translator = (key: string) => string

const eventParamExistsOperator = (t: Translator) => ({
  label: t('custom.automation.eventParam.existsOperator'),
  value: 'exists'
})

export const buildEventExistsOptions = (t: Translator = $t) => [
  {
    label: t('custom.automation.eventParam.exists'),
    value: true
  },
  {
    label: t('custom.automation.eventParam.notExists'),
    value: false
  }
]

export const getEventParamOptions = (ifItem: any) => {
  if (ifItem.eventParamOptions?.length) {
    return ifItem.eventParamOptions
  }
  return parseEventParamOptions(ifItem.eventParamsRaw)
}

export const resolveSelectedEventParams = (options: any[], triggerParam: string | null) => {
  if (!triggerParam) {
    return null
  }

  const eventGroup = options.find((item: any) => item.value === 'event')
  const selectedEvent = eventGroup?.options?.find((item: any) => item.key === triggerParam)
  return selectedEvent?.params || null
}

export const syncSelectedEventParams = (ifItem: any) => {
  const params = resolveSelectedEventParams(ifItem.triggerParamOptions || [], ifItem.trigger_param)
  if (!params && ifItem.eventParamsRaw) {
    return
  }

  Object.assign(
    ifItem,
    createEventParamUiState(
      ifItem.trigger_param_type,
      params,
      Array.isArray(ifItem.eventParamConditions) ? ifItem.eventParamConditions : []
    )
  )
}

export const getEventParamType = (ifItem: any, field: string) => {
  const option = getEventParamOptions(ifItem).find((item: any) => item.value === field)
  return String(option?.dataType || 'String').toLowerCase()
}

export const getEventOperatorOptions = (
  ifItem: any,
  condition: any,
  determineOptions: OperatorOption[],
  t: Translator = $t
) => {
  const dataType = getEventParamType(ifItem, condition.field)
  if (dataType === 'number') {
    return [...determineOptions, eventParamExistsOperator(t)]
  }
  if (dataType === 'boolean') {
    return determineOptions
      .filter((item) => item.value === '=' || item.value === '!=')
      .concat(eventParamExistsOperator(t))
  }
  return determineOptions
    .filter((item) => item.value === '=' || item.value === '!=' || item.value === 'in')
    .concat(eventParamExistsOperator(t))
}

export const addEventParamCondition = (ifItem: any) => {
  if (!Array.isArray(ifItem.eventParamConditions)) {
    ifItem.eventParamConditions = []
  }
  ifItem.eventParamConditions.push(createEventParamCondition())
}

export const deleteEventParamCondition = (ifItem: any, conditionIndex: number) => {
  ifItem.eventParamConditions.splice(conditionIndex, 1)
}

export const eventConditionOperatorChange = (condition: any) => {
  condition.value = condition.operator === 'exists' ? true : null
  condition.minValue = null
  condition.maxValue = null
}
