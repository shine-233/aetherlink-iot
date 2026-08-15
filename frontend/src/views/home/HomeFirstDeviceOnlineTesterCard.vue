<script setup lang="ts">
import { $t } from '@/locales'
import { getFirstDeviceTestCommandLabel } from './homeFirstDeviceView'

defineProps<{
  selectedTestCommand: string
  testCommands: any[]
  activeTestCommand: any
  firstDevicePublishCommand: string
  firstDeviceOnboardingGuard: any
  firstDeviceActionLoading: boolean
  firstDeviceOnlineTesterState: any
  firstDeviceTestResult: string
  firstDevicePostTestGuidance: any
  firstDeviceReadyProof: any
  firstDeviceNextActiveGuideStep: any
}>()

const emit = defineEmits<{
  'update:selectedTestCommand': [value: string]
  copyActiveFirstDeviceTestCommand: []
  copyFirstDevicePublishCommand: []
  simulateFirstDeviceTelemetry: []
  openFirstDeviceFullGuide: []
  openFirstDeviceAccessGuide: []
  openHomeGuideStep: [step: any]
  focusProof: []
  openFirstDeviceSupportSummaryPreview: []
}>()
</script>

<template>
  <div class="flex flex-col gap-8px sm:flex-row sm:items-start sm:justify-between">
    <div class="min-w-0">
      <div class="font-600">{{ $t('custom.home.firstDevice.tester.title') }}</div>
      <div class="mt-4px text-12px line-height-18px text-gray-500">
        {{ $t('custom.home.firstDevice.tester.desc') }}
      </div>
    </div>
    <n-button
      size="tiny"
      :disabled="!activeTestCommand?.code || !firstDeviceOnboardingGuard.canCopyCommand"
      @click="emit('copyActiveFirstDeviceTestCommand')"
    >
      {{ $t('custom.home.firstDevice.tester.copyCurrentCommand') }}
    </n-button>
  </div>
  <n-tabs
    v-if="testCommands.length"
    :value="selectedTestCommand"
    type="segment"
    size="small"
    class="mt-10px"
    @update:value="emit('update:selectedTestCommand', String($event))"
  >
    <n-tab-pane
      v-for="command in testCommands"
      :key="`${command.language}-${command.titleKey}`"
      :name="command.language"
      :tab="getFirstDeviceTestCommandLabel(command)"
    >
      <pre class="first-device-command">{{ command.code }}</pre>
    </n-tab-pane>
  </n-tabs>
  <pre v-else class="first-device-command">{{ firstDevicePublishCommand || $t('custom.home.firstDevice.tester.loadingParams') }}</pre>
  <div class="first-device-online-tester">
    <div class="flex flex-col gap-8px sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-8px">
          <strong>{{ firstDeviceOnlineTesterState.title }}</strong>
          <n-tag
            size="small"
            round
            :bordered="false"
            :type="firstDeviceOnlineTesterState.type"
          >
            {{ firstDeviceOnlineTesterState.statusLabel }}
          </n-tag>
        </div>
        <small>{{ firstDeviceOnlineTesterState.description }}</small>
      </div>
      <div class="first-device-online-tester-actions">
        <n-button
          size="small"
          :disabled="!firstDeviceOnboardingGuard.canCopyCommand"
          @click="emit('copyFirstDevicePublishCommand')"
        >
          {{ $t('custom.home.firstDevice.tester.copyCommand') }}
        </n-button>
        <n-button
          size="small"
          type="primary"
          :loading="firstDeviceActionLoading"
          :disabled="!firstDeviceOnboardingGuard.canRunBrowserTest"
          @click="emit('simulateFirstDeviceTelemetry')"
        >
          {{ firstDeviceOnlineTesterState.actionLabel }}
        </n-button>
        <n-button size="small" secondary @click="emit('openFirstDeviceFullGuide')">
          {{ $t('custom.devicePage.openAccessGuide') }}
        </n-button>
        <n-button size="small" @click="emit('openFirstDeviceAccessGuide')">
          {{ $t('custom.home.firstDevice.common.openReadyCheck') }}
        </n-button>
      </div>
    </div>
    <div class="first-device-online-tester-grid">
      <div v-for="row in firstDeviceOnlineTesterState.echoRows" :key="row.label">
        <span>{{ row.label }}</span>
        <strong>{{ row.value }}</strong>
      </div>
    </div>
    <div
      v-if="firstDeviceOnlineTesterState.disabledReason && !firstDeviceOnboardingGuard.canRunBrowserTest"
      class="first-device-online-tester-blocker"
    >
      {{ firstDeviceOnlineTesterState.disabledReason }}
    </div>
    <div v-else class="first-device-online-tester-signal">
      {{ firstDeviceOnlineTesterState.lastSignal }}
    </div>
  </div>
  <div v-if="firstDeviceTestResult" class="mt-8px text-12px text-gray-500">
    {{ firstDeviceTestResult }}
  </div>
  <div
    v-if="firstDevicePostTestGuidance"
    class="first-device-post-test"
    :class="`first-device-post-test--${firstDevicePostTestGuidance.type}`"
  >
    <div class="min-w-0">
      <strong>{{ firstDevicePostTestGuidance.title }}</strong>
      <small>{{ firstDevicePostTestGuidance.detail }}</small>
    </div>
    <div class="first-device-post-test-actions">
      <template v-if="firstDeviceReadyProof.ready">
        <n-button
          size="tiny"
          type="primary"
          ghost
          @click="
            firstDeviceNextActiveGuideStep
              ? emit('openHomeGuideStep', firstDeviceNextActiveGuideStep)
              : emit('openFirstDeviceFullGuide')
          "
        >
          {{ firstDeviceNextActiveGuideStep?.action || $t('custom.home.firstDevice.tester.viewFullGuide') }}
        </n-button>
      </template>
      <template v-else>
        <n-button size="tiny" type="primary" ghost @click="emit('openFirstDeviceAccessGuide')">
          {{ $t('custom.home.firstDevice.common.openReadyCheck') }}
        </n-button>
        <n-button size="tiny" ghost @click="emit('focusProof')">
          {{ $t('custom.home.firstDevice.tester.viewProofItems') }}
        </n-button>
        <n-button size="tiny" text @click="emit('openFirstDeviceSupportSummaryPreview')">
          {{ $t('custom.home.firstDevice.tester.copySupportSummary') }}
        </n-button>
      </template>
    </div>
  </div>
</template>

<style scoped>
.first-device-post-test {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: flex-start;
  justify-content: space-between;
  margin-top: 10px;
  padding: 10px;
  border: 1px solid #fed7aa;
  border-radius: 6px;
  background: #fff7ed;
}

.first-device-post-test--success {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.first-device-post-test strong {
  display: block;
  color: #0f172a;
  font-size: 13px;
}

.first-device-post-test small {
  display: block;
  margin-top: 3px;
  color: #475569;
  font-size: 12px;
  line-height: 18px;
}

.first-device-post-test-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  justify-content: flex-end;
}

.first-device-online-tester {
  display: grid;
  gap: 10px;
  margin-top: 10px;
  padding: 10px;
  border: 1px solid #bfdbfe;
  border-radius: 6px;
  background: #eff6ff;
}

.first-device-online-tester small {
  display: block;
  margin-top: 4px;
  color: #475569;
  font-size: 12px;
  line-height: 18px;
}

.first-device-online-tester-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.first-device-online-tester-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.first-device-online-tester-grid > div {
  min-width: 0;
  padding: 8px;
  border: 1px solid #dbeafe;
  border-radius: 6px;
  background: #fff;
}

.first-device-online-tester-grid span {
  display: block;
  color: #64748b;
  font-size: 11px;
}

.first-device-online-tester-grid strong {
  display: block;
  margin-top: 3px;
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 12px;
}

.first-device-online-tester-blocker,
.first-device-online-tester-signal {
  color: #1d4ed8;
  font-size: 12px;
  line-height: 18px;
}

.first-device-online-tester-blocker {
  color: #c2410c;
}

.first-device-command {
  margin: 8px 0 0;
  max-height: 96px;
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .first-device-online-tester-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
