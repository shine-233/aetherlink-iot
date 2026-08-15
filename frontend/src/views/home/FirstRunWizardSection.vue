<script setup lang="ts">
import { $t } from '@/locales'

interface FirstRunWizardStep {
  id: string
  order: number
  title: string
  evidence: string
  actionLabel: string
  status: 'done' | 'active' | 'todo' | string
}

defineProps<{
  ready: boolean
  steps: FirstRunWizardStep[]
}>()

const emit = defineEmits<{
  openStep: [step: FirstRunWizardStep]
}>()

const isStepDisabled = (step: FirstRunWizardStep) => step.status === 'todo'
</script>

<template>
  <div class="first-run-wizard">
    <div class="first-run-wizard__head">
      <div class="min-w-0">
        <div class="font-600">{{ $t('custom.home.firstDevice.wizard.title') }}</div>
        <div class="mt-3px text-12px line-height-18px text-gray-500">
          {{ $t('custom.home.firstDevice.wizard.desc') }}
        </div>
      </div>
      <n-tag size="small" round :bordered="false" :type="ready ? 'success' : 'warning'">
        {{
          ready
            ? $t('custom.home.firstDevice.wizard.loopDone')
            : $t('custom.home.firstDevice.wizard.loopPending')
        }}
      </n-tag>
    </div>

    <div class="first-run-wizard-grid">
      <button
        v-for="step in steps"
        :key="step.id"
        type="button"
        class="first-run-wizard-step"
        :class="`first-run-wizard-step--${step.status}`"
        :disabled="isStepDisabled(step)"
        @click="emit('openStep', step)"
      >
        <span>{{ step.order }}</span>
        <strong>{{ step.title }}</strong>
        <small>{{ step.evidence }}</small>
        <em>{{ step.actionLabel }}</em>
      </button>
    </div>
  </div>
</template>

<style scoped>
.first-run-wizard {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
}

.first-run-wizard__head {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.first-run-wizard-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
}

.first-run-wizard-step {
  min-height: 126px;
  display: grid;
  grid-template-rows: 24px auto 1fr 20px;
  gap: 6px;
  padding: 10px;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  background: #fff;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.first-run-wizard-step:disabled {
  cursor: not-allowed;
}

.first-run-wizard-step span {
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: #94a3b8;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
}

.first-run-wizard-step strong {
  min-width: 0;
  font-size: 13px;
  line-height: 18px;
  overflow-wrap: anywhere;
}

.first-run-wizard-step small {
  min-width: 0;
  color: #6b7280;
  font-size: 12px;
  line-height: 17px;
  overflow-wrap: anywhere;
}

.first-run-wizard-step em {
  min-width: 0;
  color: #2563eb;
  font-size: 12px;
  font-style: normal;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.first-run-wizard-step--done {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.first-run-wizard-step--done span {
  background: #16a34a;
}

.first-run-wizard-step--active {
  border-color: #f59e0b;
  background: #fffbeb;
}

.first-run-wizard-step--active span {
  background: #f59e0b;
}

@media (min-width: 640px) {
  .first-run-wizard__head {
    flex-direction: row;
    align-items: flex-start;
    justify-content: space-between;
  }
}

@media (max-width: 900px) {
  .first-run-wizard-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
