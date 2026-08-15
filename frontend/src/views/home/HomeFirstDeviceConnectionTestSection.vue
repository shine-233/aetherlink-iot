<script setup lang="ts">
import { ref } from 'vue'
import { $t } from '@/locales'
import HomeFirstDeviceConnectionParametersCard from './HomeFirstDeviceConnectionParametersCard.vue'
import HomeFirstDeviceOnlineTesterCard from './HomeFirstDeviceOnlineTesterCard.vue'

defineProps<{
  firstDevice: any
  firstDeviceAccessGuide: any
  firstDeviceSimulation: any
  firstDeviceOnboardingGuard: any
  operationChecklist: any[]
  selectedTestCommand: string
  testCommands: any[]
  activeTestCommand: any
  firstDevicePublishCommand: string
  firstDeviceActionLoading: boolean
  firstDeviceOnlineTesterState: any
  firstDeviceTestResult: string
  firstDevicePostTestGuidance: any
  firstDeviceReadyProof: any
  firstDeviceNextActiveGuideStep: any
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
  openFirstDeviceSupportSummaryPreview: []
}>()

const connectionEl = ref<HTMLElement | null>(null)
const testCommandEl = ref<HTMLElement | null>(null)

defineExpose({
  connectionEl,
  testCommandEl
})
</script>

<template>
  <div class="rounded-6px bg-gray-50 px-12px py-10px">
    <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.test.stepRange') }}</div>
    <div class="mt-2px font-600">{{ $t('custom.home.firstDevice.test.title') }}</div>
    <div class="mt-4px text-12px line-height-18px text-gray-500">
      {{ $t('custom.home.firstDevice.test.desc') }}
    </div>

    <div class="mt-10px first-device-operations-grid">
      <div ref="connectionEl" class="first-device-operation-section">
        <HomeFirstDeviceConnectionParametersCard
          :first-device="firstDevice"
          :first-device-access-guide="firstDeviceAccessGuide"
          :first-device-simulation="firstDeviceSimulation"
          :first-device-onboarding-guard="firstDeviceOnboardingGuard"
          :operation-checklist="operationChecklist"
          @copy-connection-summary="emit('copyConnectionSummary')"
        />
      </div>

      <div ref="testCommandEl" class="first-device-operation-section">
        <HomeFirstDeviceOnlineTesterCard
          :selected-test-command="selectedTestCommand"
          :test-commands="testCommands"
          :active-test-command="activeTestCommand"
          :first-device-publish-command="firstDevicePublishCommand"
          :first-device-onboarding-guard="firstDeviceOnboardingGuard"
          :first-device-action-loading="firstDeviceActionLoading"
          :first-device-online-tester-state="firstDeviceOnlineTesterState"
          :first-device-test-result="firstDeviceTestResult"
          :first-device-post-test-guidance="firstDevicePostTestGuidance"
          :first-device-ready-proof="firstDeviceReadyProof"
          :first-device-next-active-guide-step="firstDeviceNextActiveGuideStep"
          @update:selected-test-command="emit('update:selectedTestCommand', $event)"
          @copy-active-first-device-test-command="emit('copyActiveFirstDeviceTestCommand')"
          @copy-first-device-publish-command="emit('copyFirstDevicePublishCommand')"
          @simulate-first-device-telemetry="emit('simulateFirstDeviceTelemetry')"
          @open-first-device-full-guide="emit('openFirstDeviceFullGuide')"
          @open-first-device-access-guide="emit('openFirstDeviceAccessGuide')"
          @open-home-guide-step="emit('openHomeGuideStep', $event)"
          @focus-proof="emit('focusProof')"
          @open-first-device-support-summary-preview="emit('openFirstDeviceSupportSummaryPreview')"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.first-device-operations-grid {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
  gap: 12px;
}

.first-device-operation-section {
  min-width: 0;
  padding: 12px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}

@media (max-width: 900px) {
  .first-device-operations-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
