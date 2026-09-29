<!--
文件用途：告警详情弹窗（基础信息、闭环下一步、处置时间线、关联设备、评论与指派）。
核心逻辑：只读展示父级传入的告警行与派生证据；处置动作通过 emit 交回父级，审计日志/设备诊断跳转在本组件内完成。
关键注意事项：评论/指派面板依赖 NModal 默认 display-directive="if" 懒渲染，showDialog 只会在写好 alarm 后为 true。
-->
<script setup lang="ts">
import { NAlert, NButton, NCard, NFlex, NFormItem, NH3, NModal, NTable, NTag } from 'naive-ui'
import dayjs from 'dayjs'
import { useRouter } from 'vue-router'
import { $t } from '@/locales'
import {
  alarmActionField,
  alarmSeverityLabel,
  alarmSeverityValue,
  alarmTypeLabel,
  type AlarmClosureNextAction,
  type AlarmOption,
  type AlarmResolutionTimelineItem
} from './alarm-configuration.helpers'
import AlarmCommentPanel from './AlarmCommentPanel.vue'
import AlarmAssignmentPanel from './AlarmAssignmentPanel.vue'
import { alarmDeviceId, buildAlarmAuditLogRoute, buildAlarmDeviceReadyCheckRoute } from './alarm-detail-routes'

const props = defineProps<{
  alarm: Record<string, any>
  statusOptions: AlarmOption[]
  nextAction: AlarmClosureNextAction
  timelineItems: AlarmResolutionTimelineItem[]
  needsAcknowledge: boolean
  needsReset: boolean
}>()

const show = defineModel<boolean>('show', { required: true })

defineEmits<{
  acknowledge: []
  reset: []
  maintenance: []
  copyEvidence: []
  downloadEvidence: []
}>()

const router = useRouter()

const openAuditLog = () => router.push(buildAlarmAuditLogRoute(props.alarm?.create_at))
const openDeviceReadyCheck = (device: unknown) => {
  const route = buildAlarmDeviceReadyCheckRoute(device, props.alarm?.id)
  if (route) router.push(route)
}

const infoRows = () => [
  { key: 'name', label: `${$t('generate.alarmConfugName')}:`, value: props.alarm.name },
  { key: 'config', label: `${$t('generate.sceneLinkageName')}:`, value: props.alarm.alarm_config_name },
  {
    key: 'time',
    label: `${$t('common.alarm_time')}:`,
    value: dayjs(props.alarm.create_at).format('YYYY-MM-DD HH:mm:ss')
  },
  {
    key: 'status',
    label: `${$t('generate.alarm-status')}:`,
    value: props.statusOptions.find((option) => option.value === props.alarm.alarm_status)?.label || ''
  },
  {
    key: 'level',
    label: `${$t('common.alarm_level')}:`,
    value: alarmSeverityLabel(alarmSeverityValue(props.alarm), props.statusOptions)
  },
  { key: 'type', label: `${$t('rdi.overview.alarmType')}:`, value: alarmTypeLabel(props.alarm, $t) },
  { key: 'reason', label: `${$t('generate.alarmReason')}:`, value: props.alarm.content },
  { key: 'desc', label: `${$t('generate.alarm-description')}:`, value: props.alarm.description },
  {
    key: 'ackBy',
    label: `${$t('rdi.overview.acknowledgedBy')}:`,
    value: alarmActionField(props.alarm, 'acknowledged_by')
  },
  {
    key: 'ackAt',
    label: `${$t('rdi.overview.acknowledgedAt')}:`,
    value: alarmActionField(props.alarm, 'acknowledged_at')
  },
  { key: 'resetBy', label: `${$t('rdi.overview.resetBy')}:`, value: alarmActionField(props.alarm, 'reset_by') },
  { key: 'resetAt', label: `${$t('rdi.overview.resetAt')}:`, value: alarmActionField(props.alarm, 'reset_at') }
]
</script>

<template>
  <NModal v-model:show="show" :title="$t('generate.alarm-info')" class="max-w-[800px]">
    <NCard class="alarm-detail-modal-card">
      <NH3>{{ $t('generate.alarm-info') }}</NH3>
      <NAlert :type="nextAction.type" :show-icon="false" class="alarm-closure-next-alert">
        <div class="alarm-closure-next-alert__head">
          <NTag :type="nextAction.type" size="small">{{ nextAction.status }}</NTag>
          <strong>{{ nextAction.nextStep }}</strong>
        </div>
        <div class="alarm-closure-next-alert__evidence">{{ nextAction.evidence }}</div>
      </NAlert>
      <NFormItem
        v-for="row in infoRows()"
        :key="row.key"
        label-placement="left"
        :show-feedback="false"
        :label="row.label"
      >
        {{ row.value }}
      </NFormItem>
      <NCard embedded size="small" class="alarm-resolution-card">
        <div class="alarm-resolution-header">
          <div>
            <div class="alarm-resolution-title">{{ $t('custom.alarmPage.timelineTitle') }}</div>
            <div class="alarm-resolution-desc">{{ $t('custom.alarmPage.timelineDesc') }}</div>
          </div>
          <NFlex :size="8" wrap justify="end">
            <NButton size="small" secondary data-testid="alarm-copy-closure-evidence" @click="$emit('copyEvidence')">
              {{ $t('custom.alarmPage.copyClosureEvidence') }}
            </NButton>
            <NButton
              size="small"
              secondary
              data-testid="alarm-download-detail-evidence"
              @click="$emit('downloadEvidence')"
            >
              {{ $t('custom.alarmPage.downloadEvidenceBundle') }}
            </NButton>
            <NButton size="small" secondary @click="openAuditLog">
              {{ $t('custom.alarmPage.viewAuditLog') }}
            </NButton>
          </NFlex>
        </div>
        <div class="alarm-resolution-timeline">
          <div v-for="item in timelineItems" :key="item.key" class="alarm-resolution-item">
            <NTag :type="item.type" size="small">{{ item.title }}</NTag>
            <div class="alarm-resolution-item__body">
              <strong>{{ item.time }}</strong>
              <span>{{ item.description }}</span>
            </div>
          </div>
        </div>
        <NAlert type="info" :show-icon="false" class="mt-3">
          {{ $t('custom.alarmPage.auditBoundaryHint') }}
        </NAlert>
        <NFlex class="mt-3" :size="8" wrap>
          <NButton v-if="needsAcknowledge" size="small" type="success" secondary @click="$emit('acknowledge')">
            {{ $t('rdi.overview.acknowledgeAlarm') }}
          </NButton>
          <NButton v-if="needsReset" size="small" type="error" secondary @click="$emit('reset')">
            {{ $t('common.reset') }}
          </NButton>
          <NButton size="small" secondary @click="$emit('maintenance')">
            {{ $t('rdi.overview.maintenanceNote') }}
          </NButton>
        </NFlex>
      </NCard>
      <NFormItem label-placement="top" :show-feedback="false" :label="$t('generate.alarmDevices') + ':'">
        <NTable size="small" :bordered="false" :single-line="false" class="mb-6">
          <thead>
            <tr>
              <th>{{ $t('common.index') }}</th>
              <th class="min-w-180px">{{ $t('generate.device-code') }}</th>
              <th>{{ $t('custom.devicePage.deviceName') }}</th>
              <th>{{ $t('custom.device_details.readyCheck') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(device, index) in alarm.alarm_device_list" :key="index">
              <td class="min-w-100px">{{ Number(index) + 1 }}</td>
              <td>{{ device.id }}</td>
              <td>{{ device.name }}</td>
              <td>
                <NButton
                  size="small"
                  secondary
                  type="primary"
                  :disabled="!alarmDeviceId(device)"
                  data-testid="alarm-device-ready-check"
                  @click="openDeviceReadyCheck(device)"
                >
                  {{ $t('custom.commandCenter.openDeviceDiagnosis') }}
                </NButton>
              </td>
            </tr>
          </tbody>
        </NTable>
      </NFormItem>
      <NCard embedded size="small" class="alarm-resolution-card">
        <div class="alarm-resolution-title mb-12px">{{ $t('custom.alarmComment.title') }}</div>
        <AlarmCommentPanel data-testid="alarm-comment-panel" :alarm-history-id="alarm.id" />
      </NCard>
      <NCard embedded size="small" class="alarm-resolution-card">
        <div class="alarm-resolution-title mb-12px">{{ $t('custom.alarmAssignment.title') }}</div>
        <AlarmAssignmentPanel data-testid="alarm-assignment-panel" :alarm-history-id="alarm.id" />
      </NCard>
      <NFlex justify="flex-end">
        <NButton @click="show = false">{{ $t('custom.devicePage.close') }}</NButton>
      </NFlex>
    </NCard>
  </NModal>
</template>

<style scoped lang="scss">
.alarm-detail-modal-card {
  width: min(96vw, 800px);
}

.alarm-resolution-card {
  margin: 12px 0 18px;
}

.alarm-closure-next-alert {
  margin-bottom: 12px;
}

.alarm-closure-next-alert__head {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.alarm-closure-next-alert__evidence {
  margin-top: 6px;
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.alarm-resolution-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.alarm-resolution-title {
  font-weight: 600;
}

.alarm-resolution-desc {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.alarm-resolution-timeline {
  display: grid;
  gap: 10px;
}

.alarm-resolution-item {
  display: grid;
  grid-template-columns: 148px minmax(0, 1fr);
  gap: 10px;
  align-items: flex-start;
  padding: 10px 12px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}

.alarm-resolution-item__body {
  min-width: 0;

  strong,
  span {
    display: block;
  }

  span {
    margin-top: 4px;
    color: #475569;
    overflow-wrap: anywhere;
  }
}

@media (max-width: 900px) {
  .alarm-resolution-header {
    flex-direction: column;
  }

  .alarm-resolution-item {
    grid-template-columns: 1fr;
  }
}
</style>
