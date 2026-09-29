<!--
  文件用途: RDI 时长设置控件（0H-24H 滑块 + 数字输入 + 格式化标签）。
  核心逻辑: 统一原先在 RdiDeviceOperationsView 中重复 9 次的“时长 slider + input-number”模板，值通过 v-model 回传。
  关键注意事项: 步长固定 60 秒，上限默认 RDI_DURATION_MAX_SECONDS；只负责展示和双向绑定，不做保存。
-->
<script setup lang="ts">
import { RDI_DURATION_MAX_SECONDS } from './constants/rdi-ranges'

withDefaults(
  defineProps<{
    label: string
    formatLabel: (value: number) => string
    max?: number
  }>(),
  { max: RDI_DURATION_MAX_SECONDS }
)

const value = defineModel<number>({ required: true })
</script>

<template>
  <NFormItem :label="label" class="rdi-duration-field">
    <div class="rdi-duration-control">
      <div class="rdi-duration-value">{{ formatLabel(value) }}</div>
      <NSlider v-model:value="value" :min="0" :max="max" :step="60" :format-tooltip="formatLabel" />
      <div class="rdi-duration-labels">
        <span>0H</span>
        <span>24H</span>
      </div>
      <NInputNumber v-model:value="value" :min="0" :max="max" />
    </div>
  </NFormItem>
</template>

<style scoped>
.rdi-duration-field {
  min-width: 260px;
}

.rdi-duration-control {
  display: grid;
  width: 100%;
  gap: 8px;
}

.rdi-duration-value {
  color: #344054;
  font-size: 12px;
  font-weight: 600;
}

.rdi-duration-labels {
  display: flex;
  justify-content: space-between;
  color: #98a2b3;
  font-size: 11px;
  line-height: 1.2;
}

.rdi-duration-control :deep(.n-input-number) {
  width: 100%;
}

:deep(.n-form-item) {
  margin-bottom: 0;
}
</style>
