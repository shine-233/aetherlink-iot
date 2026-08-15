<script setup lang="ts">
type OnboardingAction = 'addDevice' | 'openServiceAccess' | 'backHome'

defineProps<{
  showHomeResume?: boolean
}>()

const emit = defineEmits<{
  addDevice: []
  openServiceAccess: []
  backHome: []
}>()

const guideSteps: Array<{
  order: string
  titleKey: string
  descriptionKey: string
  actionKey?: string
  action?: OnboardingAction
  statusKey?: string
  secondary?: boolean
  recommended?: boolean
}> = [
  {
    order: '1',
    titleKey: 'custom.devicePage.onboardingStepCreateTitle',
    descriptionKey: 'custom.devicePage.onboardingStepCreateDesc',
    actionKey: 'custom.devicePage.emptyAddDevice',
    action: 'addDevice',
    recommended: true
  },
  {
    order: '2',
    titleKey: 'custom.devicePage.onboardingStepCopyTitle',
    descriptionKey: 'custom.devicePage.onboardingStepCopyDesc',
    actionKey: 'custom.devicePage.emptyServiceAccess',
    action: 'openServiceAccess',
    secondary: true
  },
  {
    order: '3',
    titleKey: 'custom.devicePage.onboardingStepVerifyTitle',
    descriptionKey: 'custom.devicePage.onboardingStepVerifyDesc',
    statusKey: 'custom.devicePage.onboardingAfterDeviceCreated'
  },
  {
    order: '4',
    titleKey: 'custom.devicePage.onboardingStepOperateTitle',
    descriptionKey: 'custom.devicePage.onboardingStepOperateDesc',
    statusKey: 'custom.devicePage.onboardingAfterReadyCheck'
  }
]

const handleAction = (action: OnboardingAction) => {
  ;(emit as (event: OnboardingAction) => void)(action)
}
</script>

<template>
  <section class="device-onboarding-guide">
    <div class="device-onboarding-guide__header">
      <div>
        <div class="device-onboarding-guide__title">
          {{ $t('custom.devicePage.onboardingGuideTitle') }}
        </div>
        <div class="device-onboarding-guide__desc">
          {{ $t('custom.devicePage.onboardingGuideDesc') }}
        </div>
      </div>
      <NButton v-if="showHomeResume" size="small" quaternary @click="$emit('backHome')">
        {{ $t('common.backToHome') }}
      </NButton>
    </div>

    <div class="device-onboarding-guide__steps">
      <div
        v-for="step in guideSteps"
        :key="step.order"
        class="device-onboarding-guide__step"
        :class="{ 'device-onboarding-guide__step--recommended': step.recommended }"
      >
        <div class="device-onboarding-guide__step-index">
          {{ step.order }}
        </div>
        <div class="device-onboarding-guide__step-body">
          <div class="device-onboarding-guide__step-title">
            {{ $t(step.titleKey) }}
          </div>
          <div class="device-onboarding-guide__step-desc">
            {{ $t(step.descriptionKey) }}
          </div>
          <NButton
            v-if="step.action && step.actionKey"
            size="small"
            :type="step.secondary ? 'default' : 'primary'"
            :secondary="step.secondary"
            class="device-onboarding-guide__step-action"
            @click="handleAction(step.action)"
          >
            {{ $t(step.actionKey) }}
          </NButton>
          <NTag
            v-else-if="step.statusKey"
            size="small"
            round
            :bordered="false"
            class="device-onboarding-guide__step-status"
          >
            {{ $t(step.statusKey) }}
          </NTag>
        </div>
      </div>
    </div>

    <div class="device-onboarding-guide__footer">
      <div class="device-onboarding-guide__hint">
        {{ $t('custom.devicePage.emptyHint') }}
      </div>
      <div class="device-onboarding-guide__protocols">
        <span class="device-onboarding-guide__protocol">MQTT</span>
        <span class="device-onboarding-guide__protocol">HTTP</span>
        <span class="device-onboarding-guide__protocol">Node</span>
        <span class="device-onboarding-guide__protocol">Python</span>
        <span class="device-onboarding-guide__protocol">C</span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.device-onboarding-guide {
  width: min(920px, calc(100vw - 64px));
  margin: 18px auto 0;
  padding: 18px;
  border: 1px solid #d7dde8;
  border-radius: 8px;
  background: #fff;
  text-align: left;
}

.device-onboarding-guide__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.device-onboarding-guide__title {
  color: #1f2937;
  font-size: 15px;
  font-weight: 650;
}

.device-onboarding-guide__desc {
  margin-top: 6px;
  color: #526070;
  font-size: 13px;
  line-height: 1.6;
}

.device-onboarding-guide__steps {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.device-onboarding-guide__step {
  display: flex;
  min-width: 0;
  gap: 10px;
  padding: 10px;
  border: 1px solid transparent;
  border-radius: 8px;
}

.device-onboarding-guide__step--recommended {
  border-color: #b9d3ff;
  background: #f6f9ff;
}

.device-onboarding-guide__step-index {
  display: flex;
  width: 24px;
  height: 24px;
  flex: 0 0 24px;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #2563eb;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
}

.device-onboarding-guide__step-body {
  min-width: 0;
}

.device-onboarding-guide__step-title {
  color: #1f2937;
  font-size: 13px;
  font-weight: 650;
  line-height: 1.35;
}

.device-onboarding-guide__step-desc {
  margin-top: 5px;
  color: #667085;
  font-size: 12px;
  line-height: 1.55;
}

.device-onboarding-guide__step-action {
  margin-top: 8px;
}

.device-onboarding-guide__step-status {
  margin-top: 8px;
}

.device-onboarding-guide__footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid #eef2f6;
}

.device-onboarding-guide__hint {
  max-width: 560px;
  color: #526070;
  font-size: 12px;
  line-height: 1.55;
}

.device-onboarding-guide__protocols {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.device-onboarding-guide__protocol {
  padding: 2px 8px;
  border: 1px solid #cdd5df;
  border-radius: 999px;
  color: #3f4d5a;
  font-size: 12px;
  line-height: 20px;
}

@media (max-width: 960px) {
  .device-onboarding-guide {
    width: min(100%, calc(100vw - 32px));
  }

  .device-onboarding-guide__steps {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .device-onboarding-guide__header {
    flex-direction: column;
  }

  .device-onboarding-guide__steps {
    grid-template-columns: 1fr;
  }
}
</style>
