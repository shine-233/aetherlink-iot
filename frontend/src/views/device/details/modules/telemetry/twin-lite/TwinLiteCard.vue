<script setup lang="ts">
import { computed, h, reactive, ref, watch } from 'vue'
import { NButton } from 'naive-ui'
import { expectMessageList, getAttributeDataSet, getDeviceTwin, setDeviceTwinDesired } from '@/service/api'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'
import {
  buildTwinLiteEvidenceBundle,
  buildTwinLiteEvidenceFileName,
  buildTwinLiteState,
  getTwinLiteConvergenceStatus,
  normalizeTwinLiteConvergenceStatus,
  normalizeTwinLiteNextAction,
  type TwinLiteConvergenceStatus,
  type TwinLiteRow,
  type TwinLiteState
} from './twin-lite-normalizer'

const props = defineProps<{
  id: string
  reportedTelemetry: DeviceManagement.telemetryData[]
}>()

type EditableTwinSource = 'telemetry' | 'attribute'
type LastDesiredSave = {
  source: EditableTwinSource
  key: string
  savedAt: string
}

const loading = ref(false)
const savingDesired = ref(false)
const expectedMessages = ref<any[]>([])
const reportedAttributes = ref<any[]>([])
const loadError = ref('')
const desiredError = ref('')
const backendTwinState = ref<TwinLiteState | null>(null)
const desiredDialogVisible = ref(false)
const editingRow = ref<TwinLiteRow | null>(null)
const lastDesiredSave = ref<LastDesiredSave | null>(null)

const desiredForm = reactive<{
  source: EditableTwinSource
  key: string
  desiredText: string
}>({
  source: 'telemetry',
  key: '',
  desiredText: ''
})

const twinState = computed(
  () =>
    backendTwinState.value ||
    buildTwinLiteState(expectedMessages.value, props.reportedTelemetry || [], reportedAttributes.value)
)

const onlyShowDelta = ref(false)
const driftRows = computed(() => twinState.value.rows.filter((row) => row.comparable && !row.matched))
const previewRows = computed(() => (onlyShowDelta.value ? driftRows.value : twinState.value.rows))
const hasTwinDrift = computed(() => twinState.value.summary.deltaCount > 0)
const driftAlertType = computed(() => (hasTwinDrift.value ? 'warning' : 'success'))
const unavailableRows = computed(() => twinState.value.rows.filter((row) => row.comparable && row.reported === null))
const repairableDriftRows = computed(() =>
  driftRows.value.filter((row) => row.source !== 'command' && row.reported !== null && row.reported !== undefined)
)
const primaryRepairRow = computed(() => repairableDriftRows.value[0] || null)
const commandRows = computed(() => twinState.value.rows.filter((row) => row.source === 'command'))
const twinRepairAlertType = computed(() => {
  if (driftRows.value.length) return 'warning'
  if (unavailableRows.value.length) return 'info'
  return 'success'
})
const twinRepairHeadline = computed(() => {
  if (driftRows.value.length) return $t('custom.device_details.twinRepairNeedsAction')
  if (unavailableRows.value.length) return $t('custom.device_details.twinRepairWaitingReported')
  return $t('custom.device_details.twinRepairAllGood')
})
const twinRepairSummary = computed(() =>
  [
    `Device: ${props.id}`,
    `${$t('custom.device_details.twinMatched')}: ${twinState.value.summary.matchedCount}`,
    `${$t('custom.device_details.twinDelta')}: ${twinState.value.summary.deltaCount}`,
    `${$t('custom.device_details.twinUnavailable')}: ${twinState.value.summary.unavailableCount}`,
    '',
    'Delta rows:',
    ...(driftRows.value.length
      ? driftRows.value.map(
          (row, index) =>
            `${index + 1}. [${sourceLabelMap[row.source] || row.source}] ${row.label || row.key}: desired=${formatValue(
              row.desired
            )}; reported=${formatValue(row.reported)}`
        )
      : ['--'])
  ].join('\n')
)
const twinGuidanceItems = computed(() => {
  const items: Array<{ type: 'warning' | 'info' | 'success'; label: string; text: string }> = []

  if (driftRows.value.length) {
    items.push({
      type: 'warning',
      label: $t('custom.device_details.twinGuidanceDriftLabel'),
      text: $t('custom.device_details.twinGuidanceDriftText').replace('{count}', String(driftRows.value.length))
    })
  }

  if (unavailableRows.value.length) {
    items.push({
      type: 'info',
      label: $t('custom.device_details.twinGuidanceUnavailableLabel'),
      text: $t('custom.device_details.twinGuidanceUnavailableText').replace(
        '{count}',
        String(unavailableRows.value.length)
      )
    })
  }

  if (commandRows.value.length) {
    items.push({
      type: 'info',
      label: $t('custom.device_details.twinGuidanceCommandLabel'),
      text: $t('custom.device_details.twinGuidanceCommandText').replace('{count}', String(commandRows.value.length))
    })
  }

  if (!items.length && twinState.value.rows.length) {
    items.push({
      type: 'success',
      label: $t('custom.device_details.twinGuidanceMatchedLabel'),
      text: $t('custom.device_details.twinGuidanceMatchedText')
    })
  }

  return items
})

const twinConfirmationStatus = computed<TwinLiteConvergenceStatus>(() => {
  const backendStatus = twinState.value.summary.convergenceStatus
  const derivedStatus = getTwinLiteConvergenceStatus(
    twinState.value.summary.desiredCount,
    twinState.value.summary.deltaCount,
    twinState.value.summary.unavailableCount
  )
  return normalizeTwinLiteConvergenceStatus(backendStatus, derivedStatus)
})

const twinConfirmationType = computed(() => {
  if (twinConfirmationStatus.value === 'ready') return 'success'
  if (twinConfirmationStatus.value === 'waiting_reported' || twinConfirmationStatus.value === 'no_desired')
    return 'info'
  return 'warning'
})

const twinConfirmationTitle = computed(() =>
  $t(`custom.device_details.twinConfirmationStatus.${twinConfirmationStatus.value}`)
)
const twinConfirmationActionKey = computed(() =>
  normalizeTwinLiteNextAction(twinState.value.summary.nextAction, twinConfirmationStatus.value)
)
const twinConfirmationAction = computed(() =>
  $t(`custom.device_details.twinConfirmationAction.${twinConfirmationActionKey.value}`)
)
const twinConfirmationBoundary = computed(() =>
  $t(
    twinState.value.summary.evidenceBoundary === 'platform_visible_evidence_only'
      ? 'custom.device_details.twinConfirmationBoundaryPlatform'
      : 'custom.device_details.twinConfirmationBoundaryDefault'
  )
)

const sourceLabelMap: Record<string, string> = {
  telemetry: $t('custom.device_details.telemetry'),
  attribute: $t('custom.device_details.attributes'),
  command: $t('custom.device_details.commandDelivery')
}

const desiredSourceOptions = computed(() => [
  { label: sourceLabelMap.telemetry, value: 'telemetry' },
  { label: sourceLabelMap.attribute, value: 'attribute' }
])

const desiredDialogTitle = computed(() =>
  editingRow.value ? $t('custom.device_details.twinEditDesired') : $t('custom.device_details.twinSetDesired')
)

const lastDesiredSaveRow = computed(() => {
  const saved = lastDesiredSave.value
  if (!saved) return null

  return (
    twinState.value.rows.find(
      (row) => row.source === saved.source && (row.key === saved.key || row.label === saved.key)
    ) || null
  )
})

const desiredObservationType = computed(() => {
  const row = lastDesiredSaveRow.value
  if (!row) return 'info'
  if (!row.comparable || row.reported === null || row.reported === undefined) return 'info'
  return row.matched ? 'success' : 'warning'
})

const desiredObservationTitle = computed(() => {
  const row = lastDesiredSaveRow.value
  if (!row) return $t('custom.device_details.twinDesiredObservationTitle')
  return row.matched
    ? $t('custom.device_details.twinDesiredObservationMatched')
    : $t('custom.device_details.twinDesiredObservationWaiting')
})

const desiredObservationMeta = computed(() => {
  const saved = lastDesiredSave.value
  if (!saved) return ''

  return `${sourceLabelMap[saved.source]} / ${saved.key} / ${new Date(saved.savedAt).toLocaleString()}`
})

function formatTwinTimestamp(value?: string) {
  if (!value) return ''
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}

function twinMetadataLines(row: TwinLiteRow) {
  const lines: string[] = []
  const desiredUpdatedAt = formatTwinTimestamp(row.desired_updated_at)
  const desiredExpiresAt = formatTwinTimestamp(row.desired_expires_at)
  const reportedAt = formatTwinTimestamp(row.reported_at)
  if (desiredUpdatedAt) lines.push(`${$t('custom.device_details.twinDesiredUpdatedAt')}: ${desiredUpdatedAt}`)
  if (desiredExpiresAt) lines.push(`${$t('custom.device_details.twinDesiredExpiresAt')}: ${desiredExpiresAt}`)
  if (reportedAt) lines.push(`${$t('custom.device_details.twinReportedAt')}: ${reportedAt}`)
  if (row.desired_revision) {
    lines.push(`${$t('custom.device_details.twinDesiredRevision')}: ${row.desired_revision}`)
  }
  if (row.last_write_source) {
    lines.push(
      `${$t('custom.device_details.twinLastWriteSource')}: ${$t(
        `custom.device_details.twinLastWriteSource.${row.last_write_source}`
      )}`
    )
  }
  return lines
}

const columns = computed(() => [
  {
    title: $t('page.expect.label'),
    key: 'label'
  },
  {
    title: $t('page.expect.commandType'),
    key: 'source',
    render: (row: TwinLiteRow) => sourceLabelMap[row.source] || row.source
  },
  {
    title: $t('custom.device_details.twinDesired'),
    key: 'desired',
    render: (row: TwinLiteRow) => formatValue(row.desired)
  },
  {
    title: $t('custom.device_details.twinReported'),
    key: 'reported',
    render: (row: TwinLiteRow) => formatValue(row.reported)
  },
  {
    title: $t('custom.device_details.twinStateMetadata'),
    key: 'state_metadata',
    render: (row: TwinLiteRow) => {
      const lines = twinMetadataLines(row)
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
        {
          text: true,
          type: 'primary',
          size: 'small',
          onClick: () => openEditDesired(row)
        },
        { default: () => $t('generate.edit') }
      )
    }
  }
])

function formatValue(value: unknown) {
  if (value === null || value === undefined || value === '') return '--'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function serializeValue(value: unknown) {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function resetDesiredEditor() {
  desiredError.value = ''
  editingRow.value = null
  desiredForm.source = 'telemetry'
  desiredForm.key = ''
  desiredForm.desiredText = ''
}

function openCreateDesired() {
  resetDesiredEditor()
  desiredDialogVisible.value = true
}

function openEditDesired(row: TwinLiteRow) {
  if (row.source === 'command') return

  desiredError.value = ''
  editingRow.value = row
  desiredForm.source = row.source === 'attribute' ? 'attribute' : 'telemetry'
  desiredForm.key = row.key || row.label
  desiredForm.desiredText = serializeValue(row.desired)
  desiredDialogVisible.value = true
}

function closeDesiredDialog() {
  desiredDialogVisible.value = false
  resetDesiredEditor()
}

function useReportedValue() {
  if (!editingRow.value) return
  desiredForm.desiredText = serializeValue(editingRow.value.reported)
}

function openPrimaryRepair() {
  const row = primaryRepairRow.value
  if (!row) return

  openEditDesired(row)
  useReportedValue()
}

async function copyTwinRepairSummary() {
  const copied = await writeClipboardText(twinRepairSummary.value)
  if (copied) {
    ;(window as any).$message?.success($t('theme.configOperation.copySuccess'))
  } else {
    ;(window as any).$message?.error($t('common.copyFailed'))
  }
}

function downloadTwinEvidenceBundle() {
  try {
    const exportedAt = new Date().toISOString()
    const bundle = buildTwinLiteEvidenceBundle({
      deviceId: props.id,
      exportedAt,
      state: twinState.value,
      status: twinConfirmationStatus.value,
      nextAction: twinConfirmationAction.value,
      evidenceBoundary: twinConfirmationBoundary.value
    })
    const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: 'application/json' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = buildTwinLiteEvidenceFileName(props.id, exportedAt)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
    ;(window as any).$message?.success($t('custom.device_details.twinEvidenceBundleDownloaded'))
  } catch {
    ;(window as any).$message?.warning($t('custom.device_details.twinEvidenceBundleDownloadFailed'))
  }
}

function parseDesiredInput(value: string): unknown {
  const trimmed = value.trim()
  if (!trimmed) throw new Error($t('custom.device_details.twinDesiredEmpty'))

  try {
    return JSON.parse(trimmed)
  } catch {
    return trimmed
  }
}

async function submitDesired() {
  if (!props.id || savingDesired.value) return

  if (!desiredForm.key.trim()) {
    desiredError.value = $t('custom.device_details.twinDesiredKeyRequired')
    return
  }

  savingDesired.value = true
  desiredError.value = ''

  try {
    const desired = parseDesiredInput(desiredForm.desiredText)
    const savedSource = desiredForm.source
    const savedKey = desiredForm.key.trim()
    const response = await setDeviceTwinDesired(props.id, {
      source: savedSource,
      key: savedKey,
      desired
    })

    if (response?.error) {
      desiredError.value = response.error.message || $t('custom.device_details.twinDesiredSaveFailed')
      return
    }

    lastDesiredSave.value = {
      source: savedSource,
      key: savedKey,
      savedAt: new Date().toISOString()
    }
    ;(window as any).$message?.success(
      `${$t('custom.device_details.twinDesiredSaved')} ${$t('custom.device_details.twinDesiredSavedNextStep')}`
    )
    closeDesiredDialog()
    await loadTwinLite()
  } catch (error: any) {
    desiredError.value = error?.message || $t('custom.device_details.twinDesiredSaveFailed')
    ;(window as any).$message?.error(desiredError.value)
  } finally {
    savingDesired.value = false
  }
}

async function loadTwinLite() {
  if (!props.id) return

  loading.value = true
  loadError.value = ''
  backendTwinState.value = null
  expectedMessages.value = []
  reportedAttributes.value = []
  try {
    const twinResponse = await getDeviceTwin(props.id)
    if (isTwinStatePayload(twinResponse?.data)) {
      backendTwinState.value = twinResponse.data
      return
    }

    await loadTwinLiteCompat()
  } catch {
    await loadTwinLiteCompat()
  } finally {
    loading.value = false
  }
}

async function loadTwinLiteCompat() {
  const [expectedResponse, attributeResponse] = await Promise.all([
    expectMessageList({
      device_id: props.id,
      status: 'pending',
      page: 1,
      page_size: 100
    }),
    getAttributeDataSet({ device_id: props.id })
  ])

  if (expectedResponse?.error) {
    loadError.value = expectedResponse.error.message || 'expected-message-load-failed'
  } else {
    expectedMessages.value = Array.isArray(expectedResponse?.data?.list) ? expectedResponse.data.list : []
  }

  if (attributeResponse?.error) {
    loadError.value = loadError.value || attributeResponse.error.message || 'attribute-load-failed'
  } else {
    reportedAttributes.value = Array.isArray(attributeResponse?.data) ? attributeResponse.data : []
  }
}

function isTwinStatePayload(payload: unknown): payload is TwinLiteState {
  if (!payload || typeof payload !== 'object') return false
  const candidate = payload as TwinLiteState
  return Array.isArray(candidate.rows) && typeof candidate.summary === 'object' && candidate.summary !== null
}

watch(
  () => props.id,
  () => {
    expectedMessages.value = []
    reportedAttributes.value = []
    lastDesiredSave.value = null
    closeDesiredDialog()
    loadTwinLite()
  },
  { immediate: true }
)
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

    <n-alert :type="twinConfirmationType" class="mb-3" :show-icon="false" data-testid="device-twin-confirmation">
      <n-space vertical :size="8">
        <div class="flex flex-col gap-8px md:flex-row md:items-center md:justify-between">
          <div class="min-w-0">
            <div class="font-600">{{ twinConfirmationTitle }}</div>
            <div class="mt-4px text-12px line-height-18px text-gray-500">
              {{ twinConfirmationAction }}
            </div>
          </div>
          <n-tag round size="small" :type="twinConfirmationType">
            {{ $t('custom.device_details.twinConfirmationReadyGate') }}
          </n-tag>
        </div>
        <div class="text-12px line-height-18px text-gray-500">
          {{ twinConfirmationBoundary }}
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
      v-if="lastDesiredSave"
      :type="desiredObservationType"
      class="mb-3"
      :show-icon="false"
      data-testid="device-twin-desired-observation"
    >
      <n-space vertical :size="8">
        <div class="flex flex-col gap-8px md:flex-row md:items-center md:justify-between">
          <div class="min-w-0">
            <div class="font-600">{{ desiredObservationTitle }}</div>
            <div class="mt-4px text-12px line-height-18px text-gray-500">
              {{ $t('custom.device_details.twinDesiredSavedNextStep') }}
            </div>
            <div class="mt-4px text-12px line-height-18px text-gray-500">
              {{ desiredObservationMeta }}
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
      </n-space>
    </n-alert>

    <n-grid cols="2 700:5" :x-gap="12" :y-gap="12" class="mb-4">
      <n-gi>
        <n-statistic :label="$t('custom.device_details.twinDesired')" :value="twinState.summary.desiredCount" />
      </n-gi>
      <n-gi>
        <n-statistic :label="$t('custom.device_details.twinReported')" :value="twinState.summary.reportedCount" />
      </n-gi>
      <n-gi>
        <n-statistic :label="$t('custom.device_details.twinMatched')" :value="twinState.summary.matchedCount" />
      </n-gi>
      <n-gi>
        <n-statistic :label="$t('custom.device_details.twinDelta')" :value="twinState.summary.deltaCount" />
      </n-gi>
      <n-gi>
        <n-statistic :label="$t('custom.device_details.twinUnavailable')" :value="twinState.summary.unavailableCount" />
      </n-gi>
    </n-grid>

    <n-alert :type="driftAlertType" class="mb-3" :show-icon="true">
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

    <n-alert :type="twinRepairAlertType" class="mb-3" :show-icon="false">
      <n-space vertical :size="12">
        <div class="flex flex-col gap-10px md:flex-row md:items-center md:justify-between">
          <div class="min-w-0">
            <div class="font-600">{{ $t('custom.device_details.twinRepairChecklistTitle') }}</div>
            <div class="mt-4px text-12px line-height-18px text-gray-500">
              {{ twinRepairHeadline }}
            </div>
          </div>
          <div class="flex flex-wrap gap-8px">
            <n-button
              size="small"
              secondary
              data-testid="device-twin-download-evidence-bundle"
              @click="downloadTwinEvidenceBundle"
            >
              {{ $t('custom.device_details.twinEvidenceBundleDownload') }}
            </n-button>
            <n-button size="small" secondary @click="copyTwinRepairSummary">
              {{ $t('custom.device_details.twinRepairCopySummary') }}
            </n-button>
            <n-button size="small" type="primary" secondary :disabled="!primaryRepairRow" @click="openPrimaryRepair">
              {{ $t('custom.device_details.twinRepairUseReportedAction') }}
            </n-button>
          </div>
        </div>
        <n-grid cols="1 640:3" :x-gap="12" :y-gap="8">
          <n-gi>
            <div class="twin-repair-card">
              <span>{{ $t('custom.device_details.twinRepairCardAligned') }}</span>
              <strong>{{ twinState.summary.matchedCount }}</strong>
            </div>
          </n-gi>
          <n-gi>
            <div class="twin-repair-card twin-repair-card--warning">
              <span>{{ $t('custom.device_details.twinRepairCardDelta') }}</span>
              <strong>{{ twinState.summary.deltaCount }}</strong>
            </div>
          </n-gi>
          <n-gi>
            <div class="twin-repair-card twin-repair-card--info">
              <span>{{ $t('custom.device_details.twinRepairCardMissingReported') }}</span>
              <strong>{{ twinState.summary.unavailableCount }}</strong>
            </div>
          </n-gi>
        </n-grid>
      </n-space>
    </n-alert>

    <n-alert v-if="!twinState.rows.length" type="info" class="mb-3" :show-icon="false">
      {{ $t('custom.device_details.twinEmptyState') }}
    </n-alert>

    <n-alert v-else type="default" class="mb-3" :show-icon="false">
      <n-space vertical :size="8">
        <n-space v-for="item in twinGuidanceItems" :key="item.label" align="center" :size="8">
          <n-tag round size="small" :type="item.type">{{ item.label }}</n-tag>
          <span>{{ item.text }}</span>
        </n-space>
      </n-space>
    </n-alert>

    <n-data-table :loading="loading" :pagination="false" size="small" :data="previewRows" :columns="columns" />

    <n-modal
      v-model:show="desiredDialogVisible"
      preset="card"
      class="w-640px max-w-[calc(100vw-24px)]"
      :title="desiredDialogTitle"
    >
      <n-space vertical size="large">
        <n-alert v-if="desiredError" type="warning" :show-icon="false">
          {{ desiredError }}
        </n-alert>

        <n-form label-placement="top" :show-feedback="false">
          <n-form-item :label="$t('custom.device_details.twinDesiredSource')">
            <n-select v-model:value="desiredForm.source" :options="desiredSourceOptions" />
          </n-form-item>
          <n-form-item :label="$t('custom.device_details.twinDesiredKey')">
            <n-input v-model:value="desiredForm.key" />
          </n-form-item>
          <n-form-item :label="$t('custom.device_details.twinDesiredValue')">
            <n-input
              v-model:value="desiredForm.desiredText"
              type="textarea"
              :autosize="{ minRows: 6, maxRows: 12 }"
              :placeholder="$t('custom.device_details.twinDesiredValueHint')"
            />
          </n-form-item>
        </n-form>

        <div class="flex items-center justify-between gap-12px">
          <n-button quaternary :disabled="!editingRow || editingRow.reported === null" @click="useReportedValue">
            {{ $t('custom.device_details.twinUseReported') }}
          </n-button>
          <div class="flex items-center gap-12px">
            <n-button @click="closeDesiredDialog">
              {{ $t('generate.cancel') }}
            </n-button>
            <n-button type="primary" :loading="savingDesired" @click="submitDesired">
              {{ $t('custom.device_details.twinSaveDesired') }}
            </n-button>
          </div>
        </div>
      </n-space>
    </n-modal>
  </n-card>
</template>

<style scoped>
.twin-repair-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid #bbf7d0;
  border-radius: 8px;
  background: #f0fdf4;
}

.twin-repair-card--warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.twin-repair-card--info {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.twin-repair-card span {
  color: #64748b;
  font-size: 12px;
}

.twin-repair-card strong {
  color: #0f172a;
  font-size: 18px;
}
</style>
