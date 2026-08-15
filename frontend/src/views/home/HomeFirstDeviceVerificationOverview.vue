<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { $t } from '@/locales'

const FirstRunWizardSection = defineAsyncComponent(() => import('./FirstRunWizardSection.vue'))
const HomeFirstDeviceFlowCanvas = defineAsyncComponent(() => import('./HomeFirstDeviceFlowCanvas.vue'))

defineProps<{
  ready: boolean
  firstDeviceLoading: boolean
  statusHeroTitle: string
  statusHeroDescription: string
  latestProofText: string
  operatorCue: any
  missionControl: any
  closureSummary: any
  verificationAction: any
  focusedActionDisabled: boolean
  focusedActionLoading: boolean
  flowNodes: any[]
  wizardSteps: any[]
  getFlowNodeAction: (node: any) => { label: string; disabled: boolean; loading: boolean; run: () => void }
}>()

const emit = defineEmits<{
  refreshFirstDeviceWorkbench: []
  runVerificationAction: []
  runVerificationSecondaryAction: []
  runCurrentFocusedQuickstartAction: []
  focusCurrentFocusedQuickstartSection: []
  openFirstDeviceSupportSummaryPreview: []
  downloadSuccessProof: []
  openHomeGuideStep: [step: any]
  focusFirstDeviceSection: [key: string]
}>()
</script>

<template>
  <div class="flex items-center justify-between gap-8px">
    <div class="text-16px font-600">{{ $t('custom.home.firstDevice.overview.title') }}</div>
    <n-button size="small" :loading="firstDeviceLoading" @click="emit('refreshFirstDeviceWorkbench')">
      {{ $t('custom.home.refresh') }}
    </n-button>
  </div>

  <div class="first-device-action-hero">
    <div class="min-w-0">
      <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.overview.currentStatus') }}</div>
      <div class="mt-4px text-16px font-700">{{ statusHeroTitle }}</div>
      <div class="mt-6px text-12px line-height-18px text-gray-600">
        {{ statusHeroDescription }}
      </div>
      <div class="mt-8px first-device-latest-proof">
        <span>{{ $t('custom.home.firstDevice.overview.latestProof') }}</span>
        <strong>{{ latestProofText }}</strong>
      </div>
      <div
        class="mt-10px first-device-operator-cue"
        :class="`first-device-operator-cue--${operatorCue.type}`"
      >
        <span>{{ $t('custom.home.firstDevice.overview.doOneThing') }}</span>
        <strong>{{ operatorCue.title }}</strong>
        <small>{{ operatorCue.detail }}</small>
        <small class="first-device-operator-cue__signal">{{ operatorCue.successSignal }}</small>
      </div>
      <div
        class="mt-10px first-device-mission-control"
        :class="`first-device-mission-control--${missionControl.type}`"
      >
        <div class="first-device-mission-control__item">
          <span>{{ $t('custom.home.firstDevice.overview.currentStage') }}</span>
          <strong>{{ missionControl.currentStateLabel }}</strong>
        </div>
        <div class="first-device-mission-control__item">
          <span>{{ $t('custom.home.firstDevice.overview.clickThis') }}</span>
          <strong>{{ missionControl.nextClickLabel }}</strong>
        </div>
        <div class="first-device-mission-control__item first-device-mission-control__item--wide">
          <span>{{ $t('custom.home.firstDevice.overview.why') }}</span>
          <small>{{ missionControl.whyThisClick }}</small>
        </div>
        <div class="first-device-mission-control__item first-device-mission-control__item--wide">
          <span>{{ $t('custom.home.firstDevice.common.finishCriteria') }}</span>
          <small>{{ missionControl.finishSignal }}</small>
        </div>
        <div class="first-device-mission-control__item first-device-mission-control__item--wide">
          <span>{{ $t('custom.home.firstDevice.common.stuckWhen') }}</span>
          <small>{{ missionControl.stuckHint }}</small>
        </div>
      </div>
    </div>
    <div class="flex shrink-0 flex-col gap-8px">
      <n-tag size="small" round :bordered="false" :type="ready ? 'success' : 'warning'">
        {{
          ready
            ? $t('custom.home.firstDevice.common.loopConfirmed')
            : $t('custom.home.firstDevice.common.keepWatchingWorkspace')
        }}
      </n-tag>
      <n-button
        v-if="verificationAction"
        size="small"
        type="primary"
        :loading="verificationAction.loading"
        :disabled="verificationAction.disabled"
        @click="emit('runVerificationAction')"
      >
        {{ $t('custom.home.firstDevice.common.nextStepPrefix', { label: missionControl.nextClickLabel }) }}
      </n-button>
      <n-button
        v-else
        size="small"
        type="primary"
        :disabled="focusedActionDisabled"
        :loading="focusedActionLoading"
        @click="emit('runCurrentFocusedQuickstartAction')"
      >
        {{ $t('custom.home.firstDevice.common.nextStepPrefix', { label: missionControl.nextClickLabel }) }}
      </n-button>
      <small class="first-device-primary-signal">{{ missionControl.finishSignal }}</small>
      <n-button size="small" secondary @click="emit('focusCurrentFocusedQuickstartSection')">
        {{ $t('custom.home.firstDevice.overview.locateWorkspace') }}
      </n-button>
      <n-button size="small" tertiary @click="emit('openFirstDeviceSupportSummaryPreview')">
        {{ missionControl.supportLabel }}
      </n-button>
    </div>
  </div>

  <div class="first-device-delivery-lanes">
    <div class="first-device-delivery-lane first-device-delivery-lane--connect">
      <span>{{ $t('custom.home.firstDevice.overview.laneConnectTitle') }}</span>
      <strong>
        {{
          ready
            ? $t('custom.home.firstDevice.overview.laneConnectReady')
            : $t('custom.home.firstDevice.overview.laneConnectPending')
        }}
      </strong>
      <small>
        {{ ready ? $t('custom.home.firstDevice.overview.laneConnectReadyDetail') : missionControl.nextClickLabel }}
      </small>
      <div class="first-device-delivery-lane__actions">
        <n-button size="tiny" secondary @click="emit('focusFirstDeviceSection', 'connection')">
          {{ $t('custom.home.firstDevice.overview.locateConnection') }}
        </n-button>
      </div>
    </div>
    <div class="first-device-delivery-lane first-device-delivery-lane--verify">
      <span>{{ $t('custom.home.firstDevice.overview.laneVerifyTitle') }}</span>
      <strong>
        {{ closureSummary.ready ? $t('custom.home.firstDevice.overview.laneVerifyReady') : closureSummary.nextTitle }}
      </strong>
      <small>{{ closureSummary.completionSignal }}</small>
      <div class="first-device-delivery-lane__actions">
        <n-button size="tiny" secondary @click="emit('focusFirstDeviceSection', 'proof')">
          {{ $t('custom.home.firstDevice.common.viewProofArea') }}
        </n-button>
      </div>
    </div>
    <div class="first-device-delivery-lane first-device-delivery-lane--operate">
      <span>{{ $t('custom.home.firstDevice.overview.laneOperateTitle') }}</span>
      <strong>
        {{
          ready
            ? $t('custom.home.firstDevice.overview.laneOperateReady')
            : $t('custom.home.firstDevice.overview.laneOperatePending')
        }}
      </strong>
      <small>
        {{ ready ? $t('custom.home.firstDevice.overview.laneOperateReadyDetail') : missionControl.stuckHint }}
      </small>
      <div class="first-device-delivery-lane__actions">
        <template v-if="ready">
          <n-button size="tiny" type="primary" ghost @click="emit('downloadSuccessProof')">
            {{ $t('custom.home.firstDevice.common.downloadSuccessProof') }}
          </n-button>
          <n-button size="tiny" secondary @click="emit('focusFirstDeviceSection', 'proof')">
            {{ $t('custom.home.firstDevice.common.viewProofArea') }}
          </n-button>
        </template>
        <n-button v-else size="tiny" secondary @click="emit('openFirstDeviceSupportSummaryPreview')">
          {{ $t('custom.home.firstDevice.overview.openSupportSummary') }}
        </n-button>
      </div>
    </div>
  </div>

  <div class="first-device-closure-summary" :class="{ 'first-device-closure-summary--ready': closureSummary.ready }">
    <div class="first-device-closure-summary__head">
      <div class="min-w-0">
        <span>{{ $t('custom.home.firstDevice.overview.closureChecklist') }}</span>
        <strong>{{ closureSummary.statusLabel }}</strong>
      </div>
      <n-tag size="small" round :bordered="false" :type="closureSummary.ready ? 'success' : 'warning'">
        {{ closureSummary.doneCount }}/{{ closureSummary.totalCount }}
      </n-tag>
    </div>
    <div class="first-device-closure-progress" aria-hidden="true">
      <div class="first-device-closure-progress__bar" :style="{ width: `${closureSummary.percent}%` }"></div>
    </div>
    <div class="first-device-closure-summary__grid">
      <div>
        <span>{{ $t('custom.home.firstDevice.overview.nextBlocker') }}</span>
        <strong>{{ closureSummary.nextTitle }}</strong>
        <small>{{ closureSummary.nextDetail }}</small>
      </div>
      <div>
        <span>{{ $t('custom.home.firstDevice.common.finishCriteria') }}</span>
        <small>{{ closureSummary.completionSignal }}</small>
        <div class="first-device-delivery-lane__actions">
          <n-button size="tiny" secondary @click="emit('focusFirstDeviceSection', 'proof')">
            {{ $t('custom.home.firstDevice.common.viewProofArea') }}
          </n-button>
        </div>
      </div>
    </div>
  </div>

  <FirstRunWizardSection
    :ready="ready"
    :steps="wizardSteps"
    @open-step="emit('openHomeGuideStep', $event)"
  />

  <HomeFirstDeviceFlowCanvas
    :ready="ready"
    :flow-nodes="flowNodes"
    :get-flow-node-action="getFlowNodeAction"
    @focus-node="emit('focusFirstDeviceSection', $event)"
  />

  <div
    v-if="verificationAction"
    class="first-device-verification-action"
    :class="`first-device-verification-action--${verificationAction.type}`"
  >
    <div class="min-w-0">
      <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.overview.afterCreateVerification') }}</div>
      <strong>{{ verificationAction.title }}</strong>
      <small>{{ verificationAction.detail }}</small>
    </div>
    <div class="first-device-verification-action-buttons">
      <n-button
        size="small"
        type="primary"
        :loading="verificationAction.loading"
        :disabled="verificationAction.disabled"
        @click="emit('runVerificationAction')"
      >
        {{ verificationAction.label }}
      </n-button>
      <n-button size="small" secondary @click="emit('runVerificationSecondaryAction')">
        {{ verificationAction.secondaryLabel }}
      </n-button>
    </div>
  </div>
</template>

<style scoped>
.first-device-action-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  padding: 14px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background: #eff6ff;
}

.first-device-latest-proof {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr);
  gap: 8px;
  align-items: start;
  min-width: 0;
  font-size: 12px;
}

.first-device-latest-proof span {
  color: #64748b;
}

.first-device-latest-proof strong {
  min-width: 0;
  overflow-wrap: anywhere;
  color: #0f172a;
}

.first-device-delivery-lanes {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.first-device-delivery-lane {
  min-width: 0;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background:
    radial-gradient(circle at top right, rgb(255 255 255 / 84%), transparent 42%),
    linear-gradient(135deg, #ffffff 0%, #f8fafc 100%);
  box-shadow: 0 10px 26px rgb(15 23 42 / 6%);
}

.first-device-delivery-lane span {
  display: inline-flex;
  margin-bottom: 6px;
  color: #64748b;
  font-size: 12px;
  font-weight: 600;
}

.first-device-delivery-lane strong {
  display: block;
  color: #0f172a;
  font-size: 14px;
  line-height: 20px;
  overflow-wrap: anywhere;
}

.first-device-delivery-lane small {
  display: block;
  margin-top: 6px;
  color: #475569;
  font-size: 12px;
  line-height: 18px;
  overflow-wrap: anywhere;
}

.first-device-delivery-lane--connect {
  border-color: #bfdbfe;
}

.first-device-delivery-lane--verify {
  border-color: #fde68a;
}

.first-device-delivery-lane--operate {
  border-color: #bbf7d0;
}

.first-device-delivery-lane__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}

.first-device-operator-cue {
  display: grid;
  gap: 4px;
  padding: 10px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background: #fff;
}

.first-device-operator-cue--success {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.first-device-operator-cue--warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.first-device-operator-cue span,
.first-device-operator-cue small {
  color: #64748b;
  font-size: 12px;
  line-height: 1.45;
}

.first-device-operator-cue strong {
  color: #0f172a;
  font-size: 14px;
  overflow-wrap: anywhere;
}

.first-device-operator-cue__signal {
  color: #475569;
}

.first-device-closure-summary {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #fed7aa;
  border-radius: 8px;
  background: #fff7ed;
}

.first-device-closure-summary--ready {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.first-device-closure-summary__head {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  justify-content: space-between;
}

.first-device-closure-summary__head span,
.first-device-closure-summary__grid span {
  display: block;
  font-size: 12px;
  color: #64748b;
}

.first-device-closure-summary__head strong,
.first-device-closure-summary__grid strong {
  display: block;
  min-width: 0;
  margin-top: 2px;
  overflow-wrap: anywhere;
  color: #0f172a;
}

.first-device-closure-summary__grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 10px;
}

.first-device-closure-summary__grid small {
  display: block;
  min-width: 0;
  margin-top: 4px;
  overflow-wrap: anywhere;
  line-height: 18px;
  color: #475569;
}

.first-device-closure-progress {
  height: 8px;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.25);
}

.first-device-closure-progress__bar {
  height: 100%;
  min-width: 8px;
  border-radius: inherit;
  background: #16a34a;
  transition: width 0.2s ease;
}

.first-device-primary-signal {
  max-width: 220px;
  color: #475569;
  font-size: 12px;
  line-height: 18px;
  overflow-wrap: anywhere;
}

.first-device-mission-control {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  padding: 10px;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #fff;
}

.first-device-mission-control--success {
  border-color: #bbf7d0;
}

.first-device-mission-control--warning {
  border-color: #fed7aa;
}

.first-device-mission-control__item {
  min-width: 0;
  padding: 8px;
  border-radius: 6px;
  background: #f8fafc;
}

.first-device-mission-control__item--wide {
  grid-column: 1 / -1;
}

.first-device-mission-control__item span,
.first-device-mission-control__item small {
  display: block;
  color: #64748b;
  font-size: 11px;
  line-height: 1.45;
}

.first-device-mission-control__item strong {
  display: block;
  margin-top: 3px;
  color: #0f172a;
  font-size: 13px;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.first-device-mission-control__item small {
  margin-top: 3px;
  overflow-wrap: anywhere;
}

.first-device-verification-action {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding: 12px;
  border: 1px solid #fed7aa;
  border-radius: 8px;
  background: #fff7ed;
}

.first-device-verification-action--success {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.first-device-verification-action strong {
  display: block;
  margin-top: 3px;
  color: #0f172a;
  font-size: 14px;
}

.first-device-verification-action small {
  display: block;
  margin-top: 4px;
  color: #475569;
  font-size: 12px;
  line-height: 18px;
}

.first-device-verification-action-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

@media (max-width: 640px) {
  .first-device-action-hero {
    grid-template-columns: 1fr;
  }

  .first-device-mission-control {
    grid-template-columns: 1fr;
  }

  .first-device-delivery-lanes {
    grid-template-columns: 1fr;
  }

  .first-device-closure-summary__grid {
    grid-template-columns: 1fr;
  }

}
</style>
