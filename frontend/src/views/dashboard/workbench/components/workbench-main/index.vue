<script setup lang="ts">
import { $t } from '@/locales'
import { CapabilityCard, ShortcutsCard } from './components'

defineOptions({ name: 'DashboardWorkbenchMain' })

interface Capability {
  id: number
  name: string
  description: string
  actionLabel: string
  route: string
  icon: string
  iconColor?: string
}

const capabilities: Capability[] = [
  {
    id: 0,
    name: $t('custom.dashboardWorkbench.capabilityDeviceOnboarding'),
    description: $t('custom.dashboardWorkbench.capabilityDeviceOnboardingDesc'),
    actionLabel: $t('custom.dashboardWorkbench.openFirstDevice'),
    route: '/first-device',
    icon: 'mdi:access-point-network',
    iconColor: '#2563eb'
  },
  {
    id: 1,
    name: $t('custom.dashboardWorkbench.capabilityReadyCheck'),
    description: $t('custom.dashboardWorkbench.capabilityReadyCheckDesc'),
    actionLabel: $t('custom.dashboardWorkbench.openDeviceHealth'),
    route: '/device/manage',
    icon: 'mdi:heart-pulse',
    iconColor: '#16a34a'
  },
  {
    id: 2,
    name: $t('custom.dashboardWorkbench.capabilityTwin'),
    description: $t('custom.dashboardWorkbench.capabilityTwinDesc'),
    actionLabel: $t('custom.dashboardWorkbench.openDeviceDetail'),
    route: '/device/manage',
    icon: 'mdi:sync-circle',
    iconColor: '#7c3aed'
  },
  {
    id: 3,
    name: $t('custom.dashboardWorkbench.capabilityCommandJobs'),
    description: $t('custom.dashboardWorkbench.capabilityCommandJobsDesc'),
    actionLabel: $t('custom.dashboardWorkbench.openCommandCenter'),
    route: '/device/command-center',
    icon: 'mdi:console-network-outline',
    iconColor: '#0891b2'
  },
  {
    id: 4,
    name: $t('custom.dashboardWorkbench.capabilityOta'),
    description: $t('custom.dashboardWorkbench.capabilityOtaDesc'),
    actionLabel: $t('custom.dashboardWorkbench.openOta'),
    route: '/product/update-ota',
    icon: 'mdi:cloud-upload-outline',
    iconColor: '#d97706'
  },
  {
    id: 5,
    name: $t('custom.dashboardWorkbench.capabilityAlarmClosure'),
    description: $t('custom.dashboardWorkbench.capabilityAlarmClosureDesc'),
    actionLabel: $t('custom.dashboardWorkbench.openAlarmCenter'),
    route: '/alarm/warning-message',
    icon: 'mdi:alarm-light-outline',
    iconColor: '#dc2626'
  }
]

interface Activity {
  id: number
  content: string
  description: string
}

const activity: Activity[] = [
  {
    id: 4,
    content: $t('custom.dashboardWorkbench.activityFirstDevice'),
    description: $t('custom.dashboardWorkbench.activityFirstDeviceDesc')
  },
  {
    id: 3,
    content: $t('custom.dashboardWorkbench.activityCommandJobs'),
    description: $t('custom.dashboardWorkbench.activityCommandJobsDesc')
  },
  {
    id: 2,
    content: $t('custom.dashboardWorkbench.activityReadyCheck'),
    description: $t('custom.dashboardWorkbench.activityReadyCheckDesc')
  },
  {
    id: 1,
    content: $t('custom.dashboardWorkbench.activityOtaAlarm'),
    description: $t('custom.dashboardWorkbench.activityOtaAlarmDesc')
  },
  {
    id: 0,
    content: $t('custom.dashboardWorkbench.activityVisualization'),
    description: $t('custom.dashboardWorkbench.activityVisualizationDesc')
  }
]

interface Shortcuts {
  id: number
  label: string
  icon: string
  iconColor: string
  route?: string
}

const shortcuts: Shortcuts[] = [
  {
    id: 0,
    label: $t('custom.dashboardWorkbench.shortcutFirstDevice'),
    icon: 'mdi:access-point-network',
    iconColor: '#2563eb',
    route: '/first-device'
  },
  {
    id: 1,
    label: $t('custom.dashboardWorkbench.shortcutFleet'),
    icon: 'mdi:devices',
    iconColor: '#16a34a',
    route: '/device/manage'
  },
  {
    id: 2,
    label: $t('custom.dashboardWorkbench.shortcutAutomation'),
    icon: 'mdi:source-branch',
    iconColor: '#7c3aed',
    route: '/automation/linkage-edit'
  },
  {
    id: 3,
    label: $t('custom.dashboardWorkbench.shortcutDashboard'),
    icon: 'mdi:view-dashboard-edit',
    iconColor: '#0891b2',
    route: '/visualization/thingsvis-dashboards'
  },
  {
    id: 4,
    label: $t('custom.dashboardWorkbench.shortcutOta'),
    icon: 'mdi:cloud-upload-outline',
    iconColor: '#d97706',
    route: '/product/update-ota'
  },
  {
    id: 5,
    label: $t('custom.dashboardWorkbench.shortcutAlarm'),
    icon: 'mdi:alarm-light-outline',
    iconColor: '#dc2626',
    route: '/alarm/warning-message'
  }
]

const readinessItems = [
  $t('custom.dashboardWorkbench.readinessAccess'),
  $t('custom.dashboardWorkbench.readinessEvidence'),
  $t('custom.dashboardWorkbench.readinessSupport')
]
</script>

<template>
  <NGrid :item-responsive="true" :x-gap="16" :y-gap="16">
    <NGridItem span="0:24 640:24 1024:16">
      <NSpace :vertical="true" :size="16">
        <NCard
          :title="$t('custom.dashboardWorkbench.capabilityTitle')"
          :bordered="false"
          size="small"
          class="rounded-8px shadow-sm"
        >
          <NGrid :item-responsive="true" responsive="screen" cols="m:2 l:3" :x-gap="8" :y-gap="8">
            <NGridItem v-for="item in capabilities" :key="item.id">
              <CapabilityCard v-bind="item" />
            </NGridItem>
          </NGrid>
        </NCard>
        <NCard
          :title="$t('custom.dashboardWorkbench.activityTitle')"
          :bordered="false"
          size="small"
          class="rounded-8px shadow-sm"
        >
          <NList>
            <NListItem v-for="item in activity" :key="item.id">
              <template #prefix>
                <IconLocalAvatar class="text-48px" />
              </template>
              <NThing :title="item.content" :description="item.description" />
            </NListItem>
          </NList>
        </NCard>
      </NSpace>
    </NGridItem>
    <NGridItem span="0:24 640:24 1024:8">
      <NSpace :vertical="true" :size="16">
        <NCard
          :title="$t('custom.dashboardWorkbench.quickOperationTitle')"
          :bordered="false"
          size="small"
          class="rounded-8px shadow-sm"
        >
          <NGrid :item-responsive="true" responsive="screen" cols="m:2 l:3" :x-gap="8" :y-gap="8">
            <NGridItem v-for="item in shortcuts" :key="item.id">
              <ShortcutsCard v-bind="item" />
            </NGridItem>
          </NGrid>
        </NCard>
        <NCard
          :title="$t('custom.dashboardWorkbench.readinessTitle')"
          :bordered="false"
          size="small"
          class="rounded-8px shadow-sm"
        >
          <div class="flex flex-col gap-12px">
            <div
              v-for="item in readinessItems"
              :key="item"
              class="rounded-6px border border-#e5e7eb px-12px py-10px text-14px leading-22px dark:border-#ffffff17"
            >
              {{ item }}
            </div>
          </div>
        </NCard>
      </NSpace>
    </NGridItem>
  </NGrid>
</template>

<style scoped></style>
