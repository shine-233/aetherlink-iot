import { nextTick, ref } from 'vue'
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import { createPropertyAddOptionParameter } from '@/core/data-architecture/components/common/dynamicParameterEditorNewParam'
import {
  buildTemplateChangePlan,
  type TemplateChangePlan
} from '@/core/data-architecture/components/common/dynamicParameterEditorTemplate'
import {
  getEditingIndexAfterRemoval,
  getFirstAppendedFocusIndex
} from '@/core/data-architecture/components/common/dynamicParameterEditorParameterList'

type ParameterType = 'header' | 'query' | 'path'

type UseDynamicParameterEditSessionOptions = {
  getParameterType: () => ParameterType
  appendParameters: (parameters: EnhancedParameter[]) => EnhancedParameter[]
  updateParameter: (param: EnhancedParameter, index: number) => void
  openUnifiedDeviceConfigSelector: (isEditing: boolean) => void
}

export function useDynamicParameterEditSession(options: UseDynamicParameterEditSessionOptions) {
  const editingIndex = ref(-1)
  const isDrawerVisible = ref(false)
  const drawerParam = ref<EnhancedParameter | null>(null)

  const focusParameterAfterRender = (index: number) => {
    nextTick(() => {
      editingIndex.value = index
    })
  }

  const focusFirstAppendedParameter = (finalParams: EnhancedParameter[], appendedCount: number) => {
    const focusIndex = getFirstAppendedFocusIndex(finalParams.length, appendedCount)
    if (focusIndex !== null) {
      focusParameterAfterRender(focusIndex)
    }
  }

  const openComponentDrawer = (param: EnhancedParameter) => {
    drawerParam.value = { ...param }
    isDrawerVisible.value = true
  }

  const addPropertyParameterFromOption = () => {
    const newParam = createPropertyAddOptionParameter()
    const updatedParams = options.appendParameters([newParam])
    const newParamIndex = updatedParams.length - 1

    editingIndex.value = newParamIndex
    nextTick(() => {
      openComponentDrawer(newParam)
    })
  }

  const openUnifiedDeviceConfigForTemplateEdit = () => {
    editingIndex.value = -1
    options.openUnifiedDeviceConfigSelector(true)
  }

  const openComponentTemplateEditor = (updatedParam: EnhancedParameter, index: number) => {
    editingIndex.value = index
    options.updateParameter(updatedParam, index)

    nextTick(() => {
      openComponentDrawer(updatedParam)
    })
  }

  const executeTemplateChangePlan = (plan: TemplateChangePlan, index: number) => {
    switch (plan.action) {
      case 'noop':
        return
      case 'open-unified-device-config':
        openUnifiedDeviceConfigForTemplateEdit()
        return
      case 'open-component-editor':
        openComponentTemplateEditor(plan.updatedParam, index)
        return
      case 'commit':
        options.updateParameter(plan.updatedParam, index)
    }
  }

  const handleTemplateChange = (param: EnhancedParameter, index: number, templateId: string) => {
    executeTemplateChangePlan(buildTemplateChangePlan(param, templateId, options.getParameterType()), index)
  }

  const handleComponentDrawerSave = (param: EnhancedParameter) => {
    if (editingIndex.value === -1) {
      return
    }

    options.updateParameter(param, editingIndex.value)
  }

  const handleParameterRemoved = (removedIndex: number) => {
    nextTick(() => {
      editingIndex.value = getEditingIndexAfterRemoval(editingIndex.value, removedIndex)
    })
  }

  const clearDrawerParam = () => {
    drawerParam.value = null
  }

  return {
    editingIndex,
    isDrawerVisible,
    drawerParam,
    focusParameterAfterRender,
    focusFirstAppendedParameter,
    addPropertyParameterFromOption,
    handleTemplateChange,
    openComponentDrawer,
    handleComponentDrawerSave,
    handleParameterRemoved,
    clearDrawerParam
  }
}
