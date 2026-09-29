<!--
文件用途：定时报表工作台（计划列表、创建/编辑、立即运行、运行历史与重试）。
核心逻辑：计划 CRUD 在 useReportSchedules，运行/历史/重试/轮询在 useReportRuns，列定义在 reportColumns，
  选中运行详情在 ReportRunDetail；本文件只做编排与布局。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NDrawer,
  NDrawerContent,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPagination,
  NSelect,
  NSpace,
  NSpin,
  NSwitch,
  useMessage
} from 'naive-ui'
import { $t } from '@/locales'
import type { ReportRun, ReportSchedule } from '@/service/api/report'
import {
  REPORT_FORMAT_OPTIONS,
  REPORT_PAGE_SIZE as PAGE_SIZE,
  REPORT_RUN_PAGE_SIZE as RUN_PAGE_SIZE
} from './report-helpers'
import { createRunColumns, createScheduleColumns } from './reportColumns'
import { useReportSchedules } from './useReportSchedules'
import { useReportRuns } from './useReportRuns'
import ReportRunDetail from './ReportRunDetail.vue'

const message = useMessage()
const {
  schedules,
  total,
  page,
  searchInput,
  listLoading,
  listFailed,
  showForm,
  editing,
  saving,
  deletingId,
  formRef,
  form,
  deviceIdsText,
  keysText,
  loadSchedules,
  searchSchedules,
  changePage,
  openCreate,
  openEdit,
  saveSchedule,
  removeSchedule,
  dispose: disposeSchedules
} = useReportSchedules(message)

const {
  runningId,
  selectedSchedule,
  historyVisible,
  historyLoading,
  runs,
  runTotal,
  runPage,
  selectedRun,
  retryingRunId,
  pollFailed,
  loadRuns,
  runNow,
  openHistory,
  changeRunPage,
  selectRun,
  retryRun,
  dispose: disposeRuns
} = useReportRuns({ message, reloadSchedules: loadSchedules })

// 列标题随语言切换重算；按钮状态通过 getter 在单元格渲染时读取。
const scheduleColumns = computed(() =>
  createScheduleColumns({
    runningId: () => runningId.value,
    deletingId: () => deletingId.value,
    onRun: (row) => runNow(row),
    onHistory: openHistory,
    onEdit: openEdit,
    onDelete: removeSchedule
  })
)
const runColumns = computed(() =>
  createRunColumns({
    scheduleEnabled: () => selectedSchedule.value?.enabled,
    retryingRunId: () => retryingRunId.value,
    onSelect: selectRun,
    onRetry: (run) => retryRun(run)
  })
)

onMounted(loadSchedules)
onBeforeUnmount(() => {
  disposeSchedules()
  disposeRuns()
})
</script>

<template>
  <main class="report-workspace">
    <header class="report-header">
      <div>
        <p class="report-kicker">{{ $t('report.page.kicker') }}</p>
        <h1>{{ $t('report.page.title') }}</h1>
        <p class="report-subtitle">{{ $t('report.page.subtitle') }}</p>
      </div>
      <NButton type="primary" size="large" data-testid="report-create" @click="openCreate">
        {{ $t('report.action.create') }}
      </NButton>
    </header>

    <section class="report-command-bar">
      <div class="report-cadence">
        <span>{{ $t('report.page.cadenceLabel') }}</span>
        <strong>{{ $t('report.page.cadenceValue') }}</strong>
      </div>
      <NInput
        v-model:value="searchInput"
        clearable
        :placeholder="$t('report.page.search')"
        @keyup.enter="searchSchedules"
      />
      <NButton @click="searchSchedules">{{ $t('report.action.search') }}</NButton>
      <NButton :loading="listLoading" @click="loadSchedules">{{ $t('report.action.refresh') }}</NButton>
    </section>

    <NAlert v-if="listFailed" type="error" class="mb-4">{{ $t('report.message.loadFailed') }}</NAlert>
    <NCard class="report-ledger" :bordered="false">
      <NSpin :show="listLoading">
        <NDataTable
          v-if="schedules.length"
          :columns="scheduleColumns"
          :data="schedules"
          :row-key="(row: ReportSchedule) => row.id"
          :scroll-x="1200"
          data-testid="report-schedule-table"
        />
        <NEmpty v-else-if="!listLoading" :description="$t('report.page.empty')" class="py-20">
          <template #extra>
            <NButton type="primary" @click="openCreate">{{ $t('report.action.createFirst') }}</NButton>
          </template>
        </NEmpty>
        <div v-if="total > PAGE_SIZE" class="report-pagination">
          <NPagination :page="page" :page-size="PAGE_SIZE" :item-count="total" @update:page="changePage" />
        </div>
      </NSpin>
    </NCard>

    <NModal
      v-model:show="showForm"
      preset="card"
      :title="$t(editing ? 'report.form.editTitle' : 'report.form.createTitle')"
      class="report-form-modal"
    >
      <NForm ref="formRef" :model="form" label-placement="top">
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
            <NInput
              v-model:value="deviceIdsText"
              type="textarea"
              :rows="5"
              :placeholder="$t('report.form.linesHint')"
            />
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
          <NButton :disabled="saving" @click="showForm = false">{{ $t('report.action.cancel') }}</NButton>
          <NButton type="primary" :loading="saving" @click="saveSchedule">{{ $t('report.action.save') }}</NButton>
        </NSpace>
      </template>
    </NModal>

    <NDrawer v-model:show="historyVisible" width="min(1080px, 94vw)" placement="right">
      <NDrawerContent :title="selectedSchedule?.name || $t('report.history.title')" closable>
        <div class="report-history-header">
          <div>
            <p>{{ $t('report.history.subtitle') }}</p>
            <strong>{{ selectedSchedule?.cron_expr }} · {{ selectedSchedule?.timezone }}</strong>
          </div>
          <NButton :loading="historyLoading" @click="loadRuns">{{ $t('report.action.refresh') }}</NButton>
        </div>
        <NAlert v-if="!selectedSchedule?.enabled" type="info" class="mb-4">
          {{ $t('report.message.scheduleDisabled') }}
        </NAlert>
        <NAlert v-if="pollFailed" type="error" class="mb-4" data-testid="report-poll-failed">
          {{ $t('report.message.pollFailed') }}
        </NAlert>
        <NSpin :show="historyLoading">
          <NDataTable
            v-if="runs.length"
            :columns="runColumns"
            :data="runs"
            :row-key="(row: ReportRun) => row.run_id"
            :scroll-x="1100"
          />
          <NEmpty v-else-if="!historyLoading" :description="$t('report.history.empty')" class="py-16" />
          <div v-if="runTotal > RUN_PAGE_SIZE" class="report-pagination">
            <NPagination
              :page="runPage"
              :page-size="RUN_PAGE_SIZE"
              :item-count="runTotal"
              @update:page="changeRunPage"
            />
          </div>
        </NSpin>

        <ReportRunDetail
          v-if="selectedRun"
          :run="selectedRun"
          :schedule-enabled="selectedSchedule?.enabled"
          :retrying="retryingRunId === selectedRun.run_id"
          @retry="retryRun"
        />
      </NDrawerContent>
    </NDrawer>
  </main>
</template>

<style scoped>
.report-workspace {
  min-height: 100%;
  padding: clamp(18px, 3vw, 36px);
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.05), transparent 36%), var(--n-color);
  color: var(--n-text-color);
}
.report-header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 24px;
  max-width: 1440px;
  margin: 0 auto 20px;
  padding: 24px 4px 4px;
  border-bottom: 1px solid rgba(125, 140, 132, 0.25);
}
.report-header h1 {
  margin: 2px 0 8px;
  font:
    700 clamp(30px, 4vw, 52px)/1.05 Georgia,
    serif;
  letter-spacing: -0.035em;
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
.report-subtitle,
.report-history-header p {
  max-width: 720px;
  margin: 0;
  color: var(--text-color-2);
}
.report-command-bar {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) minmax(260px, 2fr) auto auto;
  gap: 10px;
  align-items: center;
  max-width: 1440px;
  margin: 0 auto 14px;
}
.report-cadence {
  display: flex;
  flex-direction: column;
  padding-left: 14px;
  border-left: 3px solid rgb(var(--success-color));
  color: var(--text-color-2);
  font-size: 12px;
}
.report-cadence strong {
  color: var(--n-text-color);
  font:
    600 14px ui-monospace,
    monospace;
}
.report-ledger {
  max-width: 1440px;
  margin: 0 auto;
  box-shadow: 0 16px 46px rgba(23, 54, 39, 0.08);
}
.report-muted {
  margin-top: 4px;
  color: var(--text-color-3);
  font-size: 12px;
}
.report-pagination {
  display: flex;
  justify-content: flex-end;
  padding-top: 18px;
}
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
.report-history-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
}
@media (max-width: 760px) {
  .report-header {
    align-items: start;
    flex-direction: column;
  }
  .report-command-bar {
    grid-template-columns: 1fr 1fr;
  }
  .report-cadence,
  .report-command-bar :deep(.n-input) {
    grid-column: 1 / -1;
  }
  .report-form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
