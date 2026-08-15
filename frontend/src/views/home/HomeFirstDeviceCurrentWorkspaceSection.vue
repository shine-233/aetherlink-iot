<script setup lang="ts">
import { $t } from '@/locales'

defineProps<{
  title: string
  description: string
  successSignal: string
  currentStep: any
  ready: boolean
  actionLabel: string
  actionDisabled: boolean
  actionLoading: boolean
  steps: any[]
}>()

const emit = defineEmits<{
  runCurrentFocusedQuickstartAction: []
  focusCurrentFocusedQuickstartSection: []
  openFirstDeviceSupportSummaryPreview: []
}>()
</script>

<template>
  <div class="flex flex-col gap-10px sm:flex-row sm:items-start sm:justify-between">
    <div class="min-w-0">
      <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.workspace.label') }}</div>
      <div class="mt-2px font-600">{{ title }}</div>
      <div class="mt-4px text-12px line-height-18px text-gray-500">
        {{ description }}
      </div>
      <div class="mt-6px rounded-6px border border-dashed border-gray-300 px-10px py-8px text-12px line-height-18px text-gray-600">
        {{ successSignal }}
      </div>
      <div
        v-if="currentStep?.disabled && !ready"
        class="mt-5px text-12px text-orange-600"
      >
        {{ currentStep.description }}
      </div>
    </div>
    <div class="flex shrink-0 flex-wrap gap-8px">
      <n-button
        size="small"
        type="primary"
        :disabled="actionDisabled"
        :loading="actionLoading"
        @click="emit('runCurrentFocusedQuickstartAction')"
      >
        {{ actionLabel }}
      </n-button>
      <n-button size="small" secondary @click="emit('focusCurrentFocusedQuickstartSection')">
        {{ $t('custom.home.firstDevice.workspace.locateStep') }}
      </n-button>
      <n-button size="small" ghost @click="emit('openFirstDeviceSupportSummaryPreview')">
        {{ $t('custom.home.firstDevice.workspace.stuckCopySupport') }}
      </n-button>
    </div>
  </div>
  <div class="mt-10px grid gap-8px md:grid-cols-5">
    <div
      v-for="step in steps"
      :key="step.key"
      class="first-device-quickstart-step"
      :class="`first-device-quickstart-step--${step.status}`"
    >
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-6px">
          <strong>{{ step.title }}</strong>
          <n-tag size="tiny" round :bordered="false" :type="step.statusType">{{ step.statusLabel }}</n-tag>
        </div>
        <div class="mt-3px text-12px line-height-18px text-gray-500">
          {{ step.status === 'active' ? step.description : step.actionLabel }}
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.first-device-quickstart-step {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  background: #fff;
}

.first-device-quickstart-step--done {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.first-device-quickstart-step--active {
  border-color: #fed7aa;
  background: #fff7ed;
}

.first-device-quickstart-step strong {
  overflow-wrap: anywhere;
  color: #0f172a;
}
</style>
