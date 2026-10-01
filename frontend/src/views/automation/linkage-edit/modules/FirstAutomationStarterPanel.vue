<!--
文件用途：联动编辑页顶部的「首台设备第一条遥测规则」引导面板（推荐遥测、条件/动作草稿、四步清单）。
核心逻辑：状态与动作全部来自父级 useFirstAutomationStarter，本组件只做展示与转发点击。
-->
<script setup lang="ts">
import { NAlert, NButton } from 'naive-ui'
import { $t } from '@/locales'
import type { useFirstAutomationStarter } from './useFirstAutomationStarter'

defineProps<{ starter: ReturnType<typeof useFirstAutomationStarter> }>()
</script>

<template>
  <NAlert type="info" :show-icon="false" class="mb-12px">
    <div class="first-automation-starter">
      <div>
        <div class="first-automation-starter__title">
          {{ $t('custom.automation.firstRuleStarterTitle') }}
        </div>
        <div class="first-automation-starter__desc">
          {{ starter.firstAutomationStarterDesc.value }}
        </div>
      </div>
      <div class="first-automation-starter__steps">
        <span>{{ starter.firstAutomationStarterConditionText.value }}</span>
        <span>{{ $t('custom.automation.firstRuleActionStep') }}</span>
      </div>
      <div class="first-automation-telemetry-guide">
        <div>
          <div class="first-automation-telemetry-guide__title">
            {{ $t('custom.automation.firstRuleTelemetryGuideTitle') }}
          </div>
          <div class="first-automation-telemetry-guide__desc">
            {{ $t('custom.automation.firstRuleTelemetryGuideDesc') }}
          </div>
        </div>
        <div class="first-automation-telemetry-guide__cards">
          <div
            v-for="item in starter.firstAutomationTelemetryRecommendation.value.cards"
            :key="item.key"
            class="first-automation-telemetry-guide__card"
            :class="`first-automation-telemetry-guide__card--${item.status}`"
          >
            <span>{{ item.title }}</span>
            <strong>{{ item.value }}</strong>
          </div>
        </div>
        <div class="first-automation-telemetry-guide__hint">
          {{ starter.firstAutomationTelemetryRecommendation.value.conditionHint }}
        </div>
        <div
          class="first-automation-telemetry-guide__next"
          :class="`first-automation-telemetry-guide__next--${starter.firstAutomationTelemetryRecommendation.value.nextAction.status}`"
        >
          <strong>{{ starter.firstAutomationTelemetryRecommendation.value.nextAction.title }}</strong>
          <span>{{ starter.firstAutomationTelemetryRecommendation.value.nextAction.desc }}</span>
        </div>
        <div
          class="first-automation-telemetry-guide__draft"
          :class="`first-automation-telemetry-guide__draft--${starter.firstAutomationRecommendedConditionDraft.value.status}`"
        >
          <div>
            <strong>{{ starter.firstAutomationRecommendedConditionDraft.value.title }}</strong>
            <span>{{ starter.firstAutomationRecommendedConditionDraft.value.desc }}</span>
            <small v-if="starter.firstAutomationRecommendedConditionApplied.value">
              {{ $t('custom.automation.firstRuleRecommendedConditionApplied') }}
            </small>
          </div>
          <NButton
            size="small"
            type="primary"
            secondary
            :disabled="!starter.firstAutomationRecommendedConditionDraft.value.available"
            @click="starter.applyFirstAutomationRecommendedCondition"
          >
            {{ $t('custom.automation.firstRuleApplyRecommendedCondition') }}
          </NButton>
        </div>
        <div class="first-automation-telemetry-guide__draft first-automation-telemetry-guide__draft--action">
          <div>
            <strong>{{ starter.firstAutomationRecommendedActionDraft.value.title }}</strong>
            <span>{{ starter.firstAutomationRecommendedActionDraft.value.desc }}</span>
            <small v-if="starter.firstAutomationRecommendedActionApplied.value">
              {{ $t('custom.automation.firstRuleRecommendedActionApplied') }}
            </small>
          </div>
          <div class="first-automation-telemetry-guide__draft-actions">
            <NButton size="small" type="primary" secondary @click="starter.applyFirstAutomationRecommendedAction">
              {{ $t('custom.automation.firstRuleApplyRecommendedAction') }}
            </NButton>
            <NButton size="small" tertiary @click="starter.openFirstAutomationAlarmCreator">
              {{ $t('custom.automation.firstRuleCreateAlarmTarget') }}
            </NButton>
          </div>
        </div>
      </div>
      <div class="first-automation-checklist" :aria-label="$t('custom.automation.firstRuleChecklistTitle')">
        <div
          v-for="(item, index) in starter.firstAutomationStarterChecklist.value"
          :key="item.key"
          class="first-automation-checklist__item"
          :class="`first-automation-checklist__item--${item.status}`"
        >
          <span class="first-automation-checklist__marker">{{ index + 1 }}</span>
          <span class="first-automation-checklist__body">
            <strong>{{ item.title }}</strong>
            <small>{{ item.desc }}</small>
          </span>
        </div>
      </div>
    </div>
  </NAlert>
</template>

<style scoped>
.first-automation-starter {
  display: grid;
  gap: 10px;
}

.first-automation-starter__title {
  font-size: var(--font-size-lg);
  font-weight: 700;
  color: var(--text-color-1);
}

.first-automation-starter__desc {
  margin-top: 4px;
  color: var(--text-color-2);
  line-height: 1.5;
}

.first-automation-starter__steps {
  display: grid;
  gap: 6px;
  color: var(--text-color-2);
  line-height: 1.55;
}

.first-automation-telemetry-guide {
  display: grid;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--card-color);
}

.first-automation-telemetry-guide__title {
  font-size: var(--font-size-base);
  font-weight: 700;
  color: var(--text-color-1);
}

.first-automation-telemetry-guide__desc {
  margin-top: 3px;
  color: var(--text-color-2);
  line-height: 1.45;
}

.first-automation-telemetry-guide__cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.first-automation-telemetry-guide__card {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 8px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--action-color);
}

.first-automation-telemetry-guide__card--ready {
  border-color: rgb(var(--success-color) / 0.5);
  background: rgb(var(--success-color) / 0.07);
}

.first-automation-telemetry-guide__card span {
  color: var(--text-color-3);
  font-size: var(--font-size-caption);
}

.first-automation-telemetry-guide__card strong {
  overflow-wrap: anywhere;
  color: var(--text-color-1);
  font-size: var(--font-size-secondary);
}

.first-automation-telemetry-guide__hint {
  color: var(--text-color-2);
  line-height: 1.5;
}

.first-automation-telemetry-guide__next {
  display: grid;
  gap: 3px;
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--action-color);
  color: var(--text-color-2);
  line-height: 1.45;
}

.first-automation-telemetry-guide__next--ready {
  border-color: rgb(var(--success-color) / 0.5);
  background: rgb(var(--success-color) / 0.07);
}

.first-automation-telemetry-guide__next--missing {
  border-color: rgb(var(--warning-color) / 0.6);
  background: rgb(var(--warning-color) / 0.1);
}

.first-automation-telemetry-guide__next strong {
  color: var(--text-color-1);
  font-size: var(--font-size-secondary);
}

.first-automation-telemetry-guide__next span {
  font-size: var(--font-size-secondary);
}

.first-automation-telemetry-guide__draft {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  padding: 9px 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--action-color);
}

.first-automation-telemetry-guide__draft--ready {
  border-color: rgb(var(--info-color) / 0.55);
  background: rgb(var(--info-color) / 0.08);
}

.first-automation-telemetry-guide__draft--missing {
  border-color: rgb(var(--warning-color) / 0.6);
  background: rgb(var(--warning-color) / 0.1);
}

.first-automation-telemetry-guide__draft--action {
  border-color: rgb(var(--success-color) / 0.5);
  background: rgb(var(--success-color) / 0.07);
}

.first-automation-telemetry-guide__draft > div:first-child {
  display: grid;
  gap: 3px;
  min-width: 0;
  color: var(--text-color-2);
  line-height: 1.45;
}

.first-automation-telemetry-guide__draft strong {
  color: var(--text-color-1);
  font-size: var(--font-size-secondary);
}

.first-automation-telemetry-guide__draft span,
.first-automation-telemetry-guide__draft small {
  overflow-wrap: anywhere;
  font-size: var(--font-size-secondary);
}

.first-automation-telemetry-guide__draft small {
  color: rgb(var(----color));
}

.first-automation-telemetry-guide__draft-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.first-automation-checklist {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.first-automation-checklist__item {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 8px;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--action-color);
}

.first-automation-checklist__item--done {
  border-color: rgb(var(--success-color) / 0.55);
  background: rgb(var(--success-color) / 0.06);
}

.first-automation-checklist__item--active {
  border-color: rgb(var(--info-color) / 0.55);
  background: rgb(var(--info-color) / 0.08);
}

.first-automation-checklist__marker {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: var(--radius-pill);
  background: var(--divider-color);
  color: var(--text-color-1);
  font-size: var(--font-size-caption);
  font-weight: 700;
}

.first-automation-checklist__item--done .first-automation-checklist__marker {
  background: rgb(var(--success-color));
  color: white;
}

.first-automation-checklist__item--active .first-automation-checklist__marker {
  background: rgb(var(--info-color));
  color: white;
}

.first-automation-checklist__body {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.first-automation-checklist__body strong {
  font-size: var(--font-size-secondary);
  color: var(--text-color-1);
}

.first-automation-checklist__body small {
  color: var(--text-color-2);
  line-height: 1.45;
}

@media (max-width: 960px) {
  .first-automation-telemetry-guide__cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .first-automation-checklist {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .first-automation-telemetry-guide__cards,
  .first-automation-checklist {
    grid-template-columns: 1fr;
  }

  .first-automation-telemetry-guide__draft {
    grid-template-columns: 1fr;
  }

  .first-automation-telemetry-guide__draft-actions {
    justify-content: flex-start;
  }
}
</style>
