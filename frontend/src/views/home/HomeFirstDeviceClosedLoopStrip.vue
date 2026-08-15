<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'

const props = defineProps<{
  steps: any[]
}>()

const emit = defineEmits<{
  runStep: [step: any]
}>()

const doneCount = computed(() => props.steps.filter((step: any) => step.state === 'done').length)
const percent = computed(() =>
  props.steps.length ? Math.round((doneCount.value / props.steps.length) * 100) : 0
)
</script>

<template>
  <div class="first-device-closed-loop-strip">
    <div class="flex flex-col gap-10px lg:flex-row lg:items-center lg:justify-between">
      <div class="min-w-0">
        <div class="text-12px font-600 uppercase text-blue-700">/first-device</div>
        <div class="mt-3px text-18px font-700">{{ $t('custom.home.firstDevice.common.onboardFirstDevice') }}</div>
        <div class="mt-4px text-12px line-height-18px text-gray-600">
          {{ $t('custom.home.firstDevice.strip.desc') }}
        </div>
      </div>
      <div class="first-device-closed-loop-score">
        <span>{{ $t('custom.home.firstDevice.strip.progress') }}</span>
        <strong>{{ doneCount }}/{{ steps.length }}</strong>
        <small>{{ percent }}%</small>
      </div>
    </div>
    <div class="mt-12px first-device-closed-loop-grid">
      <button
        v-for="step in steps"
        :key="step.key"
        type="button"
        class="first-device-closed-loop-step"
        :class="`first-device-closed-loop-step--${step.state}`"
        :disabled="step.loading"
        :aria-disabled="step.disabled"
        @click="emit('runStep', step)"
      >
        <span class="first-device-closed-loop-step__order">{{ step.order }}</span>
        <span class="min-w-0">
          <span class="first-device-closed-loop-step__title">{{ step.title }}</span>
          <small>{{ step.detail }}</small>
          <span class="first-device-closed-loop-step__action">
            {{ step.loading ? $t('custom.home.firstDevice.strip.processing') : step.actionLabel }}
          </span>
        </span>
        <n-tag size="tiny" round :bordered="false" :type="step.stateType">{{ step.stateLabel }}</n-tag>
      </button>
    </div>
  </div>
</template>

<style scoped>
.first-device-closed-loop-strip {
  margin-bottom: 16px;
  padding: 14px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background: linear-gradient(180deg, #eff6ff 0%, #ffffff 100%);
}

.first-device-closed-loop-score {
  display: grid;
  min-width: 104px;
  padding: 10px 12px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #fff;
  text-align: right;
}

.first-device-closed-loop-score span,
.first-device-closed-loop-score small {
  color: #64748b;
  font-size: 11px;
}

.first-device-closed-loop-score strong {
  color: #0f172a;
  font-size: 18px;
  line-height: 1.2;
}

.first-device-closed-loop-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 8px;
}

.first-device-closed-loop-step {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 8px;
  align-items: start;
  min-width: 0;
  min-height: 116px;
  padding: 10px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
  text-align: left;
  cursor: pointer;
}

.first-device-closed-loop-step:disabled {
  cursor: progress;
  opacity: 0.7;
}

.first-device-closed-loop-step[aria-disabled='true'] {
  border-style: dashed;
}

.first-device-closed-loop-step--done {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.first-device-closed-loop-step--active {
  border-color: #fdba74;
  background: #fff7ed;
}

.first-device-closed-loop-step__order {
  color: #2563eb;
  font-size: 11px;
  font-weight: 700;
}

.first-device-closed-loop-step__title,
.first-device-closed-loop-step small,
.first-device-closed-loop-step__action {
  display: block;
  overflow-wrap: anywhere;
}

.first-device-closed-loop-step__title {
  color: #0f172a;
  font-size: 13px;
  font-weight: 700;
}

.first-device-closed-loop-step small {
  margin-top: 4px;
  color: #64748b;
  font-size: 11px;
  line-height: 1.45;
}

.first-device-closed-loop-step__action {
  margin-top: 7px;
  color: #1d4ed8;
  font-size: 11px;
  font-weight: 600;
}

@media (max-width: 1100px) {
  .first-device-closed-loop-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .first-device-closed-loop-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .first-device-closed-loop-step {
    min-height: auto;
  }
}
</style>
