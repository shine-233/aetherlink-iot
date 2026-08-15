/**
 * 文件用途: 提供 DynamicParameterEditor 的纯参数合并辅助函数。
 * 核心逻辑: 从 API 模板元数据构造参数，并按 key 合并到现有参数列表。
 * 关键注意事项: API 模板种子值允许 0、false 和空字符串，合并时必须保留既有参数 _id。
 * 重构建议: 后续可继续把设备参数合并与表单校验拆到独立 helper。
 */
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'

export type ApiTemplateParameterType = 'header' | 'query' | 'path'

export type ApiTemplateParamSource = Record<string, any> & {
  name: string
  description?: string
  type?: string
  paramType?: string
  location?: string
  in?: string
  example?: any
  defaultValue?: any
}

export type ApiTemplateInfo = {
  url: string
  commonParams?: ApiTemplateParamSource[]
  pathParamNames?: string[]
}

export type CreateDefaultParameter = () => EnhancedParameter

export const resolveApiTemplateSeedValue = (param: Pick<ApiTemplateParamSource, 'example' | 'defaultValue'>) => {
  return param.example ?? param.defaultValue ?? ''
}

const isHeaderParam = (param: ApiTemplateParamSource) =>
  param.type === 'header' || param.paramType === 'header' || param.location === 'header' || param.in === 'header'

export const filterApiTemplateParams = (
  apiInfo: Pick<ApiTemplateInfo, 'commonParams' | 'pathParamNames'>,
  parameterType: ApiTemplateParameterType
) => {
  const commonParams = apiInfo.commonParams || []
  const pathParamNames = apiInfo.pathParamNames || []

  if (parameterType === 'query') {
    return commonParams.filter(param => !pathParamNames.includes(param.name) && !isHeaderParam(param))
  }

  if (parameterType === 'path') {
    return commonParams.filter(param => pathParamNames.includes(param.name))
  }

  if (parameterType === 'header') {
    return commonParams.filter(isHeaderParam)
  }

  return commonParams
}

const resolveApiTemplateDataType = (type: string | undefined): EnhancedParameter['dataType'] => {
  if (type === 'number') return 'number'
  if (type === 'boolean') return 'boolean'
  return 'string'
}

const buildFallbackTemplateParameter = (apiInfo: ApiTemplateInfo, createDefaultParameter: CreateDefaultParameter) => {
  const defaultParam = createDefaultParameter()

  if (apiInfo.url.includes('device')) {
    defaultParam.key = 'deviceId'
    defaultParam.description = '设备ID'
  } else if (apiInfo.url.includes('group')) {
    defaultParam.key = 'groupId'
    defaultParam.description = '分组ID'
  } else if (apiInfo.url.includes('user')) {
    defaultParam.key = 'userId'
    defaultParam.description = '用户ID'
  } else {
    defaultParam.key = 'id'
    defaultParam.description = '标识符'
  }

  defaultParam.selectedTemplate = 'manual'
  defaultParam.valueMode = 'manual'
  return defaultParam
}

export const buildApiTemplateParameters = (
  apiInfo: ApiTemplateInfo,
  parameterType: ApiTemplateParameterType,
  createDefaultParameter: CreateDefaultParameter
) => {
  if (apiInfo.commonParams && apiInfo.commonParams.length > 0) {
    return filterApiTemplateParams(apiInfo, parameterType).map(param => {
      const enhancedParam = createDefaultParameter()
      const seededValue = resolveApiTemplateSeedValue(param)
      enhancedParam.key = param.name
      enhancedParam.description = param.description || `${param.name}参数`
      enhancedParam.dataType = resolveApiTemplateDataType(param.type)
      enhancedParam.selectedTemplate = 'manual'
      enhancedParam.valueMode = 'manual'
      enhancedParam.value = seededValue
      enhancedParam.defaultValue = seededValue
      return enhancedParam
    })
  }

  return [buildFallbackTemplateParameter(apiInfo, createDefaultParameter)]
}

export const mergeTemplateParameters = (existingParams: EnhancedParameter[], templateParams: EnhancedParameter[]) => {
  const existingKeys = new Set(existingParams.map(param => param.key))
  const newParams = templateParams.filter(templateParam => !existingKeys.has(templateParam.key))

  return [
    ...existingParams.map(existingParam => {
      const templateParam = templateParams.find(param => param.key === existingParam.key)
      if (templateParam) {
        return { ...templateParam, _id: existingParam._id }
      }
      return existingParam
    }),
    ...newParams
  ]
}
