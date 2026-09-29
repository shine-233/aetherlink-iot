<!--
文件用途：首页「首台设备」工作台编排层。
核心逻辑：派生状态来自 homeFirstDevice* 纯函数；按钮动作经 homeFirstDeviceWorkbenchActions 解析为意图后由 dispatch 统一执行；
  区块定位与延迟挂载在 useFirstDeviceSectionFocus。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'
import { buildFirstDeviceChartProofSummary, type FirstDeviceFlowNode } from './homeFirstDeviceWorkbench'
import type { HomeFirstRunProtocol } from './homeFirstRunWizard'
import type { HomeCustomerGuideProgressStep } from './homeCustomerGuide'
import { useFirstDeviceSectionFocus } from './useFirstDeviceSectionFocus'
import { useFirstDeviceWorkbenchState, type HomeFirstDeviceWorkbenchProps } from './useFirstDeviceWorkbenchState'
import {
  resolveClosedLoopStepIntent,
  resolveFlowNodeAction,
  resolveFocusedQuickstartIntent,
  resolvePrimaryIntent,
  resolveVerificationIntent,
  resolveVerificationSecondaryIntent,
  type FirstDeviceActionContext,
  type FirstDeviceIntent,
  type FirstDeviceWorkbenchEmitEvent
} from './homeFirstDeviceWorkbenchActions'
import { buildFirstDeviceConnectionSummary, type FirstDeviceClosedLoopStep } from './homeFirstDeviceClosedLoop'

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

const props = defineProps<HomeFirstDeviceWorkbenchProps>()
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
const {
  deviceIdentitySectionRef,
  quickstartSectionRef,
  deploymentHealthSectionRef,
  setConnectionTestViewportRef,
  setConnectionTestSectionRef,
  setSuccessProofViewportRef,
  setSuccessProofSectionRef,
  setSupportSummaryViewportRef,
  setSupportSummarySectionRef,
  shouldMountConnectionTestSection,
  shouldMountSuccessProofSection,
  shouldMountSupportSummarySection,
  focusSection,
  focusDeploymentHealth,
  openSupportSummaryPreview: openFirstDeviceSupportSummaryPreview
} = useFirstDeviceSectionFocus()

const {
  firstDeviceCoreGuideSteps,
  firstDeviceNextGuideSteps,
  firstDeviceNextActiveGuideStep,
  firstDevicePostReadyHandoff,
  firstDeviceCoreGuideSummary,
  firstRunSetupBlockerStep,
  firstRunSetupBlockerTitle,
  firstRunSetupBlockerDescription,
  firstRunSetupBlockerAction,
  firstDeviceLatestProofText,
  firstDeviceFlowNodes,
  firstDeviceClosureSummary,
  firstRunWizardSteps,
  currentFocusedQuickstartStep,
  currentFocusedQuickstartSectionKey,
  focusedCopy,
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
  firstDeviceOnlineTesterState,
  firstDeviceClosedLoopSteps,
  firstDeviceOperationChecklist,
  buildFirstDeviceSupportSummaryForCopy,
  downloadFirstDeviceSuccessProof
} = useFirstDeviceWorkbenchState(props)

const copyWithFeedback = async (text: string | undefined) => {
  if (!text) return
  const copied = await writeClipboardText(text)
  if (copied) window.$message?.success($t('theme.configOperation.copySuccess'))
  else window.$message?.error($t('common.copyFailed'))
}
const copyFirstDeviceConnectionSummary = () =>
  copyWithFeedback(
    buildFirstDeviceConnectionSummary({
      device: props.firstDevice,
      accessGuide: props.firstDeviceAccessGuide,
      simulation: props.firstDeviceSimulation,
      command: activeFirstDeviceTestCommand.value?.code || props.firstDevicePublishCommand || ''
    })
  )
const copyActiveFirstDeviceTestCommand = () => copyWithFeedback(activeFirstDeviceTestCommand.value?.code)
const copyFirstDeviceChartProof = () =>
  copyWithFeedback(
    buildFirstDeviceChartProofSummary({
      device: props.firstDevice,
      chart: props.firstDeviceChart,
      readyProof: props.firstDeviceReadyProof
    })
  )

// ---- 动作分发：所有按钮先解析为意图（homeFirstDeviceWorkbenchActions），再由这里统一执行 ----
const actionContext = computed<FirstDeviceActionContext>(() => ({
  ready: props.firstDeviceReadyProof.ready,
  hasDevice: Boolean(props.firstDevice),
  deploymentHealthOk: props.deploymentHealthOk,
  deploymentHealthLoading: props.deploymentHealthLoading,
  firstRunCreateTenantRequired: props.firstRunCreateTenantRequired,
  firstRunCreateLoading: props.firstRunCreateLoading,
  firstDeviceActionLoading: props.firstDeviceActionLoading,
  canCopyCommand: props.firstDeviceOnboardingGuard.canCopyCommand,
  canRunBrowserTest: props.firstDeviceOnboardingGuard.canRunBrowserTest,
  hasActiveTestCommand: Boolean(activeFirstDeviceTestCommand.value?.code),
  chartReady: props.firstDeviceChart.ready,
  hasNextGuideStep: Boolean(firstDeviceNextActiveGuideStep.value),
  postReadyAction: firstDevicePostReadyHandoff.value?.action,
  primaryQuickstartAction:
    props.firstDeviceOnboardingGuard.activeStep?.action ||
    (props.firstDeviceReadyProof.ready ? 'ready-check' : 'health'),
  activeQuickstartAction: currentFocusedQuickstartStep.value?.action
}))

const dispatch = (intent: FirstDeviceIntent): void => {
  switch (intent.kind) {
    case 'emit':
      ;(emit as (event: FirstDeviceWorkbenchEmitEvent) => void)(intent.event)
      return
    case 'openNextGuideStep':
      if (firstDeviceNextActiveGuideStep.value) emit('openHomeGuideStep', firstDeviceNextActiveGuideStep.value)
      return
    case 'quickstart':
      emit('runFirstDeviceQuickstartAction', intent.action)
      return
    case 'focus':
      void focusSection(intent.section)
      return
    case 'copyTestCommand':
      void copyActiveFirstDeviceTestCommand()
      return
    case 'copyChartProof':
      void copyFirstDeviceChartProof()
      return
    case 'downloadProof':
      downloadFirstDeviceSuccessProof()
      return
    case 'primary':
      dispatch(resolvePrimaryIntent(actionContext.value))
  }
}

const getFirstDeviceFlowNodeAction = (node: FirstDeviceFlowNode) => {
  const { intent, ...action } = resolveFlowNodeAction(node, actionContext.value)
  return { ...action, run: () => dispatch(intent) }
}
const runFirstDeviceClosedLoopStep = (step: FirstDeviceClosedLoopStep) =>
  dispatch(resolveClosedLoopStepIntent(step, actionContext.value))
const runCurrentFocusedQuickstartAction = () => dispatch(resolveFocusedQuickstartIntent(actionContext.value))
const focusCurrentFocusedQuickstartSection = () => void focusSection(currentFocusedQuickstartSectionKey.value)
const runFirstDeviceVerificationAction = () =>
  dispatch(resolveVerificationIntent(firstDeviceVerificationAction.value, actionContext.value))
const runFirstDeviceVerificationSecondaryAction = () =>
  dispatch(resolveVerificationSecondaryIntent(firstDeviceVerificationAction.value))

defineExpose({ focusDeploymentHealth, focusSection })
</script>

<template>
  <HomeFirstDeviceClosedLoopStrip :steps="firstDeviceClosedLoopSteps" @run-step="runFirstDeviceClosedLoopStep" />

  <div
    class="grid gap-16px"
    :class="firstDeviceFocusMode ? 'lg:grid-cols-1' : 'lg:grid-cols-[minmax(0,1.5fr)_minmax(320px,0.8fr)]'"
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
          :status-hero-title="statusHeroCopy.title"
          :status-hero-description="statusHeroCopy.description"
          :latest-proof-text="firstDeviceLatestProofText"
          :operator-cue="firstDeviceOperatorCue"
          :mission-control="firstDeviceMissionControl"
          :closure-summary="firstDeviceClosureSummary"
          :verification-action="firstDeviceVerificationAction"
          :focused-action-disabled="focusedCopy.actionDisabled"
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
          @focus-first-device-section="focusSection"
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
            :title="focusedCopy.title"
            :description="focusedCopy.description"
            :success-signal="focusedCopy.successSignal"
            :current-step="currentFocusedQuickstartStep"
            :ready="firstDeviceReadyProof.ready"
            :action-label="focusedCopy.actionLabel"
            :action-disabled="focusedCopy.actionDisabled"
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
          :first-device-success-proof-title="successProofCopy.title"
          :first-device-success-proof-description="successProofCopy.description"
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
