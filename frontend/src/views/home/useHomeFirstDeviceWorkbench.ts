import { computed, ref, type Ref } from 'vue'
import { router } from '@/router'
import {
  deviceList,
  getDeviceConnectInfo,
  getDeviceConnectionGuide,
  getDeviceConnectionDiagnostics,
  getSimulationInit,
  sendSimulationData,
  telemetryDataCurrent
} from '@/service/api/device'
import {
  buildDeviceAccessGuideState,
  buildDeviceAccessGuideStateFromConnectionGuide,
  type DeviceAccessGuideDiagnosticsSummary,
  type DeviceAccessGuideState
} from '@/views/device/details/modules/device-access-guide-state'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'
import { HOME_FIRST_RUN_TENANT_REQUIRED_CODE } from './homeFirstRunWizard'
import {
  buildFirstDeviceOnboardingGuard,
  buildFirstDeviceBrowserTestState,
  buildFirstDeviceChartState,
  buildFirstDeviceReadyProof,
  buildFirstDeviceSupportSummary,
  buildHttpTelemetryRequest,
  buildPublishCommand,
  createIdleFirstDeviceBrowserTestState,
  isFirstDeviceReady,
  isUsableHttpEndpoint,
  normalizeFirstDevice,
  normalizeSimulationInit,
  normalizeTelemetryPoints,
  summarizeFirstDeviceConnectionDiagnostics,
  type FirstDeviceSummary,
  type FirstTelemetryPoint,
  type FirstDeviceBrowserTestState,
  type FirstDeviceQuickstartAction,
  type FirstDeviceDeploymentHealthRow,
  type FirstDeviceSupportTestCommand,
  type SimulationInitState
} from './homeFirstDeviceWorkbench'

type UseHomeFirstDeviceWorkbenchOptions = {
  deploymentHealthy: Readonly<Ref<boolean>>
  deploymentHealthRows?: Readonly<Ref<FirstDeviceDeploymentHealthRow[]>>
  onTenantRequired?: () => void
}

const unwrapConnectionGuideResponse = (response: any) => {
  if (response?.error) return null
  return response?.data?.data ?? response?.data ?? response ?? null
}

const hasUsableConnectionGuideAccess = (guide: any) => {
  if (!guide || typeof guide !== 'object') return false
  const access = guide.access
  if (!access || typeof access !== 'object') return false
  return Boolean(access.connection_profile || access.connection_info || access.credential_form)
}

const telemetryTimestampIsAfter = (timestamp: string | undefined, sentAt: string) => {
  if (!timestamp) return false
  const telemetryTime = Date.parse(timestamp)
  const sentTime = Date.parse(sentAt)
  return Number.isFinite(telemetryTime) && Number.isFinite(sentTime) && telemetryTime >= sentTime - 1000
}

const buildTelemetrySignature = (telemetry: FirstTelemetryPoint[]) =>
  telemetry.map((point) => `${point.key}:${point.value}:${point.ts || ''}`).join('|')

const BROWSER_TEST_CONFIRM_ATTEMPTS = 5
const BROWSER_TEST_CONFIRM_INTERVAL_MS = 1200

const waitForBrowserTestConfirmation = () =>
  new Promise((resolve) => {
    window.setTimeout(resolve, BROWSER_TEST_CONFIRM_INTERVAL_MS)
  })

const findConfirmedBrowserTelemetry = (
  telemetry: FirstTelemetryPoint[],
  options: {
    sentAt: string
    previousSignature: string
  }
) => {
  const latestTelemetry = telemetry[0]
  if (!latestTelemetry) return null
  if (telemetryTimestampIsAfter(latestTelemetry.ts, options.sentAt)) return latestTelemetry
  if (!latestTelemetry.ts && buildTelemetrySignature(telemetry) !== options.previousSignature) return latestTelemetry
  return null
}

export function useHomeFirstDeviceWorkbench(options: UseHomeFirstDeviceWorkbenchOptions) {
  const loading = ref(false)
  const device = ref<FirstDeviceSummary | null>(null)
  const telemetry = ref<FirstTelemetryPoint[]>([])
  const simulation = ref<SimulationInitState | null>(null)
  const accessGuide = ref<DeviceAccessGuideState | null>(null)
  const diagnostics = ref<DeviceAccessGuideDiagnosticsSummary>({})
  const actionLoading = ref(false)
  const testResult = ref('')
  const browserTest = ref<FirstDeviceBrowserTestState>(createIdleFirstDeviceBrowserTestState())
  let refreshPromise: Promise<void> | null = null

  const ready = computed(() => isFirstDeviceReady(device.value, telemetry.value))
  const firstChart = computed(() => buildFirstDeviceChartState(telemetry.value, browserTest.value))
  const publishCommand = computed(
    () => accessGuide.value?.commands?.[0]?.code || (simulation.value ? buildPublishCommand(simulation.value) : '')
  )
  const onboardingGuard = computed(() =>
    buildFirstDeviceOnboardingGuard({
      device: device.value,
      telemetry: telemetry.value,
      accessGuide: accessGuide.value,
      publishCommand: publishCommand.value,
      actionLoading: actionLoading.value,
      deploymentHealthy: options.deploymentHealthy.value
    })
  )
  const readyProof = computed(() =>
    buildFirstDeviceReadyProof({
      device: device.value,
      telemetry: telemetry.value,
      accessGuide: accessGuide.value,
      publishCommand: publishCommand.value,
      deploymentHealthy: options.deploymentHealthy.value,
      browserTest: browserTest.value,
      chart: firstChart.value
    })
  )
  const buildSupportSummary = (summaryOptions: {
    latestProofText: string
    activeTestCommand?: FirstDeviceSupportTestCommand | null
    delivery?: {
      firstDeviceUrl?: string
      proofUrl?: string
      proofFileHint?: string
    }
  }) =>
    buildFirstDeviceSupportSummary({
      device: device.value,
      accessGuide: accessGuide.value,
      diagnostics: diagnostics.value,
      simulation: simulation.value,
      readyProof: readyProof.value,
      latestProofText: summaryOptions.latestProofText,
      browserTest: browserTest.value,
      testResult: testResult.value,
      chart: firstChart.value,
      activeTestCommand: summaryOptions.activeTestCommand,
      onboardingGuard: onboardingGuard.value,
      deploymentHealthRows: options.deploymentHealthRows?.value || [],
      delivery: summaryOptions.delivery
    })

  const resetDeviceDetails = (resetBrowserTest = false) => {
    telemetry.value = []
    simulation.value = null
    accessGuide.value = null
    diagnostics.value = {}
    testResult.value = ''
    if (resetBrowserTest) {
      browserTest.value = createIdleFirstDeviceBrowserTestState()
    }
  }

  const loadConnectionGuide = async (currentDevice: FirstDeviceSummary) => {
    try {
      const response = await getDeviceConnectionGuide(currentDevice.id, { debug_log_limit: 3, command_log_limit: 3 })
      const guide = unwrapConnectionGuideResponse(response)
      if (!hasUsableConnectionGuideAccess(guide)) return false
      diagnostics.value = {}
      accessGuide.value = buildDeviceAccessGuideStateFromConnectionGuide(guide, currentDevice.number)
      return true
    } catch {
      return false
    }
  }

  const loadCompatConnectionInfo = async (currentDevice: FirstDeviceSummary) => {
    const [connectInfoResult, diagnosticsResult] = await Promise.all([
      getDeviceConnectInfo({ device_id: currentDevice.id }),
      getDeviceConnectionDiagnostics(currentDevice.id, { debug_log_limit: 3 }).catch(() => ({}))
    ])
    diagnostics.value = summarizeFirstDeviceConnectionDiagnostics(diagnosticsResult)
    accessGuide.value = buildDeviceAccessGuideState(
      connectInfoResult?.data || {},
      currentDevice.number,
      {},
      diagnostics.value
    )
  }

  const runRefresh = async () => {
    loading.value = true
    try {
      const previousDeviceId = device.value?.id || ''
      const listResult = await deviceList({ page: 1, page_size: 1 })
      const currentDevice = normalizeFirstDevice(listResult)
      device.value = currentDevice
      resetDeviceDetails(!currentDevice || Boolean(previousDeviceId && previousDeviceId !== currentDevice.id))

      if (!currentDevice) return

      const [telemetryResult, simulationResult, guideLoaded] = await Promise.all([
        telemetryDataCurrent(currentDevice.id, { silentError: true }),
        getSimulationInit({ device_id: currentDevice.id }),
        loadConnectionGuide(currentDevice)
      ])
      telemetry.value = normalizeTelemetryPoints(telemetryResult)
      simulation.value = normalizeSimulationInit(simulationResult)
      if (!guideLoaded) {
        await loadCompatConnectionInfo(currentDevice)
      }
    } catch {
      device.value = null
      resetDeviceDetails(true)
    } finally {
      loading.value = false
    }
  }

  const refresh = () => {
    if (refreshPromise) return refreshPromise

    refreshPromise = runRefresh().finally(() => {
      refreshPromise = null
    })

    return refreshPromise
  }

  const openReadyCheck = () => {
    if (!device.value) {
      router.push('/device/manage')
      return
    }
    router.push({
      path: '/device/details',
      query: {
        d_id: device.value.id,
        tab: 'ready-check',
        onboarding: 'first-device'
      }
    })
  }

  const openFullGuide = () => {
    if (!device.value) {
      router.push('/device/manage?onboarding=first-device&add=manual')
      return
    }
    router.push({
      path: '/device/details',
      query: {
        d_id: device.value.id,
        tab: 'join',
        onboarding: 'first-device'
      }
    })
  }

  const copyPublishCommand = async () => {
    if (!onboardingGuard.value.canCopyCommand) {
      window.$message?.warning(onboardingGuard.value.summary)
      return
    }
    const copied = await writeClipboardText(publishCommand.value)
    if (copied) {
      window.$message?.success($t('theme.configOperation.copySuccess'))
    } else {
      window.$message?.error($t('common.copyFailed'))
    }
  }

  const runHttpTelemetryTest = async (currentAccessGuide: DeviceAccessGuideState) => {
    if (!isUsableHttpEndpoint(currentAccessGuide.endpoint)) {
      window.$message?.warning($t('custom.home.browserTest.httpEndpointUnavailable'))
      return false
    }
    try {
      const request = buildHttpTelemetryRequest({
        endpoint: currentAccessGuide.endpoint,
        token: currentAccessGuide.username || currentAccessGuide.password,
        payload: currentAccessGuide.payload
      })
      const response = await fetch(request.url, request.init)
      if (!response.ok) {
        throw new Error($t('custom.home.browserTest.httpReportFailed', { status: response.status }))
      }
    } catch (error: any) {
      window.$message?.error(error?.message || $t('custom.home.browserTest.httpRequestFailed'))
      return false
    }
    return true
  }

  const confirmBrowserTelemetryTest = async (sentAt: string, sentMessage: string, previousSignature: string) => {
    browserTest.value = buildFirstDeviceBrowserTestState({
      status: 'sent',
      message: sentMessage,
      sentAt
    })
    testResult.value = sentMessage
    let latestTelemetry: FirstTelemetryPoint | null = null
    for (let attempt = 1; attempt <= BROWSER_TEST_CONFIRM_ATTEMPTS; attempt += 1) {
      if (attempt > 1) {
        await waitForBrowserTestConfirmation()
      }
      browserTest.value = buildFirstDeviceBrowserTestState({
        status: 'sent',
        message: $t('custom.home.browserTest.confirmProgress', { sentMessage, attempt, total: BROWSER_TEST_CONFIRM_ATTEMPTS }),
        sentAt
      })
      testResult.value = browserTest.value.message
      await refresh()
      latestTelemetry = findConfirmedBrowserTelemetry(telemetry.value, {
        sentAt,
        previousSignature
      })
      if (latestTelemetry) {
        browserTest.value = buildFirstDeviceBrowserTestState({
          status: 'confirmed',
          telemetry: latestTelemetry,
          sentAt
        })
        testResult.value = browserTest.value.message
        return
      }
    }
    browserTest.value = buildFirstDeviceBrowserTestState({
      status: 'sent',
      message: $t('custom.home.browserTest.confirmTimeout', { total: BROWSER_TEST_CONFIRM_ATTEMPTS }),
      sentAt
    })
    testResult.value = browserTest.value.message
  }

  const simulateTelemetry = async () => {
    if (!device.value) {
      router.push('/device/manage')
      return
    }
    if (!onboardingGuard.value.canRunBrowserTest) {
      window.$message?.warning(onboardingGuard.value.summary)
      return
    }

    actionLoading.value = true
    testResult.value = ''
    const sentAt = new Date().toISOString()
    const previousTelemetrySignature = buildTelemetrySignature(telemetry.value)
    browserTest.value = buildFirstDeviceBrowserTestState({ status: 'sending', sentAt })
    try {
      if (accessGuide.value?.endpointKind === 'http') {
        const sent = await runHttpTelemetryTest(accessGuide.value)
        if (!sent) {
          browserTest.value = buildFirstDeviceBrowserTestState({
            status: 'failed',
            message: $t('custom.home.browserTest.httpRequestFailed'),
            sentAt
          })
          testResult.value = browserTest.value.message
          return
        }
        const message = $t('custom.home.browserTest.httpRequestSent')
        window.$message?.success(message)
        await confirmBrowserTelemetryTest(sentAt, message, previousTelemetrySignature)
        return
      }

      if (!simulation.value) {
        browserTest.value = buildFirstDeviceBrowserTestState({
          status: 'failed',
          message: $t('custom.home.browserTest.simulationNotReady'),
          sentAt
        })
        testResult.value = browserTest.value.message
        router.push('/device/manage')
        return
      }

      const { error } = await sendSimulationData({
        device_id: device.value.id,
        data: simulation.value.payload,
        server: simulation.value.server,
        port: simulation.value.port,
        topic: simulation.value.topic
      })
      if (error) {
        browserTest.value = buildFirstDeviceBrowserTestState({
          status: 'failed',
          message: error?.message || $t('custom.home.browserTest.simulationFailed'),
          sentAt
        })
        testResult.value = browserTest.value.message
        window.$message?.error(testResult.value)
        return
      }
      const message = $t('custom.home.browserTest.mqttRequestSent')
      window.$message?.success(message)
      await confirmBrowserTelemetryTest(sentAt, message, previousTelemetrySignature)
    } catch (error: any) {
      if (error?.code === HOME_FIRST_RUN_TENANT_REQUIRED_CODE) {
        options.onTenantRequired?.()
      }
      browserTest.value = buildFirstDeviceBrowserTestState({
        status: 'failed',
        message: error?.message || $t('custom.home.browserTest.failed'),
        sentAt
      })
      testResult.value = browserTest.value.message
      window.$message?.error(testResult.value)
    } finally {
      actionLoading.value = false
    }
  }

  const runQuickstartAction = (
    action: FirstDeviceQuickstartAction | string,
    createDevice: () => void | Promise<void>
  ) => {
    if (action === 'copy') {
      void copyPublishCommand()
      return
    }
    if (action === 'test') {
      void simulateTelemetry()
      return
    }
    if (action === 'ready-check') {
      openReadyCheck()
      return
    }
    void createDevice()
  }

  return {
    loading,
    device,
    telemetry,
    simulation,
    accessGuide,
    diagnostics,
    actionLoading,
    testResult,
    browserTest,
    firstChart,
    ready,
    publishCommand,
    onboardingGuard,
    readyProof,
    refresh,
    openReadyCheck,
    openFullGuide,
    copyPublishCommand,
    simulateTelemetry,
    runQuickstartAction,
    buildSupportSummary
  }
}
