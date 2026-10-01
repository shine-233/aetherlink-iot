<!--
文件用途：定时报表工作台（计划列表、创建/编辑、立即运行、运行历史与重试）。
核心逻辑：计划 CRUD 在 useReportSchedules，运行/历史/重试/轮询在 useReportRuns，列定义在 reportColumns，
  两个分页列表的分页/加载/过期请求丢弃统一收口在 useListPage；创建/编辑弹窗与运行历史抽屉分别拆在
  ReportScheduleFormModal / ReportHistoryDrawer，本文件只做编排与布局。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { NAlert, NButton, NCard, NDataTable, NEmpty, NInput, NPagination, NSpin, useMessage } from 'naive-ui'
import type { FormInst } from 'naive-ui'
import type { ReportRun, ReportSchedule } from '@/service/api/report'
import { REPORT_PAGE_SIZE as PAGE_SIZE } from './report-helpers'
import { createRunColumns, createScheduleColumns } from './reportColumns'
import { useReportSchedules } from './useReportSchedules'
import { useReportRuns } from './useReportRuns'
import ReportScheduleFormModal from './ReportScheduleFormModal.vue'
import ReportHistoryDrawer from './ReportHistoryDrawer.vue'

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

// 弹窗内的 NForm 挂载/卸载时回传实例，校验仍由 saveSchedule 通过 formRef 触发。
const setFormRef = (instance: unknown) => {
  formRef.value = (instance as Pick<FormInst, 'validate'>) ?? null
}

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

    <ReportScheduleFormModal
      v-model:show="showForm"
      v-model:device-ids-text="deviceIdsText"
      v-model:keys-text="keysText"
      :editing="editing !== null"
      :form="form"
      :saving="saving"
      :register-form="setFormRef"
      @save="saveSchedule"
    />

    <ReportHistoryDrawer
      v-model:show="historyVisible"
      :schedule="selectedSchedule"
      :history-loading="historyLoading"
      :runs="runs"
      :run-total="runTotal"
      :run-page="runPage"
      :poll-failed="pollFailed"
      :run-columns="runColumns"
      :selected-run="selectedRun"
      :retrying-run-id="retryingRunId"
      @refresh="loadRuns"
      @update:run-page="changeRunPage"
      @retry="retryRun"
    />
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
.report-subtitle {
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
}
</style>
