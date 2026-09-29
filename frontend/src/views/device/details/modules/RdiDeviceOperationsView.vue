<!--
  文件用途: RDI 设备详情操作视图外壳，组合配置、遥测、历史、命令、分享和温度告警各分区。
  核心逻辑: provideRdiOperationsContext 统一装配全部 RDI composable 并注入给 rdi/ 下的分区子组件；
  本文件只负责分区编排、命令/分享操作区接线和底部保存栏。
  关键链路:
  1. 挂载时并行启动配置读取、实时状态刷新和遥测轮询；OTA 包列表等到用户进入 OTA 操作区再按需加载（useRdiOnDemandLoads）。
  2. 分区子组件通过 useRdiOperationsContext 读写同一份 config / systemInfo，保存时统一由 saveConfig 提交。
  3. 设备 ID 变化时重置分享状态、清空在线缓存，并重新触发整套 RDI 数据装载。
  关键注意事项: 字段名、命令 payload、分享语义、在线状态限制和温度单位切换必须与 backend RDI API 保持一致。
-->
<script setup lang="ts">
import { provideRdiOperationsContext } from './rdi/composables/useRdiOperationsContext'
import RdiTelemetrySummary from './rdi/RdiTelemetrySummary.vue'
import RdiBasicInfoSection from './rdi/RdiBasicInfoSection.vue'
import RdiEnergyFieldTabs from './rdi/RdiEnergyFieldTabs.vue'
import RdiAlarmSection from './rdi/RdiAlarmSection.vue'
import RdiDryContactSection from './rdi/RdiDryContactSection.vue'
import RdiNotificationSection from './rdi/RdiNotificationSection.vue'
import RdiSystemInfoSection from './rdi/RdiSystemInfoSection.vue'
import RdiOperationsView from './RdiOperationsView.vue'

const props = defineProps<{
  id: string
  online?: number
  onlineUpdatedAt?: string
  deviceData?: Record<string, any>
}>()

const emit = defineEmits<{
  change: []
}>()

const state = provideRdiOperationsContext({
  id: () => props.id,
  online: () => props.online,
  onlineUpdatedAt: () => props.onlineUpdatedAt,
  deviceData: () => props.deviceData,
  onChange: () => emit('change')
})

const { t, options, loads } = state
const { loading, applyToDevice, configCommandTrackingSummary, saveConfig } = state.config
const { telemetryRows, temperatureUnit } = state.telemetry
const {
  commandLoading,
  dryCommandDelay,
  dryTestDuration,
  commandTrackingSummary,
  otaPackageLoading,
  otaPackageId,
  latestFirmwareLoading,
  latestFirmwarePackage,
  otaCommand,
  otaPackageOptions,
  otaMissingFieldLabels,
  canSendOtaUpgrade,
  setDryContact,
  testDryContact,
  sendOtaUpgrade,
  sendUnbindDevice,
  sendFactoryReset,
  applyLatestFirmwarePackage,
  checkLatestFirmware
} = state.commands
const { shareLoading, shareExpiresIn, shareLink, shareExpiryOptions, shareExpiresAt, shareActions } = state.share
</script>

<template>
  <div class="rdi-device-operations-view">
    <NSpin :show="loading">
      <RdiBasicInfoSection />

      <RdiTelemetrySummary
        v-model:temperature-unit="temperatureUnit"
        :rows="telemetryRows"
        :temperature-unit-options="options.temperatureUnit.value"
        :t="t"
      />

      <RdiEnergyFieldTabs />
      <RdiAlarmSection />
      <RdiDryContactSection />
      <RdiNotificationSection />
      <RdiSystemInfoSection />

      <RdiOperationsView
        :command-loading="commandLoading"
        :command-tracking-summary="commandTrackingSummary"
        :dry-command-delay="dryCommandDelay"
        :dry-test-duration="dryTestDuration"
        :latest-firmware-loading="latestFirmwareLoading"
        :latest-firmware-package="latestFirmwarePackage"
        :can-send-ota-upgrade="canSendOtaUpgrade"
        :ota-command="otaCommand"
        :ota-missing-field-labels="otaMissingFieldLabels"
        :ota-package-id="otaPackageId"
        :ota-package-loading="otaPackageLoading"
        :ota-package-options="otaPackageOptions"
        :share-expires-at="shareExpiresAt"
        :share-expires-in="shareExpiresIn"
        :share-expiry-options="shareExpiryOptions"
        :share-link="shareLink"
        :share-loading="shareLoading"
        :t="t"
        @apply-latest-firmware="applyLatestFirmwarePackage"
        @check-latest-firmware="checkLatestFirmware"
        @copy-share="shareActions.copy"
        @create-share="shareActions.create"
        @ensure-ota-packages="loads.ensureOtaPackagesLoaded"
        @load-ota-packages="loads.reloadOtaPackages"
        @send-factory-reset="sendFactoryReset"
        @send-ota-upgrade="sendOtaUpgrade"
        @send-unbind-device="sendUnbindDevice"
        @set-dry-contact="setDryContact"
        @test-dry-contact="testDryContact"
        @update:dry-command-delay="dryCommandDelay = $event"
        @update:dry-test-duration="dryTestDuration = $event"
        @update:ota-package-id="otaPackageId = $event"
        @update:share-expires-in="shareExpiresIn = $event"
      />

      <div class="rdi-footer">
        <NCheckbox v-model:checked="applyToDevice">{{ t('apply') }}</NCheckbox>
        <NButton type="primary" :loading="loading" @click="saveConfig">{{ t('save') }}</NButton>
      </div>
      <NAlert v-if="configCommandTrackingSummary" type="info" class="rdi-command-tracking" :show-icon="false">
        {{ configCommandTrackingSummary }}
      </NAlert>
    </NSpin>
  </div>
</template>

<style scoped>
.rdi-device-operations-view {
  width: 100%;
}

.rdi-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 14px;
  border-top: 1px solid #e5e7eb;
  padding-top: 16px;
}

.rdi-command-tracking {
  margin-top: 12px;
  overflow-wrap: anywhere;
}
</style>
