<script setup lang="ts">
import dayjs from 'dayjs'
import AnimatedNumber from '@/components/common/AnimatedNumber.vue'
import { TrendingUpOutline, DocumentTextOutline } from '@vicons/ionicons5'
import type { DropdownOption } from 'naive-ui'
import type { TelemetryCardFreshness, TelemetryCardRecord } from './telemetryCardViewState'

const props = defineProps<{
  accentColor?: string
  cardHeight: number
  deleteOptions: DropdownOption[]
  freshness: TelemetryCardFreshness
  index: number
  item: TelemetryCardRecord
  nowTime: unknown
}>()

const emit = defineEmits<{
  (e: 'history', item: TelemetryCardRecord): void
  (e: 'sequence', item: TelemetryCardRecord): void
  (e: 'delete-select', key: string | number, item: TelemetryCardRecord): void
}>()

const telemetryTitle = () => props.item.label || props.item.key || '--'
const telemetryTimestamp = () => (props.item.ts ? dayjs(props.item.ts).format('YYYY-MM-DD HH:mm:ss') : props.nowTime)
</script>

<template>
  <n-card header-class="border-b h-36px" hoverable :style="{ height: cardHeight + 'px' }">
    <div class="card-body">
      <n-tooltip v-if="accentColor" trigger="hover" placement="top">
        <template #trigger>
          <span class="value-display-ellipsis" style="font-size: 24px">
            {{ item.value }}
          </span>
        </template>
        <div style="max-width: 300px; word-break: break-all">{{ item.value }}</div>
      </n-tooltip>
      <AnimatedNumber
        v-else
        :data-index="index"
        :m-num="item.value"
        :quantile-show="true"
      />
      <span v-if="item.unit">{{ item.unit }}</span>
    </div>

    <template #header>
      <div class="line1" :title="item.key">
        <template v-if="item.label">
          <span>{{ item.label }}</span>
          <span>({{ item.key }})</span>
        </template>
        <template v-else>
          <span>{{ telemetryTitle() }}</span>
        </template>
      </div>
    </template>

    <template #footer>
      <div class="telemetry-card-footer">
        <n-tag size="small" :type="freshness.tagType" round>
          {{ $t(freshness.i18nKey) }}
        </n-tag>
        <span class="telemetry-card-footer__time">
          {{ telemetryTimestamp() }}
        </span>
      </div>
    </template>

    <template #header-extra>
      <div class="h-24px w-120px flex items-center justify-end">
        <n-tooltip trigger="hover">
          <template #trigger>
            <NIcon size="24" class="cursor-pointer" @click="emit('history', item)">
              <DocumentTextOutline />
            </NIcon>
          </template>
          {{ $t('custom.device_details.history') }}
        </n-tooltip>
        <NDivider vertical />
        <n-tooltip trigger="hover">
          <template #trigger>
            <NIcon size="24" class="cursor-pointer" :color="accentColor" @click="emit('sequence', item)">
              <TrendingUpOutline />
            </NIcon>
          </template>
          {{ $t('custom.device_details.sequential') }}
        </n-tooltip>
        <NDivider vertical />
        <n-dropdown trigger="click" :options="deleteOptions" @select="emit('delete-select', $event, item)">
          <span class="inline-flex cursor-pointer items-center justify-center">
            <SvgIcon icon="mdi:dots-horizontal" class="text-20px" />
          </span>
        </n-dropdown>
      </div>
    </template>
  </n-card>
</template>
