import { ref } from 'vue'
import { commandDataById, getDeviceConnectionDiagnostics, getDeviceConnectionGuide } from '@/service/api/device'
import { getReadyCheckViewEvidence } from './device-connection-diagnostics-state'
import type { DeviceConnectionGuideStateInput } from './device-access-guide-state'
import { buildRecommendedCommandDraft, type RecommendedCommandDraft } from './ready-check-command-draft'

export type ReadyCheckCollectionFailure = {
  key: 'diagnostics' | 'connectionGuide' | 'commands'
  labelKey: string
}

const allCollectorFailures = (): ReadyCheckCollectionFailure[] => [
  { key: 'diagnostics', labelKey: 'custom.device_details.readyCheckCollectionDiagnostics' },
  { key: 'connectionGuide', labelKey: 'custom.device_details.readyCheckCollectionGuide' },
  { key: 'commands', labelKey: 'custom.device_details.readyCheckCollectionCommands' }
]

export const useReadyCheckCollectors = () => {
  const diagnosticsLoading = ref(false)
  const diagnostics = ref<ReturnType<typeof getReadyCheckViewEvidence>>(getReadyCheckViewEvidence({}))
  const connectionGuide = ref<DeviceConnectionGuideStateInput | null>(null)
  const recommendedCommandLoading = ref(false)
  const recommendedCommandDraft = ref<RecommendedCommandDraft | null>(null)
  const collectionFailures = ref<ReadyCheckCollectionFailure[]>([])
  let collectorRequestSeq = 0
  let inFlightDeviceId = ''
  let inFlightRefresh: Promise<void> | null = null

  const resetCollectors = () => {
    diagnostics.value = getReadyCheckViewEvidence({})
    connectionGuide.value = null
    recommendedCommandDraft.value = null
    collectionFailures.value = []
  }

  const refreshDiagnostics = async (deviceId: string) => {
    if (deviceId && inFlightRefresh && inFlightDeviceId === deviceId) {
      return inFlightRefresh
    }

    const requestSeq = ++collectorRequestSeq
    if (!deviceId) {
      inFlightDeviceId = ''
      inFlightRefresh = null
      resetCollectors()
      diagnosticsLoading.value = false
      recommendedCommandLoading.value = false
      return
    }

    inFlightDeviceId = deviceId
    diagnosticsLoading.value = true
    recommendedCommandLoading.value = true

    const refreshPromise = (async () => {
      const commandsRequest = commandDataById(deviceId).then(
        value => ({ status: 'fulfilled' as const, value }),
        () => ({ status: 'rejected' as const })
      )
      const [diagnosticsResponse, guideResponse] = await Promise.allSettled([
        getDeviceConnectionDiagnostics(deviceId, { debug_log_limit: 3 }),
        getDeviceConnectionGuide(deviceId, { debug_log_limit: 3, command_log_limit: 3 })
      ])

      if (requestSeq !== collectorRequestSeq) return

      collectionFailures.value = [
        diagnosticsResponse.status === 'rejected'
          ? { key: 'diagnostics', labelKey: 'custom.device_details.readyCheckCollectionDiagnostics' }
          : null,
        guideResponse.status === 'rejected'
          ? { key: 'connectionGuide', labelKey: 'custom.device_details.readyCheckCollectionGuide' }
          : null
      ].filter(Boolean) as ReadyCheckCollectionFailure[]

      diagnostics.value =
        diagnosticsResponse.status === 'fulfilled'
          ? getReadyCheckViewEvidence(diagnosticsResponse.value)
          : getReadyCheckViewEvidence({})
      connectionGuide.value =
        guideResponse.status === 'fulfilled' ? guideResponse.value?.data || guideResponse.value || null : null

      diagnosticsLoading.value = false

      const commandsResponse = await commandsRequest

      if (requestSeq !== collectorRequestSeq) return

      recommendedCommandDraft.value =
        commandsResponse.status === 'fulfilled' ? buildRecommendedCommandDraft(commandsResponse.value?.data) : null
      if (commandsResponse.status === 'rejected') {
        collectionFailures.value = [
          ...collectionFailures.value.filter(failure => failure.key !== 'commands'),
          { key: 'commands', labelKey: 'custom.device_details.readyCheckCollectionCommands' }
        ]
      }
    })()

    inFlightRefresh = refreshPromise

    try {
      await refreshPromise
    } catch {
      if (requestSeq === collectorRequestSeq) {
        resetCollectors()
        collectionFailures.value = allCollectorFailures()
      }
    } finally {
      if (requestSeq === collectorRequestSeq) {
        diagnosticsLoading.value = false
        recommendedCommandLoading.value = false
      }
      if (inFlightRefresh === refreshPromise) {
        inFlightDeviceId = ''
        inFlightRefresh = null
      }
    }
  }

  return {
    diagnosticsLoading,
    diagnostics,
    connectionGuide,
    recommendedCommandLoading,
    recommendedCommandDraft,
    collectionFailures,
    refreshDiagnostics
  }
}
