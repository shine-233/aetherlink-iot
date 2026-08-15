/**
 * 文件说明：
 * - 承载场景动作编辑表单的轻量状态辅助函数。
 * - 主要解决动作参数下拉、占位文案、联动清空与 JSON 输入校验。
 * - 与 mapper 文件配合使用：这里更偏 UI 状态，mapper 更偏提交/回显结构转换。
 */
import {
  ACTION_PARAM_TYPES_WITH_JSON_VALIDATION,
  ACTION_PARAM_TYPES_WITH_INLINE_JSON,
  OPERATE_DEVICE_ACTION_TYPE,
  type ActionParamOption,
  type ActionParamOptionGroup,
  type ActionParamTypeOption,
  type SceneActionGroupLike,
  type SceneInstructionLike
} from './scene-action-mappers'

export const ACTION_PARAM_PLACEHOLDERS: Record<string, string> = {
  telemetry: '20',
  attributes: 'on-line',
  command: '{"param1":1}',
  c_telemetry: '{"switch":1,"switch1":0}',
  c_attribute: '{"addr":1,"port":0}',
  c_command: '{"method":"switch1","params":{"false":0}}'
}

export const shouldHideActionParamSubSelect = (actionParamType: string | null | undefined) => {
  return ACTION_PARAM_TYPES_WITH_INLINE_JSON.has(actionParamType as string)
}

export const normalizeActionParamDataType = (actionParamData: ActionParamOption | null) => {
  if (actionParamData?.data_type) {
    // 表单组件通常按小写类型分支，这里统一一次，避免各处重复判断。
    actionParamData.data_type = actionParamData.data_type.toLowerCase()
  }

  return actionParamData
}

export const normalizeActionParamOptionsData = (source: ActionParamOptionGroup[] = []): ActionParamOptionGroup[] => {
  return source.map(item => ({
    ...item,
    value: item.data_source_type,
    label: `${item.label ? `(${item.label})` : ''}${item.data_source_type}`,
    options: (item.options || []).map((subItem: ActionParamOption) => ({
      ...subItem,
      value: subItem.key,
      label: `${subItem.key}${subItem.label ? `(${subItem.label})` : ''}`
    }))
  }))
}

export const buildActionParamTypeOptions = (actionParamOptionsData: ActionParamOptionGroup[] = []): ActionParamTypeOption[] => {
  return actionParamOptionsData.map(item => ({
    label: item.label || item.data_source_type,
    value: item.value || item.data_source_type
  }))
}

export const getActionParamOptionsByType = (
  instructItem: SceneInstructionLike,
  actionParamType = instructItem.action_param_type
) => {
  return (
    instructItem.actionParamOptionsData.find(item => item.data_source_type === actionParamType)?.options || []
  )
}

export const resetInstructionSelection = (instructItem: SceneInstructionLike) => {
  instructItem.action_target = null
  instructItem.action_param_type = null
  instructItem.action_param = null
  instructItem.action_param_key = null
  instructItem.action_value = null
}

export const resetInstructionTargetDependentState = (instructItem: SceneInstructionLike) => {
  // 切换动作目标后，下游参数来源、选项和输入值都必须一起失效。
  instructItem.action_param_type = null
  instructItem.action_param = null
  instructItem.actionValue = null
  instructItem.actionParamOptionsData = []
  instructItem.actionParamTypeOptions = []
  instructItem.actionParamOptions = []
}

export const updateInstructActionParamState = (instructItem: SceneInstructionLike) => {
  if (instructItem.action_param_type) {
    // 某些参数类型直接填写 JSON，不需要再展示二级字段选择框。
    instructItem.actionParamOptions = getActionParamOptionsByType(instructItem)
    instructItem.showSubSelect = !shouldHideActionParamSubSelect(instructItem.action_param_type)
  }

  if (instructItem.action_param && instructItem.actionParamOptions.length > 0) {
    instructItem.actionParamData = normalizeActionParamDataType(
      instructItem.actionParamOptions.find(item => item.key === instructItem.action_param) || null
    )
  }
}

export const applyActionParamOptionsData = (
  instructItem: SceneInstructionLike,
  actionParamOptionsData: ActionParamOptionGroup[]
) => {
  instructItem.actionParamOptionsData = actionParamOptionsData
  instructItem.actionParamTypeOptions = buildActionParamTypeOptions(actionParamOptionsData)
  updateInstructActionParamState(instructItem)
}

export const applyActionParamTypeChange = (instructItem: SceneInstructionLike, data: string) => {
  instructItem.action_param = null
  instructItem.actionParamData = null
  instructItem.actionParamOptions = getActionParamOptionsByType(instructItem, data)
  instructItem.placeholder = ACTION_PARAM_PLACEHOLDERS[data]
  instructItem.actionValue = null
  instructItem.showSubSelect = !shouldHideActionParamSubSelect(data)
}

export const applyActionParamSelection = (instructItem: SceneInstructionLike, data: string) => {
  instructItem.actionValue = null
  instructItem.actionParamData = normalizeActionParamDataType(
    instructItem.actionParamOptions.find(item => item.key === data) || null
  )
}

export const validateJsonActionValue = (actionParamType: string | null | undefined, actionValue: unknown) => {
  if (!actionParamType || !ACTION_PARAM_TYPES_WITH_INLINE_JSON.has(actionParamType) && actionParamType !== 'command') {
    return true
  }

  try {
    // 这里只做“是否为合法 JSON 对象”校验，字段语义仍由后端或更高层约束。
    const parsed = JSON.parse(String(actionValue))
    return typeof parsed === 'object' && parsed !== null
  } catch {
    return false
  }
}

export type SceneActionValueValidationIssue = {
  actionGroupIndex: number
  instructIndex: number
  actionParamType: string
  message: string
  instructItem: SceneInstructionLike
}

export const clearActionValueValidationState = (instructItem: SceneInstructionLike) => {
  instructItem.inputFeedback = ''
  instructItem.inputValidationStatus = undefined
}

export const markInvalidJsonActionValue = (
  instructItem: SceneInstructionLike,
  message = 'common.enterJson'
) => {
  instructItem.inputFeedback = message
  instructItem.inputValidationStatus = 'error'
}

export const validateSceneActionJsonValues = (
  actionGroups: SceneActionGroupLike[] = [],
  message = 'common.enterJson'
): SceneActionValueValidationIssue[] => {
  const issues: SceneActionValueValidationIssue[] = []

  actionGroups.forEach((actionGroup, actionGroupIndex) => {
    if (actionGroup.actionType !== OPERATE_DEVICE_ACTION_TYPE) return

    actionGroup.actionInstructList.forEach((instructItem, instructIndex) => {
      const actionParamType = instructItem.action_param_type
      if (!actionParamType || !ACTION_PARAM_TYPES_WITH_JSON_VALIDATION.has(actionParamType)) return

      if (validateJsonActionValue(actionParamType, instructItem.actionValue)) {
        clearActionValueValidationState(instructItem)
        return
      }

      markInvalidJsonActionValue(instructItem, message)
      issues.push({
        actionGroupIndex,
        instructIndex,
        actionParamType,
        message,
        instructItem
      })
    })
  })

  return issues
}
