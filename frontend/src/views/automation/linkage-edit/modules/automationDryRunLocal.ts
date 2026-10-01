/**
 * 不依赖后端响应的本地说明：状态文案、错误文案、表单条件/动作摘要。
 */
import type { SceneAutomationDryRunConditionGroup, SceneAutomationDryRunNode } from '@/service/api/automation'
import { formatPreviewValue } from './automationDryRunFormat'
import type {
  AutomationConditionSummaryGroup,
  AutomationDryRunLine,
  AutomationDryRunTone,
  BackendDryRunStatus
} from './automationDryRunTypes'

/** 预演接口错误形态（兼容 axios 包装错误与后端包装错误） */
type PreviewErrorLike = {
  error?: { message?: string } | null
  message?: string
  response?: { data?: { message?: string } | null } | null
}

/** 触发条件行（表单/后端载荷，字段宽松，运行时逐个校验） */
type TriggerConditionLike = {
  trigger_conditions_type?: unknown
  execution_time?: unknown
  task_type?: unknown
  params?: unknown
  trigger_value?: unknown
  trigger_source?: unknown
  trigger_param_type?: unknown
  trigger_param?: unknown
  trigger_operator?: unknown
  [key: string]: unknown
}

/** 动作行（表单/后端载荷，字段宽松） */
type DryRunActionLike = {
  action_type?: unknown
  actionType?: unknown
  action_target?: unknown
  action_param_type?: unknown
  action_param?: unknown
  action_value?: unknown
  [key: string]: unknown
}

export const getAutomationDryRunStatusText = (status: BackendDryRunStatus) => {
  if (status === 'pending') return '正在请求后端预演...'
  if (status === 'available') return '后端预演已返回结果。'
  if (status === 'unavailable') return '后端预演暂不可用，仅显示本地说明。'
  if (status === 'ready') return '本地说明已生成，后端预演尚未运行。'

  return '尚未请求后端预演。'
}

export const getAutomationDryRunAlertType = (status: BackendDryRunStatus): AutomationDryRunTone => {
  if (status === 'available') return 'success'
  if (status === 'unavailable') return 'warning'

  return 'info'
}

export const stringifyDryRunResponse = (response: unknown) => {
  if (!response) return ''

  return JSON.stringify(response, null, 2)
}

export const getPreviewErrorText = (error: PreviewErrorLike | null | undefined) => {
  return error?.error?.message || error?.message || error?.response?.data?.message || '后端预演暂不可用。'
}

const CONDITION_TYPE_LABELS: Record<string, string> = {
  '10': 'Single-device condition',
  '11': 'Thing model condition',
  '20': 'One-time schedule',
  '21': 'Recurring schedule',
  '22': 'Time range'
}

const ACTION_TYPE_LABELS: Record<string, string> = {
  '10': 'Single-device action',
  '11': 'Thing model action',
  '20': 'Activate scene',
  '30': 'Trigger alarm'
}

const conditionTypeLabel = (type: unknown) =>
  CONDITION_TYPE_LABELS[String(type)] || `Condition type ${formatPreviewValue(type)}`

const actionTypeLabel = (type: unknown) => ACTION_TYPE_LABELS[String(type)] || `Action type ${formatPreviewValue(type)}`

const describeCondition = (condition: TriggerConditionLike) => {
  const type = conditionTypeLabel(condition.trigger_conditions_type)
  if (condition.trigger_conditions_type === '20') {
    return `${type}: ${formatPreviewValue(condition.execution_time)}`
  }
  if (condition.trigger_conditions_type === '21') {
    return `${type}: ${formatPreviewValue(condition.task_type)} / ${formatPreviewValue(condition.params)}`
  }
  if (condition.trigger_conditions_type === '22') {
    return `${type}: ${formatPreviewValue(condition.trigger_value)}`
  }

  const source = formatPreviewValue(condition.trigger_source)
  const param = `${formatPreviewValue(condition.trigger_param_type)}:${formatPreviewValue(condition.trigger_param)}`
  const operator = formatPreviewValue(condition.trigger_operator)
  const value = formatPreviewValue(condition.trigger_value)

  return `${type}: ${source} / ${param} ${operator} ${value}`
}

const describeAction = (action: DryRunActionLike) => {
  // actionType 是表单草稿（非后端响应）里的旧写法，与预演响应归一化无关
  const type = actionTypeLabel(action.action_type || action.actionType)
  const target = formatPreviewValue(action.action_target)
  if (action.action_type === '10' || action.action_type === '11') {
    const param = `${formatPreviewValue(action.action_param_type)}:${formatPreviewValue(action.action_param)}`
    return `${type}: ${target} / ${param} = ${formatPreviewValue(action.action_value)}`
  }

  return `${type}: ${target}`
}

export const buildConditionSummaryItems = (
  conditionGroups: TriggerConditionLike[][] | SceneAutomationDryRunConditionGroup[]
): AutomationConditionSummaryGroup[] =>
  conditionGroups.map((group, groupIndex) => ({
    key: `condition-group-${groupIndex}`,
    lines: group.map((condition, conditionIndex) => ({
      key: `condition-${groupIndex}-${conditionIndex}`,
      text: describeCondition(condition)
    }))
  }))

export const buildActionSummaryItems = (
  actions: DryRunActionLike[] | SceneAutomationDryRunNode[]
): AutomationDryRunLine[] =>
  actions.map((action, actionIndex) => ({
    key: `action-${actionIndex}`,
    text: describeAction(action)
  }))
