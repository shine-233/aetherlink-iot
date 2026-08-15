<script setup lang="ts">
import type { DropdownOption } from 'naive-ui'
import type { TelemetryCardFreshness, TelemetryCardRecord } from './telemetryCardViewState'
import TelemetryDataCard from './TelemetryDataCard.vue'

type SelectOption = {
  label: string
  value: string
}

defineProps<{
  attentionTelemetryCount: number
  cardHeight: number
  cardMargin: number
  deleteOptions: DropdownOption[]
  displayTelemetryCount: number
  displayTelemetryData: TelemetryCardRecord[]
  getTelemetryFreshnessBadge: (telemetry: TelemetryCardRecord) => TelemetryCardFreshness
  hasTelemetryCardFilters: boolean
  isTelemetryHardRenderCapped: boolean
  nowTime: unknown
  showAllTelemetryCards: boolean
  telemetryAccentColor: (telemetry: TelemetryCardRecord) => string | undefined
  telemetryDataCount: number
  telemetryFreshnessOptions: SelectOption[]
  telemetryLoadError: string
  telemetryLoadStatus: string
  telemetrySortOptions: SelectOption[]
  visibleTelemetryCount: number
  visibleTelemetryData: TelemetryCardRecord[]
}>()

const telemetrySearchQuery = defineModel<string>('searchQuery', { required: true })
const telemetrySortMode = defineModel<string>('sortMode', { required: true })
const telemetryFreshnessFilter = defineModel<string>('freshnessFilter', { required: true })

const emit = defineEmits<{
  (e: 'clear-filters'): void
  (e: 'delete-select', key: string | number, item: TelemetryCardRecord): void
  (e: 'export-csv'): void
  (e: 'history', item: TelemetryCardRecord): void
  (e: 'sequence', item: TelemetryCardRecord): void
  (e: 'toggle-display-limit'): void
}>()
</script>

<template>
  <n-card class="mb-4">
    <div class="telemetry-toolbar">
      <n-space class="telemetry-toolbar__filters" align="center" :wrap-item="false">
        <n-input
          v-model:value="telemetrySearchQuery"
          clearable
          :placeholder="$t('custom.device_details.telemetrySearchPlaceholder')"
          class="telemetry-toolbar__search"
        />
        <n-select v-model:value="telemetrySortMode" :options="telemetrySortOptions" class="telemetry-toolbar__sort" />
        <n-select
          v-model:value="telemetryFreshnessFilter"
          :options="telemetryFreshnessOptions"
          class="telemetry-toolbar__freshness"
        />
        <n-button v-if="hasTelemetryCardFilters" secondary @click="emit('clear-filters')">
          {{ $t('common.clear') }}
        </n-button>
        <n-button secondary :disabled="visibleTelemetryCount === 0" @click="emit('export-csv')">
          <template #icon>
            <icon-mdi:download />
          </template>
          {{ $t('custom.device_details.telemetryExportCsv') }}
        </n-button>
      </n-space>
      <div class="telemetry-toolbar__summary">
        <n-space align="center" :size="8" :wrap-item="false">
          <span>
            {{ visibleTelemetryCount }} / {{ telemetryDataCount }} {{ $t('custom.device_details.telemetry') }}
          </span>
          <n-tag v-if="attentionTelemetryCount" size="small" type="warning" round>
            {{ attentionTelemetryCount }} {{ $t('custom.device_details.telemetryNeedsAttention') }}
          </n-tag>
        </n-space>
      </div>
    </div>
    <n-alert v-if="telemetryLoadStatus === 'error'" type="error" class="telemetry-state-alert" :show-icon="true">
      {{ $t('custom.device_details.telemetrySnapshotLoadFailed') }}
      <span v-if="telemetryLoadError">: {{ telemetryLoadError }}</span>
    </n-alert>
    <n-alert v-else-if="telemetryLoadStatus === 'empty'" type="info" class="telemetry-state-alert" :show-icon="true">
      {{ $t('custom.device_details.telemetryNoData') }}
    </n-alert>
    <n-alert
      v-else-if="telemetryDataCount > 0 && visibleTelemetryCount === 0"
      type="info"
      class="telemetry-state-alert"
      :show-icon="true"
    >
      {{ $t('custom.device_details.telemetryNoFilterResults') }}
    </n-alert>
    <n-alert
      v-if="visibleTelemetryCount > displayTelemetryCount"
      type="info"
      class="telemetry-state-alert"
      :show-icon="false"
    >
      <n-space align="center" justify="space-between" :wrap="true">
        <span>
          {{
            $t('custom.device_details.telemetryDisplayLimited')
              .replace('{shown}', String(displayTelemetryCount))
              .replace('{total}', String(visibleTelemetryCount))
          }}
        </span>
        <n-button size="small" secondary type="primary" @click="emit('toggle-display-limit')">
          {{
            showAllTelemetryCards
              ? $t('custom.device_details.telemetryShowLess')
              : $t('custom.device_details.telemetryShowMore')
          }}
        </n-button>
      </n-space>
    </n-alert>
    <n-alert
      v-if="isTelemetryHardRenderCapped"
      type="warning"
      class="telemetry-state-alert"
      :show-icon="false"
    >
      {{ $t('custom.device_details.telemetryRenderHardLimited') }}
    </n-alert>
    <n-grid :x-gap="cardMargin" :y-gap="cardMargin" cols="1 600:2 900:3 1200:4">
      <n-gi
        v-for="(telemetry, index) in displayTelemetryData"
        :key="`${telemetry.device_id || index}-${telemetry.key || index}`"
      >
        <TelemetryDataCard
          :accent-color="telemetryAccentColor(telemetry)"
          :card-height="cardHeight"
          :delete-options="deleteOptions"
          :freshness="getTelemetryFreshnessBadge(telemetry)"
          :index="index"
          :item="telemetry"
          :now-time="nowTime"
          @delete-select="(key, item) => emit('delete-select', key, item)"
          @history="(item) => emit('history', item)"
          @sequence="(item) => emit('sequence', item)"
        />
      </n-gi>
    </n-grid>
    <div v-if="showAllTelemetryCards && visibleTelemetryCount > 0" class="telemetry-show-less">
      <n-button secondary size="small" @click="emit('toggle-display-limit')">
        {{ $t('custom.device_details.telemetryShowLess') }}
      </n-button>
    </div>
  </n-card>
</template>
