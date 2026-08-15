<!-- Telemetry page: snapshot, realtime stream, history lookup, simulation publish, and operation logs. -->
<script setup lang="tsx">
import { defineAsyncComponent, ref } from 'vue'
import {
  expectMessageAdd,
  getSimulationInit,
  getTelemetryLogList,
  sendSimulationData,
  telemetryDataDel,
  telemetryDataPub
} from '@/service/api'
import { $t } from '@/locales'
import { isJSON } from '@/utils/common/tool'
import { deviceCustomControlList } from '@/service/api/system-data'
import { TELEMETRY_DETAIL_MODE, useTelemetryDetailState } from './telemetryDetailState'
import { createTelemetryLogColumns } from './telemetryLogColumns'
import { TELEMETRY_LOG_PAGE_SIZE } from './telemetryLogState'
import { buildTelemetryCsv, downloadTelemetryCsv } from './telemetryExport'
import TelemetryOperationsHeader from './TelemetryOperationsHeader.vue'
import TelemetryRealtimeView from './TelemetryRealtimeView.vue'
import { useTelemetryCardViewState } from './telemetryCardViewState'
import { useTelemetryOperationsSection } from './useTelemetryOperationsSection'
import { useTelemetryPublishDialog } from './useTelemetryPublishDialog'
import { useTelemetrySimulationDialog } from './useTelemetrySimulationDialog'
import { useTelemetryRealtimeState } from './useTelemetryRealtimeState'
import { useTelemetryViewShell } from './useTelemetryViewShell'
const props = defineProps<{
  id: string
  deviceTemplateId: string
  deviceData?: Record<string, any>
}>()
const HistoryData = defineAsyncComponent(() => import('./modules/history-data.vue'))
const TimeSeriesData = defineAsyncComponent(() => import('./modules/time-series-data.vue'))
const {
  modelType,
  onTapTableTools,
  openTelemetryHistory,
  openTelemetrySequence,
  showHistory,
  telemetryAccentColor,
  telemetryId,
  telemetryKey,
  telemetryName,
  telemetryUnit
} = useTelemetryDetailState()
// History/time-series dialogs share the current telemetry card context.
const { telemetryData, telemetryLoadError, telemetryLoadStatus, refreshTelemetry } = useTelemetryRealtimeState(
  () => props.id
)
const {
  clearTelemetryCardFilters,
  attentionTelemetryCount,
  displayTelemetryCount,
  displayTelemetryData,
  getTelemetryFreshnessBadge,
  hasTelemetryCardFilters,
  isTelemetryHardRenderCapped,
  showAllTelemetryCards,
  telemetryFreshnessFilter,
  telemetrySearchQuery,
  telemetrySortMode,
  toggleTelemetryDisplayLimit,
  visibleTelemetryCount,
  visibleTelemetryData
} = useTelemetryCardViewState(telemetryData)
const {
  controlList,
  controlListLoaded,
  controlListLoading,
  ensureControlList,
  fetchFirstLogPage,
  handleLogPageChange,
  handleSelect,
  logSectionVisible,
  loading,
  onControlChange,
  openOperationLogs,
  operationOptions,
  operationType,
  options,
  resultOptions,
  sendResult,
  showLog,
  tableData,
  total,
  refreshOperationsSection
} = useTelemetryOperationsSection({
  getDeviceId: () => props.id,
  getDeviceTemplateId: () => props.deviceTemplateId,
  getDeviceConfig: () => props.deviceData?.device_config,
  loadTelemetryLogList: getTelemetryLogList as unknown as Parameters<
    typeof useTelemetryOperationsSection
  >[0]['loadTelemetryLogList'],
  loadDeviceControlList: deviceCustomControlList,
  deleteTelemetryData: telemetryDataDel,
  publishTelemetryData: telemetryDataPub,
  refreshTelemetry,
  translate: $t
})
const {
  clearPublishPayload,
  closePublishDialog,
  form,
  formValue,
  formatPublishPayload,
  handlePositiveClick,
  inputFeedback,
  openDialog,
  showDialog,
  validationJson
} = useTelemetryPublishDialog({
  expectMessageAddRequest: expectMessageAdd,
  telemetryDataPubRequest: telemetryDataPub,
  getDeviceId: () => props.id,
  isJSON,
  onSubmitSuccess: refreshOperationsSection,
  translate: $t
})
const {
  closeSimulationDialog,
  clearSimulationData,
  copySimulationData,
  erroMessage,
  formatSimulationData,
  openUpLog,
  sendSimulationDataByForm,
  showAdvanced,
  showError,
  showLogDialog,
  simulationForm,
  simulationLoading,
  toggleAdvanced
} = useTelemetrySimulationDialog({
  getDeviceId: () => props.id,
  getSimulationInitRequest: getSimulationInit as unknown as (params: { device_id: string }) => Promise<any>,
  sendSimulationDataRequest: sendSimulationData as unknown as (payload: Record<string, any>) => Promise<any>,
  translate: $t,
  isJSON
})
const { cardHeight, cardMargin, getPlatform, nowTime, telemetryFreshnessOptions, telemetrySortOptions } =
  useTelemetryViewShell({
    translate: $t
  })

const columns = createTelemetryLogColumns()
const exportVisibleTelemetryCsv = () => {
  const csv = buildTelemetryCsv(visibleTelemetryData.value, {
    getFreshness: getTelemetryFreshnessBadge,
    translate: $t
  })
  const dateText = new Date().toISOString().slice(0, 10)
  downloadTelemetryCsv(`telemetry-${props.id}-${dateText}.csv`, csv)
}
</script>

<template>
  <n-card class="w-full">
    <TelemetryOperationsHeader
      :control-list="controlList"
      :control-list-loaded="controlListLoaded"
      :control-list-loading="controlListLoading"
      :show-log="showLog"
      @control-change="onControlChange"
      @load-controls="ensureControlList"
      @publish="openDialog"
      @simulate="openUpLog"
    />

    <!-- 遥测实时数据区域 -->
    <TelemetryRealtimeView
      v-model:freshness-filter="telemetryFreshnessFilter"
      v-model:search-query="telemetrySearchQuery"
      v-model:sort-mode="telemetrySortMode"
      :attention-telemetry-count="attentionTelemetryCount"
      :card-height="cardHeight"
      :card-margin="cardMargin"
      :delete-options="options"
      :display-telemetry-count="displayTelemetryCount"
      :display-telemetry-data="displayTelemetryData"
      :get-telemetry-freshness-badge="getTelemetryFreshnessBadge"
      :has-telemetry-card-filters="hasTelemetryCardFilters"
      :is-telemetry-hard-render-capped="isTelemetryHardRenderCapped"
      :now-time="nowTime"
      :show-all-telemetry-cards="showAllTelemetryCards"
      :telemetry-accent-color="telemetryAccentColor"
      :telemetry-data-count="telemetryData.length"
      :telemetry-freshness-options="telemetryFreshnessOptions"
      :telemetry-load-error="telemetryLoadError"
      :telemetry-load-status="telemetryLoadStatus"
      :telemetry-sort-options="telemetrySortOptions"
      :visible-telemetry-count="visibleTelemetryCount"
      :visible-telemetry-data="visibleTelemetryData"
      @clear-filters="clearTelemetryCardFilters"
      @delete-select="handleSelect"
      @export-csv="exportVisibleTelemetryCsv"
      @history="openTelemetryHistory"
      @sequence="openTelemetrySequence"
      @toggle-display-limit="toggleTelemetryDisplayLimit"
    />

    <n-card embedded class="mt-4 telemetry-log-card">
      <template #header>
        <n-space align="center" justify="space-between">
          <span>{{ $t('generate.log') }}</span>
          <n-button v-if="!logSectionVisible" secondary type="primary" :loading="loading" @click="openOperationLogs">
            {{ $t('generate.log') }}
          </n-button>
        </n-space>
      </template>

      <template v-if="logSectionVisible">
        <!-- Operation log filters narrow the list by action type and send result. -->
        <n-space>
          <n-select
            v-model:value="operationType"
            :options="operationOptions"
            style="width: 200px"
            @update:value="fetchFirstLogPage"
          />
          <n-select
            v-model:value="sendResult"
            :options="resultOptions"
            style="width: 200px"
            @update:value="fetchFirstLogPage"
          />
        </n-space>

        <!-- Operation log table and pagination. -->
        <n-data-table :loading="loading" class="mt-4" :columns="columns" :data="tableData" :pagination="false" />
        <div class="mt-4 w-full flex justify-end">
          <n-pagination :page-count="total" :page-size="TELEMETRY_LOG_PAGE_SIZE" @update:page="handleLogPageChange" />
        </div>
      </template>
      <n-empty v-else size="small" :description="$t('common.noData')" />
    </n-card>
    <n-modal
      v-model:show="showLogDialog"
      :title="$t('generate.simulate-report-data')"
      :class="getPlatform ? 'simulation-layout-modal simulation-layout-modal--platform' : 'simulation-layout-modal'"
    >
      <n-card class="simulation-layout-card">
        <n-form class="simulation-layout-form" label-placement="left">
          <!-- Explain simulation prerequisites before publishing a payload. -->
          <n-alert type="info" class="m-b-15px" :show-icon="true">
            {{ $t('generate.simulationTip') }}
          </n-alert>

          <!-- Authentication details are read-only and useful for copying debug parameters. -->
          <div class="simulation-layout-section">
            <div class="m-b-8px font-600">{{ $t('custom.device_details.authenticationInfo') }}</div>
            <n-grid :cols="3" :x-gap="12" responsive="screen" :screen="{ s: 1 }">
              <n-gi>
                <n-form-item :label="$t('generate.username')" :show-feedback="false">
                  <n-input :value="simulationForm.username" readonly class="bg-gray-50" />
                </n-form-item>
              </n-gi>
              <n-gi>
                <n-form-item :label="$t('generate.password')" :show-feedback="false">
                  <n-input :value="simulationForm.password ? '********' : ''" readonly class="bg-gray-50" />
                </n-form-item>
              </n-gi>
              <n-gi>
                <n-form-item :label="$t('generate.clientId')" :show-feedback="false">
                  <n-input :value="simulationForm.client_id" readonly class="bg-gray-50" />
                </n-form-item>
              </n-gi>
            </n-grid>
          </div>

          <!-- Reported payload tools: copy, format, and clear. -->
          <div class="simulation-layout-section">
            <div class="simulation-data-header m-b-8px">
              <div class="font-600">{{ $t('generate.reportData') }}</div>
              <n-space :size="8">
                <n-button size="small" secondary :disabled="!simulationForm.default_data" @click="copySimulationData">
                  {{ $t('generate.copy') }}
                </n-button>
                <n-button size="small" secondary :disabled="!simulationForm.default_data" @click="formatSimulationData">
                  {{ $t('common.format') }}
                </n-button>
                <n-button size="small" secondary :disabled="!simulationForm.default_data" @click="clearSimulationData">
                  {{ $t('common.clear') }}
                </n-button>
              </n-space>
            </div>
            <n-form-item>
              <n-input
                v-model:value="simulationForm.default_data"
                type="textarea"
                :rows="5"
                class="simulation-data-input"
              />
            </n-form-item>
          </div>

          <!-- Advanced parameters stay collapsed by default to keep the form short. -->
          <div class="simulation-layout-section simulation-advanced-section">
            <div class="cursor-pointer flex items-center gap-8px m-b-8px font-600" @click="toggleAdvanced">
              <span>{{ $t('generate.advancedOptions') }}</span>
              <n-icon>
                <svg viewBox="0 0 16 16" width="16" height="16">
                  <path
                    :d="showAdvanced ? 'M4 10l4-4 4 4' : 'M4 6l4 4 4-4'"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.5"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  />
                </svg>
              </n-icon>
            </div>
            <n-collapse-transition :show="showAdvanced">
              <div class="simulation-advanced-fields flex flex-col gap-12px">
                <n-form-item :label="$t('custom.device_details.server')" label-align="left" :show-feedback="false">
                  <n-input v-model:value="simulationForm.server" />
                </n-form-item>
                <n-form-item :label="$t('custom.device_details.port')" label-align="left" :show-feedback="false">
                  <n-input-number v-model:value="simulationForm.port" :min="1" :max="65535" />
                </n-form-item>
                <n-form-item label-align="left" :label="`Topic (${simulationForm.topic})`" :show-feedback="false">
                  <n-select
                    v-model:value="simulationForm.topic"
                    :options="simulationForm.topic_options"
                    :placeholder="$t('generate.selectTopic')"
                  />
                </n-form-item>
              </div>
            </n-collapse-transition>
          </div>

          <!-- Action buttons. -->
          <n-space class="simulation-layout-actions" justify="end">
            <n-button @click="closeSimulationDialog">{{ $t('generate.cancel') }}</n-button>
            <n-button type="primary" :loading="simulationLoading" @click="sendSimulationDataByForm">
              {{ $t('generate.send') }}
            </n-button>
          </n-space>

          <!-- Show backend error details directly so failures are diagnosable. -->
          <div v-if="showError" class="mt-10px w-100% flex items-center gap-5px simulation-error">
            <SvgIcon local-icon="AlertFilled" style="color: red; flex-shrink: 0" class="text-20px" />
            <span style="display: inline-block; word-break: break-all">
              {{ erroMessage }}
            </span>
          </div>
        </n-form>
      </n-card>
    </n-modal>
    <n-modal v-model:show="showDialog" :class="getPlatform ? 'w-90%' : 'w-40%'">
      <n-card :title="$t('generate.distributeControlToDevice')">
        <n-form label-placement="left">
          <div class="flex">
            <n-form-item>
              <template #label>
                <div class="flex-ai-c flex">
                  {{ $t('generate.expectedMessage') }}
                  <n-popover trigger="hover">
                    <template #trigger>
                      <SvgIcon icon="mdi:help-circle-outline" class="text-20px" />
                    </template>
                    <span>{{ $t('generate.expectedMessageTip') }}</span>
                  </n-popover>
                </div>
              </template>

              <n-switch v-model:value="form.expected" />
            </n-form-item>
            <n-form-item v-if="form.expected" :label="$t('generate.expirationTime')" class="ml-20px">
              <div class="flex-ai-c flex">
                <n-input-number v-model:value="form.time" :show-button="false" class="w-80px" />
                <div class="fs-0">{{ $t('generate.hour') }}</div>
              </div>
            </n-form-item>
          </div>
          <div class="publish-dialog-toolbar">
            <n-space :size="8">
              <n-button size="small" secondary :disabled="!formValue" @click="formatPublishPayload">
                {{ $t('common.format') }}
              </n-button>
              <n-button size="small" secondary :disabled="!formValue" @click="clearPublishPayload">
                {{ $t('common.clear') }}
              </n-button>
            </n-space>
          </div>
          <n-form-item label="" :validation-status="validationJson" :feedback="inputFeedback">
            <n-input v-model:value="formValue" type="textarea" />
          </n-form-item>
          <n-space align="end">
            <n-button @click="closePublishDialog">{{ $t('generate.cancel') }}</n-button>

            <n-popconfirm @positive-click="handlePositiveClick">
              <template #trigger>
                <n-button type="primary" :disabled="!formValue || validationJson === 'error'">
                  {{ $t('generate.send') }}
                </n-button>
              </template>
              {{ $t('common.confirmSend') }}
            </n-popconfirm>
          </n-space>
        </n-form>
      </n-card>
    </n-modal>
    <n-modal v-model:show="showHistory" :title="$t('generate.telemetry-history-data')">
      <NCard v-if="showHistory" style="width: 80%">
        <HistoryData
          v-if="showHistory && modelType === TELEMETRY_DETAIL_MODE.history"
          :device-id="telemetryId ?? ''"
          :the-key="telemetryKey ?? ''"
          :the-name="telemetryName ?? ''"
          :the-unit="telemetryUnit ?? ''"
        />
        <TimeSeriesData
          v-if="showHistory && modelType === TELEMETRY_DETAIL_MODE.sequential"
          :device-id="telemetryId ?? ''"
          :the-key="telemetryKey ?? ''"
          :the-name="telemetryName ?? ''"
          :the-unit="telemetryUnit ?? ''"
        />
      </NCard>
    </n-modal>
  </n-card>
</template>

<style lang="scss">
.line1 {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;

  span {
    &:nth-child(2) {
      color: #ccc;
      padding-left: 5px;
    }
  }
}

.card-body {
  padding: 10px 0 10px;
  display: flex;
  align-items: end;
  gap: 4px;

  span {
    &:first-child {
      font-size: 32px;
      line-height: 1;
    }
  }
}
.ml-20px {
  margin-left: 20px;
}
.flex-ai-c {
  align-items: center;
}
.w-80px {
  width: 80px;
}
.fs-0 {
  flex-shrink: 0;
}
.chart-table-dialog {
  width: 80%;
  max-width: 1000px;
}

.telemetry-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.telemetry-toolbar__filters {
  min-width: 0;
}

.telemetry-toolbar__search {
  width: min(320px, 42vw);
}

.telemetry-toolbar__sort {
  width: 220px;
}

.telemetry-toolbar__freshness {
  width: 220px;
}

.telemetry-toolbar__summary {
  color: #64748b;
  font-size: 13px;
  white-space: nowrap;
}

.telemetry-state-alert {
  margin-bottom: 16px;
}

.telemetry-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
}

.telemetry-card-footer__time {
  min-width: 0;
  overflow: hidden;
  color: #64748b;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.publish-dialog-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 10px;
}

.value-display-ellipsis {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  text-overflow: ellipsis;
  word-break: break-all;
}

.simulation-layout-modal {
  width: min(920px, calc(100vw - 48px));
}

.simulation-layout-modal--platform {
  width: min(1040px, 90vw);
}

.simulation-layout-card {
  max-height: min(82vh, 760px);
  display: flex;
  flex-direction: column;
}

.simulation-layout-card > .n-card__content {
  min-height: 0;
  padding: 20px 24px 18px;
  overflow: auto;
}

.simulation-layout-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.simulation-layout-form .n-alert {
  margin-bottom: 0;
}

.simulation-layout-section {
  margin-bottom: 0;
  padding: 14px 16px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}

.simulation-layout-section .n-form-item:last-child {
  margin-bottom: 0;
}

.simulation-data-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.simulation-data-input .n-input__textarea-el {
  min-height: 150px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.55;
}

.simulation-advanced-section {
  padding-block: 12px;
}

.simulation-advanced-section .n-collapse-transition {
  margin-top: 12px;
}

.simulation-advanced-fields .n-form-item {
  display: grid;
  grid-template-columns: 210px minmax(0, 1fr);
  align-items: center;
  column-gap: 12px;
}

.simulation-advanced-fields .n-form-item-label {
  width: 100%;
  justify-content: flex-start;
  padding-right: 0;
  white-space: nowrap;
}

.simulation-advanced-fields .n-form-item-blank,
.simulation-advanced-fields .n-input-number {
  width: 100%;
}

.simulation-layout-actions {
  padding-top: 2px;
}

.simulation-error {
  margin-top: 0;
  padding: 10px 12px;
  border: 1px solid rgba(208, 48, 80, 0.22);
  border-radius: 6px;
  background: rgba(208, 48, 80, 0.06);
}

@media (max-width: 768px) {
  .simulation-layout-modal,
  .simulation-layout-modal--platform {
    width: calc(100vw - 24px);
  }

  .simulation-layout-card > .n-card__content {
    padding: 16px;
  }

  .simulation-data-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .telemetry-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .telemetry-toolbar__filters {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
  }

  .telemetry-toolbar__search,
  .telemetry-toolbar__sort,
  .telemetry-toolbar__freshness {
    width: 100%;
  }

  .simulation-advanced-fields .n-form-item {
    grid-template-columns: 1fr;
    row-gap: 6px;
  }
}
</style>
