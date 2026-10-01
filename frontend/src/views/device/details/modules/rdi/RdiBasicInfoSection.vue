<!--
  文件用途: RDI 操作视图顶部标题栏 + 基础信息分区。
  核心逻辑: 从 RDI 操作上下文读取设备元数据、在线文案与 basicInfoColumns，刷新按钮触发整套装载链路。
  关键注意事项: 只展示，不直接请求接口；刷新动作复用 useRdiOnDemandLoads.loadConfigAndRefresh。
-->
<script setup lang="ts">
import { useRdiOperationsContext } from './composables/useRdiOperationsContext'

const { t, source, telemetry, basicInfo, loads } = useRdiOperationsContext()
const { deviceOnlineText, deviceDescriptionText } = telemetry
const { isDeviceOnline, basicInfoColumns } = basicInfo
</script>

<template>
  <div class="rdi-header">
    <div>
      <div class="rdi-title">{{ t('rdiSettings') }}</div>
      <div class="rdi-meta">
        <span>{{ t('pid') }}: {{ source.deviceData()?.device_number || '--' }}</span>
        <span>{{ t('firmware') }}: {{ source.deviceData()?.current_version || '--' }}</span>
        <span>{{ t('connection') }}: {{ source.deviceData()?.protocol || '--' }}</span>
        <span>{{ deviceOnlineText }}</span>
        <span class="rdi-meta-description">{{ t('description') }}: {{ deviceDescriptionText }}</span>
      </div>
    </div>
    <NButton @click="loads.loadConfigAndRefresh">{{ t('refresh') }}</NButton>
  </div>

  <section class="rdi-section rdi-basic-info-section">
    <div class="rdi-basic-info-header">
      <div class="rdi-section-title">{{ t('basicInfo') }}</div>
    </div>
    <div class="rdi-basic-info-layout">
      <div
        v-for="(column, columnIndex) in basicInfoColumns"
        :key="`basic-info-column-${columnIndex}`"
        class="rdi-basic-info-column"
      >
        <div
          v-for="item in column"
          :key="item.key"
          class="rdi-basic-info-row"
          :class="`rdi-basic-info-row--${item.key}`"
        >
          <span class="rdi-basic-info-label">{{ item.label }}</span>
          <div class="rdi-basic-info-value" :class="item.kind ? `rdi-basic-info-value--${item.kind}` : ''">
            <template v-if="item.kind === 'status'">
              <span
                class="rdi-basic-info-status-dot"
                :class="{ 'rdi-basic-info-status-dot--online': isDeviceOnline }"
                aria-hidden="true"
              />
              <strong>{{ item.value }}</strong>
            </template>
            <span v-else-if="item.kind === 'chip'" class="rdi-basic-info-chip">{{ item.value }}</span>
            <strong v-else>{{ item.value }}</strong>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped src="./rdi-section.css"></style>

<style scoped>
.rdi-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.rdi-title {
  font-size: 18px;
  font-weight: 600;
  line-height: 1.3;
}

.rdi-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  margin-top: 6px;
  color: #667085;
}

.rdi-meta-description {
  min-width: min(100%, 260px);
  overflow-wrap: anywhere;
}

.rdi-basic-info-section {
  padding-top: 12px;
}

.rdi-basic-info-header {
  margin-bottom: 16px;
  padding-bottom: 10px;
  border-bottom: 2px solid #3b82f6;
}

.rdi-basic-info-header .rdi-section-title {
  margin-bottom: 0;
  font-size: 16px;
}

.rdi-basic-info-layout {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 18px 48px;
}

.rdi-basic-info-column {
  display: grid;
  gap: 18px;
}

.rdi-basic-info-row {
  display: grid;
  gap: 6px;
  min-height: 56px;
}

.rdi-basic-info-label {
  color: #667085;
  font-size: 12px;
  font-weight: 500;
}

.rdi-basic-info-value {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
}

.rdi-basic-info-value strong {
  color: #0f172a;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.rdi-basic-info-value--status {
  gap: 10px;
}

.rdi-basic-info-status-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  flex: 0 0 auto;
  background: #ff6b72;
  box-shadow: 0 0 0 3px rgb(255 107 114 / 16%);
}

.rdi-basic-info-status-dot--online {
  background: #16a34a;
  box-shadow: 0 0 0 3px rgb(22 163 74 / 16%);
}

.rdi-basic-info-chip {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  max-width: 100%;
  padding: 4px 12px;
  border-radius: 999px;
  background: #f1f5f9;
  color: #0f172a;
  font-size: 14px;
  line-height: 1.4;
  overflow-wrap: anywhere;
}
</style>
