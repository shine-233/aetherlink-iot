<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { $t } from '@/locales'
import { useViewportDeferredMount } from '@/hooks/common/useViewportDeferredMount'
import { writeClipboardText } from '@/utils/clipboard'
import {
  buildReadyCheckEvidenceCards,
  type ReadyCheckEvidenceCard
} from './device-access-guide-state'
import {
  buildReadyCheckEvidenceDeepLinks,
  formatReadyCheckDeepLink,
  type ReadyCheckDeepLink
} from './ready-check-deep-links'
import {
  buildReadyCheckDiagnosticMarkdown,
  buildReadyCheckSupportBundle,
  readyCheckSupportFileName
} from './ready-check-support-bundle'
import { buildReadyCheckCommandCenterQuery } from './ready-check-command-draft'
import { buildReadyCheckSourceContext } from './ready-check-source-context'
import { useReadyCheckCollectors } from './use-ready-check-collectors'
import ReadyCheckActionPanel from './ReadyCheckActionPanel.vue'

const ReadyCheckEvidenceCenterView = defineAsyncComponent(() => import('./ReadyCheckEvidenceCenterView.vue'))

const props = defineProps<{
  id: string
  online?: number
  deviceData?: Record<string, any>
}>()

const route = useRoute()
const router = useRouter()

type ReadyCheckTelemetry = {
  current_count?: number
  latest_key?: string
  latest_at?: string
  latest_value?: unknown
}

type ReadyCheckEvidence = {
  ready?: boolean
  level?: string
  code?: string
  summary?: string
  next_actions?: string[]
  telemetry?: ReadyCheckTelemetry
}

type ReadyCheckActionStatus = 'ready' | 'attention' | 'next'

type ReadyCheckStepItem = {
  key: string
  status: ReadyCheckActionStatus
  titleKey: string
  descKey: string
  actionKey: string
  action: () => void
}

type ReadyCheckPrimaryActionItem = {
  key: string
  status: ReadyCheckActionStatus
  titleKey: string
  descKey?: string
  actionKey: string
  action: () => void
  summary?: string
}

const deviceName = computed(() => props.deviceData?.name || props.deviceData?.device_name || '--')
const deviceNumber = computed(() => props.deviceData?.device_number || '--')
const isOnline = computed(() => Number(props.online) === 1)
const hasConnectionIdentity = computed(() => Boolean(props.id && deviceNumber.value !== '--'))
const hasTemplate = computed(() =>
  Boolean(props.deviceData?.device_config_id || props.deviceData?.device_config_name || props.deviceData?.device_config)
)
const readyCheckSourceContext = computed(() => buildReadyCheckSourceContext(route.query as Record<string, unknown>))
const isFirstDeviceOnboardingSource = computed(() => readyCheckSourceContext.value.isFirstDeviceOnboardingSource)
const isOtaFailureSource = computed(() => readyCheckSourceContext.value.isOtaFailureSource)
const isCommandJobDiagnosisSource = computed(() => readyCheckSourceContext.value.isCommandJobDiagnosisSource)
const otaTaskId = computed(() => readyCheckSourceContext.value.otaTaskId)
const otaDetailId = computed(() => readyCheckSourceContext.value.otaDetailId)
const commandJobId = computed(() => readyCheckSourceContext.value.commandJobId)
const readyCheckSourceLabel = computed(() => $t(readyCheckSourceContext.value.labelKey))
const readyCheckSourceDetail = computed(() =>
  readyCheckSourceContext.value.detailText || $t(readyCheckSourceContext.value.detailKey)
)
const {
  diagnosticsLoading,
  diagnostics,
  connectionGuide,
  recommendedCommandLoading,
  recommendedCommandDraft,
  collectionFailures,
  refreshDiagnostics: refreshReadyCheckCollectors
} = useReadyCheckCollectors()
const readyCheck = computed<ReadyCheckEvidence>(() => diagnostics.value.readyCheck || {})
const evidenceCards = computed<ReadyCheckEvidenceCard[]>(() => buildReadyCheckEvidenceCards(connectionGuide.value))
const hasRecentTelemetry = computed(() => Boolean(readyCheck.value.telemetry?.latest_key))
const latestTelemetryText = computed(() => {
  const telemetry = readyCheck.value.telemetry
  if (!telemetry?.latest_key) return $t('custom.device_details.accessGuideLatestTelemetryEmpty')
  return [telemetry.latest_key, telemetry.latest_at].filter(Boolean).join(' @ ')
})
const compactValueText = (value: unknown) => {
  if (value === undefined || value === null || value === '') return '--'
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  if (!text) return '--'
  return text.length > 120 ? `${text.slice(0, 120)}...` : text
}
const latestTelemetryValueText = computed(() => compactValueText(readyCheck.value.telemetry?.latest_value))
const nextActions = computed(() => {
  return diagnostics.value.nextActions
})
const evaluatedAtText = computed(() => connectionGuide.value?.evaluated_at || $t('custom.device_details.readyCheckEvidenceUnknownTime'))
const readinessSummaryText = computed(() => {
  const readiness = connectionGuide.value?.readiness
  if (readiness?.summary) return readiness.summary
  return readySummary.value
})
const lastConnectionIssueText = computed(() => {
  return connectionGuide.value?.last_connection_error?.summary || $t('custom.device_details.readyCheckEvidenceNoLastIssue')
})
const partialResultText = computed(() => {
  const partialResults = Array.isArray(connectionGuide.value?.partial_results) ? connectionGuide.value?.partial_results : []
  if (!partialResults?.length) return $t('custom.device_details.readyCheckEvidenceComplete')
  return partialResults.map((warning) => `${warning.component || 'guide'}: ${warning.reason || 'partial'}`).join('; ')
})
const backendNextSteps = computed(() => {
  return (connectionGuide.value?.next_steps || [])
    .map((step, index) => ({
      key: step.key || `step-${index}`,
      title: step.title || $t('custom.device_details.readyCheckEvidenceNextStepUntitled'),
      description: step.description || '',
      status: step.status || 'todo'
    }))
    .slice(0, 4)
})
const collectionFailureSummary = computed(() =>
  $t('custom.device_details.readyCheckCollectionWarningDesc').replace(
    '{collectors}',
    collectionFailures.value.map(item => $t(item.labelKey)).join(', ')
  )
)
const evidenceCenterViewportRef = ref<HTMLElement | null>(null)
const {
  shouldMount: shouldMountEvidenceCenter,
  mountNow: mountEvidenceCenterNow
} = useViewportDeferredMount(evidenceCenterViewportRef, {
  rootMargin: '420px 0px',
  fallbackDelay: 650
})
const evidenceDeepLinks = computed<ReadyCheckDeepLink[]>(() =>
  buildReadyCheckEvidenceDeepLinks({
    routeQuery: route.query as Record<string, unknown>,
    deviceId: props.id,
    isOtaFailureSource: isOtaFailureSource.value,
    otaTaskId: otaTaskId.value,
    otaDetailId: otaDetailId.value
  })
)
const evidenceCenterItems = computed(() => [
  {
    key: 'source',
    labelKey: 'custom.device_details.readyCheckEvidenceSource',
    value: readyCheckSourceLabel.value,
    detail: readyCheckSourceDetail.value
  },
  {
    key: 'evaluated-at',
    labelKey: 'custom.device_details.readyCheckEvidenceEvaluatedAt',
    value: evaluatedAtText.value,
    detail: $t('custom.device_details.readyCheckEvidenceEvaluatedAtDesc')
  },
  {
    key: 'readiness',
    labelKey: 'custom.device_details.readyCheckEvidenceReadiness',
    value: readinessSummaryText.value,
    detail: [
      `ready=${connectionGuide.value?.readiness?.ready ?? readyCheck.value.ready ?? '--'}`,
      `level=${connectionGuide.value?.readiness?.level || readyCheck.value.level || '--'}`,
      `code=${connectionGuide.value?.readiness?.code || readyCheck.value.code || '--'}`
    ].join(' / ')
  },
  {
    key: 'telemetry',
    labelKey: 'custom.device_details.accessGuideLatestTelemetry',
    value: latestTelemetryText.value,
    detail: [
      `${$t('custom.device_details.readyCheckEvidenceTelemetryCount')}: ${
        readyCheck.value.telemetry?.current_count ?? '--'
      }`,
      `${$t('custom.device_details.readyCheckEvidenceTelemetryValue')}: ${latestTelemetryValueText.value}`
    ].join(' / ')
  },
  {
    key: 'last-issue',
    labelKey: 'custom.device_details.readyCheckEvidenceLastIssue',
    value: lastConnectionIssueText.value,
    detail: connectionGuide.value?.last_connection_error?.code || $t('custom.device_details.readyCheckEvidenceNoLastIssueCode')
  },
  {
    key: 'completeness',
    labelKey: 'custom.device_details.readyCheckEvidenceCompleteness',
    value: partialResultText.value,
    detail: $t('custom.device_details.readyCheckEvidenceCompletenessDesc')
  },
  {
    key: 'boundary',
    labelKey: 'custom.device_details.readyCheckEvidenceBoundary',
    value: $t('custom.device_details.readyCheckEvidenceBoundaryValue'),
    detail: $t('custom.device_details.readyCheckEvidenceBoundaryDesc')
  }
])
const readySummary = computed(() => {
  if (readyCheck.value.summary) return readyCheck.value.summary
  if (diagnostics.value.conclusion?.summary) return diagnostics.value.conclusion.summary
  return isOnline.value
    ? $t('custom.device_details.accessGuideReadyCheckNoTelemetry')
    : $t('custom.device_details.accessGuideReadyCheckOffline')
})
const readyCheckSupportBundleInput = computed(() => ({
  t: $t,
  device: {
    id: props.id || '',
    name: deviceName.value,
    number: deviceNumber.value,
    online: isOnline.value,
    hasConnectionIdentity: hasConnectionIdentity.value,
    hasTemplate: hasTemplate.value
  },
  source: {
    sourceKey: readyCheckSourceContext.value.sourceKey,
    label: readyCheckSourceLabel.value,
    detail: readyCheckSourceDetail.value,
    otaTaskId: otaTaskId.value || '',
    otaDetailId: otaDetailId.value || '',
    commandJobId: commandJobId.value || '',
    firstDeviceOnboarding: isFirstDeviceOnboardingSource.value
  },
  readiness: {
    ready: readyCheck.value.ready ?? null,
    level: readyCheck.value.level || '',
    code: readyCheck.value.code || '',
    summary: readySummary.value,
    evaluatedAt: evaluatedAtText.value,
    connectionGuideSummary: readinessSummaryText.value
  },
  telemetry: {
    latest: latestTelemetryText.value,
    latestValue: latestTelemetryValueText.value,
    currentCount: readyCheck.value.telemetry?.current_count ?? null
  },
  diagnostics: {
    nextActions: nextActions.value,
    lastConnectionIssue: lastConnectionIssueText.value,
    partialResults: partialResultText.value,
    conclusion: diagnostics.value.conclusion || null,
    debug: diagnostics.value.debug,
    recentFailures: diagnostics.value.recentFailures || [],
    partialWarnings: diagnostics.value.partialWarnings || []
  },
  evidenceCenterItems: evidenceCenterItems.value,
  evidenceCards: evidenceCards.value,
  backendNextSteps: backendNextSteps.value,
  deepLinks: evidenceDeepLinks.value,
  collectionFailures: collectionFailures.value,
  boundaryText: $t('custom.device_details.readyCheckEvidenceBoundaryDesc')
}))
const readyCheckDiagnosticSummary = computed(() => buildReadyCheckDiagnosticMarkdown(readyCheckSupportBundleInput.value))

const copyReadyCheckDiagnosticSummary = async () => {
  const copied = await writeClipboardText(readyCheckDiagnosticSummary.value)
  if (copied) {
    window.$message?.success($t('theme.configOperation.copySuccess'))
  } else {
    window.$message?.error($t('common.copyFailed'))
  }
}

const readyCheckSupportFileNameValue = () => readyCheckSupportFileName(props.id || deviceNumber.value || deviceName.value || 'device')

const buildReadyCheckSupportBundleValue = () => buildReadyCheckSupportBundle(readyCheckSupportBundleInput.value)

const downloadReadyCheckDiagnosticSummary = () => {
  try {
    if (typeof window === 'undefined' || typeof document === 'undefined') return
    const blob = new Blob([JSON.stringify(buildReadyCheckSupportBundleValue(), null, 2)], {
      type: 'application/json;charset=utf-8'
    })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = readyCheckSupportFileNameValue()
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
    window.$message?.success($t('custom.device_details.readyCheckSupportBundleDownloaded'))
  } catch {
    window.$message?.error($t('custom.device_details.readyCheckSupportBundleDownloadFailed'))
  }
}

const refreshDiagnostics = () => refreshReadyCheckCollectors(props.id)

const openTab = (tab: string) => {
  router.push({
    path: '/device/details',
    query: {
      ...route.query,
      d_id: props.id,
      tab
    }
  })
}

const openEvidenceDeepLink = (link: ReadyCheckDeepLink) => {
  router.push({
    path: link.path,
    query: link.query
  })
}

const copyEvidenceDeepLink = async (link: ReadyCheckDeepLink) => {
  const copied = await writeClipboardText(formatReadyCheckDeepLink(link))
  if (copied) {
    window.$message?.success($t('theme.configOperation.copySuccess'))
  } else {
    window.$message?.error($t('common.copyFailed'))
  }
}
const copyAllEvidenceDeepLinks = async () => {
  const text = evidenceDeepLinks.value
    .map((link) =>
      [
        `${$t(link.labelKey)}: ${formatReadyCheckDeepLink(link)}`,
        `${$t('custom.device_details.readyCheckEvidenceBoundary')}: ${$t(link.boundaryKey)}`
      ].join('\n')
    )
    .join('\n\n')
  const copied = await writeClipboardText(text)
  if (copied) {
    window.$message?.success($t('theme.configOperation.copySuccess'))
  } else {
    window.$message?.error($t('common.copyFailed'))
  }
}

const openCommandCenter = () => {
  router.push({
    path: '/device/command-center',
    query: buildReadyCheckCommandCenterQuery({
      deviceId: props.id,
      draft: recommendedCommandDraft.value
    })
  })
}

const openFirstDeviceHomeProof = () => {
  router.push({
    path: '/home',
    query: {
      onboarding: 'first-device',
      focus: 'first-device-proof'
    }
  })
}

const openFirstDeviceAutomation = () => {
  const query: Record<string, string> = {
    backType: 'automation',
    onboarding: 'first-device',
    starter: 'first-telemetry-rule',
    device_id: props.id
  }
  if (deviceName.value && deviceName.value !== '--') query.first_device_name = String(deviceName.value)
  if (deviceNumber.value && deviceNumber.value !== '--') query.first_device_number = String(deviceNumber.value)
  if (props.deviceData?.device_config_id) query.device_config_id = String(props.deviceData.device_config_id)
  if (readyCheck.value.telemetry?.latest_key) query.telemetry_key = String(readyCheck.value.telemetry.latest_key)
  if (readyCheck.value.telemetry?.latest_at) query.telemetry_at = String(readyCheck.value.telemetry.latest_at)

  router.push({
    path: '/automation/linkage-edit',
    query
  })
}

const openFirstDeviceDashboard = () => {
  router.push({
    path: '/visualization/thingsvis',
    query: {
      onboarding: 'first-device'
    }
  })
}

const getEvidenceCardActionKey = (card: ReadyCheckEvidenceCard) => {
  if (card.key === 'twin') return 'custom.device_details.readyCheckOpenTwin'
  return card.status === 'next'
    ? 'custom.device_details.readyCheckOpenCommandCenter'
    : 'custom.device_details.readyCheckOpenCommand'
}
const runEvidenceCardAction = (card: ReadyCheckEvidenceCard) => {
  if (card.key === 'twin') {
    openTab('device-twin')
    return
  }
  if (card.status === 'next') {
    openCommandCenter()
    return
  }
  openTab('command-delivery')
}
const evidenceActionCards = computed(() =>
  evidenceCards.value.map((card) => ({
    key: `evidence-${card.key}`,
    status: card.status,
    titleKey: card.titleKey,
    actionKey: getEvidenceCardActionKey(card),
    action: () => runEvidenceCardAction(card),
    summary: card.summary
  }))
)

const steps = computed<ReadyCheckStepItem[]>(() => [
  {
    key: 'connect',
    status: hasConnectionIdentity.value ? 'ready' : 'attention',
    titleKey: 'custom.device_details.readyCheckConnectTitle',
    descKey: 'custom.device_details.readyCheckConnectDesc',
    actionKey: 'custom.device_details.readyCheckOpenConnection',
    action: () => openTab('join')
  },
  {
    key: 'online',
    status: isOnline.value ? 'ready' : 'attention',
    titleKey: 'custom.device_details.readyCheckOnlineTitle',
    descKey: isOnline.value
      ? 'custom.device_details.readyCheckOnlineReadyDesc'
      : 'custom.device_details.readyCheckOnlineWaitingDesc',
    actionKey: 'custom.device_details.readyCheckOpenConnection',
    action: () => openTab('join')
  },
  {
    key: 'telemetry',
    status: hasRecentTelemetry.value ? 'ready' : isOnline.value ? 'attention' : 'next',
    titleKey: 'custom.device_details.readyCheckTelemetryTitle',
    descKey: hasTemplate.value
      ? 'custom.device_details.readyCheckTelemetryDesc'
      : 'custom.device_details.readyCheckTelemetryNoTemplateDesc',
    actionKey: 'custom.device_details.readyCheckOpenTelemetry',
    action: () => openTab('telemetry')
  },
  {
    key: 'twin',
    status: 'next',
    titleKey: 'custom.device_details.readyCheckTwinTitle',
    descKey: 'custom.device_details.readyCheckTwinDesc',
    actionKey: 'custom.device_details.readyCheckOpenTwin',
    action: () => openTab('device-twin')
  },
  {
    key: 'command',
    status: 'next',
    titleKey: 'custom.device_details.readyCheckCommandTitle',
    descKey: 'custom.device_details.readyCheckCommandDesc',
    actionKey: 'custom.device_details.readyCheckOpenCommand',
    action: () => openTab('command-delivery')
  },
  {
    key: 'jobs',
    status: 'next',
    titleKey: 'custom.device_details.readyCheckJobsTitle',
    descKey: 'custom.device_details.readyCheckJobsDesc',
    actionKey: 'custom.device_details.readyCheckOpenCommandCenter',
    action: openCommandCenter
  }
])
const primaryReadyAction = computed<ReadyCheckPrimaryActionItem>(() => {
  if (collectionFailures.value.length) {
    return {
      key: 'collection-failure',
      status: 'attention',
      titleKey: 'custom.device_details.readyCheckCollectionWarningTitle',
      actionKey: 'custom.device_details.accessGuideDiagnosticRefresh',
      action: refreshDiagnostics,
      summary: collectionFailureSummary.value
    }
  }
  if (isFirstDeviceOnboardingSource.value && readyCheck.value.ready === true) {
    return {
      key: 'first-device-automation',
      status: 'next',
      titleKey: 'custom.device_details.readyCheckFirstDeviceNextTitle',
      actionKey: 'custom.automation.createFirstTelemetryRule',
      action: openFirstDeviceAutomation,
      summary: $t('custom.device_details.readyCheckFirstDeviceNextDesc')
    }
  }
  return (
    steps.value.find(step => step.status === 'attention') ??
    evidenceActionCards.value.find(card => card.status === 'attention') ??
    steps.value.find(step => step.status === 'next') ??
    evidenceActionCards.value.find(card => card.status === 'next') ??
    steps.value[0]
  )
})
const primaryReadyActionSummary = computed(() => primaryReadyAction.value?.summary || readySummary.value)
const showFirstDeviceReadyHandoff = computed(() => isFirstDeviceOnboardingSource.value && readyCheck.value.ready === true)

onMounted(refreshDiagnostics)

watch(
  () => props.id,
  () => {
    refreshDiagnostics()
  }
)

const runReadyCheckStep = (key: string) => {
  steps.value.find(step => step.key === key)?.action?.()
}
</script>

<template>
  <div class="ready-check" data-testid="device-ready-check">
    <section class="ready-check-hero">
      <div class="ready-check-hero__intro">
        <div class="ready-check-hero__copy">
          <span class="ready-check-hero__eyebrow">{{ $t('custom.device_details.readyCheckDevice') }}</span>
          <strong :title="String(deviceName)">{{ deviceName }}</strong>
          <p>{{ $t('custom.device_details.readyCheckIntro') }}</p>
        </div>
        <span class="ready-check-hero__state" :class="{ 'is-online': isOnline }">
          {{ isOnline ? $t('custom.device_details.online') : $t('custom.device_details.offline') }}
        </span>
      </div>

      <NAlert v-if="isFirstDeviceOnboardingSource" type="success" :show-icon="false" class="ready-check-source-banner">
        <strong>{{ $t('custom.device_details.readyCheckFirstDeviceSourceTitle') }}</strong>
        <span>{{ $t('custom.device_details.readyCheckFirstDeviceSourceDesc') }}</span>
      </NAlert>
      <NAlert
        v-if="isOtaFailureSource"
        type="warning"
        :show-icon="false"
        class="ready-check-source-banner ready-check-source-banner--warning"
      >
        <strong>{{ $t('custom.device_details.readyCheckOtaFailureSourceTitle') }}</strong>
        <span>{{ $t('custom.device_details.readyCheckOtaFailureSourceDesc') }}</span>
        <span class="ready-check-source-banner__meta">
          <code v-if="otaTaskId">task={{ otaTaskId }}</code>
          <code v-if="otaDetailId">detail={{ otaDetailId }}</code>
        </span>
      </NAlert>
      <NAlert
        v-if="isCommandJobDiagnosisSource"
        type="warning"
        :show-icon="false"
        class="ready-check-source-banner ready-check-source-banner--warning"
      >
        <strong>{{ $t('custom.device_details.readyCheckCommandJobSourceTitle') }}</strong>
        <span>{{ $t('custom.device_details.readyCheckCommandJobSourceDesc') }}</span>
        <span class="ready-check-source-banner__meta">
          <code v-if="commandJobId">job={{ commandJobId }}</code>
        </span>
      </NAlert>

      <div class="ready-check-summary">
        <div>
          <span>{{ $t('custom.device_details.readyCheckDeviceNumber') }}</span>
          <strong :title="String(deviceNumber)">{{ deviceNumber }}</strong>
        </div>
        <div>
          <span>{{ $t('custom.device_details.readyCheckOnlineState') }}</span>
          <strong :title="isOnline ? $t('custom.device_details.online') : $t('custom.device_details.offline')">
            {{ isOnline ? $t('custom.device_details.online') : $t('custom.device_details.offline') }}
          </strong>
        </div>
        <div>
          <span>{{ $t('custom.device_details.readyCheckPrimaryNext') }}</span>
          <strong>{{ $t(primaryReadyAction.titleKey) }}</strong>
        </div>
      </div>
    </section>

    <NAlert
      v-if="collectionFailures.length"
      type="warning"
      :show-icon="false"
      class="ready-check-collection-warning"
      data-testid="device-ready-check-collection-warning"
    >
      <div class="ready-check-collection-warning__copy">
        <strong>{{ $t('custom.device_details.readyCheckCollectionWarningTitle') }}</strong>
        <span>{{ collectionFailureSummary }}</span>
      </div>
      <NSpace size="small">
        <NButton size="small" secondary type="primary" :loading="diagnosticsLoading" @click="refreshDiagnostics">
          {{ $t('custom.device_details.accessGuideDiagnosticRefresh') }}
        </NButton>
        <NButton size="small" secondary @click="downloadReadyCheckDiagnosticSummary">
          {{ $t('custom.device_details.readyCheckDownloadSupportBundle') }}
        </NButton>
      </NSpace>
    </NAlert>

    <div ref="evidenceCenterViewportRef">
      <ReadyCheckEvidenceCenterView
        v-if="shouldMountEvidenceCenter"
        :loading="diagnosticsLoading"
        :ready-summary="readySummary"
        :latest-telemetry-text="latestTelemetryText"
        :next-actions="nextActions"
        :evidence-center-items="evidenceCenterItems"
        :evidence-cards="evidenceCards"
        :backend-next-steps="backendNextSteps"
        :deep-links="evidenceDeepLinks"
        @refresh="refreshDiagnostics"
        @copy-support-bundle="copyReadyCheckDiagnosticSummary"
        @download-support-bundle="downloadReadyCheckDiagnosticSummary"
        @open-deep-link="openEvidenceDeepLink"
        @copy-deep-link="copyEvidenceDeepLink"
        @copy-all-deep-links="copyAllEvidenceDeepLinks"
        @run-evidence-card="runEvidenceCardAction"
      />
      <div v-else class="ready-check-deferred-placeholder">
        <div class="ready-check-deferred-placeholder__copy">
          <span>{{ $t('custom.device_details.readyCheckEvidenceBoundary') }}</span>
          <strong>{{ $t('custom.device_details.readyCheckEvidenceCenterDeferredTitle') }}</strong>
          <p>{{ $t('custom.device_details.readyCheckEvidenceCenterDeferredDesc') }}</p>
        </div>
        <NButton
          size="small"
          secondary
          type="primary"
          data-testid="device-ready-check-load-evidence-center"
          @click="mountEvidenceCenterNow"
        >
          {{ $t('custom.device_details.readyCheckLoadEvidenceCenter') }}
        </NButton>
      </div>
    </div>

    <ReadyCheckActionPanel
      :primary-action="primaryReadyAction"
      :primary-action-summary="primaryReadyActionSummary"
      :recommended-command-loading="recommendedCommandLoading"
      :recommended-command-draft="recommendedCommandDraft"
      :show-first-device-ready-handoff="showFirstDeviceReadyHandoff"
      :steps="steps"
      @run-primary-action="primaryReadyAction.action"
      @open-command-center="openCommandCenter"
      @open-first-device-home-proof="openFirstDeviceHomeProof"
      @open-first-device-automation="openFirstDeviceAutomation"
      @open-first-device-dashboard="openFirstDeviceDashboard"
      @run-step="runReadyCheckStep"
    />
  </div>
</template>

<style scoped>
.ready-check {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.ready-check-hero {
  position: relative;
  display: grid;
  gap: 14px;
  overflow: hidden;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background:
    radial-gradient(circle at 14% 8%, rgba(59, 130, 246, 0.2), transparent 28%),
    linear-gradient(135deg, #f8fbff 0%, #eef6ff 52%, #f8fafc 100%);
  padding: 18px;
  box-shadow: 0 18px 46px rgba(15, 23, 42, 0.08);
}

.ready-check-hero::after {
  position: absolute;
  top: -56px;
  right: -46px;
  width: 170px;
  height: 170px;
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(14, 165, 233, 0.22), rgba(34, 197, 94, 0.16));
  content: '';
  pointer-events: none;
}

.ready-check-hero__intro {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.ready-check-hero__copy {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.ready-check-hero__eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.ready-check-hero__copy strong {
  color: #0f172a;
  font-size: 24px;
  line-height: 1.2;
  overflow-wrap: anywhere;
}

.ready-check-hero__copy p {
  max-width: 760px;
  margin: 0;
  color: #475569;
  font-size: 13px;
  line-height: 1.6;
}

.ready-check-hero__state {
  flex: 0 0 auto;
  border: 1px solid #fed7aa;
  border-radius: 999px;
  background: #fff7ed;
  padding: 6px 12px;
  color: #9a3412;
  font-size: 12px;
  font-weight: 700;
  box-shadow: 0 10px 24px rgba(154, 52, 18, 0.1);
}

.ready-check-hero__state.is-online {
  border-color: #bbf7d0;
  background: #f0fdf4;
  color: #15803d;
  box-shadow: 0 10px 24px rgba(21, 128, 61, 0.1);
}

.ready-check-summary {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.ready-check-hero .ready-check-source-banner {
  position: relative;
  z-index: 1;
}

.ready-check-source-banner :deep(.n-alert-body__content) {
  display: grid;
  gap: 4px;
}

.ready-check-source-banner strong {
  color: #166534;
  font-size: 14px;
}

.ready-check-source-banner span {
  color: #166534;
  font-size: 12px;
  line-height: 18px;
}

.ready-check-source-banner--warning strong,
.ready-check-source-banner--warning span {
  color: #92400e;
}

.ready-check-source-banner__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.ready-check-source-banner__meta code {
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.08);
  padding: 2px 8px;
  color: #334155;
  font-size: 12px;
}

.ready-check-summary > div {
  min-width: 0;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}

.ready-check-summary > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
  border-color: rgba(148, 163, 184, 0.28);
  background: rgba(255, 255, 255, 0.72);
  padding: 12px;
  box-shadow: 0 10px 26px rgba(15, 23, 42, 0.05);
  backdrop-filter: blur(10px);
}

.ready-check-summary span {
  color: #64748b;
  font-size: 13px;
  line-height: 1.5;
}

.ready-check-summary strong {
  display: block;
  color: #0f172a;
  overflow-wrap: anywhere;
  line-height: 1.35;
  word-break: break-word;
}

.ready-check-collection-warning :deep(.n-alert-body__content) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.ready-check-collection-warning__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.ready-check-collection-warning__copy strong {
  color: #92400e;
  font-size: 14px;
}

.ready-check-collection-warning__copy span {
  color: #92400e;
  font-size: 13px;
  line-height: 1.5;
}

.ready-check-deferred-placeholder {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  border: 1px dashed #cbd5e1;
  border-radius: 16px;
  background: linear-gradient(135deg, #f8fafc 0%, #eef2ff 100%);
  padding: 18px 20px;
}

.ready-check-deferred-placeholder__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.ready-check-deferred-placeholder__copy span {
  color: #6366f1;
  font-size: 12px;
  font-weight: 700;
}

.ready-check-deferred-placeholder__copy strong {
  color: #0f172a;
  font-size: 15px;
}

.ready-check-deferred-placeholder__copy p {
  margin: 0;
  color: #475569;
  font-size: 13px;
  line-height: 1.6;
}

@media (max-width: 900px) {
  .ready-check-summary {
    grid-template-columns: 1fr;
  }

  .ready-check-hero {
    border-radius: 14px;
    padding: 14px;
  }

  .ready-check-hero__intro {
    flex-direction: column;
  }

  .ready-check-hero__copy strong {
    font-size: 20px;
  }

  .ready-check-collection-warning :deep(.n-alert-body__content) {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
