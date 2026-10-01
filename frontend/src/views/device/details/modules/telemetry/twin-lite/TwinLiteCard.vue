<!--
  文件用途: 设备孪生（Twin Lite）卡片：期望值 vs 上报值对比、收敛确认、修复清单与期望值编辑。
  核心逻辑: 状态、加载与动作由 useTwinLite 提供，派生规则在 twin-lite-view-model.ts；
  修复清单与期望值编辑弹窗拆为 TwinRepairChecklist / TwinDesiredEditorModal。
-->
<script setup lang="ts">
import { computed, h } from 'vue'
import { NButton } from 'naive-ui'
import { $t } from '@/locales'
import type { TwinLiteRow } from './twin-lite-normalizer'
import { formatTwinValue, twinMetadataLines } from './twin-lite-view-model'
import { useTwinLite } from './useTwinLite'
import TwinRepairChecklist from './TwinRepairChecklist.vue'
import TwinDesiredEditorModal from './TwinDesiredEditorModal.vue'

const props = defineProps<{
  id: string
  reportedTelemetry: DeviceManagement.telemetryData[]
}>()

const {
  loading,
  savingDesired,
  loadError,
  desiredError,
  desiredDialogVisible,
  editingRow,
  desiredForm,
  onlyShowDelta,
  sourceLabelMap,
  twinState,
  driftRows,
  previewRows,
  primaryRepairRow,
  repair,
  guidanceItems,
  confirmation,
  desiredObservation,
  openCreateDesired,
  openEditDesired,
  closeDesiredDialog,
  useReportedValue,
  openPrimaryRepair,
  copyTwinRepairSummary,
  downloadTwinEvidenceBundle,
  loadTwinLite,
  submitDesired
} = useTwinLite(props)

const hasTwinDrift = computed(() => twinState.value.summary.deltaCount > 0)

const statistics = computed(() => [
  { label: 'twinDesired', value: twinState.value.summary.desiredCount },
  { label: 'twinReported', value: twinState.value.summary.reportedCount },
  { label: 'twinMatched', value: twinState.value.summary.matchedCount },
  { label: 'twinDelta', value: twinState.value.summary.deltaCount },
  { label: 'twinUnavailable', value: twinState.value.summary.unavailableCount }
])

const columns = computed(() => [
  { title: $t('page.expect.label'), key: 'label' },
  {
    title: $t('page.expect.commandType'),
    key: 'source',
    render: (row: TwinLiteRow) => sourceLabelMap[row.source] || row.source
  },
  {
    title: $t('custom.device_details.twinDesired'),
    key: 'desired',
    render: (row: TwinLiteRow) => formatTwinValue(row.desired)
  },
  {
    title: $t('custom.device_details.twinReported'),
    key: 'reported',
    render: (row: TwinLiteRow) => formatTwinValue(row.reported)
  },
  {
    title: $t('custom.device_details.twinStateMetadata'),
    key: 'state_metadata',
    render: (row: TwinLiteRow) => {
      const lines = twinMetadataLines(row, $t)
      if (!lines.length) return '--'
      return h(
        'div',
        { class: 'text-12px leading-20px' },
        lines.map((line) => h('div', { class: 'break-all' }, line))
      )
    }
  },
  {
    title: $t('custom.device_details.twinStatus'),
    key: 'matched',
    render: (row: TwinLiteRow) => {
      if (!row.comparable) return $t('custom.device_details.twinNotComparable')
      return row.matched ? $t('custom.device_details.twinMatched') : $t('custom.device_details.twinDelta')
    }
  },
  {
    title: $t('common.actions'),
    key: 'actions',
    render: (row: TwinLiteRow) => {
      if (row.source === 'command') return '--'
      return h(
        NButton,
        { text: true, type: 'primary', size: 'small', onClick: () => openEditDesired(row) },
        { default: () => $t('generate.edit') }
      )
    }
  }
])
</script>

<template>
  <n-card class="mb-4" :title="$t('custom.device_details.twinLite')">
    <template #header-extra>
      <div class="flex items-center gap-12px">
        <n-button type="primary" secondary @click="openCreateDesired">
          {{ $t('custom.device_details.twinSetDesired') }}
        </n-button>
        <n-button text :loading="loading" @click="loadTwinLite">
          {{ $t('generate.refresh') }}
        </n-button>
      </div>
    </template>

    <n-alert v-if="loadError" type="warning" class="mb-3" :show-icon="false">
      {{ loadError }}
    </n-alert>

    <n-alert :type="confirmation.type" class="mb-3" :show-icon="false" data-testid="device-twin-confirmation">
      <n-space vertical :size="8">
        <div class="flex flex-col gap-8px md:flex-row md:items-center md:justify-between">
          <div class="min-w-0">
            <div class="font-600">{{ confirmation.title }}</div>
            <div class="mt-4px text-12px line-height-18px text-gray-500">
              {{ confirmation.action }}
            </div>
          </div>
          <n-tag round size="small" :type="confirmation.type">
            {{ $t('custom.device_details.twinConfirmationReadyGate') }}
          </n-tag>
        </div>
        <div class="text-12px line-height-18px text-gray-500">
          {{ confirmation.boundary }}
          <template v-if="twinState.summary.staleDesiredCount">
            ·
            {{
              $t('custom.device_details.twinConfirmationExpiredDesired').replace(
                '{count}',
                String(twinState.summary.staleDesiredCount)
              )
            }}
          </template>
        </div>
      </n-space>
    </n-alert>

    <n-alert
      v-if="desiredObservation"
      :type="desiredObservation.type"
      class="mb-3"
      :show-icon="false"
      data-testid="device-twin-desired-observation"
    >
      <div class="flex flex-col gap-8px md:flex-row md:items-center md:justify-between">
        <div class="min-w-0">
          <div class="font-600">{{ desiredObservation.title }}</div>
          <div class="mt-4px text-12px line-height-18px text-gray-500">
            {{ $t('custom.device_details.twinDesiredSavedNextStep') }}
          </div>
          <div class="mt-4px text-12px line-height-18px text-gray-500">
            {{ desiredObservation.meta }}
          </div>
        </div>
        <n-space :size="8" :wrap="true">
          <n-button size="small" secondary :loading="loading" @click="loadTwinLite">
            {{ $t('generate.refresh') }}
          </n-button>
          <n-button size="small" secondary @click="copyTwinRepairSummary">
            {{ $t('custom.device_details.twinRepairCopySummary') }}
          </n-button>
        </n-space>
      </div>
    </n-alert>

    <n-grid cols="2 700:5" :x-gap="12" :y-gap="12" class="mb-4">
      <n-gi v-for="item in statistics" :key="item.label">
        <n-statistic :label="$t(`custom.device_details.${item.label}`)" :value="item.value" />
      </n-gi>
    </n-grid>

    <n-alert :type="hasTwinDrift ? 'warning' : 'success'" class="mb-3" :show-icon="true">
      <n-space align="center" justify="space-between" :wrap="true">
        <span>
          <template v-if="hasTwinDrift">
            {{ twinState.summary.deltaCount }} {{ $t('custom.device_details.twinDelta') }} /
            {{ twinState.summary.desiredCount }} {{ $t('custom.device_details.twinDesired') }}
          </template>
          <template v-else>
            {{ $t('custom.device_details.twinMatched') }}
          </template>
        </span>
        <n-checkbox v-model:checked="onlyShowDelta" :disabled="!driftRows.length">
          {{ $t('custom.device_details.twinDelta') }}
        </n-checkbox>
      </n-space>
    </n-alert>

    <n-alert type="info" class="mb-3" :show-icon="false">
      {{ $t('custom.device_details.twinSourceHint') }}
    </n-alert>

    <TwinRepairChecklist
      :alert-type="repair.alertType"
      :headline="repair.headline"
      :summary="twinState.summary"
      :can-repair="Boolean(primaryRepairRow)"
      @download-evidence="downloadTwinEvidenceBundle"
      @copy-summary="copyTwinRepairSummary"
      @repair="openPrimaryRepair"
    />

    <n-alert v-if="!twinState.rows.length" type="info" class="mb-3" :show-icon="false">
      {{ $t('custom.device_details.twinEmptyState') }}
    </n-alert>

    <n-alert v-else type="default" class="mb-3" :show-icon="false">
      <n-space vertical :size="8">
        <n-space v-for="item in guidanceItems" :key="item.label" align="center" :size="8">
          <n-tag round size="small" :type="item.type">{{ item.label }}</n-tag>
          <span>{{ item.text }}</span>
        </n-space>
      </n-space>
    </n-alert>

    <n-data-table :loading="loading" :pagination="false" size="small" :data="previewRows" :columns="columns" />

    <TwinDesiredEditorModal
      v-model:show="desiredDialogVisible"
      :form="desiredForm"
      :editing-row="editingRow"
      :error="desiredError"
      :saving="savingDesired"
      :source-labels="sourceLabelMap"
      @cancel="closeDesiredDialog"
      @submit="submitDesired"
      @use-reported="useReportedValue"
    />
  </n-card>
</template>
