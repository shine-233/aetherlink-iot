<!--
  文件用途: 设备 Ready Check 顶部 hero 区：设备名称、在线态、来源横幅（首台设备 / OTA 失败 / 命令任务诊断）和摘要行。
  核心逻辑: 纯展示组件，来源上下文由 buildReadyCheckSourceContext 生成并经父级传入。
-->
<script setup lang="ts">
import type { buildReadyCheckSourceContext } from './ready-check-source-context'

defineProps<{
  deviceName: string
  deviceNumber: string
  isOnline: boolean
  source: ReturnType<typeof buildReadyCheckSourceContext>
  primaryTitleKey: string
}>()
</script>

<template>
  <section class="ready-check-hero">
    <div class="ready-check-hero__intro">
      <div class="ready-check-hero__copy">
        <span class="ready-check-hero__eyebrow">{{ $t('custom.device_details.readyCheckDevice') }}</span>
        <strong :title="String(deviceName)">{{ deviceName }}</strong>
        <p>{{ $t('custom.device_details.readyCheckIntro') }}</p>
      </div>
      <span class="ready-check-hero__state" :class="{ 'is-online': isOnline }">
        {{ isOnline ? $t('custom.device_details.online') : $t('custom.device_details.offline') }}
      </span>
    </div>

    <NAlert
      v-if="source.isFirstDeviceOnboardingSource"
      type="success"
      :show-icon="false"
      class="ready-check-source-banner"
    >
      <strong>{{ $t('custom.device_details.readyCheckFirstDeviceSourceTitle') }}</strong>
      <span>{{ $t('custom.device_details.readyCheckFirstDeviceSourceDesc') }}</span>
    </NAlert>
    <NAlert
      v-if="source.isOtaFailureSource"
      type="warning"
      :show-icon="false"
      class="ready-check-source-banner ready-check-source-banner--warning"
    >
      <strong>{{ $t('custom.device_details.readyCheckOtaFailureSourceTitle') }}</strong>
      <span>{{ $t('custom.device_details.readyCheckOtaFailureSourceDesc') }}</span>
      <span class="ready-check-source-banner__meta">
        <code v-if="source.otaTaskId">task={{ source.otaTaskId }}</code>
        <code v-if="source.otaDetailId">detail={{ source.otaDetailId }}</code>
      </span>
    </NAlert>
    <NAlert
      v-if="source.isCommandJobDiagnosisSource"
      type="warning"
      :show-icon="false"
      class="ready-check-source-banner ready-check-source-banner--warning"
    >
      <strong>{{ $t('custom.device_details.readyCheckCommandJobSourceTitle') }}</strong>
      <span>{{ $t('custom.device_details.readyCheckCommandJobSourceDesc') }}</span>
      <span class="ready-check-source-banner__meta">
        <code v-if="source.commandJobId">job={{ source.commandJobId }}</code>
      </span>
    </NAlert>

    <div class="ready-check-summary">
      <div>
        <span>{{ $t('custom.device_details.readyCheckDeviceNumber') }}</span>
        <strong :title="String(deviceNumber)">{{ deviceNumber }}</strong>
      </div>
      <div>
        <span>{{ $t('custom.device_details.readyCheckOnlineState') }}</span>
        <strong :title="isOnline ? $t('custom.device_details.online') : $t('custom.device_details.offline')">
          {{ isOnline ? $t('custom.device_details.online') : $t('custom.device_details.offline') }}
        </strong>
      </div>
      <div>
        <span>{{ $t('custom.device_details.readyCheckPrimaryNext') }}</span>
        <strong>{{ $t(primaryTitleKey) }}</strong>
      </div>
    </div>
  </section>
</template>

<style scoped>
.ready-check-hero {
  position: relative;
  display: grid;
  gap: 14px;
  overflow: hidden;
  border: 1px solid var(--border-color);
  border-radius: 18px;
  background:
    radial-gradient(circle at 14% 8%, rgba(59, 130, 246, 0.2), transparent 28%),
    linear-gradient(
      135deg,
      rgb(var(--info-color) / 0.05) 0%,
      rgb(var(--info-color) / 0.08) 52%,
      var(--action-color) 100%
    );
  padding: 18px;
  box-shadow: 0 18px 46px rgba(15, 23, 42, 0.08);
}

.ready-check-hero::after {
  position: absolute;
  top: -56px;
  right: -46px;
  width: 170px;
  height: 170px;
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(14, 165, 233, 0.22), rgba(34, 197, 94, 0.16));
  content: '';
  pointer-events: none;
}

.ready-check-hero__intro {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.ready-check-hero__copy {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.ready-check-hero__eyebrow {
  color: rgb(var(--info-color));
  font-size: var(--font-size-caption);
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.ready-check-hero__copy strong {
  color: var(--text-color-1);
  font-size: 24px;
  line-height: 1.2;
  overflow-wrap: anywhere;
}

.ready-check-hero__copy p {
  max-width: 760px;
  margin: 0;
  color: var(--text-color-2);
  font-size: var(--font-size-secondary);
  line-height: 1.6;
}

.ready-check-hero__state {
  flex: 0 0 auto;
  border: 1px solid rgb(var(--warning-color) / 0.4);
  border-radius: var(--radius-pill);
  background: rgb(var(--warning-color) / 0.1);
  padding: 6px 12px;
  color: rgb(var(--warning-800-color));
  font-size: var(--font-size-caption);
  font-weight: 700;
  box-shadow: 0 10px 24px rgba(154, 52, 18, 0.1);
}

.ready-check-hero__state.is-online {
  border-color: rgb(var(--success-color) / 0.4);
  background: rgb(var(--success-color) / 0.08);
  color: rgb(var(--success-700-color));
  box-shadow: 0 10px 24px rgba(21, 128, 61, 0.1);
}

.ready-check-summary {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.ready-check-hero .ready-check-source-banner {
  position: relative;
  z-index: 1;
}

.ready-check-source-banner :deep(.n-alert-body__content) {
  display: grid;
  gap: 4px;
}

.ready-check-source-banner strong {
  color: rgb(var(--success-800-color));
  font-size: var(--font-size-base);
}

.ready-check-source-banner span {
  color: rgb(var(--success-800-color));
  font-size: var(--font-size-caption);
  line-height: 18px;
}

.ready-check-source-banner--warning strong,
.ready-check-source-banner--warning span {
  color: rgb(var(--warning-800-color));
}

.ready-check-source-banner__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.ready-check-source-banner__meta code {
  border-radius: var(--radius-pill);
  background: rgba(15, 23, 42, 0.08);
  padding: 2px 8px;
  color: var(--text-color-2);
  font-size: var(--font-size-caption);
}

.ready-check-summary > div {
  min-width: 0;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--card-color);
}

.ready-check-summary > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
  border-color: rgba(148, 163, 184, 0.28);
  background: rgba(255, 255, 255, 0.72);
  padding: 12px;
  box-shadow: 0 10px 26px rgba(15, 23, 42, 0.05);
  backdrop-filter: blur(10px);
}

.ready-check-summary span {
  color: var(--text-color-3);
  font-size: var(--font-size-secondary);
  line-height: 1.5;
}

.ready-check-summary strong {
  display: block;
  color: var(--text-color-1);
  overflow-wrap: anywhere;
  line-height: 1.35;
  word-break: break-word;
}

@media (max-width: 900px) {
  .ready-check-summary {
    grid-template-columns: 1fr;
  }

  .ready-check-hero {
    border-radius: 14px;
    padding: 14px;
  }

  .ready-check-hero__intro {
    flex-direction: column;
  }

  .ready-check-hero__copy strong {
    font-size: 20px;
  }
}
</style>
