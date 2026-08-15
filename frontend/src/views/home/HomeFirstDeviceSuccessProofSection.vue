<script setup lang="ts">
import { ref } from 'vue'
import { $t } from '@/locales'
import HomeFirstDeviceSuccessProofCard from './HomeFirstDeviceSuccessProofCard.vue'

defineProps<{
  title: string
  description: string
  ready: boolean
  facts: any[]
  chart: any
  readyProof: any
}>()

const emit = defineEmits<{
  copyChartProof: []
  downloadSuccessProof: []
}>()

const chartSectionEl = ref<HTMLElement | null>(null)
const proofSectionEl = ref<HTMLElement | null>(null)

defineExpose({
  chartSectionEl,
  proofSectionEl
})
</script>

<template>
  <div ref="chartSectionEl" class="rounded-6px bg-gray-50 px-12px py-10px">
    <div class="flex flex-col gap-8px sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.proofSection.stepRange') }}</div>
        <div class="mt-2px font-600">{{ title }}</div>
        <div class="mt-4px text-12px line-height-18px text-gray-500">
          {{ description }}
        </div>
      </div>
      <n-tag size="small" round :bordered="false" :type="ready ? 'success' : 'warning'">
        {{
          ready
            ? $t('custom.home.firstDevice.common.loopConfirmed')
            : $t('custom.home.firstDevice.common.keepWatchingWorkspace')
        }}
      </n-tag>
    </div>

    <div ref="proofSectionEl" class="mt-10px">
      <HomeFirstDeviceSuccessProofCard
        :facts="facts"
        :chart="chart"
        :ready-proof="readyProof"
        @copy-chart-proof="emit('copyChartProof')"
        @download-success-proof="emit('downloadSuccessProof')"
      />
    </div>
  </div>
</template>
