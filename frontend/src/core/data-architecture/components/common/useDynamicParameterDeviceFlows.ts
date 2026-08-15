import { ref } from 'vue'
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import { globalParameterGroupManager } from '@/core/data-architecture/utils/device-parameter-generator'
import {
  buildDeviceConfigMergeCommitPlan,
  buildDeviceParameterGroupDeleteCommitPlan,
  buildDeviceParameterGroupReplaceCommitPlan,
  buildUnifiedDeviceConfigCommitPlan,
  getDeviceParameterSelectorPreset,
  getExistingDeviceParameters as getExistingDeviceParametersFromList,
  type DeviceConfigCommitPlan,
  type DeviceParameterGroupInfo
} from '@/core/data-architecture/components/common/dynamicParameterEditorDeviceGroup'
import { buildDeviceParametersForAvailableSlots } from '@/core/data-architecture/components/common/dynamicParameterEditorDeviceSelection'

type UseDynamicParameterDeviceFlowsOptions = {
  getParameters: () => EnhancedParameter[]
  getMaxParameters: () => number | undefined
  emitParameterUpdate: (parameters: EnhancedParameter[]) => void
  appendParametersAndFocus: (parameters: EnhancedParameter[]) => EnhancedParameter[]
  focusParameterAfterRender: (index: number) => void
}

export function useDynamicParameterDeviceFlows(options: UseDynamicParameterDeviceFlowsOptions) {
  const isAddFromDeviceDrawerVisible = ref(false)
  const isUnifiedDeviceConfigVisible = ref(false)
  const isEditingDeviceConfig = ref(false)
  const isDeviceParameterSelectorVisible = ref(false)
  const editingGroupInfo = ref<DeviceParameterGroupInfo | null>(null)

  const closeAddFromDeviceDrawer = () => {
    isAddFromDeviceDrawerVisible.value = false
  }

  const handleAddFromDevice = (params: any[]) => {
    if (params && params.length > 0) {
      const newParams = buildDeviceParametersForAvailableSlots(
        params,
        options.getParameters().length,
        options.getMaxParameters()
      )
      if (!newParams) {
        return
      }

      options.appendParametersAndFocus(newParams)
    }

    closeAddFromDeviceDrawer()
  }

  const handleDeviceParametersSelected = (parameters: EnhancedParameter[]) => {
    options.appendParametersAndFocus(parameters)
    isDeviceParameterSelectorVisible.value = false
  }

  const closeUnifiedDeviceConfigSelector = () => {
    isUnifiedDeviceConfigVisible.value = false
    isEditingDeviceConfig.value = false
  }

  const closeDeviceParameterSelector = () => {
    isDeviceParameterSelectorVisible.value = false
    editingGroupInfo.value = null
  }

  const openUnifiedDeviceConfigSelector = (isEditing: boolean) => {
    isUnifiedDeviceConfigVisible.value = true
    isEditingDeviceConfig.value = isEditing
  }

  const openDeviceParameterSelector = (groupInfo: DeviceParameterGroupInfo) => {
    editingGroupInfo.value = groupInfo
    isDeviceParameterSelectorVisible.value = true
  }

  const applyDeviceConfigCommitPlan = (plan: DeviceConfigCommitPlan) => {
    options.emitParameterUpdate(plan.parameters)

    if (plan.focusIndex !== null) {
      options.focusParameterAfterRender(plan.focusIndex)
    }
  }

  const handleNewDeviceParametersFromDrawer = (parameters: EnhancedParameter[]) => {
    applyDeviceConfigCommitPlan(buildDeviceConfigMergeCommitPlan(options.getParameters(), parameters))
  }

  const handleUnifiedDeviceConfigGenerated = (parameters: EnhancedParameter[]) => {
    applyDeviceConfigCommitPlan(
      buildUnifiedDeviceConfigCommitPlan(options.getParameters(), parameters, isEditingDeviceConfig.value)
    )
    closeUnifiedDeviceConfigSelector()
  }

  const editDeviceConfig = () => {
    const existingParams = getExistingDeviceParametersFromList(options.getParameters())
    openUnifiedDeviceConfigSelector(existingParams.length > 0)
  }

  const handleParametersUpdated = (data: { groupId: string; parameters: EnhancedParameter[] }) => {
    options.emitParameterUpdate(
      buildDeviceParameterGroupReplaceCommitPlan(
        globalParameterGroupManager,
        options.getParameters(),
        data.groupId,
        data.parameters
      ).parameters
    )
    closeDeviceParameterSelector()
  }

  const editParameterGroup = (param: EnhancedParameter) => {
    const selectorPreset = getDeviceParameterSelectorPreset(param, globalParameterGroupManager)
    if (!selectorPreset) {
      return
    }

    openDeviceParameterSelector(selectorPreset)
  }

  const deleteParameterGroup = (param: EnhancedParameter) => {
    const plan = buildDeviceParameterGroupDeleteCommitPlan(globalParameterGroupManager, options.getParameters(), param)
    if (!plan) return

    options.emitParameterUpdate(plan.parameters)
    globalParameterGroupManager.removeGroup(plan.groupId)
  }

  return {
    isAddFromDeviceDrawerVisible,
    isUnifiedDeviceConfigVisible,
    isEditingDeviceConfig,
    isDeviceParameterSelectorVisible,
    editingGroupInfo,
    openUnifiedDeviceConfigSelector,
    closeAddFromDeviceDrawer,
    handleAddFromDevice,
    handleDeviceParametersSelected,
    closeUnifiedDeviceConfigSelector,
    closeDeviceParameterSelector,
    handleNewDeviceParametersFromDrawer,
    handleUnifiedDeviceConfigGenerated,
    getExistingDeviceParameters: () => getExistingDeviceParametersFromList(options.getParameters()),
    editDeviceConfig,
    handleParametersUpdated,
    editParameterGroup,
    deleteParameterGroup
  }
}
