<script setup lang="ts">
import { defineAsyncComponent, type ComponentPublicInstance, type Ref } from 'vue'
import { LocalVisualizationViewer } from '@/components/local-visualization-viewer'
import type { VisualizationHomeDashboard } from '@/service/visualization-provider/home-dashboard'
import { NATIVE_BOARD_PROVIDER_ID } from '@/service/visualization-provider/provider-ids'

const ThingsVisAppFrame = defineAsyncComponent(() => import('@/components/thingsvis/ThingsVisAppFrame.vue'))

const props = defineProps<{
  isHomeResolving: boolean
  showHomeResolvingGate: boolean
  homeResolvingDescription: string
  isError: boolean
  useThingsVis: boolean
  thingsVisHome: VisualizationHomeDashboard | null
  thingsVisSectionRef: Ref<HTMLElement | null>
  shouldMountHomeThingsVisFrame: boolean
  showCompatHomeNotice: boolean
  compatHomeConfigCount: number
}>()

const emit = defineEmits<{
  reload: []
  openThingsVis: []
  mountHomeThingsVisFrame: []
  continueFirstDevice: []
  openDeviceManagement: []
  openRdiDashboard: []
  openRdiAlarmOverview: []
  openAlarmCenter: []
  openSystemSettings: []
}>()

const bindThingsVisSectionRef = (element: Element | ComponentPublicInstance | null) => {
  props.thingsVisSectionRef.value = element instanceof HTMLElement ? element : null
}
</script>

<template>
  <div v-if="isHomeResolving && !showHomeResolvingGate" class="home-secondary-section">
    <n-card :bordered="false" class="rounded-8px">
      <div class="flex items-start gap-14px">
        <n-spin size="small" class="mt-2px" />
        <div class="min-w-0 flex-1">
          <div class="text-16px font-600">{{ $t('custom.home.resolvingTitle') }}</div>
          <div class="mt-6px text-13px leading-20px text-gray-500">{{ homeResolvingDescription }}</div>
        </div>
      </div>
    </n-card>
  </div>

  <div v-else-if="isError && !useThingsVis" class="home-secondary-section">
    <n-card :bordered="false" class="rounded-8px">
      <div class="flex flex-col gap-10px">
        <div class="text-16px font-600">{{ $t('custom.home.title') }}</div>
        <div class="text-13px leading-20px text-gray-500">{{ $t('custom.home.description') }}</div>
        <div>
          <n-button size="small" @click="emit('reload')">{{ $t('custom.home.refresh') }}</n-button>
        </div>
      </div>
    </n-card>
  </div>

  <div v-else-if="useThingsVis && thingsVisHome" :ref="bindThingsVisSectionRef" class="home-secondary-section">
    <div class="home-secondary-header">
      <div>
        <div class="text-16px font-600">{{ $t('custom.home.dashboardSection.title') }}</div>
        <div class="mt-3px text-12px text-gray-500">
          {{ $t('custom.home.dashboardSection.description') }}
        </div>
      </div>
      <n-button size="small" quaternary @click="emit('openThingsVis')">
        {{ $t('custom.home.actions.openThingsVis') }}
      </n-button>
    </div>
    <div class="home-secondary-frame">
      <LocalVisualizationViewer
        v-if="shouldMountHomeThingsVisFrame && thingsVisHome.providerId === NATIVE_BOARD_PROVIDER_ID"
        :dashboard="thingsVisHome.rendererData"
        :fields="{}"
        class="h-full w-full"
      />
      <ThingsVisAppFrame
        v-else-if="shouldMountHomeThingsVisFrame"
        :id="thingsVisHome.id"
        :schema="thingsVisHome"
        mode="viewer"
        class="h-full w-full"
      />
      <div v-else class="home-secondary-frame-placeholder">
        <div class="text-15px font-600">{{ $t('custom.home.dashboardSection.deferredTitle') }}</div>
        <div class="mt-6px max-w-520px text-center text-13px leading-20px text-gray-500">
          {{ $t('custom.home.dashboardSection.deferredDescription') }}
        </div>
        <n-button class="mt-14px" size="small" type="primary" ghost @click="emit('mountHomeThingsVisFrame')">
          {{ $t('custom.home.dashboardSection.loadNow') }}
        </n-button>
      </div>
    </div>
  </div>

  <div v-else-if="showCompatHomeNotice" class="home-secondary-section">
    <div class="grid gap-16px lg:grid-cols-[2fr_1fr]">
      <n-card :bordered="false" class="rounded-8px">
        <div class="flex flex-col gap-12px">
          <div class="text-18px font-600">{{ $t('custom.home.compatNotice.title') }}</div>
          <div class="text-14px text-gray-500">
            {{ $t('custom.home.compatNotice.description') }}
          </div>
          <div class="flex flex-wrap gap-12px pt-8px">
            <n-button type="primary" @click="emit('continueFirstDevice')">
              {{ $t('custom.home.compatNotice.actions.continueFirstDevice') }}
            </n-button>
            <n-button type="primary" @click="emit('openDeviceManagement')">
              {{ $t('custom.home.compatNotice.actions.deviceManagement') }}
            </n-button>
            <n-button @click="emit('openRdiDashboard')">{{ $t('route.dashboard_rdi-overview') }}</n-button>
            <n-button @click="emit('openRdiAlarmOverview')">{{ $t('route.alarm_rdi-overview') }}</n-button>
            <n-button @click="emit('openAlarmCenter')">
              {{ $t('custom.home.compatNotice.actions.alarmCenter') }}
            </n-button>
            <n-button @click="emit('openSystemSettings')">
              {{ $t('custom.home.compatNotice.actions.systemSettings') }}
            </n-button>
            <n-button @click="emit('openThingsVis')">
              {{ $t('custom.home.compatNotice.actions.visualizationProject') }}
            </n-button>
          </div>
        </div>
      </n-card>
      <n-card :bordered="false" class="rounded-8px">
        <div class="flex flex-col gap-8px text-14px">
          <div class="font-600">{{ $t('custom.home.compatNotice.statusTitle') }}</div>
          <div>{{ $t('custom.home.compatNotice.statusCount', { count: compatHomeConfigCount }) }}</div>
          <div>{{ $t('custom.home.compatNotice.statusAvailable') }}</div>
          <div>{{ $t('custom.home.compatNotice.statusManageInThingsVis') }}</div>
        </div>
      </n-card>
    </div>
  </div>
</template>

<style scoped>
.home-secondary-section {
  margin-top: 20px;
}

.home-secondary-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.home-secondary-frame {
  height: 720px;
  min-height: 520px;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
}

.home-secondary-frame-placeholder {
  display: flex;
  height: 100%;
  min-height: inherit;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  padding: 24px;
  background:
    radial-gradient(circle at 50% 20%, rgba(14, 165, 233, 0.12), transparent 34%),
    linear-gradient(135deg, #f8fafc 0%, #eef6ff 100%);
  color: #0f172a;
}

@media (max-width: 640px) {
  .home-secondary-header {
    flex-direction: column;
  }

  .home-secondary-frame {
    height: 560px;
  }
}
</style>
