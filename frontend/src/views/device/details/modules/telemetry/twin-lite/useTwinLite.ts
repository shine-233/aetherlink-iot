/**
 * 文件用途: Twin Lite（轻量设备孪生）卡片的状态、加载与期望值编辑 composable。
 * 核心逻辑:
 * 1. 优先读取后端 /device/twin 聚合；不可用时回退到“待下发期望消息 + 属性集 + 上报遥测”前端拼装。
 * 2. 期望值编辑弹窗状态（新建 / 编辑 / 使用上报值）与提交；提交成功后记录 lastDesiredSave 并刷新。
 * 3. 设备 ID 变化时清空旧状态、关闭弹窗并立即重新加载。
 * 关键注意事项: setDeviceTwinDesired 的 { source, key, desired } 结构是后端契约；desired 文本按 JSON 优先解析。
 */
import { computed, reactive, ref, watch } from 'vue'
import { expectMessageList, getAttributeDataSet, getDeviceTwin, setDeviceTwinDesired } from '@/service/api'
import { $t } from '@/locales'
import { copyTextWithFeedback, downloadJsonWithFeedback } from '../../../shared/detail-feedback'
import {
  buildTwinLiteEvidenceBundle,
  buildTwinLiteEvidenceFileName,
  buildTwinLiteState,
  normalizeTwinLiteNextAction,
  type TwinLiteRow,
  type TwinLiteState
} from './twin-lite-normalizer'
import {
  buildTwinGuidanceItems,
  buildTwinRepairSummary,
  desiredObservationType as resolveDesiredObservationType,
  groupTwinRows,
  isTwinStatePayload,
  parseTwinDesiredInput,
  resolveTwinConvergenceStatus,
  serializeTwinValue,
  twinConfirmationBoundaryKey,
  twinConvergenceAlertType,
  twinRepairAlertType as resolveRepairAlertType,
  twinRepairHeadlineKey
} from './twin-lite-view-model'

export type EditableTwinSource = 'telemetry' | 'attribute'

type LastDesiredSave = {
  source: EditableTwinSource
  key: string
  savedAt: string
}

const DETAILS = 'custom.device_details'

export function useTwinLite(props: { id: string; reportedTelemetry: DeviceManagement.telemetryData[] }) {
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
  const onlyShowDelta = ref(false)

  const desiredForm = reactive<{ source: EditableTwinSource; key: string; desiredText: string }>({
    source: 'telemetry',
    key: '',
    desiredText: ''
  })

  const sourceLabelMap: Record<string, string> = {
    telemetry: $t(`${DETAILS}.telemetry`),
    attribute: $t(`${DETAILS}.attributes`),
    command: $t(`${DETAILS}.commandDelivery`)
  }

  const twinState = computed(
    () =>
      backendTwinState.value ||
      buildTwinLiteState(expectedMessages.value, props.reportedTelemetry || [], reportedAttributes.value)
  )
  const groups = computed(() => groupTwinRows(twinState.value.rows))
  const driftRows = computed(() => groups.value.drift)
  const previewRows = computed(() => (onlyShowDelta.value ? driftRows.value : twinState.value.rows))
  const primaryRepairRow = computed(() => groups.value.repairable[0] || null)

  const repair = computed(() => ({
    alertType: resolveRepairAlertType(groups.value),
    headline: $t(twinRepairHeadlineKey(groups.value)),
    summary: buildTwinRepairSummary(props.id, twinState.value, driftRows.value, sourceLabelMap, $t)
  }))
  const guidanceItems = computed(() => buildTwinGuidanceItems(groups.value, twinState.value.rows.length, $t))

  const confirmation = computed(() => {
    const status = resolveTwinConvergenceStatus(twinState.value)
    const actionKey = normalizeTwinLiteNextAction(twinState.value.summary.nextAction, status)
    return {
      status,
      type: twinConvergenceAlertType(status),
      title: $t(`${DETAILS}.twinConfirmationStatus.${status}`),
      action: $t(`${DETAILS}.twinConfirmationAction.${actionKey}`),
      boundary: $t(twinConfirmationBoundaryKey(twinState.value))
    }
  })

  const desiredObservation = computed(() => {
    const saved = lastDesiredSave.value
    if (!saved) return null
    const row =
      twinState.value.rows.find(
        (item) => item.source === saved.source && (item.key === saved.key || item.label === saved.key)
      ) || null
    return {
      type: resolveDesiredObservationType(row),
      title: !row
        ? $t(`${DETAILS}.twinDesiredObservationTitle`)
        : row.matched
          ? $t(`${DETAILS}.twinDesiredObservationMatched`)
          : $t(`${DETAILS}.twinDesiredObservationWaiting`),
      meta: `${sourceLabelMap[saved.source]} / ${saved.key} / ${new Date(saved.savedAt).toLocaleString()}`
    }
  })

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
    desiredForm.desiredText = serializeTwinValue(row.desired)
    desiredDialogVisible.value = true
  }

  function closeDesiredDialog() {
    desiredDialogVisible.value = false
    resetDesiredEditor()
  }

  function useReportedValue() {
    if (!editingRow.value) return
    desiredForm.desiredText = serializeTwinValue(editingRow.value.reported)
  }

  function openPrimaryRepair() {
    const row = primaryRepairRow.value
    if (!row) return
    openEditDesired(row)
    useReportedValue()
  }

  const copyTwinRepairSummary = () => copyTextWithFeedback(repair.value.summary)

  function downloadTwinEvidenceBundle() {
    const exportedAt = new Date().toISOString()
    downloadJsonWithFeedback(
      () =>
        buildTwinLiteEvidenceBundle({
          deviceId: props.id,
          exportedAt,
          state: twinState.value,
          status: confirmation.value.status,
          nextAction: confirmation.value.action,
          evidenceBoundary: confirmation.value.boundary
        }),
      {
        fileName: buildTwinLiteEvidenceFileName(props.id, exportedAt),
        successKey: `${DETAILS}.twinEvidenceBundleDownloaded`,
        failureKey: `${DETAILS}.twinEvidenceBundleDownloadFailed`,
        failureLevel: 'warning'
      }
    )
  }

  async function loadTwinLiteCompat() {
    const [expectedResponse, attributeResponse] = await Promise.all([
      expectMessageList({ device_id: props.id, status: 'pending', page: 1, page_size: 100 }),
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

  async function submitDesired() {
    if (!props.id || savingDesired.value) return
    if (!desiredForm.key.trim()) {
      desiredError.value = $t(`${DETAILS}.twinDesiredKeyRequired`)
      return
    }

    savingDesired.value = true
    desiredError.value = ''
    try {
      const desired = parseTwinDesiredInput(desiredForm.desiredText, $t(`${DETAILS}.twinDesiredEmpty`))
      const savedSource = desiredForm.source
      const savedKey = desiredForm.key.trim()
      const response = await setDeviceTwinDesired(props.id, { source: savedSource, key: savedKey, desired })

      if (response?.error) {
        desiredError.value = response.error.message || $t(`${DETAILS}.twinDesiredSaveFailed`)
        return
      }

      lastDesiredSave.value = { source: savedSource, key: savedKey, savedAt: new Date().toISOString() }
      ;(window as any).$message?.success(
        `${$t(`${DETAILS}.twinDesiredSaved`)} ${$t(`${DETAILS}.twinDesiredSavedNextStep`)}`
      )
      closeDesiredDialog()
      await loadTwinLite()
    } catch (error: any) {
      desiredError.value = error?.message || $t(`${DETAILS}.twinDesiredSaveFailed`)
      ;(window as any).$message?.error(desiredError.value)
    } finally {
      savingDesired.value = false
    }
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

  return {
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
  }
}
