/**
 * 文件说明:
 * - 集中维护 DynamicParameterEditor 中设备参数组相关的纯计算逻辑。
 * - 包括参数列表替换/去重、设备配置提交计划、设备参数组替换/删除计划、显示标签生成和编辑选择器回显信息构造。
 * 维护提示:
 * - 这里只读取参数组管理器，不直接调用 removeGroup、emit、message、nextTick 或抽屉状态。
 * - 参数组 ID 与 `_id` 的匹配规则会影响整组编辑和删除，调整前要同步核对设备参数选择器。
 */
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import type {
  DeviceInfo,
  DeviceMetric,
  DeviceParameterSourceType
} from '@/core/data-architecture/types/device-parameter-group'
import type { DeviceParameterGroupManager } from '@/core/data-architecture/utils/device-parameter-generator'
import {
  DEVICE_PARAMETER_GROUP_PREFIX_BY_SOURCE,
  isDeviceRelatedParameter
} from './dynamicParameterEditorState'

type DeviceParameterGroupReader = Pick<DeviceParameterGroupManager, 'getGroup' | 'getGroupParameters'>

export type DeviceParameterGroupInfo = {
  groupId: string
  preSelectedDevice?: DeviceInfo
  preSelectedMetric?: DeviceMetric
  preSelectedMode?: DeviceParameterSourceType
}

export type DeviceConfigCommitPlan = {
  parameters: EnhancedParameter[]
  focusIndex: number | null
}

export type UnifiedDeviceConfigCommitPlan = DeviceConfigCommitPlan

export type DeviceParameterGroupCommitPlan = {
  parameters: EnhancedParameter[]
  groupId: string
}

const replaceParameterSubset = (
  currentParameters: EnhancedParameter[],
  parameters: EnhancedParameter[],
  shouldReplace: (param: EnhancedParameter) => boolean
) => [...currentParameters.filter(param => !shouldReplace(param)), ...parameters]

export const mergeParametersWithDeduplication = (
  currentParameters: EnhancedParameter[],
  newParameters: EnhancedParameter[]
) => {
  const newParamKeys = new Set(newParameters.map(p => p.key))
  return replaceParameterSubset(currentParameters, newParameters, param => newParamKeys.has(param.key))
}

const getGeneratedParametersFocusIndex = (parameters: EnhancedParameter[], generatedParameters: EnhancedParameter[]) =>
  generatedParameters.length > 0 ? parameters.length - generatedParameters.length : null

export const buildDeviceConfigMergeCommitPlan = (
  currentParameters: EnhancedParameter[],
  generatedParameters: EnhancedParameter[]
): DeviceConfigCommitPlan => {
  const parameters = mergeParametersWithDeduplication(currentParameters, generatedParameters)
  return {
    parameters,
    focusIndex: getGeneratedParametersFocusIndex(parameters, generatedParameters)
  }
}

export const mergeUnifiedDeviceConfigParameters = (
  currentParameters: EnhancedParameter[],
  newParameters: EnhancedParameter[],
  isEditingDeviceConfig: boolean
) => {
  if (isEditingDeviceConfig) {
    return replaceParameterSubset(currentParameters, newParameters, isDeviceRelatedParameter)
  }

  return mergeParametersWithDeduplication(currentParameters, newParameters)
}

export const buildUnifiedDeviceConfigCommitPlan = (
  currentParameters: EnhancedParameter[],
  generatedParameters: EnhancedParameter[],
  isEditingDeviceConfig: boolean
): UnifiedDeviceConfigCommitPlan => {
  const parameters = mergeUnifiedDeviceConfigParameters(currentParameters, generatedParameters, isEditingDeviceConfig)

  return {
    parameters,
    focusIndex: getGeneratedParametersFocusIndex(parameters, generatedParameters)
  }
}

export const getExistingDeviceParameters = (parameters: EnhancedParameter[]) => {
  return parameters.filter(isDeviceRelatedParameter)
}

const getDeviceParameterGroupIds = (
  groupManager: DeviceParameterGroupReader,
  groupId: string,
  allParameters: EnhancedParameter[]
) => groupManager.getGroupParameters(groupId, allParameters).map(param => param._id)

export const withoutDeviceParameterGroup = (
  groupManager: DeviceParameterGroupReader,
  allParameters: EnhancedParameter[],
  groupId: string
) => {
  const groupParamIds = new Set(getDeviceParameterGroupIds(groupManager, groupId, allParameters))
  return allParameters.filter(param => !groupParamIds.has(param._id))
}

export const replaceDeviceParameterGroup = (
  groupManager: DeviceParameterGroupReader,
  allParameters: EnhancedParameter[],
  groupId: string,
  parameters: EnhancedParameter[]
) => {
  const groupParamIds = new Set(getDeviceParameterGroupIds(groupManager, groupId, allParameters))
  return replaceParameterSubset(allParameters, parameters, param => groupParamIds.has(param._id))
}

export const buildDeviceParameterGroupReplaceCommitPlan = (
  groupManager: DeviceParameterGroupReader,
  allParameters: EnhancedParameter[],
  groupId: string,
  parameters: EnhancedParameter[]
): DeviceParameterGroupCommitPlan => ({
  groupId,
  parameters: replaceDeviceParameterGroup(groupManager, allParameters, groupId, parameters)
})

export const isDeviceParameterGroup = (param: EnhancedParameter): boolean => {
  return param.parameterGroup?.groupId !== undefined && param.deviceContext?.sourceType === 'device-selection'
}

export const buildDeviceParameterGroupDeleteCommitPlan = (
  groupManager: DeviceParameterGroupReader,
  allParameters: EnhancedParameter[],
  param: EnhancedParameter
): DeviceParameterGroupCommitPlan | null => {
  if (!isDeviceParameterGroup(param)) {
    return null
  }

  const groupId = param.parameterGroup!.groupId
  return {
    groupId,
    parameters: withoutDeviceParameterGroup(groupManager, allParameters, groupId)
  }
}

const getDeviceParameterGroupPrefix = (groupManager: DeviceParameterGroupReader, groupId: string) => {
  const sourceType = groupManager.getGroup(groupId)?.sourceType
  return DEVICE_PARAMETER_GROUP_PREFIX_BY_SOURCE[sourceType || ''] || '🔧 参数'
}

const getDeviceParameterRoleSuffix = (role?: string) => {
  if (role === 'primary') return ' (主)'
  if (role === 'secondary') return ' (次)'
  return ''
}

export const getParameterDisplayLabel = (
  param: EnhancedParameter,
  groupManager: DeviceParameterGroupReader
): string => {
  if (!isDeviceParameterGroup(param)) {
    return param.key || '未命名参数'
  }

  const prefix = getDeviceParameterGroupPrefix(groupManager, param.parameterGroup!.groupId)
  const suffix = getDeviceParameterRoleSuffix(param.parameterGroup?.role)
  return `${prefix}: ${param.key}${suffix}`
}

export const getDeviceParameterSelectorPreset = (
  param: EnhancedParameter,
  groupManager: DeviceParameterGroupReader
): DeviceParameterGroupInfo | null => {
  if (!isDeviceParameterGroup(param)) {
    return null
  }

  const groupId = param.parameterGroup!.groupId
  const groupInfo = groupManager.getGroup(groupId)

  if (!groupInfo) {
    return null
  }

  return {
    groupId,
    preSelectedDevice: groupInfo.sourceConfig.selectedDevice,
    preSelectedMetric: groupInfo.sourceConfig.selectedMetric,
    preSelectedMode: groupInfo.sourceType
  }
}
