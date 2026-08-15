<script setup lang="ts">
import { defineAsyncComponent, type ComponentPublicInstance } from 'vue'
import { $t } from '@/locales'

const HomeFirstDeviceConnectionTestSection = defineAsyncComponent(
  () => import('./HomeFirstDeviceConnectionTestSection.vue')
)
const HomeFirstDeviceSuccessProofSection = defineAsyncComponent(
  () => import('./HomeFirstDeviceSuccessProofSection.vue')
)
const FirstDeviceSupportSummarySection = defineAsyncComponent(() => import('./FirstDeviceSupportSummarySection.vue'))

const props = defineProps<{
  setConnectionTestViewportRef: (element: HTMLElement | null) => void
  setConnectionTestSectionRef: (instance: any) => void
  shouldMountConnectionTestSection: boolean
  selectedTestCommand: string
  firstDevice: any
  firstDeviceAccessGuide: any
  firstDeviceSimulation: any
  firstDeviceOnboardingGuard: any
  operationChecklist: any[]
  testCommands: any[]
  activeTestCommand: any
  firstDevicePublishCommand: string
  firstDeviceActionLoading: boolean
  firstDeviceOnlineTesterState: any
  firstDeviceTestResult: string
  firstDevicePostTestGuidance: any
  firstDeviceReadyProof: any
  firstDeviceNextActiveGuideStep: any
  setSuccessProofViewportRef: (element: HTMLElement | null) => void
  setSuccessProofSectionRef: (instance: any) => void
  shouldMountSuccessProofSection: boolean
  firstDeviceSuccessProofTitle: string
  firstDeviceSuccessProofDescription: string
  firstDeviceSuccessFacts: any[]
  firstDeviceChart: any
  setSupportSummaryViewportRef: (element: HTMLElement | null) => void
  setSupportSummarySectionRef: (instance: any) => void
  shouldMountSupportSummarySection: boolean
  buildFirstDeviceSupportSummaryForCopy: () => string
}>()

const emit = defineEmits<{
  'update:selectedTestCommand': [value: string]
  copyConnectionSummary: []
  copyActiveFirstDeviceTestCommand: []
  copyFirstDevicePublishCommand: []
  simulateFirstDeviceTelemetry: []
  openFirstDeviceFullGuide: []
  openFirstDeviceAccessGuide: []
  openHomeGuideStep: [step: any]
  focusProof: []
  focusConnection: []
  openSupportSummaryPreview: []
  copyChartProof: []
  downloadSuccessProof: []
}>()

const bindConnectionViewportRef = (element: Element | ComponentPublicInstance | null) => {
  props.setConnectionTestViewportRef(element instanceof HTMLElement ? element : null)
}

const bindConnectionSectionRef = (instance: any) => {
  props.setConnectionTestSectionRef(instance)
}

const bindSuccessProofViewportRef = (element: Element | ComponentPublicInstance | null) => {
  props.setSuccessProofViewportRef(element instanceof HTMLElement ? element : null)
}

const bindSuccessProofSectionRef = (instance: any) => {
  props.setSuccessProofSectionRef(instance)
}

const bindSupportSummaryViewportRef = (element: Element | ComponentPublicInstance | null) => {
  props.setSupportSummaryViewportRef(element instanceof HTMLElement ? element : null)
}

const bindSupportSummarySectionRef = (instance: any) => {
  props.setSupportSummarySectionRef(instance)
}
</script>

<template>
  <template v-if="firstDevice">
    <div :ref="bindConnectionViewportRef">
      <HomeFirstDeviceConnectionTestSection
        v-if="shouldMountConnectionTestSection"
        :ref="bindConnectionSectionRef"
        :selected-test-command="selectedTestCommand"
        :first-device="firstDevice"
        :first-device-access-guide="firstDeviceAccessGuide"
        :first-device-simulation="firstDeviceSimulation"
        :first-device-onboarding-guard="firstDeviceOnboardingGuard"
        :operation-checklist="operationChecklist"
        :test-commands="testCommands"
        :active-test-command="activeTestCommand"
        :first-device-publish-command="firstDevicePublishCommand"
        :first-device-action-loading="firstDeviceActionLoading"
        :first-device-online-tester-state="firstDeviceOnlineTesterState"
        :first-device-test-result="firstDeviceTestResult"
        :first-device-post-test-guidance="firstDevicePostTestGuidance"
        :first-device-ready-proof="firstDeviceReadyProof"
        :first-device-next-active-guide-step="firstDeviceNextActiveGuideStep"
        @update:selected-test-command="emit('update:selectedTestCommand', $event)"
        @copy-connection-summary="emit('copyConnectionSummary')"
        @copy-active-first-device-test-command="emit('copyActiveFirstDeviceTestCommand')"
        @copy-first-device-publish-command="emit('copyFirstDevicePublishCommand')"
        @simulate-first-device-telemetry="emit('simulateFirstDeviceTelemetry')"
        @open-first-device-full-guide="emit('openFirstDeviceFullGuide')"
        @open-first-device-access-guide="emit('openFirstDeviceAccessGuide')"
        @open-home-guide-step="emit('openHomeGuideStep', $event)"
        @focus-proof="emit('focusProof')"
        @open-first-device-support-summary-preview="emit('openSupportSummaryPreview')"
      />
      <div v-else class="first-device-deferred-placeholder">
        <div class="min-w-0">
          <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.deferred.connectionTitle') }}</div>
          <div class="mt-2px font-600">{{ $t('custom.home.firstDevice.deferred.connectionHeading') }}</div>
          <div class="mt-4px text-12px line-height-18px text-gray-500">
            {{ $t('custom.home.firstDevice.deferred.connectionDesc') }}
          </div>
        </div>
        <n-button size="tiny" type="primary" ghost @click="emit('focusConnection')">
          {{ $t('custom.home.firstDevice.deferred.loadConnection') }}
        </n-button>
      </div>
    </div>

    <div :ref="bindSuccessProofViewportRef">
      <HomeFirstDeviceSuccessProofSection
        v-if="shouldMountSuccessProofSection"
        :ref="bindSuccessProofSectionRef"
        :title="firstDeviceSuccessProofTitle"
        :description="firstDeviceSuccessProofDescription"
        :ready="firstDeviceReadyProof.ready"
        :facts="firstDeviceSuccessFacts"
        :chart="firstDeviceChart"
        :ready-proof="firstDeviceReadyProof"
        @copy-chart-proof="emit('copyChartProof')"
        @download-success-proof="emit('downloadSuccessProof')"
      />
      <div v-else class="first-device-deferred-placeholder">
        <div class="min-w-0">
          <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.deferred.proofTitle') }}</div>
          <div class="mt-2px font-600">{{ $t('custom.home.firstDevice.deferred.proofHeading') }}</div>
          <div class="mt-4px text-12px line-height-18px text-gray-500">
            {{ $t('custom.home.firstDevice.deferred.proofDesc') }}
          </div>
        </div>
        <n-button size="tiny" type="primary" ghost @click="emit('focusProof')">
          {{ $t('custom.home.firstDevice.deferred.loadProof') }}
        </n-button>
      </div>
    </div>
  </template>

  <div :ref="bindSupportSummaryViewportRef">
    <FirstDeviceSupportSummarySection
      v-if="shouldMountSupportSummarySection"
      :ref="bindSupportSummarySectionRef"
      :get-summary="buildFirstDeviceSupportSummaryForCopy"
    />
    <div v-else class="first-device-support-summary-placeholder">
      <div class="min-w-0">
        <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.common.stuckWhen') }}</div>
        <div class="mt-2px font-600">{{ $t('custom.home.firstDevice.deferred.supportHeading') }}</div>
        <div class="mt-4px text-12px line-height-18px text-gray-500">
          {{ $t('custom.home.firstDevice.deferred.supportDesc') }}
        </div>
      </div>
      <n-button size="tiny" type="primary" ghost @click="emit('openSupportSummaryPreview')">
        {{ $t('custom.home.firstDevice.common.previewCopySupportSummary') }}
      </n-button>
    </div>
  </div>
</template>

<style scoped>
.first-device-deferred-placeholder,
.first-device-support-summary-placeholder {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: flex-start;
  justify-content: space-between;
  padding: 12px;
  border: 1px dashed #cbd5e1;
  border-radius: 6px;
  background: #f8fafc;
}
</style>
