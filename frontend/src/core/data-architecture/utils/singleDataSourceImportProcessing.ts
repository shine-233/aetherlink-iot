import { smartDeepClone } from '@/utils/deep-clone'
import { restorePlaceholderDeep } from './configurationImportExportShared'

interface SingleDataSourceImportPayload {
  dataSourceConfig: any
  relatedConfig: {
    interactions: any
    httpBindings: any
  }
}

function hasHttpParameterTemplateMarker(value: any): boolean {
  return !!(value && typeof value === 'object' && ('valueMode' in value || 'selectedTemplate' in value))
}

function detectIsDynamicParameter(param: any): boolean {
  const hasBindingFeatures =
    param.valueMode === 'component' ||
    param.selectedTemplate === 'component-property-binding' ||
    (typeof param.value === 'string' &&
      param.value.includes('.') &&
      param.value.split('.').length >= 3 &&
      param.value.length > 15) ||
    (param.variableName && param.variableName.includes('_') && param.variableName.length > 5) ||
    (param.description &&
      (param.description.includes('绑定') ||
        param.description.includes('属性') ||
        param.description.includes('component')))

  if (hasBindingFeatures) {
    return true
  }

  return param.isDynamic !== undefined ? param.isDynamic : false
}

function normalizeImportedHttpParameter(value: any): any {
  if (!hasHttpParameterTemplateMarker(value)) {
    return value
  }

  return {
    ...value,
    isDynamic: detectIsDynamicParameter(value)
  }
}

function protectParameterBindingPaths(params: any[]): any[] {
  if (!params || !Array.isArray(params)) return params

  return params.map((param) => {
    if (!param.isDynamic && !param.selectedTemplate && !param.valueMode) {
      return param
    }

    const isBindingCorrupted =
      param.value &&
      typeof param.value === 'string' &&
      !param.value.includes('.') &&
      param.value.length < 10 &&
      param.variableName &&
      param.variableName.includes('_')

    if (isBindingCorrupted && param.variableName.includes('_')) {
      const lastUnderscoreIndex = param.variableName.lastIndexOf('_')
      if (lastUnderscoreIndex > 0) {
        const componentId = param.variableName.substring(0, lastUnderscoreIndex)
        const propertyName = param.variableName.substring(lastUnderscoreIndex + 1)
        const reconstructedPath = `${componentId}.base.${propertyName}`

        return {
          ...param,
          value: reconstructedPath,
          isDynamic: true
        }
      }
    }

    return param
  })
}

function normalizeImportedHttpParameterArray(values: any[]): any[] {
  const normalizedArray = values.map((item) => normalizeImportedHttpParameter(item))
  return protectParameterBindingPaths(normalizedArray)
}

function normalizeImportedHttpParameterObject(value: Record<string, any>): any {
  if (!hasHttpParameterTemplateMarker(value)) {
    return value
  }

  return protectParameterBindingPaths([normalizeImportedHttpParameter(value)])[0]
}

function restoreImportedDataWithCurrentComponent(
  value: any,
  targetComponentId: string,
  currentComponentPlaceholder: string
): any {
  return restorePlaceholderDeep(value, currentComponentPlaceholder, targetComponentId, {
    normalizeArray: (restoredArray) => normalizeImportedHttpParameterArray(restoredArray),
    normalizeObject: (restoredObject) => normalizeImportedHttpParameterObject(restoredObject)
  })
}

export function processSingleDataSourceImportPayload<T extends SingleDataSourceImportPayload>(
  importData: T,
  targetComponentId: string,
  currentComponentPlaceholder: string
): T {
  const processedData = smartDeepClone(importData) as T

  processedData.dataSourceConfig = restoreImportedDataWithCurrentComponent(
    processedData.dataSourceConfig,
    targetComponentId,
    currentComponentPlaceholder
  )

  processedData.relatedConfig.interactions = restoreImportedDataWithCurrentComponent(
    processedData.relatedConfig.interactions,
    targetComponentId,
    currentComponentPlaceholder
  )
  processedData.relatedConfig.httpBindings = restoreImportedDataWithCurrentComponent(
    processedData.relatedConfig.httpBindings,
    targetComponentId,
    currentComponentPlaceholder
  )

  return processedData
}
