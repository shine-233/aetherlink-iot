<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'

type OperationTone = 'success' | 'warning' | 'info' | 'default'

type OperationItem = {
  key: string
  tab: string
  title: string
  description: string
  status: string
  tone: OperationTone
  action: string
}

const props = defineProps<{
  deviceId: string
  deviceData: Record<string, any>
  online: number
  onlineUpdatedAt: string
  alarmActive: boolean
  visibleTabs: string[]
}>()

const emit = defineEmits<{
  openTab: [tabKey: string]
}>()

const isOnline = computed(() => Number(props.online) === 1)
const hasDeviceConfig = computed(() => Boolean(props.deviceData?.device_config_id || props.deviceData?.device_config_name))
const visibleTabSet = computed(() => new Set(props.visibleTabs))
const onlineStatusText = computed(() =>
  isOnline.value ? $t('custom.device_details.online') : $t('custom.device_details.offline')
)
const configStatusText = computed(() =>
  hasDeviceConfig.value
    ? $t('custom.device_details.workbenchConfigBound')
    : $t('custom.device_details.workbenchConfigMissing')
)

const operations = computed<OperationItem[]>(() => [
  {
    key: 'ready',
    tab: 'ready-check',
    title: $t('custom.device_details.workbenchReadyTitle'),
    description: $t('custom.device_details.workbenchReadyDesc'),
    status:
      hasDeviceConfig.value && isOnline.value
        ? $t('custom.device_details.workbenchStatusVerifiable')
        : $t('custom.device_details.workbenchStatusNeedsAttention'),
    tone: hasDeviceConfig.value && isOnline.value ? 'success' : 'warning',
    action: $t('custom.device_details.workbenchReadyAction')
  },
  {
    key: 'telemetry',
    tab: 'telemetry',
    title: $t('custom.device_details.workbenchTelemetryTitle'),
    description: $t('custom.device_details.workbenchTelemetryDesc'),
    status: isOnline.value
      ? $t('custom.device_details.workbenchStatusOnlineReporting')
      : $t('custom.device_details.workbenchStatusWaitingOnline'),
    tone: isOnline.value ? 'success' : 'warning',
    action: $t('custom.device_details.workbenchTelemetryAction')
  },
  {
    key: 'twin',
    tab: 'device-twin',
    title: $t('custom.device_details.workbenchTwinTitle'),
    description: $t('custom.device_details.workbenchTwinDesc'),
    status: $t('custom.device_details.workbenchStatusReview'),
    tone: 'info',
    action: $t('custom.device_details.workbenchTwinAction')
  },
  {
    key: 'command',
    tab: 'command-delivery',
    title: $t('custom.device_details.workbenchCommandTitle'),
    description: $t('custom.device_details.workbenchCommandDesc'),
    status: isOnline.value
      ? $t('custom.device_details.workbenchStatusExecutable')
      : $t('custom.device_details.workbenchStatusOfflineCaution'),
    tone: isOnline.value ? 'success' : 'warning',
    action: $t('custom.device_details.workbenchCommandAction')
  },
  {
    key: 'alarm',
    tab: 'give-an-alarm',
    title: $t('custom.device_details.workbenchAlarmTitle'),
    description: $t('custom.device_details.workbenchAlarmDesc'),
    status: props.alarmActive
      ? $t('custom.device_details.workbenchStatusHasAlarm')
      : $t('custom.device_details.workbenchStatusNotTriggered'),
    tone: props.alarmActive ? 'warning' : 'default',
    action: $t('custom.device_details.workbenchAlarmAction')
  },
  {
    key: 'diagnosis',
    tab: 'device-diagnosis',
    title: $t('custom.device_details.workbenchDiagnosisTitle'),
    description: $t('custom.device_details.workbenchDiagnosisDesc'),
    status: $t('custom.device_details.workbenchStatusTroubleshoot'),
    tone: 'info',
    action: $t('custom.device_details.workbenchDiagnosisAction')
  }
])

function openOperation(item: OperationItem) {
  if (!visibleTabSet.value.has(item.tab)) return
  emit('openTab', item.tab)
}
</script>

<template>
  <section class="device-operations-workbench">
    <div class="device-operations-workbench__summary">
      <div>
        <NTag size="small" :type="isOnline ? 'success' : 'warning'">
          {{ onlineStatusText }}
        </NTag>
        <NTag size="small" :type="hasDeviceConfig ? 'success' : 'warning'">
          {{ configStatusText }}
        </NTag>
        <NTag v-if="alarmActive" size="small" type="error">
          {{ $t('custom.device_details.workbenchAlarmPending') }}
        </NTag>
      </div>
      <p>
        {{ $t('custom.device_details.workbenchIntro') }}{{ $t('custom.device_details.workbenchLastOnlineUpdate') }}
        {{ onlineUpdatedAt || '--' }}
      </p>
    </div>

    <div class="device-operations-workbench__grid">
      <article
        v-for="item in operations"
        :key="item.key"
        class="device-operation-card"
        :class="{ 'device-operation-card--disabled': !visibleTabSet.has(item.tab) }"
      >
        <div class="device-operation-card__head">
          <strong>{{ item.title }}</strong>
          <NTag size="tiny" :type="item.tone">{{ item.status }}</NTag>
        </div>
        <p>{{ item.description }}</p>
        <NButton
          size="small"
          secondary
          :disabled="!visibleTabSet.has(item.tab)"
          @click="openOperation(item)"
        >
          {{ visibleTabSet.has(item.tab) ? item.action : $t('custom.device_details.workbenchTabUnsupported') }}
        </NButton>
      </article>
    </div>
  </section>
</template>

<style scoped>
.device-operations-workbench {
  display: grid;
  gap: 12px;
  padding: 14px;
  border: 1px solid #dbeafe;
  border-radius: 12px;
  background: linear-gradient(135deg, #eff6ff 0%, #f8fafc 58%, #ecfeff 100%);
}

.device-operations-workbench__summary {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.device-operations-workbench__summary > div {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.device-operations-workbench__summary p {
  max-width: 720px;
  margin: 0;
  color: #475569;
  font-size: 13px;
  line-height: 20px;
}

.device-operations-workbench__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.device-operation-card {
  display: grid;
  gap: 8px;
  min-height: 142px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.86);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.06);
}

.device-operation-card--disabled {
  opacity: 0.62;
}

.device-operation-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.device-operation-card__head strong {
  color: #0f172a;
  font-size: 14px;
}

.device-operation-card p {
  margin: 0;
  color: #64748b;
  font-size: 12px;
  line-height: 19px;
}

@media (max-width: 1080px) {
  .device-operations-workbench__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .device-operations-workbench__summary {
    flex-direction: column;
  }

  .device-operations-workbench__grid {
    grid-template-columns: 1fr;
  }
}
</style>
