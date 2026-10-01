<!--
  文件用途: RDI 操作视图“能耗统计 / 字段设置”双 tab 分区。
  核心逻辑: 能耗 tab 复用 useRdiHistory 的范围、导出和图表状态；字段 tab 读写 n00-n07 / sw1-sw4 并下发 field_setting 命令。
  关键注意事项: 图表仅在首次加载完成后渲染（hasLoadedEnergyStatistics），避免空数据初始化 echarts。
-->
<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { useRdiOperationsContext } from './composables/useRdiOperationsContext'

const ChartComponent = defineAsyncComponent(() => import('../telemetry/modules/ChartComponent.vue'))

const N_FIELD_KEYS = Array.from({ length: 8 }, (_, index) => `n0${index}`)
const SW_FIELD_KEYS = Array.from({ length: 4 }, (_, index) => `sw${index + 1}`)

const { t, history, config: configState, commands, loads } = useRdiOperationsContext()
const {
  energyLoading,
  historyExportLoading,
  energyRange,
  energyCustomRange,
  historyChartSeriesKeys,
  historyExportKey,
  historyExportFormat,
  energyStats,
  historyChartOptions,
  energyRangeOptions,
  historyChartSeriesOptions,
  historyExportKeyOptions,
  historyExportFormatOptions,
  formatEnergyValue,
  exportHistoryData
} = history
const { fieldEntries, getFieldValue, setFieldValue } = configState
const { commandLoading, sendFieldSetting } = commands
const { hasLoadedEnergyStatistics, loadEnergyStatisticsOnDemand } = loads
</script>

<template>
  <section class="rdi-section rdi-feature-tabs-section">
    <NTabs type="line" animated class="rdi-feature-tabs">
      <NTabPane name="electricity-statistics" :tab="t('energy')">
        <div class="rdi-tab-pane">
          <div class="rdi-energy-toolbar">
            <NSelect v-model:value="energyRange" :options="energyRangeOptions" class="rdi-select" />
            <NSelect
              v-model:value="historyChartSeriesKeys"
              multiple
              :options="historyChartSeriesOptions"
              :placeholder="t('historyKey')"
              class="rdi-select rdi-series-select"
              max-tag-count="responsive"
            />
            <NSelect v-model:value="historyExportKey" :options="historyExportKeyOptions" class="rdi-select" />
            <NSelect
              v-model:value="historyExportFormat"
              :options="historyExportFormatOptions"
              :aria-label="t('exportFormat')"
              class="rdi-select"
            />
            <NDatePicker
              v-if="energyRange === 'custom'"
              v-model:value="energyCustomRange"
              type="datetimerange"
              class="rdi-date-range"
            />
            <NButton :loading="energyLoading" @click="loadEnergyStatisticsOnDemand">{{ t('load') }}</NButton>
            <NButton :loading="historyExportLoading" @click="exportHistoryData">{{ t('exportData') }}</NButton>
          </div>
          <div class="rdi-telemetry-grid">
            <div class="rdi-telemetry-cell">
              <span>{{ t('latest') }}</span>
              <strong>{{ formatEnergyValue(energyStats.latest) }}</strong>
            </div>
            <div class="rdi-telemetry-cell">
              <span>{{ t('delta') }}</span>
              <strong>{{ formatEnergyValue(energyStats.delta) }}</strong>
            </div>
            <div class="rdi-telemetry-cell">
              <span>{{ t('minMax') }}</span>
              <strong>{{ formatEnergyValue(energyStats.min) }} / {{ formatEnergyValue(energyStats.max) }}</strong>
            </div>
            <div class="rdi-telemetry-cell">
              <span>{{ t('dataPoints') }}</span>
              <strong>{{ energyStats.sample_count }}</strong>
            </div>
          </div>
          <NSpin :show="energyLoading">
            <div class="rdi-history-chart">
              <ChartComponent v-if="hasLoadedEnergyStatistics" :initial-options="historyChartOptions" />
              <NEmpty v-else :description="t('empty')" />
            </div>
          </NSpin>
        </div>
      </NTabPane>
      <NTabPane name="field-setting" :tab="t('field')">
        <div class="rdi-tab-pane">
          <div class="rdi-grid rdi-grid--two">
            <div class="rdi-fieldset">
              <div class="rdi-fieldset-title">{{ t('nFields') }}</div>
              <div class="rdi-field-grid">
                <NInput
                  v-for="key in N_FIELD_KEYS"
                  :key="key"
                  :value="getFieldValue(key)"
                  :placeholder="key"
                  @update:value="(value) => setFieldValue(key, value)"
                />
              </div>
            </div>
            <div class="rdi-fieldset">
              <div class="rdi-fieldset-title">{{ t('swFields') }}</div>
              <div class="rdi-field-grid">
                <NInput
                  v-for="key in SW_FIELD_KEYS"
                  :key="key"
                  :value="getFieldValue(key)"
                  :placeholder="key"
                  @update:value="(value) => setFieldValue(key, value)"
                />
              </div>
            </div>
          </div>
          <div v-if="fieldEntries.length" class="rdi-tags">
            <NTag v-for="[key, value] in fieldEntries" :key="key" size="small">{{ key }}={{ value }}</NTag>
          </div>
          <div class="rdi-section-actions">
            <NButton :loading="commandLoading" @click="sendFieldSetting">{{ t('sendField') }}</NButton>
          </div>
        </div>
      </NTabPane>
    </NTabs>
  </section>
</template>

<style scoped src="./rdi-section.css"></style>

<style scoped>
.rdi-feature-tabs-section {
  padding-bottom: 12px;
}

.rdi-feature-tabs {
  --n-tab-gap: 18px;
}

.rdi-tab-pane {
  padding-top: 4px;
}

.rdi-energy-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.rdi-history-chart {
  width: 100%;
  height: 360px;
  min-height: 320px;
  margin-top: 16px;
}

.rdi-select {
  width: 168px;
}

.rdi-date-range {
  width: 320px;
}

.rdi-field-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(88px, 1fr));
  gap: 8px;
}

.rdi-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.rdi-section-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

@media (max-width: 900px) {
  .rdi-date-range,
  .rdi-select {
    width: 100%;
  }
}
</style>
