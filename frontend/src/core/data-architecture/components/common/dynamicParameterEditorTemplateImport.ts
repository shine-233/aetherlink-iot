/**
 * 文件说明：
 * - 集中维护 DynamicParameterEditor 的接口模板导入纯逻辑。
 * - 负责生成默认占位参数、根据当前接口元数据构造参数，并计算导入后应该聚焦的参数行。
 * 维护提示：
 * - 本文件不调用 emit、nextTick、message、logger，也不读取 Vue ref；组件只根据返回结果更新 UI。
 * - `focusIndex` 为 null 表示没有新增或可聚焦参数，调用方不应强制打开编辑态。
 * 审查建议：
 * - 后续若接口模板支持批量覆盖、冲突预览或撤销，应优先扩展本模块的返回结构。
 */
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import {
  buildApiTemplateParameters,
  mergeTemplateParameters,
  type ApiTemplateInfo,
  type ApiTemplateParameterType,
  type CreateDefaultParameter
} from '@/core/data-architecture/components/common/parameter-editor-helpers'
import { ParameterTemplateType } from '@/core/data-architecture/components/common/templates/index'

export type TemplateImportResult = {
  parameters: EnhancedParameter[]
  focusIndex: number | null
}

export const createDefaultApiTemplatePlaceholder = (createDefaultParameter: CreateDefaultParameter) => {
  const defaultParam = createDefaultParameter()
  defaultParam.key = 'deviceId'
  defaultParam.description = '设备ID（通用参数）'
  defaultParam.selectedTemplate = 'manual'
  defaultParam.valueMode = ParameterTemplateType.MANUAL
  return defaultParam
}

export const buildDefaultTemplateImportResult = (
  existingParams: EnhancedParameter[],
  createDefaultParameter: CreateDefaultParameter
): TemplateImportResult => {
  const placeholder = createDefaultApiTemplatePlaceholder(createDefaultParameter)
  const parameters = [...existingParams, placeholder]

  return {
    parameters,
    focusIndex: parameters.length - 1
  }
}

export const buildCurrentApiTemplateImportResult = (
  existingParams: EnhancedParameter[],
  apiInfo: ApiTemplateInfo,
  parameterType: ApiTemplateParameterType,
  createDefaultParameter: CreateDefaultParameter
): TemplateImportResult => {
  const templateParams = buildApiTemplateParameters(apiInfo, parameterType, createDefaultParameter)
  const parameters = mergeTemplateParameters(existingParams, templateParams)

  return {
    parameters,
    focusIndex: templateParams.length > 0 ? parameters.length - templateParams.length : null
  }
}
