<script setup lang="ts">
import { computed, defineAsyncComponent, nextTick, ref, watch, type ComponentPublicInstance } from 'vue'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'
import {
  buildFirstDeviceChartProofSummary,
  buildFirstDeviceClosureSummary,
  buildFirstDeviceFlowNodes,
  buildFirstDeviceLatestProofText,
  buildFirstDeviceOnlineTesterState,
  buildFirstDevicePostReadyHandoff,
  buildFirstDevicePostTestGuidance,
  buildFirstDeviceSuccessFacts,
  buildFirstDeviceVerificationAction,
  resolveFirstDeviceFocusedSectionKey
} from './homeFirstDeviceWorkbench'
import {
  buildFirstDeviceCoreGuideSummary,
  buildFirstDeviceMissionControl,
  buildFirstDeviceOperationChecklist,
  buildFirstDeviceOperatorCue,
  buildFirstDeviceTestCommands,
  buildFirstDeviceStatusHeroCopy,
  buildFirstDeviceSuccessProofCopy,
  buildFirstRunWizardSteps,
  buildFocusedQuickstartCopy,
  filterFirstDeviceCoreGuideSteps,
  filterFirstDeviceNextGuideSteps,
  getFirstDeviceTestCommandLabel,
  getFocusedQuickstartActionLoading
} from './homeFirstDeviceView'
import type { HomeFirstRunProtocol } from './homeFirstRunWizard'
import {
  buildFirstDeviceProofDelivery,
  buildFirstDeviceProofFilename,
  buildFirstDeviceSuccessProofDeliveryPacket,
  downloadFirstDeviceSuccessProofPacket,
  type FirstDeviceProofDeliveryState
} from './homeFirstDeviceProofDelivery'
import { useViewportDeferredMount } from './useViewportDeferredMount'
import type {
  FirstDeviceBrowserTestState,
  FirstDeviceChartState,
  FirstDeviceDeploymentHealthRow,
  FirstDeviceOnboardingGuard,
  FirstDeviceReadyProof,
  FirstDeviceSummary,
  SimulationInitState
} from './homeFirstDeviceWorkbench'
import type { DeviceAccessGuideState } from '@/views/device/details/modules/device-access-guide-state'
import type { HomeCustomerGuideProgressStep, HomeCustomerGuideSummary } from './homeCustomerGuide'
import type { HomeFirstRunQuickCreateResult } from './homeFirstRunWizard'
import type { NormalizedDeploymentHealthRow } from './homeDeploymentHealth'

const HomeFirstDeviceGuideProgress = defineAsyncComponent(() => import('./HomeFirstDeviceGuideProgress.vue'))
const HomeFirstDeviceClosedLoopStrip = defineAsyncComponent(() => import('./HomeFirstDeviceClosedLoopStrip.vue'))
const HomeFirstDeviceCurrentWorkspaceSection = defineAsyncComponent(
  () => import('./HomeFirstDeviceCurrentWorkspaceSection.vue')
)
const HomeFirstDeviceDeploymentHealthSection = defineAsyncComponent(
  () => import('./HomeFirstDeviceDeploymentHealthSection.vue')
)
const HomeFirstDeviceIdentitySection = defineAsyncComponent(() => import('./HomeFirstDeviceIdentitySection.vue'))
const HomeFirstDeviceVerificationOverview = defineAsyncComponent(
  () => import('./HomeFirstDeviceVerificationOverview.vue')
)
const HomeFirstDeviceDeferredSections = defineAsyncComponent(() => import('./HomeFirstDeviceDeferredSections.vue'))

interface Props {
  homeCustomerGuideSummary: HomeCustomerGuideSummary
  homeFirstRunResumeText: string
  homeCustomerGuideProgress: HomeCustomerGuideProgressStep[]
  firstDeviceFocusMode: boolean
  firstDeviceWorkbenchLoaded: boolean
  firstDeviceReadyProof: FirstDeviceReadyProof
  firstDevice: FirstDeviceSummary | null
  firstDeviceLoading: boolean
  deploymentHealthLoading: boolean
  automationGuideLoading: boolean
  firstRunCreateLoading: boolean
  firstRunProtocol: HomeFirstRunProtocol
  deploymentHealthOk: boolean
  firstRunCreateResult: HomeFirstRunQuickCreateResult | null
  firstRunCreateTenantRequired: boolean
  firstRunSetupBlockerStep?: HomeCustomerGuideProgressStep | null
  firstDeviceAccessGuide: DeviceAccessGuideState | null
  firstDeviceSimulation: SimulationInitState | null
  firstDevicePublishCommand: string
  firstDeviceOnboardingGuard: FirstDeviceOnboardingGuard
  firstDeviceActionLoading: boolean
  firstDeviceTestResult: string
  firstDeviceBrowserTest: FirstDeviceBrowserTestState
  firstDeviceChart: FirstDeviceChartState
  deploymentHealthRows: NormalizedDeploymentHealthRow[]
  buildFirstDeviceSupportSummary: (options: {
    latestProofText: string
    activeTestCommand?: { label: string } | null
    delivery?: {
      firstDeviceUrl?: string
      proofUrl?: string
      proofFileHint?: string
    }
  }) => string
}

const props = defineProps<Props>()
const emit = defineEmits<{
  openHomeGuideStep: [step: HomeCustomerGuideProgressStep]
  refreshHomeGuideProgress: []
  refreshFirstDeviceWorkbench: []
  updateFirstRunProtocol: [protocol: HomeFirstRunProtocol]
  createFirstRunFirstDevice: []
  openManualDeviceAdd: []
  openThingsModel: []
  copyFirstDevicePublishCommand: []
  simulateFirstDeviceTelemetry: []
  openFirstDeviceFullGuide: []
  openFirstDeviceAccessGuide: []
  runFirstDeviceQuickstartAction: [action: string]
  refreshDeploymentHealth: []
}>()

const firstDeviceCoreGuideSteps = computed(() => filterFirstDeviceCoreGuideSteps(props.homeCustomerGuideProgress))
const firstDeviceNextGuideSteps = computed(() => filterFirstDeviceNextGuideSteps(props.homeCustomerGuideProgress))
const firstDeviceNextActiveGuideStep = computed<HomeCustomerGuideProgressStep | null>(
  () =>
    (firstDeviceNextGuideSteps.value.find(step => step.status === 'active') as HomeCustomerGuideProgressStep) ||
    null
)
const firstDevicePostReadyHandoff = computed(() =>
  buildFirstDevicePostReadyHandoff({
    ready: props.firstDeviceReadyProof.ready,
    nextStep: firstDeviceNextActiveGuideStep.value
  })
)
const firstDeviceReadyNextGuideDescription = computed(() => {
  return (
    firstDevicePostReadyHandoff.value?.description ||
    $t('custom.home.firstDevice.readyHandoff.fallbackDescription')
  )
})
const firstDeviceCoreGuideSummary = computed(() => buildFirstDeviceCoreGuideSummary(firstDeviceCoreGuideSteps.value))
const firstRunSetupBlockerStep = computed(
  () =>
    props.firstRunSetupBlockerStep || props.homeCustomerGuideProgress.find(step => step.id === 'setup') || null
)
const firstRunSetupBlockerTitle = computed(() => firstRunSetupBlockerStep.value?.title || $t('custom.home.firstDevice.setupBlocker.title'))
const firstRunSetupBlockerDescription = computed(
  () => firstRunSetupBlockerStep.value?.description || $t('custom.home.firstDevice.setupBlocker.description')
)
const firstRunSetupBlockerAction = computed(() => firstRunSetupBlockerStep.value?.action || $t('custom.home.firstDevice.setupBlocker.action'))
const deviceIdentitySectionRef = ref<HTMLElement | null>(null)
const connectionTestViewportRef = ref<HTMLElement | null>(null)
const connectionTestSectionRef = ref<{ connectionEl: HTMLElement | null; testCommandEl: HTMLElement | null } | null>(
  null
)
const successProofViewportRef = ref<HTMLElement | null>(null)
const successProofSectionRef = ref<{ chartSectionEl: HTMLElement | null; proofSectionEl: HTMLElement | null } | null>(
  null
)
const quickstartSectionRef = ref<HTMLElement | null>(null)
const supportSummaryViewportRef = ref<HTMLElement | null>(null)
const supportSummarySectionRef = ref<{ openPreview: () => void } | null>(null)
const setConnectionTestViewportRef = (element: HTMLElement | null) => {
  connectionTestViewportRef.value = element
}
const setConnectionTestSectionRef = (instance: Element | ComponentPublicInstance | null) => {
  connectionTestSectionRef.value = instance as {
    connectionEl: HTMLElement | null
    testCommandEl: HTMLElement | null
  } | null
}
const setSuccessProofViewportRef = (element: HTMLElement | null) => {
  successProofViewportRef.value = element
}
const setSuccessProofSectionRef = (instance: Element | ComponentPublicInstance | null) => {
  successProofSectionRef.value = instance as {
    chartSectionEl: HTMLElement | null
    proofSectionEl: HTMLElement | null
  } | null
}
const setSupportSummaryViewportRef = (element: HTMLElement | null) => {
  supportSummaryViewportRef.value = element
}
const setSupportSummarySectionRef = (instance: Element | ComponentPublicInstance | null) => {
  supportSummarySectionRef.value = instance as { openPreview: () => void } | null
}
const { shouldMount: shouldMountConnectionTestSection, mountNow: mountConnectionTestSection } =
  useViewportDeferredMount(connectionTestViewportRef, { rootMargin: '480px 0px', fallbackDelay: 600 })
const { shouldMount: shouldMountSuccessProofSection, mountNow: mountSuccessProofSection } = useViewportDeferredMount(
  successProofViewportRef,
  { rootMargin: '520px 0px', fallbackDelay: 700 }
)
const { shouldMount: shouldMountSupportSummarySection, mountNow: mountSupportSummarySection } =
  useViewportDeferredMount(supportSummaryViewportRef, { rootMargin: '480px 0px', fallbackDelay: 600 })
const pendingSupportSummaryPreviewOpen = ref(false)
const deploymentHealthSectionRef = ref<HTMLElement | null>(null)
const firstFailedDeploymentHealthRow = computed(() => props.deploymentHealthRows.find((row) => !row.ok) || null)
const firstDeviceCurrentBlocker = computed(
  () => props.firstDeviceReadyProof.items?.find(item => !item.ok) || null
)
const firstDevicePrimaryAction = computed(
  () =>
    props.firstDeviceOnboardingGuard.activeStep?.action ||
    (props.firstDeviceReadyProof.ready ? 'ready-check' : 'health')
)
const firstDeviceLatestProofText = computed(() =>
  buildFirstDeviceLatestProofText({
    device: props.firstDevice,
    chart: props.firstDeviceChart,
    testResult: props.firstDeviceTestResult
  })
)
const firstDeviceFlowNodes = computed(() => buildFirstDeviceFlowNodes(props.firstDeviceReadyProof.items || []))
const firstDeviceClosureSummary = computed(() => buildFirstDeviceClosureSummary(firstDeviceFlowNodes.value))

const firstRunWizardSteps = computed(() =>
  buildFirstRunWizardSteps(firstDeviceCoreGuideSteps.value, {
    setupBlockerDescription: firstRunSetupBlockerDescription.value,
    deploymentHealthOk: props.deploymentHealthOk,
    firstFailedDeploymentHealthRow: firstFailedDeploymentHealthRow.value,
    firstDevice: props.firstDevice,
    firstRunProtocol: props.firstRunProtocol,
    firstDeviceChart: props.firstDeviceChart,
    firstDeviceTestResult: props.firstDeviceTestResult
  })
)
const currentFocusedQuickstartStep = computed(() => props.firstDeviceOnboardingGuard.activeStep || null)
const currentFocusedQuickstartSectionKey = computed(() =>
  resolveFirstDeviceFocusedSectionKey({
    activeStep: currentFocusedQuickstartStep.value,
    ready: props.firstDeviceReadyProof.ready,
    readyProofItems: props.firstDeviceReadyProof.items || [],
    chartReady: props.firstDeviceChart.ready
  })
)
const currentFocusedQuickstartCopy = computed(() =>
  buildFocusedQuickstartCopy({
    ready: props.firstDeviceReadyProof.ready,
    activeStep: currentFocusedQuickstartStep.value,
    readyDescription: firstDeviceReadyNextGuideDescription.value,
    guardSummary: props.firstDeviceOnboardingGuard.summary,
    nextAction: props.firstDeviceOnboardingGuard.nextAction,
    postReadyHandoff: firstDevicePostReadyHandoff.value
  })
)
const currentFocusedQuickstartTitle = computed(() => currentFocusedQuickstartCopy.value.title)
const currentFocusedQuickstartDescription = computed(() => currentFocusedQuickstartCopy.value.description)
const currentFocusedQuickstartSuccessSignal = computed(() => currentFocusedQuickstartCopy.value.successSignal)
const currentFocusedQuickstartActionLabel = computed(() => currentFocusedQuickstartCopy.value.actionLabel)
const currentFocusedQuickstartActionDisabled = computed(() => currentFocusedQuickstartCopy.value.actionDisabled)
const firstDeviceOperatorCue = computed(() =>
  buildFirstDeviceOperatorCue({
    ready: props.firstDeviceReadyProof.ready,
    activeStep: currentFocusedQuickstartStep.value,
    actionLabel: currentFocusedQuickstartActionLabel.value,
    successSignal: currentFocusedQuickstartSuccessSignal.value,
    readyDescription: firstDeviceReadyNextGuideDescription.value,
    currentBlocker: firstDeviceCurrentBlocker.value
  })
)
const firstDeviceMissionControl = computed(() =>
  buildFirstDeviceMissionControl({
    ready: props.firstDeviceReadyProof.ready,
    activeStep: currentFocusedQuickstartStep.value,
    actionLabel: currentFocusedQuickstartActionLabel.value,
    successSignal: currentFocusedQuickstartSuccessSignal.value,
    readyDescription: firstDeviceReadyNextGuideDescription.value,
    currentBlocker: firstDeviceCurrentBlocker.value
  })
)
const currentFocusedQuickstartActionLoading = computed(() =>
  getFocusedQuickstartActionLoading({
    ready: props.firstDeviceReadyProof.ready,
    activeStep: currentFocusedQuickstartStep.value,
    firstDeviceActionLoading: props.firstDeviceActionLoading,
    deploymentHealthLoading: props.deploymentHealthLoading,
    firstRunCreateLoading: props.firstRunCreateLoading
  })
)
const firstDeviceStatusHeroCopy = computed(() =>
  buildFirstDeviceStatusHeroCopy({
    ready: props.firstDeviceReadyProof.ready,
    currentBlocker: firstDeviceCurrentBlocker.value,
    activeStep: currentFocusedQuickstartStep.value,
    guardSummary: props.firstDeviceOnboardingGuard.summary
  })
)
const firstDeviceStatusHeroTitle = computed(() => firstDeviceStatusHeroCopy.value.title)
const firstDeviceStatusHeroDescription = computed(() => firstDeviceStatusHeroCopy.value.description)
const firstDeviceSuccessProofCopy = computed(() =>
  buildFirstDeviceSuccessProofCopy({
    ready: props.firstDeviceReadyProof.ready,
    chartReady: props.firstDeviceChart.ready,
    testResult: props.firstDeviceTestResult
  })
)
const firstDeviceSuccessProofTitle = computed(() => firstDeviceSuccessProofCopy.value.title)
const firstDeviceSuccessProofDescription = computed(() => firstDeviceSuccessProofCopy.value.description)
const firstDeviceSuccessFacts = computed(() =>
  buildFirstDeviceSuccessFacts({
    device: props.firstDevice,
    chart: props.firstDeviceChart,
    latestProofText: firstDeviceLatestProofText.value
  })
)
const firstDevicePostTestGuidance = computed(() =>
  buildFirstDevicePostTestGuidance({
    testResult: props.firstDeviceTestResult,
    ready: props.firstDeviceReadyProof.ready,
    readyDescription: firstDeviceReadyNextGuideDescription.value,
    chartReady: props.firstDeviceChart.ready,
    currentBlocker: firstDeviceCurrentBlocker.value
  })
)

const firstDeviceVerificationAction = computed(() =>
  buildFirstDeviceVerificationAction({
    hasDevice: Boolean(props.firstDevice),
    ready: props.firstDeviceReadyProof.ready,
    postReadyHandoff: firstDevicePostReadyHandoff.value,
    readyDescription: firstDeviceReadyNextGuideDescription.value,
    chartReady: props.firstDeviceChart.ready,
    canRunBrowserTest: props.firstDeviceOnboardingGuard.canRunBrowserTest,
    testResult: props.firstDeviceTestResult,
    actionLoading: props.firstDeviceActionLoading,
    currentBlocker: firstDeviceCurrentBlocker.value
  })
)

const getFirstDeviceFlowNodeAction = node => {
  if (node.key === 'deployment') {
    return {
      label: node.ok ? $t('custom.home.firstDevice.canvas.action.recheckDeployment') : $t('custom.home.firstDevice.canvas.action.checkDeployment'),
      disabled: false,
      loading: props.deploymentHealthLoading,
      run: () => emit('refreshDeploymentHealth')
    }
  }
  if (node.key === 'identity') {
    return props.firstDevice
      ? {
          label: $t('custom.home.firstDevice.common.openReadyCheck'),
          disabled: false,
          loading: false,
          run: () => emit('openFirstDeviceAccessGuide')
        }
      : {
          label: $t('custom.home.firstDevice.canvas.action.generateDevice'),
          disabled: props.firstRunCreateTenantRequired || !props.deploymentHealthOk,
          loading: props.firstRunCreateLoading,
          run: () => emit('createFirstRunFirstDevice')
        }
  }
  if (node.key === 'connection') {
    if (props.firstDeviceOnboardingGuard.canCopyCommand && activeFirstDeviceTestCommand.value?.code) {
      return {
        label: $t('custom.home.firstDevice.canvas.action.copyTestCommand'),
        disabled: false,
        loading: false,
        run: () => void copyActiveFirstDeviceTestCommand()
      }
    }
    return {
      label: $t('custom.home.firstDevice.canvas.action.openAccessGuide'),
      disabled: false,
      loading: false,
      run: () => emit('openFirstDeviceFullGuide')
    }
  }
  if (node.key === 'browser_test') {
    return props.firstDeviceOnboardingGuard.canRunBrowserTest
      ? {
          label: node.ok ? $t('custom.home.firstDevice.canvas.action.rerunBrowserTest') : $t('custom.home.firstDevice.canvas.action.browserTest'),
          disabled: false,
          loading: props.firstDeviceActionLoading,
          run: () => emit('simulateFirstDeviceTelemetry')
        }
      : {
          label: $t('custom.home.firstDevice.common.openReadyCheck'),
          disabled: false,
          loading: false,
          run: () => emit('openFirstDeviceAccessGuide')
        }
  }
  if (node.key === 'online' || node.key === 'telemetry') {
    return {
      label: $t('custom.home.firstDevice.canvas.action.viewReadyCheck'),
      disabled: false,
      loading: false,
      run: () => emit('openFirstDeviceAccessGuide')
    }
  }
  return {
    label: node.ok ? $t('custom.home.firstDevice.canvas.action.viewAccessGuide') : $t('custom.home.firstDevice.proof.keepGoing'),
    disabled: false,
    loading: false,
    run: () => (node.ok ? emit('openFirstDeviceFullGuide') : runFirstDevicePrimaryAction())
  }
}

const ensureDeferredSectionMounted = async (key: string) => {
  if ((key === 'connection' || key === 'test') && !shouldMountConnectionTestSection.value) {
    mountConnectionTestSection()
    await nextTick()
  }
  if (
    (key === 'chart' || key === 'proof' || key === 'online' || key === 'telemetry') &&
    !shouldMountSuccessProofSection.value
  ) {
    mountSuccessProofSection()
    await nextTick()
  }
  if (key === 'support' && !shouldMountSupportSummarySection.value) {
    mountSupportSummarySection()
    await nextTick()
  }
}

const focusFirstDeviceSection = async (key: string) => {
  await ensureDeferredSectionMounted(key)
  const connectionEl = connectionTestSectionRef.value?.connectionEl || null
  const testCommandEl = connectionTestSectionRef.value?.testCommandEl || null
  const chartEl = successProofSectionRef.value?.chartSectionEl || null
  const proofEl = successProofSectionRef.value?.proofSectionEl || null
  const targetMap: Record<string, HTMLElement | null> = {
    deployment: deploymentHealthSectionRef.value,
    identity: deviceIdentitySectionRef.value,
    connection: connectionEl || connectionTestViewportRef.value,
    browser_test: testCommandEl || connectionTestViewportRef.value,
    online: proofEl || successProofViewportRef.value,
    telemetry: chartEl || successProofViewportRef.value,
    chart: chartEl || successProofViewportRef.value
  }
  const target = targetMap[key] || quickstartSectionRef.value || supportSummaryViewportRef.value
  if (!target) return
  await nextTick()
  target.scrollIntoView({ behavior: 'smooth', block: 'center' })
}
const focusSection = async (key: string) => {
  await ensureDeferredSectionMounted(key)
  const connectionEl = connectionTestSectionRef.value?.connectionEl || null
  const testCommandEl = connectionTestSectionRef.value?.testCommandEl || null
  const chartEl = successProofSectionRef.value?.chartSectionEl || null
  const proofEl = successProofSectionRef.value?.proofSectionEl || null
  const targetMap: Record<string, HTMLElement | null> = {
    device: deviceIdentitySectionRef.value,
    connection: connectionEl || connectionTestViewportRef.value,
    test: testCommandEl || connectionTestViewportRef.value,
    chart: chartEl || successProofViewportRef.value,
    quickstart: quickstartSectionRef.value,
    proof: proofEl || successProofViewportRef.value,
    support: supportSummaryViewportRef.value,
    deployment: deploymentHealthSectionRef.value
  }
  const target = targetMap[key] || quickstartSectionRef.value
  if (!target) return
  await nextTick()
  target.scrollIntoView({ behavior: 'smooth', block: 'center' })
}
const selectedFirstDeviceTestCommand = ref('')
const firstDeviceTestCommands = computed(() =>
  buildFirstDeviceTestCommands({
    accessGuide: props.firstDeviceAccessGuide,
    publishCommand: props.firstDevicePublishCommand
  })
)
const activeFirstDeviceTestCommand = computed(
  () =>
    firstDeviceTestCommands.value.find(command => command.language === selectedFirstDeviceTestCommand.value) ||
    firstDeviceTestCommands.value[0] ||
    null
)
const firstDeviceOnlineTesterState = computed(() =>
  buildFirstDeviceOnlineTesterState({
    guard: props.firstDeviceOnboardingGuard,
    browserTest: props.firstDeviceBrowserTest,
    chart: props.firstDeviceChart,
    activeTestCommandLabel: activeFirstDeviceTestCommand.value
      ? getFirstDeviceTestCommandLabel(activeFirstDeviceTestCommand.value)
      : ''
  })
)
const getFirstDeviceClosedLoopState = (key: string, done = false) => {
  const node = firstDeviceFlowNodes.value.find(item => item.key === key)
  const state = done || node?.ok ? 'done' : node?.state === 'active' ? 'active' : 'todo'

  return {
    state,
    stateLabel:
      state === 'done'
        ? $t('custom.home.firstDevice.canvas.state.done')
        : state === 'active'
          ? $t('custom.home.firstDevice.canvas.state.active')
          : $t('custom.home.firstDevice.canvas.state.todo'),
    stateType: state === 'done' ? 'success' : state === 'active' ? 'warning' : 'default'
  }
}
const firstDeviceClosedLoopSteps = computed(() => {
  const deployment = getFirstDeviceClosedLoopState('deployment', props.deploymentHealthOk)
  const identity = getFirstDeviceClosedLoopState('identity', Boolean(props.firstDevice))
  const connection = getFirstDeviceClosedLoopState(
    'connection',
    Boolean(props.firstDeviceOnboardingGuard.canCopyCommand && activeFirstDeviceTestCommand.value?.code)
  )
  const browserTest = getFirstDeviceClosedLoopState(
    'browser_test',
    props.firstDeviceBrowserTest?.status === 'confirmed'
  )
  const telemetry = getFirstDeviceClosedLoopState(
    'telemetry',
    Boolean(props.firstDevice?.online && props.firstDeviceChart.ready)
  )
  const proof = getFirstDeviceClosedLoopState('chart', props.firstDeviceReadyProof.ready)

  return [
    {
      key: 'deployment',
      section: 'deployment',
      order: '01',
      title: '部署健康',
      detail: props.deploymentHealthOk ? '前端、API、DB、Redis、MQTT 可用' : '先确认部署组件都正常',
      actionLabel: props.deploymentHealthOk ? '重新检查' : '检查部署',
      disabled: false,
      loading: props.deploymentHealthLoading,
      ...deployment
    },
    {
      key: 'identity',
      section: 'device',
      order: '02',
      title: '创建产品/设备',
      detail: props.firstDevice
        ? props.firstDevice.name || props.firstDevice.number || '第一台设备已生成'
        : '一键生成默认产品、物模型和第一台设备',
      actionLabel: props.firstDevice ? '定位设备信息' : '一键生成',
      disabled: props.firstRunCreateTenantRequired || !props.deploymentHealthOk,
      loading: props.firstRunCreateLoading,
      ...identity
    },
    {
      key: 'connection',
      section: 'connection',
      order: '03',
      title: '复制 MQTT/HTTP 参数',
      detail: activeFirstDeviceTestCommand.value?.label || '等待连接参数和测试命令',
      actionLabel: props.firstDeviceOnboardingGuard.canCopyCommand ? '复制测试命令' : '看接入指南',
      disabled: !props.firstDevice,
      loading: false,
      ...connection
    },
    {
      key: 'browser_test',
      section: 'test',
      order: '04',
      title: '浏览器发测试数据',
      detail: props.firstDeviceBrowserTest?.message || '不用真实设备，先在浏览器里模拟上报',
      actionLabel: props.firstDeviceOnboardingGuard.canRunBrowserTest ? '发送测试' : '打开 Ready Check',
      disabled: !props.firstDevice,
      loading: props.firstDeviceActionLoading,
      ...browserTest
    },
    {
      key: 'telemetry',
      section: 'chart',
      order: '05',
      title: '确认在线/遥测/首图',
      detail: firstDeviceLatestProofText.value,
      actionLabel: props.firstDeviceChart.ready ? '查看首图' : '刷新确认',
      disabled: !props.firstDevice,
      loading: props.firstDeviceLoading,
      ...telemetry
    },
    {
      key: 'proof',
      section: 'proof',
      order: '06',
      title: '下载成功证明',
      detail: props.firstDeviceReadyProof.ready ? '设备已准备好，可以交付证明' : '等所有证明项变绿后下载',
      actionLabel: props.firstDeviceReadyProof.ready ? '下载证明' : '查看缺口',
      disabled: !props.firstDevice,
      loading: false,
      ...proof
    }
  ]
})
watch(
  firstDeviceTestCommands,
  (commands) => {
    if (!commands.length) {
      selectedFirstDeviceTestCommand.value = ''
      return
    }
    if (!commands.some(command => command.language === selectedFirstDeviceTestCommand.value)) {
      selectedFirstDeviceTestCommand.value = commands[0].language
    }
  },
  { immediate: true }
)

const buildFirstDeviceConnectionSummary = () => {
  const endpoint =
    props.firstDeviceAccessGuide?.endpoint ||
    [props.firstDeviceSimulation?.server, props.firstDeviceSimulation?.port].filter(Boolean).join(':') ||
    'not-ready'
  const reportEntry =
    props.firstDeviceAccessGuide?.endpointKind === 'http'
      ? props.firstDeviceAccessGuide.endpoint || endpoint
      : props.firstDeviceAccessGuide?.reportTopic || props.firstDeviceSimulation?.topic || 'devices/telemetry'
  const controlEntry = props.firstDeviceAccessGuide?.controlTopic || 'open Ready Check'
  const command = activeFirstDeviceTestCommand.value?.code || props.firstDevicePublishCommand || ''

  return [
    'AetherLink first-device connection summary',
    `Device: ${props.firstDevice?.name || props.firstDevice?.number || 'first device'}`,
    `Protocol: ${props.firstDeviceAccessGuide?.protocol || 'MQTT'}`,
    `Endpoint: ${endpoint}`,
    `Report entry: ${reportEntry}`,
    `Control entry: ${controlEntry}`,
    command
      ? `Device connection command:
${command}`
      : 'Device connection command: not-ready'
  ].join('\n')
}

const copyFirstDeviceConnectionSummary = async () => {
  const copied = await writeClipboardText(buildFirstDeviceConnectionSummary())
  if (copied) {
    window.$message?.success($t('theme.configOperation.copySuccess'))
  } else {
    window.$message?.error($t('common.copyFailed'))
  }
}

const copyActiveFirstDeviceTestCommand = async () => {
  if (!activeFirstDeviceTestCommand.value?.code) return
  const copied = await writeClipboardText(activeFirstDeviceTestCommand.value.code)
  if (copied) {
    window.$message?.success($t('theme.configOperation.copySuccess'))
  } else {
    window.$message?.error($t('common.copyFailed'))
  }
}
const runFirstDeviceClosedLoopStep = step => {
  if (step.disabled) {
    void focusSection(step.section)
    return
  }
  if (step.key === 'deployment') {
    emit('refreshDeploymentHealth')
    return
  }
  if (step.key === 'identity') {
    if (props.firstDevice) {
      void focusSection('device')
      return
    }
    emit('createFirstRunFirstDevice')
    return
  }
  if (step.key === 'connection') {
    if (props.firstDeviceOnboardingGuard.canCopyCommand && activeFirstDeviceTestCommand.value?.code) {
      void copyActiveFirstDeviceTestCommand()
      return
    }
    emit('openFirstDeviceFullGuide')
    return
  }
  if (step.key === 'browser_test') {
    if (props.firstDeviceOnboardingGuard.canRunBrowserTest) {
      emit('simulateFirstDeviceTelemetry')
      return
    }
    emit('openFirstDeviceAccessGuide')
    return
  }
  if (step.key === 'telemetry') {
    if (props.firstDeviceChart.ready) {
      void focusSection('chart')
      return
    }
    emit('refreshFirstDeviceWorkbench')
    return
  }
  if (step.key === 'proof') {
    if (props.firstDeviceReadyProof.ready) {
      downloadFirstDeviceSuccessProof()
      return
    }
    void focusSection('proof')
  }
}
const firstDeviceOperationChecklist = computed(() =>
  buildFirstDeviceOperationChecklist({
    canCopyCommand: props.firstDeviceOnboardingGuard.canCopyCommand,
    activeTestCommand: activeFirstDeviceTestCommand.value,
    canRunBrowserTest: props.firstDeviceOnboardingGuard.canRunBrowserTest,
    deploymentHealthOk: props.deploymentHealthOk
  })
)

const firstDeviceProofOrigin = () => (typeof window === 'undefined' ? undefined : window.location.origin)
const firstDeviceProofDeliveryState = computed<FirstDeviceProofDeliveryState>(() => ({
  device: props.firstDevice,
  accessGuide: props.firstDeviceAccessGuide,
  simulation: props.firstDeviceSimulation,
  readyProof: props.firstDeviceReadyProof,
  onboardingGuard: props.firstDeviceOnboardingGuard,
  chart: props.firstDeviceChart,
  browserTest: props.firstDeviceBrowserTest,
  deploymentHealthRows: props.deploymentHealthRows
}))
const firstDeviceProofDelivery = computed(() =>
  buildFirstDeviceProofDelivery(firstDeviceProofDeliveryState.value, firstDeviceProofOrigin())
)

const buildFirstDeviceSupportSummaryForCopy = () =>
  props.buildFirstDeviceSupportSummary({
    latestProofText: firstDeviceLatestProofText.value,
    activeTestCommand: activeFirstDeviceTestCommand.value
      ? {
          label: getFirstDeviceTestCommandLabel(activeFirstDeviceTestCommand.value)
        }
      : null,
    delivery: firstDeviceProofDelivery.value
  })

const firstDeviceChartProofSummary = computed(() =>
  buildFirstDeviceChartProofSummary({
    device: props.firstDevice,
    chart: props.firstDeviceChart,
    readyProof: props.firstDeviceReadyProof
  })
)
const firstDeviceSuccessProofPacket = computed(() =>
  buildFirstDeviceSuccessProofDeliveryPacket(firstDeviceProofDeliveryState.value, firstDeviceProofOrigin())
)

const downloadFirstDeviceSuccessProof = () => {
  downloadFirstDeviceSuccessProofPacket(
    firstDeviceSuccessProofPacket.value,
    buildFirstDeviceProofFilename(props.firstDevice)
  )
}

const copyFirstDeviceChartProof = async () => {
  const copied = await writeClipboardText(firstDeviceChartProofSummary.value)
  if (copied) {
    window.$message?.success($t('theme.configOperation.copySuccess'))
  } else {
    window.$message?.error($t('common.copyFailed'))
  }
}

const openFirstDeviceSupportSummaryPreview = async () => {
  if (!shouldMountSupportSummarySection.value) {
    mountSupportSummarySection()
    await nextTick()
  }
  if (supportSummarySectionRef.value) {
    supportSummarySectionRef.value.openPreview()
    return
  }
  pendingSupportSummaryPreviewOpen.value = true
  await nextTick()
  const summarySection = supportSummarySectionRef.value as { openPreview: () => void } | null
  if (summarySection) {
    summarySection.openPreview()
    pendingSupportSummaryPreviewOpen.value = false
  }
}

const runCurrentFocusedQuickstartAction = () => {
  if (props.firstDeviceReadyProof.ready) {
    if (firstDeviceNextActiveGuideStep.value) {
      emit('openHomeGuideStep', firstDeviceNextActiveGuideStep.value)
      return
    }
    emit('openFirstDeviceFullGuide')
    return
  }
  if (!currentFocusedQuickstartStep.value) return
  emit('runFirstDeviceQuickstartAction', currentFocusedQuickstartStep.value.action)
}

const focusCurrentFocusedQuickstartSection = () => {
  void focusSection(currentFocusedQuickstartSectionKey.value)
}

const runFirstDeviceVerificationAction = () => {
  const action = firstDeviceVerificationAction.value?.action
  if (action === 'simulate') {
    emit('simulateFirstDeviceTelemetry')
    return
  }
  if (action === 'ready-check') {
    emit('openFirstDeviceAccessGuide')
    return
  }
  if (action === 'guide') {
    emit('openFirstDeviceFullGuide')
    return
  }
  if (action === 'next-guide' && firstDeviceNextActiveGuideStep.value) {
    emit('openHomeGuideStep', firstDeviceNextActiveGuideStep.value)
    return
  }
  if (action === 'proof') {
    void focusSection('proof')
  }
}

const runFirstDeviceVerificationSecondaryAction = () => {
  const action = firstDeviceVerificationAction.value
  if (!action) return
  if (action.action === 'next-guide') {
    emit('openFirstDeviceFullGuide')
    return
  }
  if (action.action === 'proof') {
    void copyFirstDeviceChartProof()
    return
  }
  void focusSection(action.section)
}

const focusDeploymentHealth = async () => {
  await nextTick()
  deploymentHealthSectionRef.value?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}

defineExpose({ focusDeploymentHealth, focusSection })

watch(supportSummarySectionRef, (summarySection) => {
  if (!summarySection || !pendingSupportSummaryPreviewOpen.value) return
  summarySection.openPreview()
  pendingSupportSummaryPreviewOpen.value = false
})

const runFirstDevicePrimaryAction = () => {
  if (props.firstDeviceReadyProof.ready) {
    if (firstDevicePostReadyHandoff.value?.action === 'next-guide' && firstDeviceNextActiveGuideStep.value) {
      emit('openHomeGuideStep', firstDeviceNextActiveGuideStep.value)
      return
    }
    emit('openFirstDeviceFullGuide')
    return
  }
  emit('runFirstDeviceQuickstartAction', firstDevicePrimaryAction.value)
}
</script>

<template>
  <HomeFirstDeviceClosedLoopStrip :steps="firstDeviceClosedLoopSteps" @run-step="runFirstDeviceClosedLoopStep" />

  <div
    class="grid gap-16px"
    :class="
      firstDeviceFocusMode
        ? 'lg:grid-cols-1'
        : 'lg:grid-cols-[minmax(0,1.5fr)_minmax(320px,0.8fr)]'
    "
  >
    <HomeFirstDeviceGuideProgress
      :first-device="firstDevice"
      :ready="firstDeviceReadyProof.ready"
      :core-guide-summary="firstDeviceCoreGuideSummary"
      :core-guide-steps="firstDeviceCoreGuideSteps"
      :next-guide-steps="firstDeviceNextGuideSteps"
      :resume-text="homeFirstRunResumeText"
      :first-device-loading="firstDeviceLoading"
      :deployment-health-loading="deploymentHealthLoading"
      :automation-guide-loading="automationGuideLoading"
      @open-home-guide-step="emit('openHomeGuideStep', $event as HomeCustomerGuideProgressStep)"
      @refresh-home-guide-progress="emit('refreshHomeGuideProgress')"
    />

    <n-card :bordered="false" class="rounded-8px">
      <div class="flex h-full flex-col gap-12px text-14px">
        <HomeFirstDeviceVerificationOverview
          :ready="firstDeviceReadyProof.ready"
          :first-device-loading="firstDeviceLoading"
          :status-hero-title="firstDeviceStatusHeroTitle"
          :status-hero-description="firstDeviceStatusHeroDescription"
          :latest-proof-text="firstDeviceLatestProofText"
          :operator-cue="firstDeviceOperatorCue"
          :mission-control="firstDeviceMissionControl"
          :closure-summary="firstDeviceClosureSummary"
          :verification-action="firstDeviceVerificationAction"
          :focused-action-disabled="currentFocusedQuickstartActionDisabled"
          :focused-action-loading="currentFocusedQuickstartActionLoading"
          :flow-nodes="firstDeviceFlowNodes"
          :wizard-steps="firstRunWizardSteps"
          :get-flow-node-action="getFirstDeviceFlowNodeAction"
          @refresh-first-device-workbench="emit('refreshFirstDeviceWorkbench')"
          @run-verification-action="runFirstDeviceVerificationAction"
          @run-verification-secondary-action="runFirstDeviceVerificationSecondaryAction"
          @run-current-focused-quickstart-action="runCurrentFocusedQuickstartAction"
          @focus-current-focused-quickstart-section="focusCurrentFocusedQuickstartSection"
          @open-first-device-support-summary-preview="openFirstDeviceSupportSummaryPreview"
          @download-success-proof="downloadFirstDeviceSuccessProof"
          @open-home-guide-step="emit('openHomeGuideStep', $event as HomeCustomerGuideProgressStep)"
          @focus-first-device-section="focusFirstDeviceSection"
        />

        <div ref="deviceIdentitySectionRef">
          <HomeFirstDeviceIdentitySection
            :first-device="firstDevice"
            :first-device-focus-mode="firstDeviceFocusMode"
            :first-device-workbench-loaded="firstDeviceWorkbenchLoaded"
            :first-device-loading="firstDeviceLoading"
            :first-run-protocol="firstRunProtocol"
            :first-run-create-loading="firstRunCreateLoading"
            :first-run-create-tenant-required="firstRunCreateTenantRequired"
            :deployment-health-ok="deploymentHealthOk"
            :first-run-create-result="firstRunCreateResult"
            :first-run-setup-blocker-step="firstRunSetupBlockerStep"
            :first-run-setup-blocker-title="firstRunSetupBlockerTitle"
            :first-run-setup-blocker-description="firstRunSetupBlockerDescription"
            :first-run-setup-blocker-action="firstRunSetupBlockerAction"
            @refresh-first-device-workbench="emit('refreshFirstDeviceWorkbench')"
            @refresh-deployment-health="emit('refreshDeploymentHealth')"
            @update-first-run-protocol="emit('updateFirstRunProtocol', $event)"
            @create-first-run-first-device="emit('createFirstRunFirstDevice')"
            @open-manual-device-add="emit('openManualDeviceAdd')"
            @open-things-model="emit('openThingsModel')"
            @open-home-guide-step="emit('openHomeGuideStep', $event as HomeCustomerGuideProgressStep)"
          />
        </div>

        <div ref="quickstartSectionRef" class="rounded-6px bg-gray-50 px-12px py-10px">
          <HomeFirstDeviceCurrentWorkspaceSection
            :title="currentFocusedQuickstartTitle"
            :description="currentFocusedQuickstartDescription"
            :success-signal="currentFocusedQuickstartSuccessSignal"
            :current-step="currentFocusedQuickstartStep"
            :ready="firstDeviceReadyProof.ready"
            :action-label="currentFocusedQuickstartActionLabel"
            :action-disabled="currentFocusedQuickstartActionDisabled"
            :action-loading="currentFocusedQuickstartActionLoading"
            :steps="firstDeviceOnboardingGuard.steps"
            @run-current-focused-quickstart-action="runCurrentFocusedQuickstartAction"
            @focus-current-focused-quickstart-section="focusCurrentFocusedQuickstartSection"
            @open-first-device-support-summary-preview="openFirstDeviceSupportSummaryPreview"
          />
        </div>

        <HomeFirstDeviceDeferredSections
          v-model:selected-test-command="selectedFirstDeviceTestCommand"
          :set-connection-test-viewport-ref="setConnectionTestViewportRef"
          :set-connection-test-section-ref="setConnectionTestSectionRef"
          :should-mount-connection-test-section="shouldMountConnectionTestSection"
          :first-device="firstDevice"
          :first-device-access-guide="firstDeviceAccessGuide"
          :first-device-simulation="firstDeviceSimulation"
          :first-device-onboarding-guard="firstDeviceOnboardingGuard"
          :operation-checklist="firstDeviceOperationChecklist"
          :test-commands="firstDeviceTestCommands"
          :active-test-command="activeFirstDeviceTestCommand"
          :first-device-publish-command="firstDevicePublishCommand"
          :first-device-action-loading="firstDeviceActionLoading"
          :first-device-online-tester-state="firstDeviceOnlineTesterState"
          :first-device-test-result="firstDeviceTestResult"
          :first-device-post-test-guidance="firstDevicePostTestGuidance"
          :first-device-ready-proof="firstDeviceReadyProof"
          :first-device-next-active-guide-step="firstDeviceNextActiveGuideStep"
          :set-success-proof-viewport-ref="setSuccessProofViewportRef"
          :set-success-proof-section-ref="setSuccessProofSectionRef"
          :should-mount-success-proof-section="shouldMountSuccessProofSection"
          :first-device-success-proof-title="firstDeviceSuccessProofTitle"
          :first-device-success-proof-description="firstDeviceSuccessProofDescription"
          :first-device-success-facts="firstDeviceSuccessFacts"
          :first-device-chart="firstDeviceChart"
          :set-support-summary-viewport-ref="setSupportSummaryViewportRef"
          :set-support-summary-section-ref="setSupportSummarySectionRef"
          :should-mount-support-summary-section="shouldMountSupportSummarySection"
          :build-first-device-support-summary-for-copy="buildFirstDeviceSupportSummaryForCopy"
          @copy-connection-summary="copyFirstDeviceConnectionSummary"
          @copy-active-first-device-test-command="copyActiveFirstDeviceTestCommand"
          @copy-first-device-publish-command="emit('copyFirstDevicePublishCommand')"
          @simulate-first-device-telemetry="emit('simulateFirstDeviceTelemetry')"
          @open-first-device-full-guide="emit('openFirstDeviceFullGuide')"
          @open-first-device-access-guide="emit('openFirstDeviceAccessGuide')"
          @open-home-guide-step="emit('openHomeGuideStep', $event as HomeCustomerGuideProgressStep)"
          @focus-proof="focusSection('proof')"
          @focus-connection="focusSection('connection')"
          @open-support-summary-preview="openFirstDeviceSupportSummaryPreview"
          @copy-chart-proof="copyFirstDeviceChartProof"
          @download-success-proof="downloadFirstDeviceSuccessProof"
        />

        <div class="rounded-6px bg-gray-50 px-12px py-10px">
          <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.workbench.afterOnboarding') }}</div>
          <div class="mt-2px font-600">{{ $t('custom.home.firstDevice.workbench.minimalLoop') }}</div>
          <div class="mt-4px text-gray-500">
            {{ $t('custom.home.firstDevice.workbench.minimalLoopDesc') }}
          </div>
        </div>

        <div ref="deploymentHealthSectionRef">
          <HomeFirstDeviceDeploymentHealthSection
            :deployment-health-loading="deploymentHealthLoading"
            :deployment-health-ok="deploymentHealthOk"
            :deployment-health-rows="deploymentHealthRows"
            @refresh-deployment-health="emit('refreshDeploymentHealth')"
          />
        </div>
      </div>
    </n-card>
  </div>
</template>

<style scoped></style>
