<script setup lang="ts">
import { $t } from '@/locales'
import {
  getHomeGuideStepActionLabel,
  isHomeGuideStepActionDisabled,
  shouldExpandHomeGuideStep,
  type HomeFirstDeviceCoreGuideSummary,
  type HomeFirstDeviceGuideStep
} from './homeFirstDeviceView'

defineProps<{
  firstDevice: any
  ready: boolean
  coreGuideSummary: HomeFirstDeviceCoreGuideSummary
  coreGuideSteps: HomeFirstDeviceGuideStep[]
  nextGuideSteps: HomeFirstDeviceGuideStep[]
  resumeText: string
  firstDeviceLoading: boolean
  deploymentHealthLoading: boolean
  automationGuideLoading: boolean
}>()

const emit = defineEmits<{
  openHomeGuideStep: [step: HomeFirstDeviceGuideStep]
  refreshHomeGuideProgress: []
}>()

type GuideStepTagType = 'default' | 'primary' | 'info' | 'success' | 'warning' | 'error'
const guideStepTagType = (step: HomeFirstDeviceGuideStep) => step.statusType as GuideStepTagType | undefined
</script>

<template>
  <n-card :bordered="false" class="rounded-8px">
    <div class="flex flex-col gap-16px">
      <div>
        <div class="text-18px font-600">{{ $t('custom.home.firstDevice.common.onboardFirstDevice') }}</div>
        <div class="mt-6px text-14px text-gray-500">
          {{ $t('custom.home.firstDevice.guide.desc') }}
        </div>
      </div>

      <n-alert :type="ready ? 'success' : firstDevice ? 'warning' : 'info'" :show-icon="false">
        <template v-if="ready">
          {{ $t('custom.home.firstDevice.guide.readyAlert', { name: firstDevice?.name }) }}
        </template>
        <template v-else-if="firstDevice">
          {{ $t('custom.home.firstDevice.guide.foundAlert', { name: firstDevice.name }) }}
        </template>
        <template v-else>{{ $t('custom.home.firstDevice.guide.emptyAlert') }}</template>
      </n-alert>

      <div class="home-guide-summary rounded-8px border px-14px py-12px">
        <div class="flex flex-col gap-8px sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <div class="text-15px font-600">{{ coreGuideSummary.headline }}</div>
            <div class="mt-4px text-13px line-height-20px text-gray-500">
              {{ coreGuideSummary.description }}
            </div>
          </div>
          <div class="shrink-0 text-right text-12px text-gray-500">
            {{
              $t('custom.home.firstDevice.guide.doneCount', {
                done: coreGuideSummary.doneCount,
                total: coreGuideSummary.totalCount
              })
            }}
          </div>
        </div>
        <n-progress
          class="mt-10px"
          type="line"
          :percentage="coreGuideSummary.percent"
          :height="8"
          :border-radius="4"
          :fill-border-radius="4"
          :show-indicator="false"
          status="success"
        />
        <div v-if="resumeText" class="mt-8px text-12px text-gray-500">
          {{ resumeText }}
        </div>
        <div v-if="coreGuideSummary.nextStep" class="mt-10px flex flex-wrap gap-8px">
          <n-button size="small" type="primary" @click="emit('openHomeGuideStep', coreGuideSummary.nextStep)">
            {{ coreGuideSummary.nextStep.action }}
          </n-button>
          <n-button
            size="small"
            quaternary
            :loading="firstDeviceLoading || deploymentHealthLoading || automationGuideLoading"
            @click="emit('refreshHomeGuideProgress')"
          >
            {{ $t('custom.home.firstDevice.guide.refreshProgress') }}
          </n-button>
        </div>
      </div>

      <div class="grid gap-12px">
        <div
          v-for="(step, index) in coreGuideSteps"
          :key="step.route || step.id || index"
          class="home-guide-step flex flex-col gap-10px rounded-8px border px-14px py-12px sm:flex-row sm:items-center sm:justify-between"
          :class="[
            `home-guide-step--${step.status}`,
            { 'home-guide-step--compact': !shouldExpandHomeGuideStep(step, coreGuideSummary) }
          ]"
        >
          <div class="min-w-0 flex gap-12px">
            <div
              class="home-guide-step-index h-28px w-28px shrink-0 flex items-center justify-center rounded-full text-13px text-white font-600"
            >
              {{ index + 1 }}
            </div>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-8px">
                <div class="text-15px font-600">{{ step.title }}</div>
                <n-tag size="small" round :bordered="false" :type="guideStepTagType(step)">
                  {{ step.statusLabel }}
                </n-tag>
              </div>
              <div
                v-if="shouldExpandHomeGuideStep(step, coreGuideSummary)"
                class="mt-4px text-13px line-height-20px text-gray-500"
              >
                {{ step.description }}
              </div>
              <div v-else class="mt-3px text-12px text-gray-500">
                {{ step.action }}
              </div>
            </div>
          </div>
          <n-button
            size="small"
            type="primary"
            ghost
            class="shrink-0"
            :disabled="isHomeGuideStepActionDisabled(step)"
            @click="emit('openHomeGuideStep', step)"
          >
            {{ getHomeGuideStepActionLabel(step) }}
          </n-button>
        </div>
      </div>

      <div v-if="nextGuideSteps.length" class="home-guide-followup rounded-8px border px-14px py-12px">
        <div class="text-14px font-600">{{ $t('custom.home.firstDevice.guide.followupTitle') }}</div>
        <div class="mt-4px text-12px line-height-18px text-gray-500">
          {{ $t('custom.home.firstDevice.guide.followupDesc') }}
        </div>
        <div class="mt-10px grid gap-8px">
          <div
            v-for="step in nextGuideSteps"
            :key="step.route || step.id"
            class="home-guide-followup-step flex flex-col gap-8px rounded-6px border px-12px py-10px sm:flex-row sm:items-center sm:justify-between"
          >
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-8px">
                <div class="text-14px font-600">{{ step.title }}</div>
                <n-tag size="small" round :bordered="false" :type="guideStepTagType(step)">
                  {{ step.statusLabel }}
                </n-tag>
              </div>
              <div class="mt-3px text-12px line-height-18px text-gray-500">{{ step.description }}</div>
            </div>
            <n-button
              size="small"
              ghost
              type="primary"
              :disabled="isHomeGuideStepActionDisabled(step)"
              @click="emit('openHomeGuideStep', step)"
            >
              {{ getHomeGuideStepActionLabel(step) }}
            </n-button>
          </div>
        </div>
      </div>
    </div>
  </n-card>
</template>

<style scoped>
.home-guide-summary {
  border-color: #dbeafe;
  background: #eff6ff;
}

.home-guide-followup {
  border-color: #e2e8f0;
  background: #f8fafc;
}

.home-guide-followup-step {
  border-color: #e2e8f0;
  background: #fff;
}

.home-guide-step {
  border-color: #e5e7eb;
  background: #fff;
}

.home-guide-step-index {
  background: #94a3b8;
}

.home-guide-step--done {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.home-guide-step--done .home-guide-step-index {
  background: #16a34a;
}

.home-guide-step--active {
  border-color: #fde68a;
  background: #fffbeb;
}

.home-guide-step--active .home-guide-step-index {
  background: #d97706;
}

.home-guide-step--compact {
  gap: 8px;
  padding-top: 10px;
  padding-bottom: 10px;
}
</style>
