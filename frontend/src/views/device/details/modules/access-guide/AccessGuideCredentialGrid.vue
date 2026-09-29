<!--
  文件用途: MQTT/HTTP 接入凭证与端点信息网格（点击即复制）。
  核心逻辑: DeviceAccessGuide 拆分出的展示分区；复制与调试动作通过 emit 交回 DeviceAccessGuide 统一转发。
-->
<script setup lang="ts">
import type { DeviceAccessGuideState } from '../device-access-guide-state'

defineProps<{
  accessGuide: DeviceAccessGuideState
  credentialsMasked?: boolean
  passwordDisplayVisible: boolean
}>()

const emit = defineEmits<{
  copy: [text: unknown]
}>()
</script>

<template>
  <div class="access-guide-grid">
    <div class="access-guide-metric">
      <span class="access-guide-label">{{ $t('custom.device_details.accessGuideProtocol') }}</span>
      <strong>{{ accessGuide.protocol }}</strong>
    </div>
    <div class="access-guide-metric">
      <span class="access-guide-label">{{ $t('custom.device_details.accessGuideAuthMode') }}</span>
      <strong>{{ accessGuide.authMode }}</strong>
    </div>
    <div class="access-guide-metric" data-testid="device-access-guide-endpoint">
      <span class="access-guide-label">{{ $t('custom.device_details.accessGuideEndpoint') }}</span>
      <button type="button" class="access-guide-copy" @click="emit('copy', accessGuide.endpoint)">
        {{ accessGuide.endpoint }}
      </button>
    </div>
    <div class="access-guide-metric">
      <span class="access-guide-label">{{ $t('custom.device_details.accessGuideClientId') }}</span>
      <button type="button" class="access-guide-copy" @click="emit('copy', accessGuide.clientId)">
        {{ accessGuide.clientId }}
      </button>
    </div>
    <div class="access-guide-metric">
      <span class="access-guide-label">{{ $t('custom.device_details.accessGuideUsername') }}</span>
      <button type="button" class="access-guide-copy" @click="emit('copy', accessGuide.username)">
        {{ accessGuide.username }}
      </button>
    </div>
    <div class="access-guide-metric">
      <span class="access-guide-label">{{ $t('custom.device_details.accessGuidePassword') }}</span>
      <!-- 脱敏态固定展示占位符，不提供明文/复制按钮（Phase 2a）。 -->
      <strong v-if="credentialsMasked">******</strong>
      <button
        v-else-if="passwordDisplayVisible"
        type="button"
        class="access-guide-copy"
        @click="emit('copy', accessGuide.password)"
      >
        {{ accessGuide.password }}
      </button>
      <strong v-else>{{ $t('custom.device_details.accessGuidePasswordEmpty') }}</strong>
    </div>
    <div v-if="accessGuide.endpointKind === 'mqtt'" class="access-guide-metric">
      <span class="access-guide-label">{{ $t('custom.device_details.accessGuideReportTopic') }}</span>
      <button type="button" class="access-guide-copy" @click="emit('copy', accessGuide.reportTopic)">
        {{ accessGuide.reportTopic }}
      </button>
    </div>
    <div v-if="accessGuide.endpointKind === 'mqtt'" class="access-guide-metric">
      <span class="access-guide-label">{{ $t('custom.device_details.accessGuideControlTopic') }}</span>
      <button type="button" class="access-guide-copy" @click="emit('copy', accessGuide.controlTopic)">
        {{ accessGuide.controlTopic }}
      </button>
    </div>
    <div class="access-guide-metric">
      <span class="access-guide-label">{{ $t('custom.device_details.accessGuideTls') }}</span>
      <strong>{{ $t(accessGuide.tlsHintKey) }}</strong>
    </div>
    <div class="access-guide-metric">
      <span class="access-guide-label">{{ $t('custom.device_details.accessGuideOnlineCheck') }}</span>
      <strong>{{ $t('custom.device_details.accessGuideOnlineHint') }}</strong>
    </div>
  </div>
</template>

<style scoped src="./access-guide-shared.css"></style>

<style scoped>
.access-guide-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
  margin-bottom: 18px;
}

.access-guide-metric {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--action-color);
}
</style>
