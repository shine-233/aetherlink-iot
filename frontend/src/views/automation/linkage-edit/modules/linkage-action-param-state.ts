import { deviceConfigMetricsMenu, deviceMetricsMenu } from '@/service/api/automation'
import { $t } from '@/locales'

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

const normalizeMetricMenu = (items: any[]) =>
  items.map((item: any) => ({
    ...item,
    value: item.data_source_type,
    label: `${item.data_source_type}${item.label ? `(${item.label})` : ''}`,
    options: item.options.map((subItem: any) => ({
      ...subItem,
      value: subItem.key,
      label: `${subItem.key}${subItem.label ? `(${subItem.label})` : ''}`
    }))
  }))

const selectCurrentParamType = (instructItem: any) => {
  if (!instructItem.action_param_type) return

  instructItem.actionParamOptions =
    instructItem.actionParamOptionsData.find((item: any) => item.data_source_type === instructItem.action_param_type)
      ?.options || []
  instructItem.showSubSelect = shouldShowSubSelect(instructItem.action_param_type)
}

const selectCurrentParam = (instructItem: any) => {
  if (!instructItem.action_param || instructItem.actionParamOptions.length <= 0) return

  instructItem.actionParamData =
    instructItem.actionParamOptions.find((item: any) => item.key === instructItem.action_param) || null
  if (instructItem.actionParamData?.data_type) {
    instructItem.actionParamData.data_type = instructItem.actionParamData.data_type?.toLowerCase()
  }
}

export const useLinkageActionParamState = (message: any) => {
  const actionParamShow = async (instructItem: any) => {
    if (!instructItem.action_target) return

    let res: any = null
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
    if (!res?.data || (Array.isArray(res.data) && res.data.length === 0 && instructItem.actionParamOptions?.length)) {
      selectCurrentParam(instructItem)
      return
    }

    const metricMenu = normalizeMetricMenu(res.data)
    instructItem.actionParamOptionsData = metricMenu
    instructItem.actionParamTypeOptions = metricMenu.map((item: any) => ({
      label: item.label,
      value: item.value
    }))
    selectCurrentParamType(instructItem)
    selectCurrentParam(instructItem)
  }

  const actionParamTypeChange = (instructItem: any, data: string) => {
    instructItem.action_param = null
    instructItem.actionParamData = null
    instructItem.actionParamOptions =
      instructItem.actionParamOptionsData.find((item: any) => item.data_source_type === data)?.options || []
    instructItem.placeholder = placeholderMap[data]
    instructItem.actionValue = null
    instructItem.showSubSelect = shouldShowSubSelect(instructItem.action_param_type)
  }

  const actionParamChange = (instructItem: any, data: any) => {
    instructItem.actionValue = null
    instructItem.actionParamData = instructItem.actionParamOptions.find((item: any) => item.key === data) || null
    if (instructItem.actionParamData?.data_type) {
      instructItem.actionParamData.data_type = instructItem.actionParamData.data_type?.toLowerCase()
    }
  }

  const actionValueChange = (instructItem: any) => {
    if (!JSON_ACTION_PARAM_TYPES.has(instructItem.action_param_type)) return

    try {
      JSON.parse(instructItem.actionValue)
      if (typeof JSON.parse(instructItem.actionValue) === 'object') {
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
