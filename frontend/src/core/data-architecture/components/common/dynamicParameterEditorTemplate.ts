/**
 * 文件说明:
 * - 集中维护 DynamicParameterEditor 的参数模板切换纯逻辑。
 * - 包括统一设备配置模板识别、模板默认值应用、模板变化动作计划和下拉模板能力查询。
 * 维护提示:
 * - 这里不打开抽屉、不触发 emit，也不读取 Vue props；调用方根据 TemplateChangePlan 决定 UI 行为。
 * - 组件模板默认值不能无条件覆盖已有绑定路径，否则会破坏已保存的组件属性绑定。
 */
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import { generateVariableName } from '@/core/data-architecture/types/http-config'
import {
  getTemplateById,
  ParameterTemplateType,
  type ParameterTemplate
} from '@/core/data-architecture/components/common/templates/index'
import { DEVICE_CONFIG_TEMPLATE_ID } from './dynamicParameterEditorState'

export type TemplateParameterNextAction = 'commit' | 'open-component-editor'

export type TemplateChangePlan =
  | { action: 'noop' }
  | { action: 'open-unified-device-config' }
  | { action: 'commit'; updatedParam: EnhancedParameter }
  | { action: 'open-component-editor'; updatedParam: EnhancedParameter }

type ParameterType = 'header' | 'query' | 'path'

const PARAMETER_TYPE_DISPLAY_NAME: Record<ParameterType, string> = {
  header: '请求头',
  query: '查询',
  path: '路径'
}

export const isUnifiedDeviceConfigTemplate = (templateId: string) => templateId === DEVICE_CONFIG_TEMPLATE_ID

const applyTemplateDefaultValue = (updatedParam: EnhancedParameter, template: ParameterTemplate) => {
  if (template.type === ParameterTemplateType.COMPONENT) {
    if (!updatedParam.value && template.defaultValue !== undefined) {
      updatedParam.value = template.defaultValue
    }

    if (template.defaultValue !== undefined && !updatedParam.defaultValue) {
      updatedParam.defaultValue = template.defaultValue
    }
    return
  }

  if (template.defaultValue !== undefined) {
    updatedParam.value = template.defaultValue
  }
}

const applyPropertyTemplateMetadata = (
  updatedParam: EnhancedParameter,
  sourceParam: EnhancedParameter,
  parameterType: ParameterType
) => {
  if (sourceParam.key) {
    updatedParam.variableName = generateVariableName(sourceParam.key)
    updatedParam.description = updatedParam.description || `${PARAMETER_TYPE_DISPLAY_NAME[parameterType]}参数：${sourceParam.key}`
  }
  updatedParam.isDynamic = true
}

const applyStaticTemplateMetadata = (updatedParam: EnhancedParameter) => {
  updatedParam.variableName = ''
  updatedParam.description = ''
  updatedParam.isDynamic = false
}

const applyTemplateMetadata = (
  updatedParam: EnhancedParameter,
  sourceParam: EnhancedParameter,
  template: ParameterTemplate,
  parameterType: ParameterType
): TemplateParameterNextAction => {
  if (template.type === ParameterTemplateType.PROPERTY) {
    applyPropertyTemplateMetadata(updatedParam, sourceParam, parameterType)
    return 'commit'
  }

  if (template.type === ParameterTemplateType.COMPONENT) {
    updatedParam.isDynamic = true
    return 'open-component-editor'
  }

  applyStaticTemplateMetadata(updatedParam)
  return 'commit'
}

export const prepareTemplateParameterUpdate = (
  param: EnhancedParameter,
  templateId: string,
  template: ParameterTemplate,
  parameterType: ParameterType
) => {
  const updatedParam: EnhancedParameter = {
    ...param,
    selectedTemplate: templateId,
    valueMode: template.type as EnhancedParameter['valueMode']
  }

  applyTemplateDefaultValue(updatedParam, template)
  const nextAction = applyTemplateMetadata(updatedParam, param, template, parameterType)

  return { updatedParam, nextAction }
}

export const buildTemplateChangePlan = (
  param: EnhancedParameter,
  templateId: string,
  parameterType: ParameterType
): TemplateChangePlan => {
  if (isUnifiedDeviceConfigTemplate(templateId)) {
    return { action: 'open-unified-device-config' }
  }

  const template = getTemplateById(templateId)
  if (!template) {
    return { action: 'noop' }
  }

  const { updatedParam, nextAction } = prepareTemplateParameterUpdate(param, templateId, template, parameterType)
  if (nextAction === 'open-component-editor') {
    return { action: 'open-component-editor', updatedParam }
  }

  return { action: 'commit', updatedParam }
}

export const getCurrentTemplateOptions = (param: EnhancedParameter) => {
  if (param.valueMode !== ParameterTemplateType.DROPDOWN || !param.selectedTemplate) return []
  const template = getTemplateById(param.selectedTemplate)
  return template?.options || []
}

export const isCustomInputAllowed = (param: EnhancedParameter) => {
  if (param.valueMode !== ParameterTemplateType.DROPDOWN || !param.selectedTemplate) return false
  const template = getTemplateById(param.selectedTemplate)
  return template?.allowCustom || false
}
