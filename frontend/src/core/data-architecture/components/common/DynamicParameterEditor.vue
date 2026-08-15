<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NSpace, NTag, NText, NDrawer, NDrawerContent, NIcon, NAlert, useMessage } from 'naive-ui'
import { type EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import AddParameterFromDevice from '@/core/data-architecture/components/common/AddParameterFromDevice.vue'
import DynamicParameterAddDrawer from '@/core/data-architecture/components/common/DynamicParameterAddDrawer.vue'
import DynamicParameterComponentDrawer from '@/core/data-architecture/components/common/DynamicParameterComponentDrawer.vue'
import DynamicParameterInlineRow from '@/core/data-architecture/components/common/DynamicParameterInlineRow.vue'
import { useDynamicParameterDeviceFlows } from '@/core/data-architecture/components/common/useDynamicParameterDeviceFlows'
import { useDynamicParameterEditSession } from '@/core/data-architecture/components/common/useDynamicParameterEditSession'
import UnifiedDeviceConfigSelector from '@/core/data-architecture/components/device-selectors/UnifiedDeviceConfigSelector.vue'
import DeviceParameterSelector from '@/core/data-architecture/components/device-selectors/DeviceParameterSelector.vue'
import {
  Sparkles as SparkleIcon,
  AddCircleOutline as AddIcon,
  PhonePortraitOutline as DeviceIcon
} from '@vicons/ionicons5'
import { createLogger } from '@/utils/logger'
import {
  createDefaultParameter,
  createManualAddOptionParameter
} from '@/core/data-architecture/components/common/dynamicParameterEditorNewParam'
import { isDeviceParameterGroup } from '@/core/data-architecture/components/common/dynamicParameterEditorDeviceGroup'
import {
  getCurrentTemplateOptions,
  isCustomInputAllowed
} from '@/core/data-architecture/components/common/dynamicParameterEditorTemplate'
import {
  createDefaultParameterKey,
  ensureParameterHasId,
  removeParameterAt,
  updateParameterAt,
  updateParameterKeyAt,
  updateParameterValueAt,
  validateExistingParameterKey
} from '@/core/data-architecture/components/common/dynamicParameterEditorParameterList'
import {
  buildAddParameterOptions,
  loadRecommendedTemplates,
  resolveAddParameterOptionAction,
  type AddParameterOptionAction
} from '@/core/data-architecture/components/common/dynamicParameterEditorAddOptions'
import {
  buildCurrentApiTemplateImportResult,
  buildDefaultTemplateImportResult,
  type TemplateImportResult
} from '@/core/data-architecture/components/common/dynamicParameterEditorTemplateImport'

const logger = createLogger('DynamicParameterEditor')

// Props接口
interface Props {
  modelValue: EnhancedParameter[]
  parameterType: 'header' | 'query' | 'path'
  title?: string
  addButtonText?: string
  keyPlaceholder?: string
  valuePlaceholder?: string
  showDataType?: boolean
  showEnabled?: boolean
  customClass?: string
  maxParameters?: number // 最大参数数量限制
  currentApiInfo?: any // 当前选择的内部接口信息，用于接口模板功能
  currentComponentId?: string // 当前组件ID，用于属性绑定
}

// Emits接口
interface Emits {
  (e: 'update:modelValue', value: EnhancedParameter[]): void
}

const props = withDefaults(defineProps<Props>(), {
  title: '参数配置',
  addButtonText: '添加参数',
  keyPlaceholder: '参数名',
  valuePlaceholder: '参数值',
  showDataType: true,
  showEnabled: true,
  customClass: ''
})

const emit = defineEmits<Emits>()
const message = useMessage()

// 添加参数抽屉控制
const showAddParamDrawer = ref(false)

/**
 * 参数添加选项，支持接口模板导入。
 */
const addParameterOptions = computed(() => {
  return buildAddParameterOptions(props.currentApiInfo)
})

/**
 * 获取推荐的模板列表
 */
const recommendedTemplates = computed(() => {
  return loadRecommendedTemplates(props.parameterType)
})

/**
 * 是否可以添加更多参数
 */
const canAddMoreParameters = computed(() => {
  if (props.maxParameters === undefined) return true
  return props.modelValue.length < props.maxParameters
})

/**
 * Ensure every rendered parameter has a stable ID so focus is not lost.
 */
const parametersWithStableIds = computed(() => {
  return props.modelValue.map((param, index) => ensureParameterHasId(param, index))
})

const templateSelectOptions = computed(() => {
  return recommendedTemplates.value.map((template) => ({
    label: template.name,
    value: template.id
  }))
})

/**
 * 打开添加参数抽屉。
 */
const openNewParamForm = () => {
  showAddParamDrawer.value = true
}

const appendParameters = (parameters: EnhancedParameter[]) => {
  const updatedParams = [...props.modelValue, ...parameters]
  emit('update:modelValue', updatedParams)
  return updatedParams
}

const updateParameter = (param: EnhancedParameter, index: number) => {
  emit('update:modelValue', updateParameterAt(props.modelValue, index, param))
}

let openUnifiedDeviceConfigSelectorFromDevice: (isEditing: boolean) => void = () => {}
const openUnifiedDeviceConfigSelector = (isEditing: boolean) => {
  openUnifiedDeviceConfigSelectorFromDevice(isEditing)
}

const {
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
} = useDynamicParameterEditSession({
  getParameterType: () => props.parameterType,
  appendParameters,
  updateParameter,
  openUnifiedDeviceConfigSelector
})

const appendParametersAndFocus = (parameters: EnhancedParameter[]) => {
  const updatedParams = appendParameters(parameters)
  focusFirstAppendedParameter(updatedParams, parameters.length)
  return updatedParams
}

const {
  isAddFromDeviceDrawerVisible,
  isUnifiedDeviceConfigVisible,
  isEditingDeviceConfig,
  isDeviceParameterSelectorVisible,
  editingGroupInfo,
  openUnifiedDeviceConfigSelector: openDeviceConfigSelector,
  closeAddFromDeviceDrawer,
  closeUnifiedDeviceConfigSelector,
  handleAddFromDevice,
  handleDeviceParametersSelected,
  handleNewDeviceParametersFromDrawer,
  handleUnifiedDeviceConfigGenerated,
  getExistingDeviceParameters,
  editDeviceConfig,
  handleParametersUpdated,
  editParameterGroup,
  deleteParameterGroup
} = useDynamicParameterDeviceFlows({
  getParameters: () => props.modelValue,
  getMaxParameters: () => props.maxParameters,
  emitParameterUpdate: (parameters: EnhancedParameter[]) => emit('update:modelValue', parameters),
  appendParametersAndFocus,
  focusParameterAfterRender
})

openUnifiedDeviceConfigSelectorFromDevice = openDeviceConfigSelector

const existingDeviceParameters = computed(() => getExistingDeviceParameters())

const addManualParameterFromOption = () => {
  const updatedParams = appendParameters([createManualAddOptionParameter()])
  focusParameterAfterRender(updatedParams.length - 1)
}

const executeAddParameterOptionAction = (action: AddParameterOptionAction) => {
  switch (action) {
    case 'import-template':
      handleTemplateImport()
      return
    case 'add-property':
      addPropertyParameterFromOption()
      return
    case 'open-device-config':
      openUnifiedDeviceConfigSelector(false)
      return
    case 'blocked-by-limit':
      return
    case 'add-manual':
      addManualParameterFromOption()
  }
}

/**
 * 处理添加参数的下拉选项，具体分发规则由 add-options helper 维护。
 */
const handleSelectAddOption = (key: string) => {
  executeAddParameterOptionAction(resolveAddParameterOptionAction(key, canAddMoreParameters.value))
}

const applyTemplateImportResult = (result: TemplateImportResult) => {
  emit('update:modelValue', result.parameters)
  if (result.focusIndex !== null) {
    focusParameterAfterRender(result.focusIndex)
  }
}

/**
 * 处理接口模板导入，根据当前选择的接口生成参数。
 */
const handleTemplateImport = () => {
  if (!props.currentApiInfo) {
    logger.warn('[handleTemplateImport] currentApiInfo is empty; using a default parameter')
    applyTemplateImportResult(buildDefaultTemplateImportResult(props.modelValue, createDefaultParameter))
    return
  }

  applyTemplateImportResult(
    buildCurrentApiTemplateImportResult(
      props.modelValue,
      props.currentApiInfo,
      props.parameterType,
      createDefaultParameter
    )
  )
}

/**
 * 删除参数并发出新的参数列表。
 */
const removeParameter = (index: number) => {
  const updatedParams = removeParameterAt(props.modelValue, index)

  // 立即发射更新事件
  emit('update:modelValue', updatedParams)
  handleParameterRemoved(index)
}

const updateParameterKey = (param: EnhancedParameter, index: number, newKey: string) => {
  // 立即更新本地显示，避免输入延迟
  emit('update:modelValue', updateParameterKeyAt(props.modelValue, index, param, newKey))
}

const resetParameterKey = (param: EnhancedParameter, index: number, reason?: string) => {
  const defaultKey = createDefaultParameterKey(index)
  updateParameter({ ...param, key: defaultKey }, index)

  if (reason) {
    message.error(`参数 key "${reason}" 已存在，不允许重复！已自动重置为 "${defaultKey}"`)
    logger.error(`[DynamicParameterEditor] duplicate parameter key "${reason}" reset to "${defaultKey}"`)
  }
}

/**
 * 检查参数 key 是否为空或重复。
 */
const ensureParameterKeyNotEmpty = (param: EnhancedParameter, index: number) => {
  const validation = validateExistingParameterKey(props.modelValue, param, index)
  if (!validation.ok) {
    resetParameterKey(param, index, validation.duplicateKey)
  }
}

/**
 * 更新参数 value 的防抖处理。
 * 立即更新显示，避免输入延迟
 */
const updateParameterValue = (param: EnhancedParameter, index: number, newValue: string) => {
  // 立即更新显示，保持输入的流畅性
  emit('update:modelValue', updateParameterValueAt(props.modelValue, index, param, newValue))
}
</script>

<template>
  <div :class="['dynamic-parameter-editor-v3-enhanced', customClass]">
    <!-- 标题和添加按钮区 -->
    <div class="editor-header-enhanced">
      <span v-if="title" class="editor-title">{{ title }}</span>

      <n-space :size="8">
        <!-- 单个添加参数按钮 -->
        <n-button size="small" type="primary" :disabled="!canAddMoreParameters" @click="openNewParamForm">
          <template #icon>
            <n-icon><add-icon /></n-icon>
          </template>
          {{ addButtonText }}
        </n-button>

        <!-- 应用接口模板 -->
        <n-button
          size="small"
          type="success"
          :disabled="!canAddMoreParameters"
          @click="() => handleSelectAddOption('apply-interface-template')"
        >
          <template #icon>
            <n-icon><SparkleIcon /></n-icon>
          </template>
          应用接口模板
        </n-button>
      </n-space>
    </div>

    <!-- 设备参数提示（如果存在设备相关参数） -->
    <div v-if="existingDeviceParameters.length > 0" class="device-config-info">
      <n-alert type="info" size="small" :show-icon="false">
        <template #header>
          <n-space align="center">
            <n-icon size="16"><DeviceIcon /></n-icon>
            <span>当前设备配置</span>
          </n-space>
        </template>
        <n-space>
          <n-tag v-for="param in existingDeviceParameters" :key="param.key" size="small" type="info">
            {{ param.key }}: {{ param.value }}
          </n-tag>
        </n-space>
        <template #action>
          <n-button size="small" text type="primary" @click="editDeviceConfig">重新配置</n-button>
        </template>
      </n-alert>
    </div>

    <!-- 参数列表 - 一行展开所有配置项 -->
    <div v-if="parametersWithStableIds.length > 0" class="parameter-list-inline">
      <DynamicParameterInlineRow
        v-for="(param, index) in parametersWithStableIds"
        :key="param._id"
        :param="param"
        :show-enabled="showEnabled"
        :key-placeholder="keyPlaceholder"
        :value-placeholder="valuePlaceholder"
        :template-options="templateSelectOptions"
        :value-options="getCurrentTemplateOptions(param)"
        :allow-custom-value="isCustomInputAllowed(param)"
        :is-device-parameter-group="isDeviceParameterGroup(param)"
        @update="(updatedParam) => updateParameter(updatedParam, index)"
        @update-key="(value) => updateParameterKey(param, index, value)"
        @validate-key="() => ensureParameterKeyNotEmpty(param, index)"
        @update-value="(value) => updateParameterValue(param, index, value)"
        @template-change="(templateId) => handleTemplateChange(param, index, templateId)"
        @configure-component="() => openComponentDrawer(param)"
        @remove="() => removeParameter(index)"
        @edit-device-group="() => editParameterGroup(param)"
        @delete-device-group="() => deleteParameterGroup(param)"
      />
    </div>

    <!-- 空状态 -->
    <div v-else class="empty-state">
      <n-text depth="3">暂无参数，点击"{{ addButtonText }}"添加</n-text>
    </div>

    <DynamicParameterAddDrawer
      v-model:show="showAddParamDrawer"
      :parameter-type="parameterType"
      :existing-parameters="modelValue"
      :current-component-id="currentComponentId"
      @add-parameter="appendParameters([$event])"
      @device-parameters-generated="handleNewDeviceParametersFromDrawer"
    />

    <!-- 从设备添加参数抽屉 -->
    <n-drawer v-model:show="isAddFromDeviceDrawerVisible" :width="500">
      <n-drawer-content title="从设备添加参数" closable>
        <AddParameterFromDevice @add="handleAddFromDevice" @cancel="closeAddFromDeviceDrawer" />
      </n-drawer-content>
    </n-drawer>

    <DynamicParameterComponentDrawer
      v-model:show="isDrawerVisible"
      :parameter="drawerParam"
      :current-component-id="currentComponentId"
      @save="handleComponentDrawerSave"
      @closed="clearDrawerParam"
    />

    <!-- 统一设备配置选择器 -->
    <n-drawer v-model:show="isUnifiedDeviceConfigVisible" width="650" placement="right">
      <n-drawer-content title="设备配置" closable>
        <UnifiedDeviceConfigSelector
          :existing-parameters="existingDeviceParameters"
          :edit-mode="isEditingDeviceConfig"
          @parameters-generated="handleUnifiedDeviceConfigGenerated"
          @cancel="closeUnifiedDeviceConfigSelector"
        />
      </n-drawer-content>
    </n-drawer>

    <!-- 设备参数选择器 -->
    <DeviceParameterSelector
      :visible="isDeviceParameterSelectorVisible"
      :editing-group-id="editingGroupInfo?.groupId"
      :pre-selected-device="editingGroupInfo?.preSelectedDevice"
      :pre-selected-metric="editingGroupInfo?.preSelectedMetric"
      :pre-selected-mode="editingGroupInfo?.preSelectedMode"
      @update:visible="isDeviceParameterSelectorVisible = $event"
      @parameters-selected="handleDeviceParametersSelected"
      @parameters-updated="handleParametersUpdated"
    />
  </div>
</template>

<style scoped>
/* 增强版编辑器样式 */
.dynamic-parameter-editor-v3-enhanced {
  width: 100%;
  font-size: 12px;
}

/* Retained CSS hook for callers that still pass dynamic-parameter-editor-v3. */
.dynamic-parameter-editor-v3 {
  width: 100%;
  font-size: 12px;
}

/* 增强的编辑器头部 */
.editor-header-enhanced {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  padding: 12px;
  background: linear-gradient(135deg, var(--primary-color-suppl) 0%, transparent 100%);
  border-radius: 6px;
  border: 1px solid var(--border-color);
}

.editor-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-color-1);
}

.device-config-info {
  margin-bottom: 16px;
}

.empty-state {
  padding: 24px;
  text-align: center;
  background: var(--body-color);
  border: 1px dashed var(--border-color);
  border-radius: 6px;
}

/* 一行展开的参数列表样式 */
.parameter-list-inline {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
</style>
