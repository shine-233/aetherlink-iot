<script setup lang="ts">
defineProps<{
  nextStep: {
    type: 'success' | 'warning' | 'info' | 'default'
    step: string
    title: string
    description: string
    actionLabel: string
  }
  hasPackages: boolean
  hasSelectedPackage: boolean
}>()

const emit = defineEmits<{
  action: []
}>()
</script>

<template>
  <NCard class="ota-next-step-card" :bordered="false">
    <NSpace vertical size="small">
      <NSpace align="center" justify="space-between" :wrap="true">
        <div>
          <NTag :type="nextStep.type" round>{{ nextStep.step }}</NTag>
          <div class="ota-next-step-card__title">{{ nextStep.title }}</div>
          <div class="ota-next-step-card__desc">{{ nextStep.description }}</div>
        </div>
        <NButton :type="nextStep.type === 'success' ? 'primary' : 'default'" @click="emit('action')">
          {{ nextStep.actionLabel }}
        </NButton>
      </NSpace>
      <NSpace :wrap="true">
        <NTag :type="hasPackages ? 'success' : 'warning'" size="small">
          {{ $t('page.product.update-ota.onboardingPackageChecklist') }}
        </NTag>
        <NTag :type="hasSelectedPackage ? 'success' : 'default'" size="small">
          {{ $t('page.product.update-ota.onboardingSelectChecklist') }}
        </NTag>
        <NTag :type="hasSelectedPackage ? 'info' : 'default'" size="small">
          {{ $t('page.product.update-ota.onboardingPreflightChecklist') }}
        </NTag>
      </NSpace>
    </NSpace>
  </NCard>
</template>

<style scoped>
.ota-next-step-card {
  border: 1px solid #dbeafe;
  background: linear-gradient(135deg, #f8fbff 0%, #eef6ff 100%);
}

.ota-next-step-card__title {
  margin-top: 8px;
  font-size: 16px;
  font-weight: 700;
}

.ota-next-step-card__desc {
  margin-top: 4px;
  color: var(--text-color-2);
  line-height: 1.6;
}
</style>
