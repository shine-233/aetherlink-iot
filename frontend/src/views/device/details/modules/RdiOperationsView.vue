<script setup lang="ts">
import type { SelectOption } from 'naive-ui'
import {
  RDI_DRY_CONTACT_DELAY_MAX_SECONDS,
  RDI_DRY_CONTACT_DELAY_MIN_SECONDS,
  RDI_DRY_CONTACT_TEST_DURATION_MAX_SECONDS,
  RDI_DRY_CONTACT_TEST_DURATION_MIN_SECONDS
} from './rdi/constants/rdi-ranges'
import type { LabelKey } from './rdi/constants/rdi-labels'

type RdiOtaCommand = {
  firmware_url: string
  version: string
  size: number | null
  md5: string
}

defineProps<{
  commandLoading: boolean
  dryCommandDelay: number
  dryTestDuration: number
  commandTrackingSummary: string
  otaPackageLoading: boolean
  otaPackageId: string
  latestFirmwareLoading: boolean
  latestFirmwarePackage: unknown
  otaCommand: RdiOtaCommand
  otaPackageOptions: SelectOption[]
  otaMissingFieldLabels: string[]
  canSendOtaUpgrade: boolean
  shareLoading: boolean
  shareExpiresIn: number
  shareLink: string
  shareExpiryOptions: SelectOption[]
  shareExpiresAt: string
  t: (key: LabelKey) => string
}>()

const emit = defineEmits<{
  (e: 'update:dryCommandDelay', value: number): void
  (e: 'update:dryTestDuration', value: number): void
  (e: 'update:otaPackageId', value: string): void
  (e: 'update:shareExpiresIn', value: number): void
  (e: 'set-dry-contact', level: 'high' | 'low'): void
  (e: 'test-dry-contact'): void
  (e: 'ensure-ota-packages'): void
  (e: 'load-ota-packages'): void
  (e: 'check-latest-firmware'): void
  (e: 'apply-latest-firmware'): void
  (e: 'send-ota-upgrade'): void
  (e: 'send-unbind-device'): void
  (e: 'send-factory-reset'): void
  (e: 'create-share'): void
  (e: 'copy-share'): void
}>()
</script>

<template>
  <section class="rdi-section">
    <div class="rdi-section-title">{{ t('commands') }}</div>
    <div class="rdi-grid rdi-grid--commands">
      <NFormItem :label="`${t('dryContact')} ${t('duration')} (s)`">
        <NInputNumber
          :value="dryCommandDelay"
          :min="RDI_DRY_CONTACT_DELAY_MIN_SECONDS"
          :max="RDI_DRY_CONTACT_DELAY_MAX_SECONDS"
          @update:value="emit('update:dryCommandDelay', $event || 0)"
        />
      </NFormItem>
      <NButton :loading="commandLoading" @click="emit('set-dry-contact', 'high')">{{ t('sendHigh') }}</NButton>
      <NButton :loading="commandLoading" @click="emit('set-dry-contact', 'low')">{{ t('sendLow') }}</NButton>
      <NFormItem :label="`${t('testDuration')} (s)`">
        <NInputNumber
          :value="dryTestDuration"
          :min="RDI_DRY_CONTACT_TEST_DURATION_MIN_SECONDS"
          :max="RDI_DRY_CONTACT_TEST_DURATION_MAX_SECONDS"
          @update:value="emit('update:dryTestDuration', $event || 1)"
        />
      </NFormItem>
      <NButton :loading="commandLoading" @click="emit('test-dry-contact')">{{ t('test') }}</NButton>
    </div>
    <div class="rdi-grid rdi-grid--ota">
      <NSelect
        :value="otaPackageId"
        :options="otaPackageOptions"
        :loading="otaPackageLoading"
        :placeholder="t('otaPackage')"
        filterable
        clearable
        @focus="emit('ensure-ota-packages')"
        @update:value="emit('update:otaPackageId', String($event || ''))"
      />
      <NButton :loading="otaPackageLoading" @click="emit('load-ota-packages')">{{ t('reloadPackages') }}</NButton>
      <NButton :loading="latestFirmwareLoading" @click="emit('check-latest-firmware')">{{ t('checkUpdate') }}</NButton>
      <NButton :disabled="!latestFirmwarePackage" @click="emit('apply-latest-firmware')">
        {{ t('useLatestFirmware') }}
      </NButton>
      <NInput v-model:value="otaCommand.firmware_url" :placeholder="t('firmwareUrl')" />
      <NInput v-model:value="otaCommand.version" :placeholder="t('version')" />
      <NInputNumber v-model:value="otaCommand.size" :min="1" :placeholder="t('size')" />
      <NInput v-model:value="otaCommand.md5" :placeholder="t('md5')" />
      <div class="rdi-ota-status" :class="{ 'rdi-ota-status--ready': canSendOtaUpgrade }">
        <template v-if="otaMissingFieldLabels.length">
          {{ t('otaMissingFields') }}: {{ otaMissingFieldLabels.join(', ') }}
        </template>
        <template v-else>
          {{ t('otaReadyHint') }}
        </template>
      </div>
      <NButton :disabled="!canSendOtaUpgrade" :loading="commandLoading" @click="emit('send-ota-upgrade')">
        {{ t('ota') }}
      </NButton>
    </div>
    <div class="rdi-grid rdi-grid--danger-commands">
      <NPopconfirm
        :positive-text="t('confirm')"
        :negative-text="t('cancel')"
        @positive-click="emit('send-unbind-device')"
      >
        <template #trigger>
          <NButton secondary type="warning" :loading="commandLoading">{{ t('unbindDevice') }}</NButton>
        </template>
        {{ t('confirmUnbind') }}
      </NPopconfirm>
      <NPopconfirm
        :positive-text="t('confirm')"
        :negative-text="t('cancel')"
        @positive-click="emit('send-factory-reset')"
      >
        <template #trigger>
          <NButton secondary type="error" :loading="commandLoading">{{ t('factoryReset') }}</NButton>
        </template>
        {{ t('confirmFactoryReset') }}
      </NPopconfirm>
    </div>
    <NAlert v-if="commandTrackingSummary" type="info" class="rdi-command-tracking" :show-icon="false">
      {{ commandTrackingSummary }}
    </NAlert>
  </section>

  <section class="rdi-section">
    <div class="rdi-section-title">{{ t('share') }}</div>
    <div class="rdi-share-row">
      <NSelect
        :value="shareExpiresIn"
        :options="shareExpiryOptions"
        class="rdi-select"
        @update:value="emit('update:shareExpiresIn', Number($event || 0))"
      />
      <NButton :loading="shareLoading" @click="emit('create-share')">{{ t('createShare') }}</NButton>
      <NButton :disabled="!shareLink" @click="emit('copy-share')">{{ t('copy') }}</NButton>
      <span v-if="shareExpiresAt" class="rdi-muted">{{ t('expires') }}: {{ shareExpiresAt }}</span>
    </div>
    <NInput :value="shareLink" readonly />
  </section>
</template>

<style scoped>
.rdi-section {
  border-top: 1px solid #e5e7eb;
  padding: 16px 0;
}

.rdi-section-title {
  margin-bottom: 12px;
  font-size: 15px;
  font-weight: 600;
}

.rdi-grid {
  display: grid;
  gap: 14px;
}

.rdi-grid--commands {
  grid-template-columns: minmax(180px, 260px) auto auto minmax(180px, 260px) auto;
  align-items: end;
}

.rdi-grid--ota {
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  align-items: center;
  margin-top: 12px;
}

.rdi-ota-status {
  grid-column: 1 / -1;
  border: 1px solid #fedf89;
  border-radius: 6px;
  padding: 8px 10px;
  background: #fffaeb;
  color: #b54708;
  font-size: 12px;
}

.rdi-ota-status--ready {
  border-color: #abefc6;
  background: #ecfdf3;
  color: #067647;
}

.rdi-grid--danger-commands {
  grid-template-columns: repeat(auto-fit, minmax(180px, max-content));
  align-items: center;
  margin-top: 12px;
}

.rdi-command-tracking {
  margin-top: 12px;
}

.rdi-share-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}

.rdi-select {
  width: 220px;
}

.rdi-muted {
  color: #667085;
}

@media (max-width: 720px) {
  .rdi-grid--commands,
  .rdi-grid--ota {
    grid-template-columns: 1fr;
  }

  .rdi-share-row .rdi-select {
    width: 100%;
  }
}
</style>
