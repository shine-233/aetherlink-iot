<script setup lang="ts">
import type {
  AutomationConditionSummaryGroup,
  AutomationDryRunCustomerStatus,
  AutomationDryRunCustomerView,
  AutomationDryRunBackendView,
  AutomationDryRunBeginnerGuideCard,
  AutomationDryRunLine,
  AutomationDryRunOperatorPlan,
  AutomationDryRunQuickFixAction
} from './automationDryRunPreview'
import { $t } from '@/locales'

const getCustomerStatusLabelKey = (status: AutomationDryRunCustomerStatus) => {
  if (status === 'passed') return 'generate.automationDryRunStatusPassed'
  if (status === 'risk') return 'generate.automationDryRunStatusRisk'

  return 'generate.automationDryRunStatusUnchecked'
}

const getCustomerStatusDescriptionKey = (status: AutomationDryRunCustomerStatus) => {
  if (status === 'passed') return 'generate.automationDryRunPassedHint'
  if (status === 'risk') return 'generate.automationDryRunRiskHint'

  return 'generate.automationDryRunUncheckedHint'
}

withDefaults(defineProps<{
  localStatusText: string
  backendStatusText: string
  backendAlertType: 'default' | 'error' | 'success' | 'warning' | 'info'
  backendError: string
  conditionGroupCount: number
  conditionCount: number
  actionCount: number
  conditionSummaryItems: AutomationConditionSummaryGroup[]
  actionSummaryItems: AutomationDryRunLine[]
  operatorPlan: AutomationDryRunOperatorPlan
  backendDryRunView: AutomationDryRunBackendView
  customerDryRunView: AutomationDryRunCustomerView
  beginnerGuideCards: AutomationDryRunBeginnerGuideCard[]
  quickFixActions?: AutomationDryRunQuickFixAction[]
  localBlockingErrors: AutomationDryRunLine[]
  dryRunResponseText: string
  isBackendDryRunLoading: boolean
  sceneActionOnly?: boolean
  backendRequestDisabled?: boolean
  backendRequestDisabledText?: string
}>(), {
  sceneActionOnly: false,
  backendRequestDisabled: false,
  backendRequestDisabledText: ''
})

defineEmits<{
  (event: 'refresh'): void
  (event: 'runBackendDryRun'): void
  (event: 'quickFix', key: string): void
}>()
</script>

<template>
  <NCard class="execution-contract-preview" size="small" :bordered="false" data-testid="automation-dry-run-preview">
    <NFlex vertical :size="10">
      <NAlert :type="customerDryRunView.alertType" :show-icon="false">
        <template #header>
          <NFlex align="center" :size="8">
            <span>{{ $t('generate.automationDryRunResult') }}</span>
            <NTag :type="customerDryRunView.tagType" round>
              {{ $t(getCustomerStatusLabelKey(customerDryRunView.status)) }}
            </NTag>
          </NFlex>
        </template>
        {{ $t(getCustomerStatusDescriptionKey(customerDryRunView.status)) }}
      </NAlert>
      <div class="dry-run-beginner-guide" :aria-label="$t('generate.automationDryRunBeginnerGuideTitle')">
        <div
          v-for="item in beginnerGuideCards"
          :key="item.key"
          class="dry-run-beginner-guide__card"
          :class="`dry-run-beginner-guide__card--${item.type}`"
        >
          <NTag size="small" :type="item.type" round>{{ $t(item.titleKey) }}</NTag>
          <strong>{{ $t(item.textKey) }}</strong>
          <span>{{ item.detail }}</span>
        </div>
      </div>
      <NCard
        v-if="quickFixActions?.length"
        size="small"
        embedded
        class="dry-run-quick-fix-section"
        data-testid="automation-dry-run-quick-fix-card"
      >
        <NFlex vertical :size="8">
          <strong>{{ $t('generate.automationDryRunQuickFixTitle') }}</strong>
          <div class="dry-run-quick-fix-grid">
            <div
              v-for="item in quickFixActions"
              :key="item.key"
              class="dry-run-quick-fix-action"
              :data-testid="`automation-dry-run-quick-fix-${item.key}`"
            >
              <div>
                <strong>{{ item.title }}</strong>
                <span>{{ item.desc }}</span>
              </div>
              <NButton
                size="small"
                :type="item.type || 'primary'"
                secondary
                :disabled="item.disabled"
                :data-testid="`automation-dry-run-quick-fix-button-${item.key}`"
                @click="$emit('quickFix', item.key)"
              >
                {{ item.buttonLabel }}
              </NButton>
            </div>
          </div>
        </NFlex>
      </NCard>
      <NFlex class="dry-run-closure-grid" :size="12">
        <NCard size="small" embedded class="dry-run-closure-card">
          <NFlex vertical :size="6">
            <strong>{{ $t('generate.automationDryRunBlockingErrors') }}</strong>
            <NEmpty
              v-if="customerDryRunView.blockingErrors.length === 0 && localBlockingErrors.length === 0"
              :description="$t('generate.automationDryRunNoBlockingErrors')"
            />
            <template v-else>
              <NAlert v-for="item in localBlockingErrors" :key="item.key" type="error" :show-icon="false">
                <template #header>{{ $t('generate.automationDryRunLocalExplanation') }}</template>
                {{ item.text }}
              </NAlert>
              <NAlert v-for="item in customerDryRunView.blockingErrors" :key="item.key" type="error" :show-icon="false">
                {{ item.text }}
              </NAlert>
            </template>
          </NFlex>
        </NCard>
        <NCard size="small" embedded class="dry-run-closure-card">
          <NFlex vertical :size="6">
            <strong>{{ $t('generate.automationDryRunWarnings') }}</strong>
            <NEmpty
              v-if="customerDryRunView.warnings.length === 0"
              :description="$t('generate.automationDryRunNoWarnings')"
            />
            <template v-else>
              <NAlert v-for="item in customerDryRunView.warnings" :key="item.key" type="warning" :show-icon="false">
                {{ item.text }}
              </NAlert>
            </template>
          </NFlex>
        </NCard>
        <NCard size="small" embedded class="dry-run-closure-card">
          <NFlex vertical :size="6">
            <strong>{{ $t('generate.automationDryRunReferenceCounts') }}</strong>
            <NEmpty
              v-if="customerDryRunView.referenceCounts.length === 0"
              :description="$t('generate.automationDryRunNoReferenceCounts')"
            />
            <NFlex v-else vertical :size="4">
              <span v-for="item in customerDryRunView.referenceCounts" :key="item.key" class="execution-preview-line">
                {{ item.text }}
              </span>
            </NFlex>
          </NFlex>
        </NCard>
        <NCard size="small" embedded class="dry-run-closure-card">
          <NFlex vertical :size="6">
            <strong>{{ $t('generate.automationDryRunNextSteps') }}</strong>
            <NEmpty
              v-if="customerDryRunView.nextSteps.length === 0"
              :description="$t('generate.automationDryRunNoNextSteps')"
            />
            <NFlex v-else vertical :size="4">
              <span v-for="item in customerDryRunView.nextSteps" :key="item.key" class="execution-preview-line">
                - {{ item.text }}
              </span>
            </NFlex>
          </NFlex>
        </NCard>
      </NFlex>
      <NAlert type="info" :show-icon="false">
        <template #header>{{ $t('generate.automationDryRunLocalExplanation') }}</template>
        {{ localStatusText }}
      </NAlert>
      <NCard size="small" embedded class="operator-plan-card">
        <NFlex vertical :size="8">
          <strong>{{ $t('generate.automationDryRunOperatorPlan') }}</strong>
          <div class="operator-plan-grid">
            <div class="operator-plan-section">
              <span class="operator-plan-title">{{ $t('generate.automationDryRunOperatorSource') }}</span>
              <span v-for="item in operatorPlan.source" :key="item.key" class="execution-preview-line">
                {{ item.text }}
              </span>
            </div>
            <div v-if="!sceneActionOnly" class="operator-plan-section">
              <span class="operator-plan-title">{{ $t('generate.automationDryRunOperatorConditions') }}</span>
              <span v-for="item in operatorPlan.conditions" :key="item.key" class="execution-preview-line">
                {{ item.text }}
              </span>
            </div>
            <div class="operator-plan-section">
              <span class="operator-plan-title">{{ $t('generate.automationDryRunOperatorActions') }}</span>
              <span v-for="item in operatorPlan.actions" :key="item.key" class="execution-preview-line">
                {{ item.text }}
              </span>
            </div>
            <div class="operator-plan-section">
              <span class="operator-plan-title">{{ $t('generate.automationDryRunOperatorLimits') }}</span>
              <span v-for="item in operatorPlan.limits" :key="item.key" class="execution-preview-line">
                {{ item.text }}
              </span>
            </div>
          </div>
        </NFlex>
      </NCard>
      <NAlert :type="backendAlertType" :show-icon="false">
        <template #header>{{ $t('generate.automationDryRunBackendTitle') }}</template>
        {{ backendStatusText }}
        <template v-if="backendError">: {{ backendError }}</template>
      </NAlert>
      <NFlex align="center" :size="8" wrap>
        <NTag v-if="customerDryRunView.canSave === false" type="error" round>
          {{ $t('generate.automationDryRunSaveBlocked') }}
        </NTag>
        <NTag v-else-if="customerDryRunView.canSave === true" type="success" round>
          {{ $t('generate.automationDryRunSaveAllowed') }}
        </NTag>
        <NTag v-if="!sceneActionOnly" type="info" round data-testid="automation-dry-run-condition-count">
          {{ $t('generate.automationDryRunConditions') }} {{ conditionGroupCount }}
          {{ $t('generate.automationDryRunGroups') }} / {{ conditionCount }} {{ $t('generate.automationDryRunRows') }}
        </NTag>
        <NTag type="info" round data-testid="automation-dry-run-action-count">
          {{ $t('generate.automationDryRunActions') }} {{ actionCount }} {{ $t('generate.automationDryRunRows') }}
        </NTag>
        <NTag type="warning" round>{{ $t('generate.automationDryRunNoExecutionClaim') }}</NTag>
      </NFlex>
      <NFlex v-if="backendDryRunView.metrics.length > 0" class="backend-dry-run-grid" :size="12">
        <NCard size="small" embedded class="backend-dry-run-card">
          <NFlex vertical :size="6">
            <strong>{{ $t('generate.automationDryRunDiagnostics') }}</strong>
            <NFlex v-for="item in backendDryRunView.metrics" :key="item.key" align="center" :size="6">
              <NTag size="small" type="info">预演</NTag>
              <span class="execution-preview-line">{{ item.text }}</span>
            </NFlex>
            <NFlex v-if="backendDryRunView.diagnostics.length > 0" vertical :size="6">
              <NAlert
                v-for="item in backendDryRunView.diagnostics"
                :key="item.key"
                :type="item.type"
                :show-icon="false"
              >
                <template #header>{{ item.scope }}</template>
                {{ item.message }}
              </NAlert>
            </NFlex>
          </NFlex>
        </NCard>
        <NCard size="small" embedded class="backend-dry-run-card">
          <NFlex vertical :size="6">
            <strong>{{ $t('generate.automationDryRunExecutionContract') }}</strong>
            <NEmpty
              v-if="
                backendDryRunView.conditionTypes.length === 0 &&
                backendDryRunView.actionTypes.length === 0 &&
                backendDryRunView.targetKinds.length === 0
              "
              :description="$t('generate.automationDryRunRunForBreakdown')"
            />
            <NFlex v-else vertical :size="4">
              <span
                v-for="item in [
                  ...backendDryRunView.conditionTypes,
                  ...backendDryRunView.actionTypes,
                  ...backendDryRunView.targetKinds
                ]"
                :key="item.key"
                class="execution-preview-line"
              >
                {{ item.text }}
              </span>
            </NFlex>
            <NFlex v-if="backendDryRunView.nextSteps.length > 0" vertical :size="4">
              <strong>{{ $t('generate.automationDryRunNextSteps') }}</strong>
              <span v-for="item in backendDryRunView.nextSteps" :key="item.key" class="execution-preview-line">
                - {{ item.text }}
              </span>
            </NFlex>
          </NFlex>
        </NCard>
      </NFlex>
      <NCard
        v-if="(backendDryRunView.trace?.steps?.length ?? 0) > 0"
        size="small"
        embedded
        class="dry-run-trace-card"
        data-testid="automation-dry-run-trace-card"
      >
        <NFlex vertical :size="8">
          <NFlex align="center" :size="8" wrap>
            <strong>{{ $t('generate.automationDryRunTraceTitle') }}</strong>
            <NTag size="small" type="info" round>
              {{ backendDryRunView.trace.stepCount }} {{ $t('generate.automationDryRunTraceSteps') }}
            </NTag>
            <NTag v-if="backendDryRunView.trace.isSimulation" size="small" type="warning" round>
              {{ $t('generate.automationDryRunTraceSimulationTag') }}
            </NTag>
          </NFlex>
          <span v-if="backendDryRunView.trace.explanation" class="execution-preview-line">
            {{ backendDryRunView.trace.explanation }}
          </span>
          <ol class="dry-run-trace-list">
            <li
              v-for="step in backendDryRunView.trace.steps"
              :key="step.key"
              class="dry-run-trace-step"
              :class="`dry-run-trace-step--${step.statusType}`"
              :data-testid="`automation-dry-run-trace-step-${step.index}`"
            >
              <NFlex align="center" :size="6" wrap>
                <NTag size="small" :type="step.statusType" round>{{ step.index }}</NTag>
                <NTag size="small" :type="step.phase === 'action' ? 'primary' : 'success'">
                  {{ step.phase === 'action'
                    ? $t('generate.automationDryRunTracePhaseAction')
                    : $t('generate.automationDryRunTracePhaseTrigger') }}
                </NTag>
                <strong>{{ step.label }}</strong>
                <NTag v-if="step.kind" size="small" type="default">{{ step.kind }}</NTag>
              </NFlex>
              <span class="execution-preview-line">{{ step.detail }}</span>
              <span
                v-for="(note, noteIndex) in step.notes"
                :key="`${step.key}-note-${noteIndex}`"
                class="dry-run-trace-note"
              >
                - {{ note }}
              </span>
            </li>
          </ol>
        </NFlex>
      </NCard>
      <NFlex class="execution-preview-grid" :size="12">
        <NFlex vertical :size="6" class="execution-preview-column">
          <template v-if="!sceneActionOnly">
            <strong>{{ $t('generate.automationDryRunConditionSummary') }}</strong>
            <NEmpty v-if="conditionSummaryItems.length === 0" :description="$t('generate.automationDryRunNoLocal')" />
            <NFlex v-else vertical :size="6">
              <NFlex v-for="(group, groupIndex) in conditionSummaryItems" :key="group.key" vertical :size="4">
                <NTag size="small" type="success">
                  {{ $t('generate.automationDryRunConditionGroup') }} #{{ groupIndex + 1 }}
                </NTag>
                <span v-for="line in group.lines" :key="line.key" class="execution-preview-line">
                  {{ line.text }}
                </span>
              </NFlex>
            </NFlex>
          </template>
          <template v-else>
            <strong>{{ $t('generate.sceneDryRunActionOnlyTitle') }}</strong>
            <NAlert type="info" :show-icon="false">
              {{ $t('generate.sceneDryRunActionOnlyHint') }}
            </NAlert>
          </template>
        </NFlex>
        <NFlex vertical :size="6" class="execution-preview-column">
          <strong>{{ $t('generate.automationDryRunActionSummary') }}</strong>
          <NEmpty v-if="actionSummaryItems.length === 0" :description="$t('generate.automationDryRunNoLocal')" />
          <NFlex v-else vertical :size="4">
            <span v-for="(item, actionIndex) in actionSummaryItems" :key="item.key" class="execution-preview-line">
              #{{ actionIndex + 1 }} {{ item.text }}
            </span>
          </NFlex>
        </NFlex>
      </NFlex>
      <pre v-if="dryRunResponseText" class="dry-run-response">{{ dryRunResponseText }}</pre>
      <NAlert v-if="backendRequestDisabled && backendRequestDisabledText" type="warning" :show-icon="false">
        {{ backendRequestDisabledText }}
      </NAlert>
      <NFlex justify="end" :size="8">
        <NButton secondary data-testid="automation-dry-run-refresh-local" @click="$emit('refresh')">
          {{ $t('generate.automationDryRunRefreshLocal') }}
        </NButton>
        <NButton
          secondary
          type="primary"
          :loading="isBackendDryRunLoading"
          :disabled="backendRequestDisabled"
          data-testid="automation-dry-run-request-backend"
          @click="$emit('runBackendDryRun')"
        >
          {{ $t('generate.automationDryRunRequestBackend') }}
        </NButton>
      </NFlex>
    </NFlex>
  </NCard>
</template>

<style scoped>
.execution-contract-preview {
  margin-top: 12px;
  background: rgba(32, 128, 240, 0.06);
}

.execution-preview-grid {
  align-items: stretch;
}

.backend-dry-run-grid {
  align-items: stretch;
}

.operator-plan-card {
  background: rgba(24, 160, 88, 0.07);
}

.dry-run-beginner-guide {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.dry-run-beginner-guide__card {
  display: grid;
  gap: 6px;
  min-width: 0;
  padding: 10px;
  border: 1px solid #d7dde8;
  border-radius: 8px;
  background: #fff;
}

.dry-run-beginner-guide__card--success {
  border-color: rgba(24, 160, 88, 0.36);
  background: rgba(24, 160, 88, 0.06);
}

.dry-run-beginner-guide__card--warning {
  border-color: rgba(240, 160, 32, 0.4);
  background: rgba(240, 160, 32, 0.08);
}

.dry-run-beginner-guide__card--error {
  border-color: rgba(208, 48, 80, 0.34);
  background: rgba(208, 48, 80, 0.06);
}

.dry-run-beginner-guide__card strong {
  color: #1f2937;
  font-size: 13px;
  line-height: 1.35;
}

.dry-run-beginner-guide__card span {
  min-width: 0;
  overflow-wrap: anywhere;
  color: #4b5563;
  line-height: 1.45;
}

.dry-run-quick-fix-section {
  background: rgba(32, 128, 240, 0.06);
}

.dry-run-quick-fix-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.dry-run-quick-fix-action {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  min-width: 0;
  padding: 10px;
  border: 1px solid rgba(32, 128, 240, 0.22);
  border-radius: 8px;
  background: #fff;
}

.dry-run-quick-fix-action > div {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.dry-run-quick-fix-action strong {
  color: #1f2937;
  font-size: 13px;
}

.dry-run-quick-fix-action span {
  overflow-wrap: anywhere;
  color: #4b5563;
  line-height: 1.45;
}

.operator-plan-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.operator-plan-section {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  padding: 10px;
  border: 1px solid rgba(24, 160, 88, 0.22);
  border-radius: 6px;
  background: #fff;
}

.operator-plan-title {
  color: #0f172a;
  font-weight: 600;
}

.dry-run-closure-grid {
  align-items: stretch;
}

.dry-run-closure-card {
  flex: 1 1 240px;
  min-width: 0;
}

.backend-dry-run-card {
  flex: 1 1 320px;
  min-width: 0;
}

.execution-preview-column {
  flex: 1 1 320px;
  min-width: 0;
}

.execution-preview-line {
  display: block;
  min-width: 0;
  overflow-wrap: anywhere;
  line-height: 1.5;
}

.dry-run-trace-card {
  background: rgba(32, 128, 240, 0.05);
}

.dry-run-trace-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.dry-run-trace-step {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  padding: 10px;
  border: 1px solid #d7dde8;
  border-left-width: 3px;
  border-radius: 6px;
  background: #fff;
}

.dry-run-trace-step--success {
  border-left-color: rgba(24, 160, 88, 0.7);
}

.dry-run-trace-step--warning {
  border-left-color: rgba(240, 160, 32, 0.75);
}

.dry-run-trace-step--error {
  border-left-color: rgba(208, 48, 80, 0.7);
}

.dry-run-trace-step--info {
  border-left-color: rgba(32, 128, 240, 0.6);
}

.dry-run-trace-step strong {
  color: #1f2937;
  font-size: 13px;
}

.dry-run-trace-note {
  min-width: 0;
  overflow-wrap: anywhere;
  color: #6b7280;
  font-size: 12px;
  line-height: 1.45;
}

.dry-run-response {
  max-height: 180px;
  overflow: auto;
  border-radius: 6px;
  padding: 10px;
  background: rgba(0, 0, 0, 0.06);
  white-space: pre-wrap;
  word-break: break-word;
}

@media (max-width: 900px) {
  .dry-run-beginner-guide {
    grid-template-columns: 1fr;
  }

  .operator-plan-grid {
    grid-template-columns: 1fr;
  }

  .dry-run-quick-fix-grid,
  .dry-run-quick-fix-action {
    grid-template-columns: 1fr;
  }
}
</style>
