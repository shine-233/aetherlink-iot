// 文件用途：联动动作参数选择的共享状态逻辑。
// 核心逻辑：加载指标菜单、回显当前参数并维护参数值校验反馈。
// 关键注意事项：InstructActionItem 字段与编辑页表单模型耦合，调整前需核对模板绑定。
import { deviceConfigMetricsMenu, deviceMetricsMenu } from '@/service/api/automation'
import { $t } from '@/locales'

export type ActionParamOption = {
  key: string
  label?: string
  data_type?: string
}

export type MetricMenuSubItem = {
  key: string
  label?: string
  data_type?: string
}

export type MetricMenuItem = {
  data_source_type: string
  label?: string
  options: MetricMenuSubItem[]
}

export type InstructActionItem = {
  action_target?: unknown
  action_type?: unknown
  action_param_type?: unknown
  action_param?: unknown
  actionValue?: unknown
  placeholder?: unknown
  showSubSelect?: unknown
  inputFeedback?: unknown
  inputValidationStatus?: unknown
  actionParamOptionsData?: unknown[]
  actionParamTypeOptions?: unknown[]
  actionParamOptions?: unknown[]
  actionParamData?: unknown
  [key: string]: unknown
}

const placeholderMap: Record<string, string> = {
  telemetry: '20',
  attributes: 'on-line',
  command: '{"param1":1}',
  c_telemetry: '{"switch":1,"switch1":0}',
  c_attribute: '{"addr":1,"port":0}',
  c_command: '{"method":"switch1","params":{"false":0}}'
}

const JSON_ACTION_PARAM_TYPES = new Set(['command', 'c_attribute', 'c_telemetry', 'c_command'])
const HIDE_SUB_SELECT_PARAM_TYPES = new Set(['c_attribute', 'c_telemetry', 'c_command'])

const shouldShowSubSelect = (paramType: string) => !HIDE_SUB_SELECT_PARAM_TYPES.has(paramType)

const normalizeMetricMenu = (items: MetricMenuItem[]) =>
  items.map(item => ({
    ...item,
    value: item.data_source_type,
    label: `${item.data_source_type}${item.label ? `(${item.label})` : ''}`,
    options: item.options.map(subItem => ({
      ...subItem,
      value: subItem.key,
      label: `${subItem.key}${subItem.label ? `(${subItem.label})` : ''}`
    }))
  }))

const selectCurrentParamType = (instructItem: InstructActionItem) => {
  if (!instructItem.action_param_type) return

  instructItem.actionParamOptions =
    (instructItem.actionParamOptionsData as MetricMenuItem[]).find(
      item => item.data_source_type === instructItem.action_param_type
    )?.options || []
  instructItem.showSubSelect = shouldShowSubSelect(instructItem.action_param_type as string)
}

const selectCurrentParam = (instructItem: InstructActionItem) => {
  if (!instructItem.action_param || instructItem.actionParamOptions!.length <= 0) return

  const currentParam =
    (instructItem.actionParamOptions as MetricMenuSubItem[]).find(
      item => item.key === instructItem.action_param
    ) || null
  instructItem.actionParamData = currentParam
  if (currentParam?.data_type) {
    currentParam.data_type = currentParam.data_type?.toLowerCase()
  }
}

export const useLinkageActionParamState = (message: { error: (content: string) => void }) => {
  const actionParamShow = async (instructItem: InstructActionItem) => {
    if (!instructItem.action_target) return

    let res: { data?: unknown } | null = null
    if (instructItem.action_type === '10') {
      res = await deviceMetricsMenu({ device_id: instructItem.action_target })
    } else if (instructItem.action_type === '11') {
      res = await deviceConfigMetricsMenu({
        device_config_id: instructItem.action_target
      })
    }

    // Echoed forms may already carry a parameter catalog while the optional
    // refresh endpoint returns an empty list (offline/device-specific plugins).
    // Keep that usable state instead of replacing it with an empty selection.
    if (
      !res?.data ||
      (Array.isArray(res.data) && res.data.length === 0 && instructItem.actionParamOptions?.length)
    ) {
      selectCurrentParam(instructItem)
      return
    }

    const metricMenu = normalizeMetricMenu(res.data as MetricMenuItem[])
    instructItem.actionParamOptionsData = metricMenu
    instructItem.actionParamTypeOptions = metricMenu.map(item => ({
      label: item.label,
      value: item.value
    }))
    selectCurrentParamType(instructItem)
    selectCurrentParam(instructItem)
  }

  const actionParamTypeChange = (instructItem: InstructActionItem, data: string) => {
    instructItem.action_param = null
    instructItem.actionParamData = null
    instructItem.actionParamOptions =
      (instructItem.actionParamOptionsData as MetricMenuItem[]).find(item => item.data_source_type === data)
        ?.options || []
    instructItem.placeholder = placeholderMap[data]
    instructItem.actionValue = null
    instructItem.showSubSelect = shouldShowSubSelect(instructItem.action_param_type as string)
  }

  const actionParamChange = (instructItem: InstructActionItem, data: unknown) => {
    instructItem.actionValue = null
    const currentParam =
      (instructItem.actionParamOptions as MetricMenuSubItem[]).find(item => item.key === data) || null
    instructItem.actionParamData = currentParam
    if (currentParam?.data_type) {
      currentParam.data_type = currentParam.data_type?.toLowerCase()
    }
  }

  const actionValueChange = (instructItem: InstructActionItem) => {
    if (!JSON_ACTION_PARAM_TYPES.has(instructItem.action_param_type as string)) return

    try {
      JSON.parse(instructItem.actionValue as string)
      if (typeof JSON.parse(instructItem.actionValue as string) === 'object') {
        instructItem.inputFeedback = ''
        instructItem.inputValidationStatus = undefined
      } else {
        message.error($t('common.enterJson'))
        instructItem.inputValidationStatus = 'error'
      }
    } catch (e) {
      message.error($t('common.enterJson'))
      instructItem.inputValidationStatus = 'error'
    }
  }

  return {
    actionParamShow,
    actionParamTypeChange,
    actionParamChange,
    actionValueChange
  }
}
