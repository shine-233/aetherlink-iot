<!--
  文件用途: RDI 实时遥测摘要区块。
  核心逻辑: 展示实时 telemetry 行，并把温度单位选择通过 v-model 回传给父级 RDI 操作视图。
  关键注意事项: 这里只负责展示，不启动轮询、不读取设备、不保存配置。
-->
<script setup lang="ts">
import type { LabelKey } from './constants/rdi-labels'

type RdiTelemetryRow = {
  label: string
  value: unknown
  unit?: string
}

defineProps<{
  rows: RdiTelemetryRow[]
  temperatureUnit: string
  temperatureUnitOptions: Array<{ label: string; value: string }>
  t: (key: LabelKey) => string
}>()

defineEmits<{
  (e: 'update:temperatureUnit', value: string): void
}>()

const normalizeTemperatureUnit = (value: string | number | null) => String(value ?? '')
</script>

<template>
  <section class="rdi-section">
    <div class="rdi-section-header">
      <div class="rdi-section-title">{{ t('telemetry') }}</div>
      <NFormItem :label="t('temperatureUnit')" :show-feedback="false" class="rdi-unit-field">
        <NSelect
          :value="temperatureUnit"
          :options="temperatureUnitOptions"
          size="small"
          class="rdi-unit-select"
          @update:value="$emit('update:temperatureUnit', normalizeTemperatureUnit($event))"
        />
      </NFormItem>
    </div>
    <div class="rdi-telemetry-grid">
      <div v-for="row in rows" :key="row.label" class="rdi-telemetry-cell">
        <span>{{ row.label }}</span>
        <strong>{{ row.value }}{{ row.unit ? ` ${row.unit}` : '' }}</strong>
      </div>
    </div>
  </section>
</template>

<style scoped>
.rdi-section {
  border-top: 1px solid #e5e7eb;
  padding: 16px 0;
}

.rdi-section-title {
  margin-bottom: 12px;
  font-size: 15px;
  font-weight: 600;
}

.rdi-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.rdi-section-header .rdi-section-title {
  margin-bottom: 0;
}

.rdi-unit-field {
  margin-bottom: 0;
}

.rdi-unit-select {
  width: 170px;
}

.rdi-telemetry-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 10px;
}

.rdi-telemetry-cell {
  display: flex;
  min-height: 56px;
  flex-direction: column;
  justify-content: center;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 8px 10px;
  background: #f8fafc;
}

.rdi-telemetry-cell span {
  color: #667085;
  font-size: 12px;
}

.rdi-telemetry-cell strong {
  margin-top: 4px;
  font-size: 16px;
}
</style>
