/**
 * 文件用途: 设备 Ready Check（上线就绪检查）页面状态 composable。
 * 核心逻辑: 组合 useReadyCheckCollectors 的采集结果、路由来源上下文和 ready-check-view-model 纯函数，
 * 产出模板所需的全部派生状态与导航/复制/下载动作；设备 ID 变化时自动重新采集。
 * 关键注意事项:
 * 1. 导航目标（/device/details、/device/command-center、/automation/linkage-edit 等）与查询参数是外部契约，保持不变。
 * 2. 支持包内容由 ready-check-support-bundle 构造，这里只负责拼装输入。
 */
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { $t } from '@/locales'
import { compactValueText, copyTextWithFeedback, downloadJsonWithFeedback } from '../shared/detail-feedback'
import { buildReadyCheckEvidenceCards, type ReadyCheckEvidenceCard } from './device-access-guide-state'
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
import {
  buildBackendNextSteps,
  buildEvidenceCenterItems,
  buildReadyCheckSteps,
  formatLatestTelemetryText,
  formatPartialResultText,
  selectPrimaryReadyAction,
  type ReadyCheckEvidence,
  type ReadyCheckPrimaryActionItem
} from './ready-check-view-model'

export type OnboardingReadyCheckProps = {
  id: string
  online?: number
  deviceData?: Record<string, any>
}

const DETAILS = 'custom.device_details'

export function useOnboardingReadyCheck(props: OnboardingReadyCheckProps) {
  const route = useRoute()
  const router = useRouter()

  const deviceName = computed(() => props.deviceData?.name || props.deviceData?.device_name || '--')
  const deviceNumber = computed(() => props.deviceData?.device_number || '--')
  const isOnline = computed(() => Number(props.online) === 1)
  const hasConnectionIdentity = computed(() => Boolean(props.id && deviceNumber.value !== '--'))
  const hasTemplate = computed(() =>
    Boolean(
      props.deviceData?.device_config_id || props.deviceData?.device_config_name || props.deviceData?.device_config
    )
  )

  const source = computed(() => buildReadyCheckSourceContext(route.query as Record<string, unknown>))
  const sourceLabel = computed(() => $t(source.value.labelKey))
  const sourceDetail = computed(() => source.value.detailText || $t(source.value.detailKey))

  const collectors = useReadyCheckCollectors()
  const { diagnostics, connectionGuide, recommendedCommandDraft, collectionFailures } = collectors

  const readyCheck = computed<ReadyCheckEvidence>(() => diagnostics.value.readyCheck || {})
  const evidenceCards = computed<ReadyCheckEvidenceCard[]>(() => buildReadyCheckEvidenceCards(connectionGuide.value))
  const latestTelemetryText = computed(() => formatLatestTelemetryText(readyCheck.value.telemetry, $t))
  const latestTelemetryValueText = computed(() => compactValueText(readyCheck.value.telemetry?.latest_value))
  const nextActions = computed(() => diagnostics.value.nextActions)
  const evaluatedAtText = computed(
    () => connectionGuide.value?.evaluated_at || $t(`${DETAILS}.readyCheckEvidenceUnknownTime`)
  )
  const readySummary = computed(() => {
    if (readyCheck.value.summary) return readyCheck.value.summary
    if (diagnostics.value.conclusion?.summary) return diagnostics.value.conclusion.summary
    return isOnline.value
      ? $t(`${DETAILS}.accessGuideReadyCheckNoTelemetry`)
      : $t(`${DETAILS}.accessGuideReadyCheckOffline`)
  })
  const readinessSummaryText = computed(() => connectionGuide.value?.readiness?.summary || readySummary.value)
  const lastConnectionIssueText = computed(
    () => connectionGuide.value?.last_connection_error?.summary || $t(`${DETAILS}.readyCheckEvidenceNoLastIssue`)
  )
  const partialResultText = computed(() => formatPartialResultText(connectionGuide.value, $t))
  const backendNextSteps = computed(() => buildBackendNextSteps(connectionGuide.value, $t))
  const collectionFailureSummary = computed(() =>
    $t(`${DETAILS}.readyCheckCollectionWarningDesc`).replace(
      '{collectors}',
      collectionFailures.value.map((item) => $t(item.labelKey)).join(', ')
    )
  )

  const evidenceDeepLinks = computed<ReadyCheckDeepLink[]>(() =>
    buildReadyCheckEvidenceDeepLinks({
      routeQuery: route.query as Record<string, unknown>,
      deviceId: props.id,
      isOtaFailureSource: source.value.isOtaFailureSource,
      otaTaskId: source.value.otaTaskId,
      otaDetailId: source.value.otaDetailId
    })
  )

  const evidenceCenterItems = computed(() =>
    buildEvidenceCenterItems(
      {
        sourceLabel: sourceLabel.value,
        sourceDetail: sourceDetail.value,
        evaluatedAt: evaluatedAtText.value,
        readinessSummary: readinessSummaryText.value,
        guide: connectionGuide.value,
        readyCheck: readyCheck.value,
        latestTelemetryText: latestTelemetryText.value,
        latestTelemetryValueText: latestTelemetryValueText.value,
        lastConnectionIssue: lastConnectionIssueText.value,
        partialResultText: partialResultText.value
      },
      $t
    )
  )

  const supportBundleInput = computed(() => ({
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
      sourceKey: source.value.sourceKey,
      label: sourceLabel.value,
      detail: sourceDetail.value,
      otaTaskId: source.value.otaTaskId || '',
      otaDetailId: source.value.otaDetailId || '',
      commandJobId: source.value.commandJobId || '',
      firstDeviceOnboarding: source.value.isFirstDeviceOnboardingSource
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
    boundaryText: $t(`${DETAILS}.readyCheckEvidenceBoundaryDesc`)
  }))
  const readyCheckDiagnosticSummary = computed(() => buildReadyCheckDiagnosticMarkdown(supportBundleInput.value))

  const copyReadyCheckDiagnosticSummary = () => copyTextWithFeedback(readyCheckDiagnosticSummary.value)

  const downloadReadyCheckDiagnosticSummary = () =>
    downloadJsonWithFeedback(() => buildReadyCheckSupportBundle(supportBundleInput.value), {
      fileName: readyCheckSupportFileName(props.id || deviceNumber.value || deviceName.value || 'device'),
      successKey: `${DETAILS}.readyCheckSupportBundleDownloaded`,
      failureKey: `${DETAILS}.readyCheckSupportBundleDownloadFailed`,
      mimeType: 'application/json;charset=utf-8'
    })

  const refreshDiagnostics = () => collectors.refreshDiagnostics(props.id)

  const openTab = (tab: string) => {
    router.push({ path: '/device/details', query: { ...route.query, d_id: props.id, tab } })
  }

  const openEvidenceDeepLink = (link: ReadyCheckDeepLink) => {
    router.push({ path: link.path, query: link.query })
  }

  const copyEvidenceDeepLink = (link: ReadyCheckDeepLink) => copyTextWithFeedback(formatReadyCheckDeepLink(link))

  const copyAllEvidenceDeepLinks = async () => {
    const text = evidenceDeepLinks.value
      .map((link) =>
        [
          `${$t(link.labelKey)}: ${formatReadyCheckDeepLink(link)}`,
          `${$t(`${DETAILS}.readyCheckEvidenceBoundary`)}: ${$t(link.boundaryKey)}`
        ].join('\n')
      )
      .join('\n\n')
    await copyTextWithFeedback(text)
  }

  const openCommandCenter = () => {
    router.push({
      path: '/device/command-center',
      query: buildReadyCheckCommandCenterQuery({ deviceId: props.id, draft: recommendedCommandDraft.value })
    })
  }

  const openFirstDeviceHomeProof = () => {
    router.push({ path: '/home', query: { onboarding: 'first-device', focus: 'first-device-proof' } })
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
    router.push({ path: '/automation/linkage-edit', query })
  }

  const openFirstDeviceDashboard = () => {
    router.push({ path: '/visualization/thingsvis', query: { onboarding: 'first-device' } })
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

  const evidenceActionCards = computed<ReadyCheckPrimaryActionItem[]>(() =>
    evidenceCards.value.map((card) => ({
      key: `evidence-${card.key}`,
      status: card.status,
      titleKey: card.titleKey,
      actionKey:
        card.key === 'twin'
          ? `${DETAILS}.readyCheckOpenTwin`
          : card.status === 'next'
            ? `${DETAILS}.readyCheckOpenCommandCenter`
            : `${DETAILS}.readyCheckOpenCommand`,
      action: () => runEvidenceCardAction(card),
      summary: card.summary
    }))
  )

  const steps = computed(() =>
    buildReadyCheckSteps({
      hasConnectionIdentity: hasConnectionIdentity.value,
      isOnline: isOnline.value,
      hasRecentTelemetry: Boolean(readyCheck.value.telemetry?.latest_key),
      hasTemplate: hasTemplate.value,
      openTab,
      openCommandCenter
    })
  )

  const showFirstDeviceReadyHandoff = computed(
    () => source.value.isFirstDeviceOnboardingSource && readyCheck.value.ready === true
  )

  const primaryReadyAction = computed(() =>
    selectPrimaryReadyAction({
      collectionFailureSummary: collectionFailures.value.length ? collectionFailureSummary.value : null,
      firstDeviceReady: showFirstDeviceReadyHandoff.value,
      steps: steps.value,
      evidenceActions: evidenceActionCards.value,
      refresh: refreshDiagnostics,
      openFirstDeviceAutomation,
      t: $t
    })
  )
  const primaryReadyActionSummary = computed(() => primaryReadyAction.value?.summary || readySummary.value)

  const runReadyCheckStep = (key: string) => {
    steps.value.find((step) => step.key === key)?.action?.()
  }

  onMounted(refreshDiagnostics)
  watch(
    () => props.id,
    () => {
      refreshDiagnostics()
    }
  )

  return {
    ...collectors,
    deviceName,
    deviceNumber,
    isOnline,
    source,
    readySummary,
    latestTelemetryText,
    nextActions,
    evidenceCards,
    evidenceCenterItems,
    evidenceDeepLinks,
    backendNextSteps,
    collectionFailureSummary,
    readyCheckDiagnosticSummary,
    steps,
    primaryReadyAction,
    primaryReadyActionSummary,
    showFirstDeviceReadyHandoff,
    refreshDiagnostics,
    copyReadyCheckDiagnosticSummary,
    downloadReadyCheckDiagnosticSummary,
    openEvidenceDeepLink,
    copyEvidenceDeepLink,
    copyAllEvidenceDeepLinks,
    openCommandCenter,
    openFirstDeviceHomeProof,
    openFirstDeviceAutomation,
    openFirstDeviceDashboard,
    runEvidenceCardAction,
    runReadyCheckStep
  }
}
