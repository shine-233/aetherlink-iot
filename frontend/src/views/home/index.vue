<!--
文件用途：承载首页和第一台设备接入控制台的页面级视图。
核心逻辑：组合首次接入向导、设备状态、可视化仪表盘、接口请求和国际化文案，完成页面初始化、查询与交互反馈。
关键注意事项：页面通常依赖权限、分页、远端接口和路由状态，改动时需同步检查测试与接口契约。
重构建议：租户引导、后台探测、workbench 门控与客户引导动作已拆分至同目录 composable，新增逻辑优先沉淀到对应模块。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { router } from '@/router'
import { $t } from '@/locales'
import { useAuthStore } from '@/store/modules/auth'
import { isSysAdminUser } from '@/utils/thingsvis/space'
import { createHomeGuideRefreshCoordinator } from './homeGuideRefreshCoordinator'
import { createHomeFirstRunCreateState } from './homeFirstRunCreateState'
import { resolveHomeTenantRouteContext } from './homeTenantRouteContext'
import { scheduleIdleHomeTask } from './homeIdleScheduler'
import { useFirstDeviceWorkbenchGate } from './useFirstDeviceWorkbenchGate'
import { useHomeCustomerGuideActions } from './useHomeCustomerGuideActions'
import { useHomeFirstDeviceWorkbench } from './useHomeFirstDeviceWorkbench'
import { isNativeHomeProvider, useHomeLayoutResolver } from './useHomeLayoutResolver'
import { useFirstDeviceFocusRouter, type HomeFirstDeviceWorkbenchViewExpose } from './useFirstDeviceFocusRouter'
import { useHomeGuideProbes } from './useHomeGuideProbes'
import { useHomeTenantSetupGuide } from './useHomeTenantSetupGuide'

const HomeSecondaryPanel = defineAsyncComponent(() => import('./HomeSecondaryPanel.vue'))
const HomeFirstDeviceWorkbenchView = defineAsyncComponent(() => import('./HomeFirstDeviceWorkbenchView.vue'))

const firstDeviceWorkbenchViewRef = ref<HomeFirstDeviceWorkbenchViewExpose | null>(null)
const active = ref<boolean>(true)
const authStore = useAuthStore()
const route = useRoute()
const isSysAdmin = computed(() => isSysAdminUser(authStore.userInfo))

const {
  isFirstDeviceOnboardingRoute,
  nativeHomeTenantId,
  hasHomeFirstRunTenantContext,
  hasNativeHomeTenantContext
} = resolveHomeTenantRouteContext({ route, userInfo: () => authStore.userInfo })

const { refreshTenantSetupGuideState, homeSetupReady, homeSetupGuideStep } = useHomeTenantSetupGuide({
  hasFirstRunTenantContext: hasHomeFirstRunTenantContext
})

const {
  deploymentHealthLoading,
  deploymentHealthRows,
  deploymentHealthOk,
  refreshDeploymentHealth,
  automationGuideLoading,
  hasSceneAutomation,
  refreshAutomationGuideState
} = useHomeGuideProbes()

const firstRunCreate = createHomeFirstRunCreateState()
const firstRunCreateLoading = firstRunCreate.loading
const firstRunProtocol = firstRunCreate.protocol
const firstRunCreateResult = firstRunCreate.result
const firstRunCreateTenantRequired = firstRunCreate.tenantRequired

const firstDeviceWorkbench = useHomeFirstDeviceWorkbench({
  deploymentHealthy: deploymentHealthOk,
  deploymentHealthRows,
  onTenantRequired: () => {
    firstRunCreate.tenantRequired.value = true
  }
})
const firstDevice = firstDeviceWorkbench.device
const firstDeviceLoading = firstDeviceWorkbench.loading
const firstDeviceAccessGuide = firstDeviceWorkbench.accessGuide
const firstDeviceSimulation = firstDeviceWorkbench.simulation
const firstDeviceActionLoading = firstDeviceWorkbench.actionLoading
const firstDeviceTestResult = firstDeviceWorkbench.testResult
const firstDeviceBrowserTest = firstDeviceWorkbench.browserTest
const firstDevicePublishCommand = firstDeviceWorkbench.publishCommand
const firstDeviceOnboardingGuard = firstDeviceWorkbench.onboardingGuard
const firstDeviceReadyProof = firstDeviceWorkbench.readyProof
const firstDeviceChart = firstDeviceWorkbench.firstChart
const buildFirstDeviceSupportSummary = firstDeviceWorkbench.buildSupportSummary

const shouldShowHomeSecondarySections = computed(
  () => !isFirstDeviceOnboardingRoute.value || Boolean(firstDeviceReadyProof.value.ready)
)

const {
  layoutFetched,
  hasCompatHomeConfig,
  compatHomeConfigCount,
  isError,
  showSysAdminSetup,
  thingsVisHome,
  useThingsVis,
  nativeTenantContextRequired,
  homeDashboardUnavailable,
  showCompatHomeNotice,
  isHomeResolving,
  sysAdminSetupTitle,
  sysAdminSetupDescription,
  homeResolvingDescription,
  homeThingsVisSectionRef,
  shouldMountHomeThingsVisFrame,
  mountHomeThingsVisFrame,
  resetHomeThingsVisFrame,
  observeHomeThingsVisFrame,
  refreshThingsVisHomeDashboardInBackground,
  getLayout
} = useHomeLayoutResolver({
  isSysAdmin,
  nativeTenantId: nativeHomeTenantId,
  hasNativeHomeTenantContext,
  shouldShowSecondarySections: shouldShowHomeSecondarySections,
  scheduleIdleTask: scheduleIdleHomeTask
})

const sysAdminSetupAction = computed(() =>
  isNativeHomeProvider ? $t('custom.home.actions.openNativeBoards') : $t('custom.home.actions.openThingsVis')
)
const homeVisualizationPath = computed(() =>
  isNativeHomeProvider ? '/visualization/native-boards' : '/visualization/thingsvis'
)
const showHomeResolvingGate = computed(
  () => isSysAdmin.value && isHomeResolving.value && !isFirstDeviceOnboardingRoute.value
)

const shouldLoadFirstRunWorkbenchData = () =>
  isFirstDeviceOnboardingRoute.value || (!showSysAdminSetup.value && !isError.value)

const { firstDeviceWorkbenchLoaded, ensureFirstDeviceWorkbenchLoaded, refreshFirstDeviceWorkbench } =
  useFirstDeviceWorkbenchGate({
    shouldLoad: shouldLoadFirstRunWorkbenchData,
    refresh: firstDeviceWorkbench.refresh
  })

const { focusHomeWorkbenchSection, consumeFirstDeviceFocusQuery } = useFirstDeviceFocusRouter({
  workbenchViewRef: firstDeviceWorkbenchViewRef,
  ensureWorkbenchLoaded: async () => {
    await ensureFirstDeviceWorkbenchLoaded()
  },
  route
})

const {
  homeFirstRunResumeText,
  homeCustomerGuideProgress,
  homeCustomerGuideSummary,
  currentHomeSetupGuideStep,
  openHomeGuideStep,
  createFirstRunFirstDevice,
  runFirstDeviceQuickstartAction,
  openFirstDeviceAccessGuideAfterLoad,
  openFirstDeviceFullGuideAfterLoad,
  copyFirstDevicePublishCommandAfterLoad,
  simulateFirstDeviceTelemetryAfterLoad
} = useHomeCustomerGuideActions({
  userInfo: () => authStore.userInfo,
  firstRunCreate,
  workbench: firstDeviceWorkbench,
  thingsVisHome,
  hasSceneAutomation,
  deploymentHealthOk,
  homeSetupReady,
  homeSetupGuideStep,
  refreshDeploymentHealth,
  ensureFirstDeviceWorkbenchLoaded,
  refreshFirstDeviceWorkbench,
  focusHomeWorkbenchSection
})

const firstRunTenantBlocked = computed(() => firstRunCreateTenantRequired.value || !homeSetupReady.value)

const homeGuideRefreshCoordinator = createHomeGuideRefreshCoordinator({
  schedule: scheduleIdleHomeTask,
  refreshTenantSetup: refreshTenantSetupGuideState,
  refreshDeploymentHealth,
  refreshFirstDeviceWorkbench,
  refreshAutomation: refreshAutomationGuideState,
  shouldRefreshAutomation: () => shouldShowHomeSecondarySections.value
})

const refreshHomeGuideProgress = () => {
  homeGuideRefreshCoordinator.refreshFromUser()
}

const refreshInitialHomeGuideProgress = () => {
  if (!shouldLoadFirstRunWorkbenchData()) return
  homeGuideRefreshCoordinator.refreshOnInitialLoad()
}

const scheduleInitialHomeGuideProgress = (retryCount = 0) => {
  scheduleIdleHomeTask(() => {
    if (
      !isFirstDeviceOnboardingRoute.value &&
      !layoutFetched.value &&
      !showSysAdminSetup.value &&
      !isError.value &&
      retryCount < 8
    ) {
      window.setTimeout(() => scheduleInitialHomeGuideProgress(retryCount + 1), 250)
      return
    }
    refreshInitialHomeGuideProgress()
  }, 100)
}

onMounted(() => {
  consumeFirstDeviceFocusQuery()
  if (shouldShowHomeSecondarySections.value) {
    void getLayout()
  }
  scheduleInitialHomeGuideProgress()
  void observeHomeThingsVisFrame()
})

watch(
  () => `${route.query.onboarding || ''}|${route.query.focus || ''}|${route.hash || ''}`,
  () => {
    consumeFirstDeviceFocusQuery()
  }
)

watch(
  () => `${useThingsVis.value}|${thingsVisHome.value?.id || ''}`,
  () => {
    void observeHomeThingsVisFrame()
  }
)

watch(shouldShowHomeSecondarySections, (shouldShow) => {
  if (!shouldShow) {
    resetHomeThingsVisFrame()
    return
  }

  void getLayout()
  void refreshAutomationGuideState()
  void observeHomeThingsVisFrame()
})
</script>

<template>
  <div v-if="showHomeResolvingGate" class="h-full w-full flex-center px-16px">
    <div
      class="w-full max-w-520px rounded-8px border border-gray-200 bg-white px-24px py-22px shadow-sm dark:border-gray-700 dark:bg-#18181c"
    >
      <div class="flex items-start gap-14px">
        <n-spin size="small" class="mt-2px" />
        <div class="min-w-0 flex-1">
          <div class="text-16px font-600 leading-24px">{{ $t('custom.home.resolvingTitle') }}</div>
          <div class="mt-6px text-14px leading-22px text-gray-500">{{ homeResolvingDescription }}</div>
        </div>
      </div>
    </div>
  </div>

  <div v-else-if="isError && !useThingsVis && isSysAdmin && !isFirstDeviceOnboardingRoute" class="h-full w-full flex-center">
    <n-result status="418" :title="$t('custom.home.title')" :description="$t('custom.home.description')">
      <template #footer>
        <n-button
          type="primary"
          :disabled="active"
          @click="
            () => {
              router.go(0)
            }
          "
        >
          <n-countdown
            v-if="active"
            :duration="60000"
            :render="(props) => props.seconds + 's'"
            :active="active"
            @finish="active = false"
          />
          {{ active ? '' : $t('custom.home.refresh') }}
        </n-button>
      </template>
    </n-result>
  </div>

  <div v-else-if="showSysAdminSetup && !isFirstDeviceOnboardingRoute" class="h-full w-full flex-center">
    <n-result
      status="info"
      :title="sysAdminSetupTitle"
      :description="sysAdminSetupDescription"
    >
      <template #footer>
        <div class="flex items-center gap-3">
          <n-button
            type="primary"
            @click="
              () => {
                router.push(homeVisualizationPath)
              }
            "
          >
            {{ sysAdminSetupAction }}
          </n-button>
          <n-button
            @click="
              () => {
                router.go(0)
              }
            "
          >
            {{ $t('custom.home.actions.reload') }}
          </n-button>
        </div>
      </template>
    </n-result>
  </div>

  <div v-else class="home-workspace h-full w-full px-16px py-16px">
    <HomeFirstDeviceWorkbenchView
      ref="firstDeviceWorkbenchViewRef"
      :home-customer-guide-summary="homeCustomerGuideSummary"
      :home-first-run-resume-text="homeFirstRunResumeText"
      :home-customer-guide-progress="homeCustomerGuideProgress"
      :first-device-focus-mode="isFirstDeviceOnboardingRoute"
      :first-device-workbench-loaded="firstDeviceWorkbenchLoaded"
      :first-device-ready-proof="firstDeviceReadyProof"
      :first-device="firstDevice"
      :first-device-loading="firstDeviceLoading"
      :deployment-health-loading="deploymentHealthLoading"
      :automation-guide-loading="automationGuideLoading"
      :first-run-create-loading="firstRunCreateLoading"
      :first-run-protocol="firstRunProtocol"
      :deployment-health-ok="deploymentHealthOk"
      :first-run-create-result="firstRunCreateResult"
      :first-run-create-tenant-required="firstRunTenantBlocked"
      :first-run-setup-blocker-step="currentHomeSetupGuideStep"
      :first-device-access-guide="firstDeviceAccessGuide"
      :first-device-simulation="firstDeviceSimulation"
      :first-device-publish-command="firstDevicePublishCommand"
      :first-device-onboarding-guard="firstDeviceOnboardingGuard"
      :first-device-action-loading="firstDeviceActionLoading"
      :first-device-test-result="firstDeviceTestResult"
      :first-device-browser-test="firstDeviceBrowserTest"
      :first-device-chart="firstDeviceChart"
      :deployment-health-rows="deploymentHealthRows"
      :build-first-device-support-summary="buildFirstDeviceSupportSummary"
      @open-home-guide-step="openHomeGuideStep"
      @refresh-home-guide-progress="refreshHomeGuideProgress"
      @refresh-first-device-workbench="refreshFirstDeviceWorkbench"
      @update-first-run-protocol="firstRunProtocol = $event"
      @create-first-run-first-device="createFirstRunFirstDevice"
      @open-manual-device-add="router.push('/device/manage?onboarding=first-device&add=manual')"
      @open-things-model="router.push('/device/thingsmodel')"
      @copy-first-device-publish-command="copyFirstDevicePublishCommandAfterLoad"
      @simulate-first-device-telemetry="simulateFirstDeviceTelemetryAfterLoad"
      @open-first-device-full-guide="openFirstDeviceFullGuideAfterLoad"
      @open-first-device-access-guide="openFirstDeviceAccessGuideAfterLoad"
      @run-first-device-quickstart-action="runFirstDeviceQuickstartAction"
      @refresh-deployment-health="refreshDeploymentHealth"
    />

    <HomeSecondaryPanel
      v-if="shouldShowHomeSecondarySections"
      :is-home-resolving="isHomeResolving"
      :show-home-resolving-gate="showHomeResolvingGate"
      :home-resolving-description="homeResolvingDescription"
      :is-error="isError"
      :use-things-vis="useThingsVis"
      :things-vis-home="thingsVisHome"
      :things-vis-section-ref="(homeThingsVisSectionRef as any)"
      :should-mount-home-things-vis-frame="shouldMountHomeThingsVisFrame"
      :show-compat-home-notice="showCompatHomeNotice"
      :compat-home-config-count="compatHomeConfigCount"
      @reload="router.go(0)"
      @open-things-vis="router.push(homeVisualizationPath)"
      @mount-home-things-vis-frame="mountHomeThingsVisFrame"
      @continue-first-device="router.push('/home?onboarding=first-device')"
      @open-device-management="router.push('/device/manage')"
      @open-rdi-dashboard="router.push('/dashboard/rdi-overview')"
      @open-rdi-alarm-overview="router.push('/alarm/rdi-overview')"
      @open-alarm-center="router.push('/alarm')"
      @open-system-settings="router.push('/management/setting')"
    />
  </div>
</template>

<style scoped>
.home-workspace {
  overflow-y: auto;
  overflow-x: hidden;
  scrollbar-width: thin;
  scrollbar-color: rgba(0, 0, 0, 0.5) transparent;
}
</style>
