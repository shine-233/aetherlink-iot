<script setup lang="ts">
import { computed } from 'vue'
import DeviceOnboardingGuide from './DeviceOnboardingGuide.vue'

const props = defineProps<{
  searchCriteria?: Record<string, unknown>
  firstDeviceOnboarding?: boolean
}>()

defineEmits<{
  addDevice: []
  openServiceAccess: []
  clearFilters: []
  backHome: []
}>()

const hasDeviceListFilters = (criteria?: Record<string, unknown>) =>
  Object.values(criteria || {}).some((value) => {
    if (Array.isArray(value)) return value.length > 0
    return value !== '' && value !== null && value !== undefined
  })

const isFilteredEmpty = computed(() => hasDeviceListFilters(props.searchCriteria))
const emptyTitleKey = computed(() => {
  if (isFilteredEmpty.value) return 'custom.devicePage.emptyFilteredTitle'
  if (props.firstDeviceOnboarding) return 'custom.devicePage.firstDeviceOnboardingTitle'
  return 'custom.devicePage.emptyTitle'
})
const emptySummaryKey = computed(() =>
  props.firstDeviceOnboarding ? 'custom.devicePage.firstDeviceOnboardingDesc' : 'custom.devicePage.onboardingGuideDesc'
)
</script>

<template>
  <div class="device-empty-state">
    <NEmpty :description="$t(emptyTitleKey)">
      <template #extra>
        <div v-if="!isFilteredEmpty" class="device-empty-recommendation">
          <div class="device-empty-recommendation__title">
            {{ $t('custom.devicePage.onboardingGuideTitle') }}
          </div>
          <div class="device-empty-recommendation__desc">
            {{ $t(emptySummaryKey) }}
          </div>
        </div>
        <div class="device-empty-actions" :class="{ 'device-empty-actions--filtered': isFilteredEmpty }">
          <NButton type="primary" @click="$emit('addDevice')">
            {{ $t('custom.devicePage.emptyAddDevice') }}
          </NButton>
          <NButton secondary @click="$emit('openServiceAccess')">
            {{ $t('custom.devicePage.emptyServiceAccess') }}
          </NButton>
          <NButton v-if="props.firstDeviceOnboarding && !isFilteredEmpty" quaternary @click="$emit('backHome')">
            {{ $t('common.backToHome') }}
          </NButton>
          <NButton v-if="isFilteredEmpty" quaternary @click="$emit('clearFilters')">
            {{ $t('custom.devicePage.emptyClearFilters') }}
          </NButton>
        </div>
        <DeviceOnboardingGuide
          v-if="!isFilteredEmpty"
          :show-home-resume="props.firstDeviceOnboarding"
          @add-device="$emit('addDevice')"
          @open-service-access="$emit('openServiceAccess')"
          @back-home="$emit('backHome')"
        />
        <div class="device-empty-hint">
          {{ $t('custom.devicePage.emptyHint') }}
        </div>
      </template>
    </NEmpty>
  </div>
</template>

<style scoped>
.device-empty-state {
  display: flex;
  min-height: 320px;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
}

.device-empty-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
}

.device-empty-actions--filtered {
  margin-top: 0;
}

.device-empty-recommendation {
  width: min(720px, calc(100vw - 64px));
  margin: 0 auto 14px;
  padding: 16px;
  border: 1px solid #c9ddff;
  border-radius: 10px;
  background: linear-gradient(135deg, #f5f9ff 0%, #eef6ff 100%);
  text-align: center;
}

.device-empty-recommendation__title {
  color: #174ea6;
  font-size: 14px;
  font-weight: 650;
}

.device-empty-recommendation__desc {
  margin-top: 6px;
  color: #526070;
  font-size: 13px;
  line-height: 1.5;
}

.device-empty-hint {
  max-width: 520px;
  margin-top: 12px;
  color: #666;
  font-size: 13px;
  line-height: 1.6;
  text-align: center;
}

@media (max-width: 880px) {
  .device-empty-recommendation {
    width: min(100%, calc(100vw - 32px));
  }
}
</style>
