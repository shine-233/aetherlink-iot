/**
 * 文件用途：首台设备工作台的派生状态（引导步骤、聚焦步骤文案、闭环步骤、测试命令、证明交付）。
 * 核心逻辑：只读 props，全部为 computed；副作用（复制、下载、emit）留在视图的 dispatch。
 */
import { computed, ref, watch } from 'vue'
import type { DeviceAccessGuideState } from '@/views/device/details/modules/device-access-guide-state'
import {
  buildFirstDeviceClosureSummary,
  buildFirstDeviceFlowNodes,
  buildFirstDeviceLatestProofText,
  buildFirstDeviceOnlineTesterState,
  buildFirstDevicePostReadyHandoff,
  buildFirstDevicePostTestGuidance,
  buildFirstDeviceSuccessFacts,
  buildFirstDeviceVerificationAction,
  resolveFirstDeviceFocusedSectionKey,
  type FirstDeviceBrowserTestState,
  type FirstDeviceChartState,
  type FirstDeviceOnboardingGuard,
  type FirstDeviceReadyProof,
  type FirstDeviceSummary,
  type SimulationInitState
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
import type { HomeFirstRunProtocol, HomeFirstRunQuickCreateResult } from './homeFirstRunWizard'
import {
  buildFirstDeviceProofDelivery,
  buildFirstDeviceProofFilename,
  buildFirstDeviceSuccessProofDeliveryPacket,
  downloadFirstDeviceSuccessProofPacket,
  type FirstDeviceProofDeliveryState
} from './homeFirstDeviceProofDelivery'
import { buildFirstDeviceClosedLoopSteps } from './homeFirstDeviceClosedLoop'
import type { HomeCustomerGuideProgressStep, HomeCustomerGuideSummary } from './homeCustomerGuide'
import type { NormalizedDeploymentHealthRow } from './homeDeploymentHealth'

export interface HomeFirstDeviceWorkbenchProps {
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

export function useFirstDeviceWorkbenchState(props: HomeFirstDeviceWorkbenchProps) {
  const firstDeviceCoreGuideSteps = computed(() => filterFirstDeviceCoreGuideSteps(props.homeCustomerGuideProgress))
  const firstDeviceNextGuideSteps = computed(() => filterFirstDeviceNextGuideSteps(props.homeCustomerGuideProgress))
  const firstDeviceNextActiveGuideStep = computed<HomeCustomerGuideProgressStep | null>(
    () =>
      (firstDeviceNextGuideSteps.value.find((step) => step.status === 'active') as HomeCustomerGuideProgressStep) ||
      null
  )
  const firstDevicePostReadyHandoff = computed(() =>
    buildFirstDevicePostReadyHandoff({
      ready: props.firstDeviceReadyProof.ready,
      nextStep: firstDeviceNextActiveGuideStep.value
    })
  )
  const firstDeviceReadyNextGuideDescription = computed(
    () => firstDevicePostReadyHandoff.value?.description || '首台设备已就绪，暂无待执行的下一步引导。'
  )
  const firstDeviceCoreGuideSummary = computed(() => buildFirstDeviceCoreGuideSummary(firstDeviceCoreGuideSteps.value))
  const firstRunSetupBlockerStep = computed(
    () => props.firstRunSetupBlockerStep || props.homeCustomerGuideProgress.find((step) => step.id === 'setup') || null
  )
  const firstRunSetupBlockerTitle = computed(() => firstRunSetupBlockerStep.value?.title || '先完成租户初始化')
  const firstRunSetupBlockerDescription = computed(
    () => firstRunSetupBlockerStep.value?.description || '当前存在未完成的初始化步骤，请先按引导完成租户与部署检查'
  )
  const firstRunSetupBlockerAction = computed(() => firstRunSetupBlockerStep.value?.action || '去处理初始化')

  const firstFailedDeploymentHealthRow = computed(() => props.deploymentHealthRows.find((row) => !row.ok) || null)
  const firstDeviceCurrentBlocker = computed(() => props.firstDeviceReadyProof.items?.find((item) => !item.ok) || null)
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
  const focusedCopy = computed(() =>
    buildFocusedQuickstartCopy({
      ready: props.firstDeviceReadyProof.ready,
      activeStep: currentFocusedQuickstartStep.value,
      readyDescription: firstDeviceReadyNextGuideDescription.value,
      guardSummary: props.firstDeviceOnboardingGuard.summary,
      nextAction: props.firstDeviceOnboardingGuard.nextAction,
      postReadyHandoff: firstDevicePostReadyHandoff.value
    })
  )
  const cueInput = computed(() => ({
    ready: props.firstDeviceReadyProof.ready,
    activeStep: currentFocusedQuickstartStep.value,
    actionLabel: focusedCopy.value.actionLabel,
    successSignal: focusedCopy.value.successSignal,
    readyDescription: firstDeviceReadyNextGuideDescription.value,
    currentBlocker: firstDeviceCurrentBlocker.value
  }))
  const firstDeviceOperatorCue = computed(() => buildFirstDeviceOperatorCue(cueInput.value))
  const firstDeviceMissionControl = computed(() => buildFirstDeviceMissionControl(cueInput.value))
  const currentFocusedQuickstartActionLoading = computed(() =>
    getFocusedQuickstartActionLoading({
      ready: props.firstDeviceReadyProof.ready,
      activeStep: currentFocusedQuickstartStep.value,
      firstDeviceActionLoading: props.firstDeviceActionLoading,
      deploymentHealthLoading: props.deploymentHealthLoading,
      firstRunCreateLoading: props.firstRunCreateLoading
    })
  )
  const statusHeroCopy = computed(() =>
    buildFirstDeviceStatusHeroCopy({
      ready: props.firstDeviceReadyProof.ready,
      currentBlocker: firstDeviceCurrentBlocker.value,
      activeStep: currentFocusedQuickstartStep.value,
      guardSummary: props.firstDeviceOnboardingGuard.summary
    })
  )
  const successProofCopy = computed(() =>
    buildFirstDeviceSuccessProofCopy({
      ready: props.firstDeviceReadyProof.ready,
      chartReady: props.firstDeviceChart.ready,
      testResult: props.firstDeviceTestResult
    })
  )
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

  const selectedFirstDeviceTestCommand = ref('')
  const firstDeviceTestCommands = computed(() =>
    buildFirstDeviceTestCommands({
      accessGuide: props.firstDeviceAccessGuide,
      publishCommand: props.firstDevicePublishCommand
    })
  )
  const activeFirstDeviceTestCommand = computed(
    () =>
      firstDeviceTestCommands.value.find((command) => command.language === selectedFirstDeviceTestCommand.value) ||
      firstDeviceTestCommands.value[0] ||
      null
  )
  watch(
    firstDeviceTestCommands,
    (commands) => {
      if (!commands.some((command) => command.language === selectedFirstDeviceTestCommand.value)) {
        selectedFirstDeviceTestCommand.value = commands[0]?.language || ''
      }
    },
    { immediate: true }
  )
  const activeTestCommandLabel = computed(() =>
    activeFirstDeviceTestCommand.value ? getFirstDeviceTestCommandLabel(activeFirstDeviceTestCommand.value) : ''
  )
  const firstDeviceOnlineTesterState = computed(() =>
    buildFirstDeviceOnlineTesterState({
      guard: props.firstDeviceOnboardingGuard,
      browserTest: props.firstDeviceBrowserTest,
      chart: props.firstDeviceChart,
      activeTestCommandLabel: activeTestCommandLabel.value
    })
  )
  const firstDeviceClosedLoopSteps = computed(() =>
    buildFirstDeviceClosedLoopSteps({
      flowNodes: firstDeviceFlowNodes.value,
      device: props.firstDevice,
      deploymentHealthOk: props.deploymentHealthOk,
      deploymentHealthLoading: props.deploymentHealthLoading,
      firstRunCreateTenantRequired: props.firstRunCreateTenantRequired,
      firstRunCreateLoading: props.firstRunCreateLoading,
      firstDeviceLoading: props.firstDeviceLoading,
      firstDeviceActionLoading: props.firstDeviceActionLoading,
      canCopyCommand: props.firstDeviceOnboardingGuard.canCopyCommand,
      canRunBrowserTest: props.firstDeviceOnboardingGuard.canRunBrowserTest,
      activeTestCommand: activeFirstDeviceTestCommand.value,
      browserTest: props.firstDeviceBrowserTest,
      chartReady: props.firstDeviceChart.ready,
      ready: props.firstDeviceReadyProof.ready,
      latestProofText: firstDeviceLatestProofText.value
    })
  )
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
  const buildFirstDeviceSupportSummaryForCopy = () =>
    props.buildFirstDeviceSupportSummary({
      latestProofText: firstDeviceLatestProofText.value,
      activeTestCommand: activeFirstDeviceTestCommand.value ? { label: activeTestCommandLabel.value } : null,
      delivery: buildFirstDeviceProofDelivery(firstDeviceProofDeliveryState.value, firstDeviceProofOrigin())
    })
  const downloadFirstDeviceSuccessProof = () =>
    downloadFirstDeviceSuccessProofPacket(
      buildFirstDeviceSuccessProofDeliveryPacket(firstDeviceProofDeliveryState.value, firstDeviceProofOrigin()),
      buildFirstDeviceProofFilename(props.firstDevice)
    )

  return {
    firstDeviceCoreGuideSteps,
    firstDeviceNextGuideSteps,
    firstDeviceNextActiveGuideStep,
    firstDevicePostReadyHandoff,
    firstDeviceReadyNextGuideDescription,
    firstDeviceCoreGuideSummary,
    firstRunSetupBlockerStep,
    firstRunSetupBlockerTitle,
    firstRunSetupBlockerDescription,
    firstRunSetupBlockerAction,
    firstFailedDeploymentHealthRow,
    firstDeviceCurrentBlocker,
    firstDeviceLatestProofText,
    firstDeviceFlowNodes,
    firstDeviceClosureSummary,
    firstRunWizardSteps,
    currentFocusedQuickstartStep,
    currentFocusedQuickstartSectionKey,
    focusedCopy,
    cueInput,
    firstDeviceOperatorCue,
    firstDeviceMissionControl,
    currentFocusedQuickstartActionLoading,
    statusHeroCopy,
    successProofCopy,
    firstDeviceSuccessFacts,
    firstDevicePostTestGuidance,
    firstDeviceVerificationAction,
    selectedFirstDeviceTestCommand,
    firstDeviceTestCommands,
    activeFirstDeviceTestCommand,
    activeTestCommandLabel,
    firstDeviceOnlineTesterState,
    firstDeviceClosedLoopSteps,
    firstDeviceOperationChecklist,
    firstDeviceProofOrigin,
    firstDeviceProofDeliveryState,
    buildFirstDeviceSupportSummaryForCopy,
    downloadFirstDeviceSuccessProof
  }
}
