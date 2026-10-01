<!--
文件用途：定时报表计划的创建/编辑弹窗（纯展示层，从 index.vue 拆出）。
核心逻辑：表单数据与校验仍归 useReportSchedules 所有：form 以只读方式传入（v-model 直接改其字段，
  与原实现一致）；show/deviceIdsText/keysText 走 defineModel；保存时 emit save，由页面 saveSchedule
  统一校验与提交；NForm 实例经 registerForm 函数引用回传页面，写入组合式的 formRef。
-->
<script setup lang="ts">
import { NButton, NForm, NFormItem, NInput, NInputNumber, NModal, NSelect, NSpace, NSwitch } from 'naive-ui'
import { $t } from '@/locales'
import type { ReportSchedulePayload } from '@/service/api/report'
import { REPORT_FORMAT_OPTIONS } from './report-helpers'

defineProps<{
  /** 编辑已有计划时标题切换为编辑态（对应 report.form.editTitle）。 */
  editing: boolean
  form: ReportSchedulePayload
  saving: boolean
  /** NForm 挂载/卸载时回传实例（卸载传 null），页面据此维护 useReportSchedules 的 formRef。 */
  registerForm: (instance: unknown) => void
}>()

const emit = defineEmits<{ save: [] }>()

const show = defineModel<boolean>('show', { required: true })
const deviceIdsText = defineModel<string>('deviceIdsText', { required: true })
const keysText = defineModel<string>('keysText', { required: true })
</script>

<template>
  <NModal
    v-model:show="show"
    preset="card"
    :title="$t(editing ? 'report.form.editTitle' : 'report.form.createTitle')"
    class="report-form-modal"
  >
    <NForm :ref="registerForm" :model="form" label-placement="top">
      <div class="report-form-grid">
        <NFormItem
          :label="$t('report.form.name')"
          path="name"
          :rule="{ required: true, message: $t('report.form.required') }"
        >
          <NInput v-model:value="form.name" :maxlength="128" />
        </NFormItem>
        <NFormItem
          :label="$t('report.form.cron')"
          path="cron_expr"
          :rule="{ required: true, message: $t('report.form.required') }"
        >
          <NInput v-model:value="form.cron_expr" placeholder="0 8 * * 1-5" />
        </NFormItem>
        <NFormItem
          :label="$t('report.form.timezone')"
          path="timezone"
          :rule="{ required: true, message: $t('report.form.required') }"
        >
          <NInput v-model:value="form.timezone" placeholder="Europe/Paris" />
        </NFormItem>
        <NFormItem :label="$t('report.form.lookback')" path="lookback_hours">
          <NInputNumber v-model:value="form.lookback_hours" :min="1" :max="8760" class="w-full" />
        </NFormItem>
      </div>
      <NFormItem
        :label="$t('report.form.recipients')"
        path="recipients"
        :rule="{ required: true, message: $t('report.form.required') }"
      >
        <NInput v-model:value="form.recipients" :placeholder="$t('report.form.recipientsHint')" />
      </NFormItem>
      <div class="report-form-grid">
        <NFormItem :label="$t('report.form.devices')">
          <NInput v-model:value="deviceIdsText" type="textarea" :rows="5" :placeholder="$t('report.form.linesHint')" />
        </NFormItem>
        <NFormItem :label="$t('report.form.keys')">
          <NInput v-model:value="keysText" type="textarea" :rows="5" :placeholder="$t('report.form.linesHint')" />
        </NFormItem>
      </div>
      <div class="report-form-footer-row">
        <NSelect
          v-model:value="form.format"
          class="!w-36"
          :options="REPORT_FORMAT_OPTIONS"
          :consistent-menu-width="false"
          data-testid="report-format"
        />
        <label>
          <span>{{ $t('report.form.enabled') }}</span>
          <NSwitch v-model:value="form.enabled" />
        </label>
      </div>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton :disabled="saving" @click="show = false">{{ $t('report.action.cancel') }}</NButton>
        <NButton type="primary" :loading="saving" @click="emit('save')">{{ $t('report.action.save') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.report-form-modal {
  width: min(760px, 92vw);
}
.report-form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 18px;
}
.report-form-footer-row,
.report-form-footer-row label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
@media (max-width: 760px) {
  .report-form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
