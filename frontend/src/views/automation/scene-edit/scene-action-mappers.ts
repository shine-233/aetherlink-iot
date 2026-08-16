/**
 * 文件说明：
 * - 场景动作编辑的结构映射层，负责“表单分组态 <-> 接口 actions 扁平态”双向转换。
 * - 同时沉淀动作参数类型常量，避免页面与子模块散落硬编码。
 * - 这里的提交映射与回显映射需要长期保持对称，是联调与回归的高风险点。
 */
export const OPERATE_DEVICE_ACTION_TYPE = '1'
export const SINGLE_DEVICE_ACTION_TARGET_TYPE = '10'
export const SINGLE_CLASS_DEVICE_ACTION_TARGET_TYPE = '11'

export const ACTION_PARAM_TYPES_WITH_INLINE_JSON = new Set(['c_attribute', 'c_telemetry', 'c_command'])
export const ACTION_PARAM_TYPES_WITH_JSON_VALIDATION = new Set(['command', 'c_attribute', 'c_telemetry', 'c_command'])
export const ACTION_PARAM_TYPES_WITH_KEY_VALUE_PAYLOAD = new Set(['telemetry', 'attributes'])
const RESERVED_ACTION_PARAM_KEYS = new Set(['__proto__', 'prototype', 'constructor'])

export interface ActionParamOption {
  key: string
  value?: string
  label?: string
  data_type?: string | null
  [key: string]: any
}

export interface ActionParamOptionGroup {
  data_source_type: string
  label?: string
  value?: string
  options: ActionParamOption[]
  [key: string]: any
}

export interface ActionParamTypeOption {
  label: string
  value: string
}

export interface SceneInstructionLike {
  action_target: string | null
  action_type: string | null
  action_param_type: string | null
  action_param: string | null
  action_param_key?: string | null
  action_value?: any
  actionValue?: any
  deviceGroupId?: string | null
  actionParamOptions: ActionParamOption[]
  actionParamOptionsData: ActionParamOptionGroup[]
  actionParamTypeOptions: ActionParamTypeOption[]
  actionParamData?: ActionParamOption | null
  showSubSelect?: boolean
  placeholder?: string
  inputFeedback?: string
  inputValidationStatus?: string
  [key: string]: any
}

export interface SceneActionGroupLike {
  actionType: string | null
  action_type?: string | null
  action_target?: string | null
  actionInstructList: SceneInstructionLike[]
  [key: string]: any
}

export const isOperateDeviceInstructionAction = (actionType: any) => {
  return actionType === SINGLE_DEVICE_ACTION_TARGET_TYPE || actionType === SINGLE_CLASS_DEVICE_ACTION_TARGET_TYPE
}

const tryParseActionValueObject = (actionValue: any) => {
  if (typeof actionValue !== 'string') {
    return null
  }
  try {
    const parsed = JSON.parse(actionValue)
    return typeof parsed === 'object' && parsed !== null ? parsed : null
  } catch {
    return null
  }
}

const normalizeActionParamKey = (value: unknown) => {
  const key = typeof value === 'string' ? value.trim() : ''
  if (!key || RESERVED_ACTION_PARAM_KEYS.has(key) || key.length > 128) return null
  return key
}

export const buildActionValuePayload = (instructItem: SceneInstructionLike) => {
  if (ACTION_PARAM_TYPES_WITH_INLINE_JSON.has(instructItem.action_param_type as string)) {
    // c_* 类型约定直接透传 JSON 字符串，不再重复包装。
    return instructItem.actionValue
  }

  if (ACTION_PARAM_TYPES_WITH_KEY_VALUE_PAYLOAD.has(instructItem.action_param_type as string)) {
    const key = normalizeActionParamKey(instructItem.action_param)
    if (!key) return instructItem.action_value
    // Keep the key validation above, but serialize the one-entry JSON object
    // directly instead of assigning a remote value to an object property.
    // JSON.stringify quotes the key and escapes control characters for us.
    const serializedValue = JSON.stringify(instructItem.actionValue)
    return serializedValue === undefined
      ? '{}'
      : `{${JSON.stringify(key)}:${serializedValue}}`
  }

  if (instructItem.action_param_type === 'command') {
    // command 需要保留 method 与 params，两层结构由后端设备指令逻辑消费。
    return JSON.stringify({
      method: instructItem.action_param,
      params: JSON.stringify(JSON.parse(instructItem.actionValue))
    })
  }

  return instructItem.action_value
}

export const formatActionGroupForSubmit = (actionGroupItem: SceneActionGroupLike): any[] => {
  if (actionGroupItem.actionType === OPERATE_DEVICE_ACTION_TYPE) {
    // “操作设备”在 UI 中是一个动作组，但接口层需要拆成多条 action 记录。
    return actionGroupItem.actionInstructList.map((instructItem: SceneInstructionLike) => ({
      ...instructItem,
      action_value: buildActionValuePayload(instructItem)
    }))
  }

  return [
    {
      ...actionGroupItem,
      action_type: actionGroupItem.actionType
    }
  ]
}

export const buildActionsPayload = (actionGroups: SceneActionGroupLike[]) => {
  return actionGroups.flatMap(formatActionGroupForSubmit)
}

export const parseActionValueForEcho = (item: SceneInstructionLike) => {
  if (ACTION_PARAM_TYPES_WITH_INLINE_JSON.has(item.action_param_type as string)) {
    return item.action_value
  }

  const parsed = tryParseActionValueObject(item.action_value)

  if (ACTION_PARAM_TYPES_WITH_KEY_VALUE_PAYLOAD.has(item.action_param_type as string)) {
    return parsed?.[item.action_param as string]
  }

  if (item.action_param_type === 'command') {
    return parsed?.params ?? item.actionValue
  }

  return item.actionValue
}

export const formatActionGroupsForEcho = (actionsData: SceneInstructionLike[]) => {
  // 把接口层扁平 actions 回组装成 UI 可编辑的动作组，供编辑态回显使用。
  const actionGroupsData = [] as SceneActionGroupLike[]
  const actionInstructList = [] as SceneInstructionLike[]

  actionsData.forEach((item: SceneInstructionLike) => {
    if (isOperateDeviceInstructionAction(item.action_type)) {
      actionInstructList.push({
        ...item,
        actionParamOptions: [],
        actionValue: parseActionValueForEcho(item)
      })
      return
    }

    actionGroupsData.push({
      ...item,
      actionType: item.action_type
    } as unknown as SceneActionGroupLike)
  })

  if (actionInstructList.length > 0) {
    actionGroupsData.push({
      actionType: OPERATE_DEVICE_ACTION_TYPE,
      actionInstructList
    })
  }

  return actionGroupsData
}
