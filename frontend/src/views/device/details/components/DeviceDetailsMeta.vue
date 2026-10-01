<!--
  文件用途: 设备详情页头部元信息行（ID、配置模板、网关、在线状态、告警历史入口）。
  核心逻辑: 纯展示组件，所有跳转与弹窗动作通过 emit 交回详情页壳层（壳层负责只读权限拦截）。
  关键注意事项: 只读分享模式下链接降级为纯文本，告警历史入口隐藏。
-->
<script setup lang="ts">
import { computed } from 'vue'
import type { DeviceDetailData } from '../device-edit-state'

const props = defineProps<{
  deviceId: string
  deviceData: DeviceDetailData
  deviceType: string
  online: number
  onlineUpdatedAtDisplay: string
  alarmActive: boolean
  canUseOwnerActions: boolean
}>()

defineEmits<{
  openConfig: []
  openGateway: []
  openStatusHistory: []
  openAlarmHistory: []
}>()

const INACTIVE_COLOR = '#ccc'
const ONLINE_COLOR = 'rgb(2,153,52)'
const ALARM_COLOR = '#ee0808'

const onlineColor = computed(() => (props.online === 1 ? ONLINE_COLOR : INACTIVE_COLOR))
const alarmColor = computed(() => (props.alarmActive ? ALARM_COLOR : INACTIVE_COLOR))
</script>

<template>
  <NFlex class="device-details-meta">
    <div class="device-details-meta-item">
      <span class="device-details-meta-label">ID:</span>
      <span>{{ deviceId || '--' }}</span>
    </div>
    <div class="device-details-meta-item">
      <span class="device-details-meta-label">{{ $t('custom.devicePage.configTemplate') }} :</span>
      <span
        v-if="deviceData?.device_config_name && canUseOwnerActions"
        class="device-details-link"
        @click="$emit('openConfig')"
      >
        {{ deviceData?.device_config_name }}
      </span>
      <span v-else-if="deviceData?.device_config_name">{{ deviceData?.device_config_name }}</span>
      <span v-else>--</span>
    </div>
    <div v-if="deviceType === '3'" class="device-details-meta-item">
      <span class="device-details-meta-label">{{ $t('generate.gateway') }}:</span>
      <span v-if="canUseOwnerActions" class="device-details-link" @click="$emit('openGateway')">
        {{ deviceData?.gateway_device_name || '--' }}
      </span>
      <span v-else>{{ deviceData?.gateway_device_name || '--' }}</span>
    </div>
    <!-- Click the online-status badge to open the status history dialog. -->
    <div
      class="device-details-status"
      :class="{ 'device-details-status--read-only': !canUseOwnerActions }"
      @click="$emit('openStatusHistory')"
    >
      <SvgIcon
        local-icon="CellTowerRound"
        :style="{ color: INACTIVE_COLOR, marginRight: '5px' }"
        class="text-20px text-primary"
        :stroke="onlineColor"
      />
      <span :style="{ color: onlineColor }">
        {{ online === 1 ? $t('custom.device_details.online') : $t('custom.device_details.offline') }}
      </span>
      <span class="device-details-status-time">
        {{ $t('custom.device_details.lastUpdate') }}: {{ onlineUpdatedAtDisplay }}
      </span>
      <SvgIcon v-if="canUseOwnerActions" local-icon="history" style="margin-left: 5px" class="text-18px text-primary" />
    </div>
    <div
      v-if="canUseOwnerActions"
      class="device-details-status"
      :class="{ 'device-details-status--alarm': alarmActive }"
      @click="$emit('openAlarmHistory')"
    >
      <SvgIcon
        local-icon="AlertFilled"
        :style="{ color: alarmColor, marginRight: '5px' }"
        class="text-20px text-primary"
      />
      <span :style="{ color: alarmColor }">{{ $t('generate.alarmHistory') }}</span>
    </div>
  </NFlex>
</template>

<style scoped lang="scss">
.device-details-meta {
  margin-top: 10px;
  gap: 10px 16px;
  color: inherit;
}

.device-details-meta-item {
  display: flex;
  align-items: center;
  min-height: 28px;
}

.device-details-meta-label {
  margin-right: 8px;
  color: #666;
}

.device-details-link {
  color: blue;
  cursor: pointer;
}

.device-details-status {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
}

.device-details-status--read-only {
  cursor: default;
}

.device-details-status-time {
  max-width: 180px;
  overflow: hidden;
  color: #888;
  font-size: 12px;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-details-status--alarm {
  cursor: pointer;
}

@include mobile {
  /* 头部信息栅格降级单列：meta 纵向堆叠，状态时间戳允许换行不再截断 */
  .device-details-meta {
    flex-direction: column;
    align-items: stretch;
    gap: 6px;
  }

  .device-details-status-time {
    max-width: none;
    white-space: normal;
  }
}
</style>
