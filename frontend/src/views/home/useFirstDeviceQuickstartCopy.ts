// 文件用途：首台设备工作台"聚焦快启动"文案与验证动作的纯派生视图模型（ROADMAP D1 拆分第一批）。
// 核心逻辑：从工作台输入切片推导聚焦步骤、区块键、标题/描述/操作标签、状态英雄区、成功证明文案等只读计算属性。
// 关键注意事项：纯派生无副作用，不持有 ref/emit；入参为响应式 computed 投影（由调用方基于 props 构建），
// 函数内部一律经 props.value.* 访问，保证随 props 变化联动。
import { computed, type ComputedRef } from 'vue'
import {
  buildFirstDeviceLatestProofText,
  buildFirstDevicePostReadyHandoff,
  buildFirstDevicePostTestGuidance,
  buildFirstDeviceSuccessFacts,
  buildFirstDeviceVerificationAction,
  resolveFirstDeviceFocusedSectionKey,
  type FirstDeviceChartState,
  type FirstDeviceOnboardingGuard,
  type FirstDeviceReadyProof,
  type FirstDeviceSummary
} from './homeFirstDeviceWorkbench'
import {
  buildFirstDeviceMissionControl,
  buildFirstDeviceOperatorCue,
  buildFirstDeviceStatusHeroCopy,
  buildFirstDeviceSuccessProofCopy,
  filterFirstDeviceCoreGuideSteps,
  filterFirstDeviceNextGuideSteps,
  buildFirstDeviceCoreGuideSummary,
  buildFocusedQuickstartCopy,
  getFocusedQuickstartActionLoading
} from './homeFirstDeviceView'
import type { HomeCustomerGuideProgressStep } from './homeCustomerGuide'
import type { NormalizedDeploymentHealthRow } from './homeDeploymentHealth'

/** 工作台 Props 中与本 composable 相关的切片。 */
export interface FirstDeviceQuickstartCopyInputs {
  homeCustomerGuideProgress: HomeCustomerGuideProgressStep[]
  firstRunSetupBlockerStep?: HomeCustomerGuideProgressStep | null
  deploymentHealthRows: NormalizedDeploymentHealthRow[]
  readyProof: FirstDeviceReadyProof
  onboardingGuard: FirstDeviceOnboardingGuard
  chart: FirstDeviceChartState
  testResult: string
  firstDevice: FirstDeviceSummary | null
  deploymentHealthOk: boolean
  deploymentHealthLoading: boolean
  firstDeviceActionLoading: boolean
  firstRunCreateLoading: boolean
}

export function useFirstDeviceQuickstartCopy(props: ComputedRef<FirstDeviceQuickstartCopyInputs>) {
  const firstDeviceCoreGuideSteps = computed(() => filterFirstDeviceCoreGuideSteps(props.value.homeCustomerGuideProgress))
  const firstDeviceNextGuideSteps = computed(() => filterFirstDeviceNextGuideSteps(props.value.homeCustomerGuideProgress))
  const firstDeviceNextActiveGuideStep = computed<HomeCustomerGuideProgressStep | null>(
    () =>
      (firstDeviceNextGuideSteps.value.find(step => step.status === 'active') as HomeCustomerGuideProgressStep) ||
      null
  )
  const firstDevicePostReadyHandoff = computed(() =>
    buildFirstDevicePostReadyHandoff({
      ready: props.value.readyProof.ready,
      nextStep: firstDeviceNextActiveGuideStep.value
    })
  )
  const firstDeviceReadyNextGuideDescription = computed(() => {
    return (
      firstDevicePostReadyHandoff.value?.description ||
      '首台设备已就绪，暂无待执行的下一步引导。'
    )
  })
  const firstDeviceCoreGuideSummary = computed(() => buildFirstDeviceCoreGuideSummary(firstDeviceCoreGuideSteps.value))
  const firstRunSetupBlockerStep = computed(
    () =>
      props.value.firstRunSetupBlockerStep ||
      props.value.homeCustomerGuideProgress.find(step => step.id === 'setup') ||
      null
  )
  const firstRunSetupBlockerTitle = computed(() => firstRunSetupBlockerStep.value?.title || '先完成租户初始化')
  const firstRunSetupBlockerDescription = computed(
    () => firstRunSetupBlockerStep.value?.description || '当前存在未完成的初始化步骤，请先按引导完成租户与部署检查',
  )
  const firstRunSetupBlockerAction = computed(() => firstRunSetupBlockerStep.value?.action || '去处理初始化')
  const firstFailedDeploymentHealthRow = computed(
    () => props.value.deploymentHealthRows.find((row) => !row.ok) || null
  )
  const firstDeviceCurrentBlocker = computed(
    () => props.value.readyProof.items?.find(item => !item.ok) || null
  )
  const firstDevicePrimaryAction = computed(
    () =>
      props.value.onboardingGuard.activeStep?.action ||
      (props.value.readyProof.ready ? 'ready-check' : 'health')
  )
  const firstDeviceLatestProofText = computed(() =>
    buildFirstDeviceLatestProofText({
      device: props.value.firstDevice,
      chart: props.value.chart,
      testResult: props.value.testResult
    })
  )
  const currentFocusedQuickstartStep = computed(() => props.value.onboardingGuard.activeStep || null)
  const currentFocusedQuickstartSectionKey = computed(() =>
    resolveFirstDeviceFocusedSectionKey({
      activeStep: currentFocusedQuickstartStep.value,
      ready: props.value.readyProof.ready,
      readyProofItems: props.value.readyProof.items || [],
      chartReady: props.value.chart.ready
    })
  )
  const currentFocusedQuickstartCopy = computed(() =>
    buildFocusedQuickstartCopy({
      ready: props.value.readyProof.ready,
      activeStep: currentFocusedQuickstartStep.value,
      readyDescription: firstDeviceReadyNextGuideDescription.value,
      guardSummary: props.value.onboardingGuard.summary,
      nextAction: props.value.onboardingGuard.nextAction,
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
      ready: props.value.readyProof.ready,
      activeStep: currentFocusedQuickstartStep.value,
      actionLabel: currentFocusedQuickstartActionLabel.value,
      successSignal: currentFocusedQuickstartSuccessSignal.value,
      readyDescription: firstDeviceReadyNextGuideDescription.value,
      currentBlocker: firstDeviceCurrentBlocker.value
    })
  )
  const firstDeviceMissionControl = computed(() =>
    buildFirstDeviceMissionControl({
      ready: props.value.readyProof.ready,
      activeStep: currentFocusedQuickstartStep.value,
      actionLabel: currentFocusedQuickstartActionLabel.value,
      successSignal: currentFocusedQuickstartSuccessSignal.value,
      readyDescription: firstDeviceReadyNextGuideDescription.value,
      currentBlocker: firstDeviceCurrentBlocker.value
    })
  )
  const currentFocusedQuickstartActionLoading = computed(() =>
    getFocusedQuickstartActionLoading({
      ready: props.value.readyProof.ready,
      activeStep: currentFocusedQuickstartStep.value,
      firstDeviceActionLoading: props.value.firstDeviceActionLoading,
      deploymentHealthLoading: props.value.deploymentHealthLoading,
      firstRunCreateLoading: props.value.firstRunCreateLoading
    })
  )
  const firstDeviceStatusHeroCopy = computed(() =>
    buildFirstDeviceStatusHeroCopy({
      ready: props.value.readyProof.ready,
      currentBlocker: firstDeviceCurrentBlocker.value,
      activeStep: currentFocusedQuickstartStep.value,
      guardSummary: props.value.onboardingGuard.summary
    })
  )
  const firstDeviceStatusHeroTitle = computed(() => firstDeviceStatusHeroCopy.value.title)
  const firstDeviceStatusHeroDescription = computed(() => firstDeviceStatusHeroCopy.value.description)
  const firstDeviceSuccessProofCopy = computed(() =>
    buildFirstDeviceSuccessProofCopy({
      ready: props.value.readyProof.ready,
      chartReady: props.value.chart.ready,
      testResult: props.value.testResult
    })
  )
  const firstDeviceSuccessProofTitle = computed(() => firstDeviceSuccessProofCopy.value.title)
  const firstDeviceSuccessProofDescription = computed(() => firstDeviceSuccessProofCopy.value.description)
  const firstDeviceSuccessFacts = computed(() =>
    buildFirstDeviceSuccessFacts({
      device: props.value.firstDevice,
      chart: props.value.chart,
      latestProofText: firstDeviceLatestProofText.value
    })
  )
  const firstDevicePostTestGuidance = computed(() =>
    buildFirstDevicePostTestGuidance({
      testResult: props.value.testResult,
      ready: props.value.readyProof.ready,
      readyDescription: firstDeviceReadyNextGuideDescription.value,
      chartReady: props.value.chart.ready,
      currentBlocker: firstDeviceCurrentBlocker.value
    })
  )
  const firstDeviceVerificationAction = computed(() =>
    buildFirstDeviceVerificationAction({
      hasDevice: Boolean(props.value.firstDevice),
      ready: props.value.readyProof.ready,
      postReadyHandoff: firstDevicePostReadyHandoff.value,
      readyDescription: firstDeviceReadyNextGuideDescription.value,
      chartReady: props.value.chart.ready,
      canRunBrowserTest: props.value.onboardingGuard.canRunBrowserTest,
      testResult: props.value.testResult,
      actionLoading: props.value.firstDeviceActionLoading,
      currentBlocker: firstDeviceCurrentBlocker.value
    })
  )

  return {
    firstDeviceCoreGuideSteps,
    firstDeviceCoreGuideSummary,
    firstDeviceNextGuideSteps,
    firstDeviceNextActiveGuideStep,
    firstDevicePostReadyHandoff,
    firstDeviceReadyNextGuideDescription,
    firstRunSetupBlockerStep,
    firstRunSetupBlockerTitle,
    firstRunSetupBlockerDescription,
    firstRunSetupBlockerAction,
    firstFailedDeploymentHealthRow,
    firstDeviceCurrentBlocker,
    firstDevicePrimaryAction,
    firstDeviceLatestProofText,
    currentFocusedQuickstartStep,
    currentFocusedQuickstartSectionKey,
    currentFocusedQuickstartCopy,
    currentFocusedQuickstartTitle,
    currentFocusedQuickstartDescription,
    currentFocusedQuickstartSuccessSignal,
    currentFocusedQuickstartActionLabel,
    currentFocusedQuickstartActionDisabled,
    currentFocusedQuickstartActionLoading,
    firstDeviceOperatorCue,
    firstDeviceMissionControl,
    firstDeviceStatusHeroCopy,
    firstDeviceStatusHeroTitle,
    firstDeviceStatusHeroDescription,
    firstDeviceSuccessProofCopy,
    firstDeviceSuccessProofTitle,
    firstDeviceSuccessProofDescription,
    firstDeviceSuccessFacts,
    firstDevicePostTestGuidance,
    firstDeviceVerificationAction
  }
}
