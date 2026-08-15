<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { NButton, NPopconfirm, NTag, NTooltip } from 'naive-ui'
import { $t } from '@/locales'
import type { DashboardMenuConfig } from '@/service/api/dashboard-menu'
import type { VisualizationDashboardSummary } from '@/service/visualization-provider/index'
import { buildThingsVisDashboardViewerHref } from './thingsVisDashboardSharing'

const props = defineProps<{
  dashboard: VisualizationDashboardSummary
  menuConfig?: DashboardMenuConfig | null
  menuConfigLoaded?: boolean
  thumbnailUrl?: string
  publishing?: boolean
  duplicating?: boolean
}>()

const emit = defineEmits<{
  edit: [dashboardId: string]
  menu: [dashboard: VisualizationDashboardSummary]
  setHome: [dashboard: VisualizationDashboardSummary]
  publish: [dashboard: VisualizationDashboardSummary]
  duplicate: [dashboard: VisualizationDashboardSummary]
  copyLink: [dashboard: VisualizationDashboardSummary]
  requestThumbnail: [dashboard: VisualizationDashboardSummary]
  requestMenuConfig: [dashboard: VisualizationDashboardSummary]
  delete: [dashboardId: string, dashboardName: string]
}>()

const viewerHref = buildThingsVisDashboardViewerHref(props.dashboard)
const cardRef = ref<HTMLElement | null>(null)
let lazyResourceObserver: IntersectionObserver | null = null

const stopObservingLazyResources = () => {
  lazyResourceObserver?.disconnect()
  lazyResourceObserver = null
}

const needsLazyResources = () => !props.thumbnailUrl || !props.menuConfigLoaded

const requestLazyResources = () => {
  if (!props.thumbnailUrl) {
    emit('requestThumbnail', props.dashboard)
  }
  if (!props.menuConfigLoaded) {
    emit('requestMenuConfig', props.dashboard)
  }
  stopObservingLazyResources()
}

onMounted(() => {
  if (!needsLazyResources()) return
  if (typeof window === 'undefined' || !('IntersectionObserver' in window) || !cardRef.value) {
    requestLazyResources()
    return
  }

  lazyResourceObserver = new IntersectionObserver(
    (entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        requestLazyResources()
      }
    },
    { rootMargin: '160px' }
  )
  lazyResourceObserver.observe(cardRef.value)
})

watch(
  () => [props.thumbnailUrl, props.menuConfigLoaded] as const,
  ([thumbnailUrl, menuConfigLoaded]) => {
    if (thumbnailUrl && menuConfigLoaded) stopObservingLazyResources()
  }
)

onBeforeUnmount(stopObservingLazyResources)
</script>

<template>
  <div
    ref="cardRef"
    class="group relative overflow-hidden rounded-lg border border-gray-200 bg-white transition-all hover:border-primary hover:shadow-lg"
    data-testid="thingsvis-dashboard-card"
    :data-dashboard-id="dashboard.id"
  >
    <a class="block cursor-pointer no-underline" :href="viewerHref" target="_blank" rel="noopener noreferrer">
      <div
        class="relative h-40 bg-gradient-to-br from-primary/20 to-primary/5 flex items-center justify-center overflow-hidden"
      >
        <img
          v-if="thumbnailUrl"
          :src="thumbnailUrl"
          class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
          alt="thumbnail"
          loading="lazy"
          decoding="async"
        />
        <icon-mdi:chart-box v-else class="text-64px text-primary/40" />

        <div
          v-if="dashboard.home"
          class="absolute top-2 right-2 h-24px w-24px border-2 border-red-500 rounded-full text-center text-12px text-red-500 font-600 flex items-center justify-center bg-white shadow-sm"
        >
          {{ $t('rdi.thingsvis.homeShort') }}
        </div>
      </div>

      <div class="p-4">
        <div class="mb-2 flex items-start justify-between gap-1">
          <h3 class="flex-1 truncate font-semibold text-gray-900">
            {{ dashboard.name }}
          </h3>
          <NTag v-if="dashboard.published" size="small" type="success">
            {{ $t('rdi.thingsvis.published') }}
          </NTag>
          <NTag v-if="menuConfig?.enabled" size="small" type="info">
            {{ $t('rdi.thingsvis.systemMenu') }}
          </NTag>
        </div>

        <div class="flex items-center justify-between text-xs text-gray-400">
          <div class="flex items-center gap-1">
            <icon-mdi:tag-outline />
            <span>v{{ dashboard.version }}</span>
          </div>
          <div class="flex items-center gap-1">
            <icon-mdi:clock-outline />
            <span>{{ new Date(dashboard.updatedAt).toLocaleDateString() }}</span>
          </div>
        </div>
      </div>
    </a>

    <div class="px-4 pb-4">
      <div class="flex gap-2 border-t border-gray-100 pt-3">
        <NButton
          size="small"
          secondary
          class="flex-1"
          data-testid="thingsvis-dashboard-edit"
          @click.stop="emit('edit', dashboard.id)"
        >
          <template #icon>
            <icon-mdi:pencil />
          </template>
          {{ $t('rdi.thingsvis.edit') }}
        </NButton>

        <NTooltip>
          <template #trigger>
            <NButton
              size="small"
              secondary
              :type="dashboard.published ? 'success' : 'default'"
              :disabled="dashboard.published"
              :loading="publishing"
              data-testid="thingsvis-dashboard-publish"
              @click.stop="emit('publish', dashboard)"
            >
              <template #icon>
                <icon-mdi:cloud-upload-outline />
              </template>
            </NButton>
          </template>
          {{ dashboard.published ? $t('rdi.thingsvis.alreadyPublished') : $t('rdi.thingsvis.publish') }}
        </NTooltip>

        <NTooltip>
          <template #trigger>
            <NButton
              size="small"
              secondary
              :loading="duplicating"
              data-testid="thingsvis-dashboard-duplicate"
              @click.stop="emit('duplicate', dashboard)"
            >
              <template #icon>
                <icon-mdi:content-copy />
              </template>
            </NButton>
          </template>
          {{ $t('rdi.thingsvis.duplicate') }}
        </NTooltip>

        <NTooltip>
          <template #trigger>
            <NButton size="small" secondary data-testid="thingsvis-dashboard-copy-link" @click.stop="emit('copyLink', dashboard)">
              <template #icon>
                <icon-mdi:link-variant />
              </template>
            </NButton>
          </template>
          {{ $t('rdi.thingsvis.copyLink') }}
        </NTooltip>

        <NTooltip>
          <template #trigger>
            <NButton
              size="small"
              :type="menuConfig?.enabled ? 'info' : 'default'"
              secondary
              data-testid="thingsvis-dashboard-menu"
              @click.stop="emit('menu', dashboard)"
            >
              <template #icon>
                <icon-mdi:menu />
              </template>
            </NButton>
          </template>
          {{ menuConfig?.enabled ? $t('rdi.thingsvis.editSystemMenu') : $t('rdi.thingsvis.setSystemMenu') }}
        </NTooltip>

        <NTooltip v-if="!dashboard.home">
          <template #trigger>
            <NPopconfirm @positive-click.stop="emit('setHome', dashboard)">
              <template #trigger>
                <NButton size="small" secondary data-testid="thingsvis-dashboard-set-home" @click.stop>
                  <template #icon>
                    <icon-mdi:home-outline />
                  </template>
                </NButton>
              </template>
              {{ $t('rdi.thingsvis.setHomeHint') }}
            </NPopconfirm>
          </template>
          {{ $t('rdi.thingsvis.setHome') }}
        </NTooltip>
        <NTooltip v-else>
          <template #trigger>
            <NButton size="small" type="primary" secondary disabled data-testid="thingsvis-dashboard-current-home" @click.stop>
              <template #icon>
                <icon-mdi:home />
              </template>
            </NButton>
          </template>
          {{ $t('rdi.thingsvis.currentHome') }}
        </NTooltip>

        <NButton
          size="small"
          secondary
          type="error"
          data-testid="thingsvis-dashboard-delete"
          @click.stop="emit('delete', dashboard.id, dashboard.name)"
        >
          <template #icon>
            <icon-mdi:delete />
          </template>
        </NButton>
      </div>
    </div>
  </div>
</template>
