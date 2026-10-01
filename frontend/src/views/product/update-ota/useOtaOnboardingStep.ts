/*
 * Onboarding "next step" card for the OTA task page: tells the operator what to do when there is
 * no package yet, no package selected, or everything is ready for a task.
 */
import { computed } from 'vue'
import type { Ref } from 'vue'
import { $t } from '@/locales'

interface OnboardingStepOptions {
  packageLoading: Ref<boolean>
  packageOptions: Ref<unknown[]>
  selectedPackageId: Ref<string | null | undefined>
  onUploadPackage: () => void
  onCreateTask: () => void
  onRefreshPackages: () => void
}

export function useOtaOnboardingStep(options: OnboardingStepOptions) {
  const nextStep = computed(() => {
    if (options.packageLoading.value) {
      return {
        type: 'info' as const,
        step: $t('page.product.update-ota.onboardingStepLoading'),
        title: $t('common.loading'),
        description: $t('page.product.update-ota.onboardingLoadingDesc'),
        actionLabel: $t('common.refresh'),
        action: 'refresh'
      }
    }

    if (!options.packageOptions.value.length) {
      return {
        type: 'warning' as const,
        step: $t('page.product.update-ota.onboardingStepPackage'),
        title: $t('page.product.update-ota.onboardingNoPackageTitle'),
        description: $t('page.product.update-ota.onboardingNoPackageDesc'),
        actionLabel: $t('page.product.update-ota.onboardingUploadPackageAction'),
        action: 'upload'
      }
    }

    if (!options.selectedPackageId.value) {
      return {
        type: 'info' as const,
        step: $t('page.product.update-ota.onboardingStepSelect'),
        title: $t('page.product.update-ota.onboardingSelectPackageTitle'),
        description: $t('page.product.update-ota.onboardingSelectPackageDesc').replace(
          '{count}',
          String(options.packageOptions.value.length)
        ),
        actionLabel: $t('common.refresh'),
        action: 'refresh'
      }
    }

    return {
      type: 'success' as const,
      step: $t('page.product.update-ota.onboardingStepCreate'),
      title: $t('page.product.update-ota.onboardingReadyTitle'),
      description: $t('page.product.update-ota.onboardingReadyDesc'),
      actionLabel: $t('page.product.update-ota.onboardingCreateTaskAction'),
      action: 'create'
    }
  })

  function handleNextStep() {
    if (nextStep.value.action === 'upload') {
      options.onUploadPackage()
      return
    }

    if (nextStep.value.action === 'create') {
      options.onCreateTask()
      return
    }

    options.onRefreshPackages()
  }

  return { nextStep, handleNextStep }
}
