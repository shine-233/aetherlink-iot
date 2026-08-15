import { ref } from 'vue'

export const TELEMETRY_DETAIL_MODE = {
  history: 'history',
  sequential: 'sequential'
} as const

export type TelemetryDetailMode = (typeof TELEMETRY_DETAIL_MODE)[keyof typeof TELEMETRY_DETAIL_MODE]

type TelemetryDetailRecord = {
  key?: string
  label?: string
  device_id?: string
  unit?: string
  value?: unknown
}

type ValueRef<T> = { value: T }

export const applyTelemetryDetailContext = (
  options: {
    showHistory: ValueRef<boolean>
    telemetryId: ValueRef<string | undefined>
    telemetryKey: ValueRef<string | undefined>
    telemetryName: ValueRef<string | undefined>
    telemetryUnit: ValueRef<string | undefined>
    modelType: ValueRef<TelemetryDetailMode | ''>
  },
  mode: TelemetryDetailMode,
  telemetry: TelemetryDetailRecord
) => {
  options.modelType.value = mode
  options.telemetryKey.value = telemetry.key
  options.telemetryName.value = telemetry.label
  options.telemetryId.value = telemetry.device_id
  options.telemetryUnit.value = telemetry.unit
  options.showHistory.value = true
}

export const isNumericTelemetry = (telemetry: TelemetryDetailRecord) => typeof telemetry.value === 'number'

export const telemetryAccentColor = (telemetry: TelemetryDetailRecord) => {
  if (!isNumericTelemetry(telemetry)) {
    return '#cccccc'
  }
  return ''
}

export const useTelemetryDetailState = () => {
  const showHistory = ref(false)
  const telemetryId = ref<string>()
  const telemetryKey = ref<string>()
  const telemetryName = ref<string>()
  const telemetryUnit = ref<string>()
  const modelType = ref<TelemetryDetailMode | ''>('')

  const openTelemetryDetail = (mode: TelemetryDetailMode, telemetry: TelemetryDetailRecord) => {
    applyTelemetryDetailContext(
      {
        modelType,
        showHistory,
        telemetryId,
        telemetryKey,
        telemetryName,
        telemetryUnit
      },
      mode,
      telemetry
    )
  }

  const openTelemetryHistory = (telemetry: TelemetryDetailRecord) => {
    openTelemetryDetail(TELEMETRY_DETAIL_MODE.history, telemetry)
  }

  const openTelemetrySequence = (telemetry: TelemetryDetailRecord) => {
    if (!isNumericTelemetry(telemetry)) return
    openTelemetryDetail(TELEMETRY_DETAIL_MODE.sequential, telemetry)
  }

  const onTapTableTools = (telemetry: TelemetryDetailRecord) => {
    openTelemetrySequence(telemetry)
  }

  return {
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
  }
}
