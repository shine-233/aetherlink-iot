<!--
文件用途：定时报表「运行历史」抽屉（历史分页表、停用/轮询失败提示、选中运行详情，从 index.vue 拆出）。
核心逻辑：历史列表的分页/加载/过期请求丢弃收口在 useReportRuns 的 useListPage；本组件只做展示编排：
  show 走 defineModel；刷新/翻页/重试以事件回传页面；运行列定义由页面经 run-columns 传入，
  选中运行详情复用 ReportRunDetail。
-->
<script setup lang="ts">
import { NAlert, NButton, NDataTable, NDrawer, NDrawerContent, NEmpty, NPagination, NSpin } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { $t } from '@/locales'
import type { ReportRun, ReportSchedule } from '@/service/api/report'
import { REPORT_RUN_PAGE_SIZE as RUN_PAGE_SIZE } from './report-helpers'
import ReportRunDetail from './ReportRunDetail.vue'

defineProps<{
  /** 抽屉对应的计划；停用时展示 scheduleDisabled 提示。 */
  schedule: ReportSchedule | null
  historyLoading: boolean
  runs: ReportRun[]
  runTotal: number
  runPage: number
  pollFailed: boolean
  runColumns: DataTableColumns<ReportRun>
  selectedRun: ReportRun | null
  retryingRunId: string
}>()

const emit = defineEmits<{
  refresh: []
  'update:runPage': [page: number]
  retry: [run: ReportRun]
}>()

const show = defineModel<boolean>('show', { required: true })
</script>

<template>
  <NDrawer v-model:show="show" width="min(1080px, 94vw)" placement="right">
    <NDrawerContent :title="schedule?.name || $t('report.history.title')" closable>
      <div class="report-history-header">
        <div>
          <p>{{ $t('report.history.subtitle') }}</p>
          <strong>{{ schedule?.cron_expr }} · {{ schedule?.timezone }}</strong>
        </div>
        <NButton :loading="historyLoading" @click="emit('refresh')">{{ $t('report.action.refresh') }}</NButton>
      </div>
      <NAlert v-if="!schedule?.enabled" type="info" class="mb-4">
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
            @update:page="emit('update:runPage', $event)"
          />
        </div>
      </NSpin>

      <ReportRunDetail
        v-if="selectedRun"
        :run="selectedRun"
        :schedule-enabled="schedule?.enabled"
        :retrying="retryingRunId === selectedRun.run_id"
        @retry="(run: ReportRun) => emit('retry', run)"
      />
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.report-history-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
}
.report-history-header p {
  max-width: 720px;
  margin: 0;
  color: var(--text-color-2);
}
.report-pagination {
  display: flex;
  justify-content: flex-end;
  padding-top: 18px;
}
</style>
