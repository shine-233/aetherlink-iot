/**
 * 文件说明:
 * - 集中维护 DynamicParameterEditor 新增参数流程中的纯业务逻辑。
 * - 包括默认参数创建、参数 key 校验、按新增配置生成参数，以及快捷新增选项的参数预设。
 * 维护提示:
 * - 这里不直接调用 message、emit、nextTick 或任何 Vue ref，组件只负责 UI 编排。
 * - 修改默认字段时，需要同步检查参数导入、模板合并和设备参数补水逻辑。
 */
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import { ParameterTemplateType } from '@/core/data-architecture/components/common/templates/index'
import { DEVICE_CONFIG_TEMPLATE_ID, type NewParamConfig } from './dynamicParameterEditorState'

type NewParameterValidationResult =
  | { ok: true; key: string }
  | { ok: false; reason: 'empty' | 'duplicate'; key: string }

/**
 * 创建编辑器内部使用的默认参数。
 *
 * `_id` 用于 Vue 列表稳定追踪，不能替换为业务 key，否则空 key 新增和重复 key 提示会出现错位。
 */
export const createDefaultParameter = (): EnhancedParameter => ({
  key: '',
  value: '',
  enabled: true,
  isDynamic: false,
  valueMode: ParameterTemplateType.MANUAL,
  selectedTemplate: 'manual',
  dataType: 'string',
  variableName: '',
  description: '',
  _id: `param_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`
})

export const validateNewParameterKey = (
  rawKey: string,
  existingParameters: EnhancedParameter[]
): NewParameterValidationResult => {
  const key = rawKey.trim()

  if (!key) {
    return { ok: false, reason: 'empty', key }
  }

  if (existingParameters.some(param => param.key === key)) {
    return { ok: false, reason: 'duplicate', key }
  }

  return { ok: true, key }
}

const applyManualNewParamConfig = (param: EnhancedParameter, config: NewParamConfig) => {
  param.valueMode = ParameterTemplateType.MANUAL
  param.selectedTemplate = 'manual'
  param.value = config.value
  param.isDynamic = false
}

const applyPropertyNewParamConfig = (param: EnhancedParameter, config: NewParamConfig) => {
  param.valueMode = ParameterTemplateType.COMPONENT
  param.selectedTemplate = 'component-property-binding'
  param.value = config.propertyBinding || ''
  param.isDynamic = true
}

const applyDeviceNewParamConfig = (param: EnhancedParameter, config: NewParamConfig) => {
  param.valueMode = ParameterTemplateType.COMPONENT
  param.selectedTemplate = DEVICE_CONFIG_TEMPLATE_ID
  param.value = config.deviceConfig || ''
  param.isDynamic = true
}

const applyNewParamConfigType = (param: EnhancedParameter, config: NewParamConfig) => {
  switch (config.configType) {
    case 'manual':
      applyManualNewParamConfig(param, config)
      break
    case 'property':
      applyPropertyNewParamConfig(param, config)
      break
    case 'device':
      applyDeviceNewParamConfig(param, config)
      break
  }
}

export const buildNewParamFromDrawerConfig = (config: NewParamConfig, key: string) => {
  const newParam = createDefaultParameter()
  newParam.key = key
  newParam.description = config.description || `${newParam.key}参数`
  applyNewParamConfigType(newParam, config)
  return newParam
}

export const createManualAddOptionParameter = () => {
  const newParam = createDefaultParameter()
  newParam.selectedTemplate = 'manual'
  newParam.valueMode = ParameterTemplateType.MANUAL
  return newParam
}

export const createPropertyAddOptionParameter = () => {
  const newParam = createDefaultParameter()
  newParam.selectedTemplate = 'component-property-binding'
  newParam.valueMode = ParameterTemplateType.COMPONENT
  newParam.isDynamic = true
  return newParam
}
