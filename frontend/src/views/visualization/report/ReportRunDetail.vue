<!--
文件用途：定时报表运行历史抽屉里的「选中运行」详情卡。
核心逻辑：只读展示生成/投递状态、SMTP 接受/不确定、错误码；可重试时 emit retry。
-->
<script setup lang="ts">
import { NAlert, NButton, NDescriptions, NDescriptionsItem, NSpace, NTag } from 'naive-ui'
import { $t } from '@/locales'
import type { ReportRun } from '@/service/api/report'
import { canRetryReportRun, reportStatusTagType } from './report-model'
import { formatReportTime, isSmtpAccepted, isSmtpAmbiguous, reportRunWindow, reportStatusText } from './report-helpers'

defineProps<{
  run: ReportRun
  scheduleEnabled?: boolean
  retrying: boolean
}>()

defineEmits<{ retry: [run: ReportRun] }>()

const yesNo = (value: boolean) => (value ? $t('report.common.yes') : $t('report.common.no'))
</script>

<template>
  <section class="report-run-detail" data-testid="report-run-detail">
    <div class="report-detail-heading">
      <div>
        <p class="report-kicker">{{ $t('report.detail.kicker') }}</p>
        <h2>{{ run.run_id }}</h2>
      </div>
      <NTag :type="reportStatusTagType(run.overall_status)">{{ reportStatusText(run.overall_status) }}</NTag>
    </div>
    <NAlert v-if="isSmtpAmbiguous(run) || run.duplicate_delivery_risk" type="warning" class="mb-4">
      {{ $t('report.risk.warning') }}
    </NAlert>
    <NDescriptions bordered :column="2" label-placement="left">
      <NDescriptionsItem :label="$t('report.history.window')">{{ reportRunWindow(run) }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('report.detail.updated')">
        {{ formatReportTime(run.delivery_completed_at || run.generation_completed_at || run.created_at) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('report.history.generation')">
        {{ reportStatusText(run.generation_status) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('report.history.delivery')">
        {{ reportStatusText(run.delivery_status) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('report.detail.smtpAccepted')">{{ yesNo(isSmtpAccepted(run)) }}</NDescriptionsItem>
      <NDescriptionsItem :label="$t('report.detail.smtpAmbiguous')">
        {{ yesNo(isSmtpAmbiguous(run)) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('report.detail.generationAttempts')">
        {{ run.generation_attempts }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="$t('report.detail.deliveryAttempts')">{{ run.delivery_attempts }}</NDescriptionsItem>
    </NDescriptions>
    <NAlert v-if="isSmtpAccepted(run)" type="info" class="mt-4">
      {{ $t('report.detail.smtpAcceptanceCaveat') }}
    </NAlert>
    <h3>{{ $t('report.detail.errors') }}</h3>
    <NAlert v-if="run.generation_error_code" type="error" class="mb-2">
      {{ $t('report.history.generation') }}: {{ run.generation_error_code }}
    </NAlert>
    <NAlert v-if="run.delivery_error_code" type="error" class="mb-2">
      {{ $t('report.history.delivery') }}: {{ run.delivery_error_code }}
    </NAlert>
    <p v-if="!run.generation_error_code && !run.delivery_error_code" class="report-muted">
      {{ $t('report.detail.noErrors') }}
    </p>
    <NSpace justify="end">
      <NButton
        v-if="canRetryReportRun(run, scheduleEnabled)"
        type="warning"
        :loading="retrying"
        @click="$emit('retry', run)"
      >
        {{ $t('report.action.retry') }}
      </NButton>
    </NSpace>
  </section>
</template>

<style scoped>
.report-run-detail {
  margin-top: 26px;
  padding: 22px;
  border: 1px solid rgba(125, 140, 132, 0.28);
  border-radius: 10px;
  background: rgba(32, 166, 106, 0.035);
}
.report-detail-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
}
.report-kicker {
  margin: 0;
  color: rgb(var(--success-800-color));
  font:
    700 12px/1.4 ui-monospace,
    monospace;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}
.report-muted {
  margin-top: 4px;
  color: var(--text-color-3);
  font-size: 12px;
}
h2 {
  margin: 3px 0 0;
  font:
    650 21px ui-monospace,
    monospace;
  overflow-wrap: anywhere;
}
h3 {
  margin: 24px 0 12px;
  font-size: 15px;
}
</style>
