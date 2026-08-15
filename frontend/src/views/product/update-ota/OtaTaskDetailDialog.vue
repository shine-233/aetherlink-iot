<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'
import type { OtaFailureGroup, OtaRetryRecommendationCard } from './ota-task-failure-workbench'
import type { OtaTaskDetailRecord, RolloutGuidanceItem, RolloutSummaryItem } from './ota-task-types'

type DetailQueryModel = {
  device_name?: string
  task_status?: string | number | null
}

const props = defineProps<{
  show: boolean
  readyCheckOtaDetailContextMessage: string
  detailLastRefreshLabel: string
  detailAutoRefreshActive: boolean
  rolloutFailedCount: number
  rolloutSuccessRate: string
  detailLoading: boolean
  rolloutActiveCount: number
  detailAutoRefreshEnabled: boolean
  rolloutSummaryItems: RolloutSummaryItem[]
  rolloutGuidanceItems: RolloutGuidanceItem[]
  failedDeviceCount: number
  supportBundleLoading: boolean
  canCopyFailureSupportBundle: boolean
  hasFirstFailedDiagnosticDevice: boolean
  retryRecommendationCards: OtaRetryRecommendationCard[]
  failureGroups: OtaFailureGroup[]
  detailQuery: DetailQueryModel
  statusOptions: any[]
  detailColumns: any[]
  detailList: OtaTaskDetailRecord[]
  detailPagination: any
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'update:detailAutoRefreshEnabled', value: boolean): void
  (e: 'update:detailQueryDeviceName', value: string): void
  (e: 'update:detailQueryTaskStatus', value: string | number | null): void
  (e: 'refresh'): void
  (e: 'resetDetailQuery'): void
  (e: 'copyFailedDevices'): void
  (e: 'copyFailureSupportBundle'): void
  (e: 'downloadTaskSupportBundle'): void
  (e: 'exportFailedDevices'): void
  (e: 'openFirstFailedDiagnostics'): void
  (e: 'openFailedDeviceDiagnostics', row: OtaTaskDetailRecord): void
}>()

const visible = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value)
})

const detailAutoRefreshEnabledValue = computed({
  get: () => props.detailAutoRefreshEnabled,
  set: (value: boolean) => emit('update:detailAutoRefreshEnabled', value)
})

const detailDeviceNameValue = computed({
  get: () => props.detailQuery.device_name || '',
  set: (value: string) => emit('update:detailQueryDeviceName', value)
})

const detailTaskStatusValue = computed({
  get: () => props.detailQuery.task_status ?? null,
  set: (value: string | number | null) => emit('update:detailQueryTaskStatus', value)
})
</script>

<template>
  <NModal v-model:show="visible" preset="card" class="detail-modal" :title="$t('page.product.update-ota.taskDetail')">
    <NSpace vertical size="medium">
      <NAlert v-if="readyCheckOtaDetailContextMessage" type="info" :show-icon="true">
        {{ readyCheckOtaDetailContextMessage }}
      </NAlert>

      <NCard embedded>
        <NSpace vertical size="small">
          <NSpace align="center" justify="space-between" :wrap="true">
            <div>
              <div class="font-600">{{ $t('page.product.update-ota.rolloutSummary') }}</div>
              <div class="rollout-refresh-meta">{{ detailLastRefreshLabel }}</div>
            </div>
            <NSpace align="center" :wrap="true">
              <NTag v-if="detailAutoRefreshActive" type="info" round>
                {{ $t('page.product.update-ota.autoRefreshingProgress') }}
              </NTag>
              <NTag :type="rolloutFailedCount ? 'error' : 'success'" round>
                {{ $t('page.product.update-ota.rolloutSuccessRate') }} {{ rolloutSuccessRate }}
              </NTag>
              <NButton size="small" secondary :loading="detailLoading" @click="emit('refresh')">
                {{ $t('page.product.update-ota.refreshProgress') }}
              </NButton>
            </NSpace>
          </NSpace>
          <NAlert type="info" :show-icon="false">
            {{ $t('page.product.update-ota.rolloutGovernanceHint') }}
          </NAlert>
          <NSpace v-if="rolloutActiveCount > 0" align="center" size="small" :wrap="true">
            <NSwitch v-model:value="detailAutoRefreshEnabledValue" size="small" />
            <span class="rollout-refresh-meta">
              {{
                $t('page.product.update-ota.autoRefreshProgressHint').replace(
                  '{count}',
                  String(rolloutActiveCount)
                )
              }}
            </span>
          </NSpace>
          <NGrid x-gap="12" y-gap="12" cols="2 s:3 m:4 l:7" responsive="screen">
            <NGridItem v-for="item in rolloutSummaryItems" :key="item.key">
              <NCard size="small" class="rollout-summary-card">
                <div class="rollout-summary-card__label">{{ item.label }}</div>
                <NTag :type="item.type" round>{{ item.value }}</NTag>
              </NCard>
            </NGridItem>
          </NGrid>
          <NGrid x-gap="12" y-gap="12" cols="1 s:2 m:3" responsive="screen">
            <NGridItem v-for="item in rolloutGuidanceItems" :key="item.key">
              <NCard size="small" class="rollout-guidance-card">
                <NSpace align="center" justify="space-between" :wrap="false">
                  <div class="rollout-guidance-card__title">{{ item.title }}</div>
                  <NTag :type="item.type" round>{{ item.value }}</NTag>
                </NSpace>
                <div class="rollout-guidance-card__desc">{{ item.description }}</div>
              </NCard>
            </NGridItem>
          </NGrid>
        </NSpace>
      </NCard>

      <NCard embedded>
        <NSpace vertical size="small">
          <NSpace align="center" justify="space-between" :wrap="true">
            <div>
              <div class="font-600">{{ $t('page.product.update-ota.failureWorkbenchTitle') }}</div>
              <div class="failure-workbench-desc">{{ $t('page.product.update-ota.failureWorkbenchDesc') }}</div>
            </div>
            <NSpace>
              <NButton size="small" secondary :disabled="failedDeviceCount === 0" @click="emit('copyFailedDevices')">
                {{ $t('page.product.update-ota.copyFailedDevices') }}
              </NButton>
              <NButton
                size="small"
                secondary
                :loading="supportBundleLoading"
                :disabled="!canCopyFailureSupportBundle"
                @click="emit('copyFailureSupportBundle')"
              >
                {{ $t('page.product.update-ota.copyFailureSupportBundle') }}
              </NButton>
              <NButton
                size="small"
                secondary
                type="primary"
                data-testid="ota-download-task-support-bundle"
                :loading="supportBundleLoading"
                @click="emit('downloadTaskSupportBundle')"
              >
                {{ $t('page.product.update-ota.downloadTaskSupportBundle') }}
              </NButton>
              <NButton size="small" secondary :disabled="failedDeviceCount === 0" @click="emit('exportFailedDevices')">
                {{ $t('page.product.update-ota.exportFailedDevices') }}
              </NButton>
            </NSpace>
          </NSpace>
          <NAlert v-if="failedDeviceCount === 0" type="success" :show-icon="false">
            {{ $t('page.product.update-ota.noFailedDevices') }}
          </NAlert>
          <template v-else>
            <NAlert type="error" :show-icon="true" class="first-failure-diagnostics-cta">
              <NSpace align="center" justify="space-between" :wrap="true">
                <div>
                  <div class="first-failure-diagnostics-cta__title">
                    {{ $t('page.product.update-ota.firstFailureDiagnosticsTitle') }}
                  </div>
                  <div class="first-failure-diagnostics-cta__desc">
                    {{ $t('page.product.update-ota.firstFailureDiagnosticsDesc') }}
                  </div>
                </div>
                <NButton
                  type="error"
                  secondary
                  strong
                  :disabled="!hasFirstFailedDiagnosticDevice"
                  @click="emit('openFirstFailedDiagnostics')"
                >
                  {{ $t('page.product.update-ota.diagnoseFirstFailure') }}
                </NButton>
              </NSpace>
            </NAlert>
            <NGrid v-if="retryRecommendationCards.length" x-gap="12" y-gap="12" cols="1 m:3" responsive="screen">
              <NGridItem v-for="item in retryRecommendationCards" :key="item.key">
                <NCard size="small" class="retry-recommendation-card">
                  <NSpace vertical size="small">
                    <NSpace align="center" justify="space-between" :wrap="false">
                      <div class="retry-recommendation-card__title">{{ item.title }}</div>
                      <NTag :type="item.type" round>{{ item.count }}</NTag>
                    </NSpace>
                    <div class="retry-recommendation-card__desc">{{ item.description }}</div>
                    <div v-if="item.devices.length" class="retry-recommendation-card__devices">
                      {{ $t('page.product.update-ota.retryRecommendation.representativeDevices') }}:
                      {{ item.devices.join(', ') }}
                    </div>
                  </NSpace>
                </NCard>
              </NGridItem>
            </NGrid>
            <NGrid x-gap="12" y-gap="12" cols="1 s:2 m:3" responsive="screen">
              <NGridItem v-for="group in failureGroups" :key="group.key">
                <NCard size="small" class="failure-group-card">
                  <NSpace vertical size="small">
                    <NSpace align="center" justify="space-between" :wrap="false">
                      <div class="failure-group-card__reason">{{ group.reason }}</div>
                      <NTag type="error" round>{{ group.count }}</NTag>
                    </NSpace>
                    <div class="failure-group-card__devices">
                      <div v-for="device in group.devices" :key="device.id" class="failure-group-card__device">
                        <span>{{ device.name || device.device_number || device.id }}</span>
                        <NButton
                          size="tiny"
                          secondary
                          type="info"
                          :disabled="!device.device_id"
                          @click="emit('openFailedDeviceDiagnostics', device)"
                        >
                          {{ $t('page.product.update-ota.openFailureDiagnostics') }}
                        </NButton>
                      </div>
                    </div>
                  </NSpace>
                </NCard>
              </NGridItem>
            </NGrid>
          </template>
        </NSpace>
      </NCard>

      <NSpace align="center" :wrap="true">
        <NInput
          v-model:value="detailDeviceNameValue"
          class="detail-filter"
          clearable
          :placeholder="$t('page.product.update-ota.deviceName')"
        />
        <NSelect
          v-model:value="detailTaskStatusValue"
          class="detail-filter"
          clearable
          :options="statusOptions"
          :placeholder="$t('page.product.update-ota.statusTask')"
        />
        <NButton type="primary" @click="emit('refresh')">{{ $t('common.search') }}</NButton>
        <NButton @click="emit('resetDetailQuery')">{{ $t('common.reset') }}</NButton>
      </NSpace>
      <NDataTable
        remote
        :columns="detailColumns"
        :data="detailList"
        :loading="detailLoading"
        :pagination="detailPagination"
        :scroll-x="1200"
      />
    </NSpace>
  </NModal>
</template>
