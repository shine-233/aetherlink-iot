<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NCheckbox, NIcon, NInput, NSelect, NSpace, NText } from 'naive-ui'
import { CreateOutline as EditOutline, PhonePortraitOutline, TrashOutline } from '@vicons/ionicons5'
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import type { TemplateOption } from '@/core/data-architecture/components/common/templates/index'
import type { SelectMixedOption } from 'naive-ui/es/select/src/interface'

const props = defineProps<{
  param: EnhancedParameter
  showEnabled: boolean
  keyPlaceholder: string
  valuePlaceholder: string
  templateOptions: Array<{ label: string; value: string }>
  valueOptions: TemplateOption[]
  allowCustomValue: boolean
  isDeviceParameterGroup: boolean
}>()

const emit = defineEmits<{
  (e: 'update', value: EnhancedParameter): void
  (e: 'update-key', value: string): void
  (e: 'validate-key'): void
  (e: 'update-value', value: any): void
  (e: 'template-change', templateId: string): void
  (e: 'configure-component'): void
  (e: 'remove'): void
  (e: 'edit-device-group'): void
  (e: 'delete-device-group'): void
}>()

const isPrimaryDeviceGroup = computed(
  () => props.isDeviceParameterGroup && props.param.parameterGroup?.role === 'primary'
)
const isSecondaryDeviceGroup = computed(
  () => props.isDeviceParameterGroup && props.param.parameterGroup?.role !== 'primary'
)
</script>

<template>
  <div
    class="parameter-item-inline"
    :class="{
      'is-device-param-group': isDeviceParameterGroup,
      'is-primary-param': isPrimaryDeviceGroup,
      'is-secondary-param': isSecondaryDeviceGroup
    }"
    :data-param-type="param.valueMode || 'manual'"
  >
    <!-- 参数组标识 -->
    <div v-if="isDeviceParameterGroup" class="param-group-indicator">
      <n-icon size="14" color="#2080f0">
        <PhonePortraitOutline />
      </n-icon>
    </div>

    <!-- 启用checkbox -->
    <n-checkbox
      v-if="showEnabled"
      :checked="param.enabled"
      @update:checked="(value) => emit('update', { ...param, enabled: value })"
    />

    <!-- 参数名输入 -->
    <n-input
      :value="param.key"
      :placeholder="keyPlaceholder"
      size="small"
      class="param-key-input-inline"
      @input="(value) => emit('update-key', value)"
      @blur="() => emit('validate-key')"
    />

    <!-- 类型选择（下拉） -->
    <n-select
      :value="param.selectedTemplate"
      :options="templateOptions"
      size="small"
      class="param-type-select-inline"
      @update:value="(templateId) => emit('template-change', String(templateId))"
    />

    <!-- 值输入区域（根据类型动态显示） -->
    <div class="param-value-input-inline">
      <!-- 手动输入 -->
      <n-input
        v-if="param.valueMode === 'manual'"
        :value="param.value"
        :placeholder="valuePlaceholder"
        size="small"
        @input="(value) => emit('update-value', value)"
      />

      <!-- 属性绑定 -->
      <div v-else-if="param.valueMode === 'property'" class="property-binding-inline">
        <n-input
          :value="param.value"
          placeholder="替换值"
          size="small"
          @input="(value) => emit('update-value', value)"
        />
      </div>

      <!-- 组件绑定（属性/设备） -->
      <div v-else-if="param.valueMode === 'component'" class="component-binding-inline">
        <n-input :value="param.value || '(点击配置)'" size="small" readonly />
        <n-button size="small" type="primary" text @click="emit('configure-component')">配置</n-button>
      </div>

      <!-- 下拉选择 -->
      <n-select
        v-else-if="param.valueMode === 'dropdown'"
        :value="param.value"
        :options="valueOptions as SelectMixedOption[]"
        :filterable="allowCustomValue"
        :tag="allowCustomValue"
        size="small"
        placeholder="选择或输入值"
        @update:value="(value) => emit('update', { ...param, value: value })"
      />
    </div>

    <!-- 操作按钮 - 使用小图标 -->
    <n-space class="param-actions-inline" :size="4">
      <!-- 普通参数 - 只显示删除图标 -->
      <template v-if="!isDeviceParameterGroup">
        <n-button size="small" type="error" quaternary circle @click="emit('remove')">
          <template #icon>
            <n-icon><TrashOutline /></n-icon>
          </template>
        </n-button>
      </template>

      <!-- 参数组（主参数） - 编辑和删除图标 -->
      <template v-else-if="isPrimaryDeviceGroup">
        <n-button size="small" type="info" quaternary circle @click="emit('edit-device-group')">
          <template #icon>
            <n-icon><EditOutline /></n-icon>
          </template>
        </n-button>
        <n-button size="small" type="error" quaternary circle @click="emit('delete-device-group')">
          <template #icon>
            <n-icon><TrashOutline /></n-icon>
          </template>
        </n-button>
      </template>

      <!-- 参数组（子参数） -->
      <template v-else>
        <n-text depth="3" style="font-size: 11px">设备组</n-text>
      </template>
    </n-space>
  </div>
</template>

<style scoped>
.parameter-item-inline {
  display: grid;
  grid-template-columns: auto auto 250px 180px 1fr auto;
  align-items: center;
  gap: 16px;
  padding: 12px 16px;
  background: var(--card-color);
  border: 1px solid var(--border-color);
  border-left-width: 3px;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.parameter-item-inline:hover {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

/* 参数类型视觉分组 - 通过边框颜色 */
.parameter-item-inline[data-param-type='manual'] {
  border-left-color: var(--info-color);
}

.parameter-item-inline[data-param-type='property'] {
  border-left-color: var(--success-color);
  background: linear-gradient(90deg, var(--success-color-suppl) 0%, var(--card-color) 15%);
}

.parameter-item-inline[data-param-type='component'] {
  border-left-color: var(--warning-color);
  background: linear-gradient(90deg, var(--warning-color-suppl) 0%, var(--card-color) 15%);
}

/* 参数组样式 */
.parameter-item-inline.is-device-param-group {
  border-left-color: var(--primary-color);
  border-left-width: 4px;
}

.parameter-item-inline.is-primary-param {
  box-shadow: 0 2px 4px rgba(32, 128, 240, 0.1);
}

.parameter-item-inline.is-secondary-param {
  margin-left: 20px;
  opacity: 0.95;
}

/* 参数组指示器样式 */
.param-group-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  background: var(--primary-color-suppl);
  border-radius: 4px;
  flex-shrink: 0;
}

/* 参数名输入框 */
.param-key-input-inline {
  width: 100%;
}

/* 类型选择下拉 */
.param-type-select-inline {
  width: 100%;
}

/* 值输入区域 */
.param-value-input-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.property-binding-inline,
.component-binding-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.component-binding-inline .n-input {
  flex: 1;
}

/* 操作按钮 */
.param-actions-inline {
  flex-shrink: 0;
}
</style>
