/*
 * Telemetry loading for the selected device.
 *
 * Every preview switch bumps a sequence, so a slow reply for a previously selected device can
 * never overwrite the telemetry of the device currently in view.
 */
import { ref } from 'vue'
import { deviceMapTelemetry } from '@/service/api/device'
import type { MapTelemetry } from './equipment-map-model'

export function useEquipmentTelemetry() {
  const telemetryLoading = ref(false)
  const telemetryRequested = ref(false)
  const selectedTelemetry = ref<MapTelemetry | null>(null)

  let telemetryRequestSeq = 0

  function reset() {
    telemetryRequestSeq++
    selectedTelemetry.value = null
    telemetryRequested.value = false
    telemetryLoading.value = false
  }

  async function load(deviceId: string) {
    if (!deviceId) return
    const requestSeq = ++telemetryRequestSeq
    telemetryRequested.value = true
    telemetryLoading.value = true
    try {
      const { data, error } = await deviceMapTelemetry(deviceId)
      if (requestSeq !== telemetryRequestSeq) return
      selectedTelemetry.value = error ? null : data
    } finally {
      if (requestSeq === telemetryRequestSeq) telemetryLoading.value = false
    }
  }

  return { telemetryLoading, telemetryRequested, selectedTelemetry, load, reset }
}
