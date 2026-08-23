// 文件用途：承载首页客户引导的进度推导与首设备引导动作编排。
// 核心逻辑：组合引导状态持久化、引导进度计算、步骤跳转、首台设备快速创建与快捷动作。
// 关键注意事项：步骤 id 分支与跳转路由是页面对外契约，调整需同步引导链接生成方。
import { computed, ref, watch, type ComputedRef, type Ref } from 'vue'
import { router } from '@/router'
import { $t } from '@/locales'
import {
  buildHomeCustomerGuideProgress,
  buildHomeCustomerGuideSummary,
  type HomeCustomerGuideProgressStep,
  type HomeCustomerGuideStep
} from './homeCustomerGuide'
import { createHomeFirstRunFirstDevice } from './homeFirstRunWizard'
import {
  loadHomeFirstRunGuideState,
  saveHomeFirstRunGuideState,
  type HomeFirstRunGuideState
} from './homeFirstRunStorage'
import { isCompleteThingsVisDashboard } from './useHomeLayoutResolver'
import type { VisualizationHomeDashboard } from '@/service/visualization-provider/home-dashboard'
import { useHomeFirstDeviceWorkbench } from './useHomeFirstDeviceWorkbench'
import type { HomeFirstRunCreateState } from './homeFirstRunCreateState'

type Workbench = ReturnType<typeof useHomeFirstDeviceWorkbench>

type UseHomeCustomerGuideActionsOptions = {
  userInfo: () => Record<string, unknown> | null | undefined
  firstRunCreate: HomeFirstRunCreateState
  workbench: Workbench
  thingsVisHome: Ref<VisualizationHomeDashboard | null>
  hasSceneAutomation: Ref<boolean>
  deploymentHealthOk: ComputedRef<boolean>
  homeSetupReady: ComputedRef<boolean>
  homeSetupGuideStep: ComputedRef<{
    id: string
    title: string
    description: string
    route: string
    action: string
  }>
  refreshDeploymentHealth: () => Promise<void>
  ensureFirstDeviceWorkbenchLoaded: () => Promise<void>
  refreshFirstDeviceWorkbench: () => Promise<void>
  focusHomeWorkbenchSection: (key: string) => Promise<void>
}

export function useHomeCustomerGuideActions(options: UseHomeCustomerGuideActionsOptions) {
  const { firstRunCreate } = options
  const firstDevice = options.workbench.device
  const firstDeviceTelemetry = options.workbench.telemetry
  const firstDeviceChart = options.workbench.firstChart

  const firstRunGuideState = ref<HomeFirstRunGuideState | null>(
    loadHomeFirstRunGuideState(typeof window === 'undefined' ? null : window.localStorage)
  )

  const homeFirstRunResumeText = computed(() => {
    const state = firstRunGuideState.value
    if (!state?.lastTitle) return ''
    if (state.quickCreateDeviceName) {
      return $t('custom.home.resume.afterCreate', { deviceName: state.quickCreateDeviceName, lastTitle: state.lastTitle })
    }
    return $t('custom.home.resume.continue', {
      lastTitle: state.lastTitle,
      lastAction: state.lastAction || $t('custom.home.resume.nextAction')
    })
  })

  const saveFirstRunGuideStep = (
    step: HomeCustomerGuideProgressStep,
    quickCreateDeviceName = firstRunCreate.result.value?.deviceName || ''
  ) => {
    firstRunGuideState.value = saveHomeFirstRunGuideState(typeof window === 'undefined' ? null : window.localStorage, {
      lastStep: step.id,
      lastTitle: step.title,
      lastAction: step.action,
      lastRoute: step.route,
      quickCreateDeviceName
    })
  }

  const homeCustomerGuideProgress = computed(() =>
    buildHomeCustomerGuideProgress({
      setupReady: options.homeSetupReady.value,
      setupStep: options.homeSetupGuideStep.value as Partial<HomeCustomerGuideStep>,
      hasDevice: Boolean(firstDevice.value),
      hasTemplate: Boolean(firstDevice.value?.configName),
      deviceOnline: Boolean(firstDevice.value?.online),
      hasTelemetry: firstDeviceTelemetry.value.length > 0,
      hasFirstChart: firstDeviceChart.value.ready,
      hasAutomation: options.hasSceneAutomation.value,
      hasDashboard: isCompleteThingsVisDashboard(options.thingsVisHome.value),
      deploymentHealthy: options.deploymentHealthOk.value
    })
  )
  const homeCustomerGuideSummary = computed(() => buildHomeCustomerGuideSummary(homeCustomerGuideProgress.value))
  const currentHomeSetupGuideStep = computed(
    () => homeCustomerGuideProgress.value.find((step) => step.id === 'setup') ?? null
  )

  const buildFirstAutomationStarterQuery = () => {
    const query: Record<string, string> = {
      backType: 'automation',
      onboarding: 'first-device',
      starter: 'first-telemetry-rule'
    }
    const device = firstDevice.value
    const telemetry = firstDeviceTelemetry.value[0]

    if (device?.id) query.device_id = device.id
    if (device?.configId) query.device_config_id = device.configId
    if (device?.name) query.first_device_name = device.name
    if (device?.number) query.first_device_number = device.number
    if (telemetry?.key) query.telemetry_key = telemetry.key
    if (telemetry?.value !== undefined && telemetry?.value !== null) query.telemetry_value = String(telemetry.value)
    if (telemetry?.ts) query.telemetry_at = telemetry.ts

    return query
  }

  const openHomeGuideStep = async (step: HomeCustomerGuideProgressStep) => {
    saveFirstRunGuideStep(step)
    if (step.id === 'setup') {
      firstRunCreate.tenantRequired.value = step.status !== 'done'
      router.push(step.route)
      return
    }
    if (step.id === 'deployment') {
      void options.refreshDeploymentHealth().finally(() => {
        void options.focusHomeWorkbenchSection('deployment')
      })
      return
    }
    if (step.id === 'device') {
      await options.ensureFirstDeviceWorkbenchLoaded()
      if (!firstDevice.value) {
        void options.focusHomeWorkbenchSection('device')
        if (options.homeSetupReady.value && options.deploymentHealthOk.value) {
          void createFirstRunFirstDevice()
        }
        return
      }
      if (!firstDevice.value.online) {
        void options.focusHomeWorkbenchSection('test')
        return
      }
      void options.focusHomeWorkbenchSection('proof')
      return
    }
    if (step.id === 'telemetry') {
      await options.ensureFirstDeviceWorkbenchLoaded()
      if (!firstDevice.value) {
        void options.focusHomeWorkbenchSection('device')
        return
      }
      if (!firstDeviceTelemetry.value.length) {
        void options.focusHomeWorkbenchSection('test')
        return
      }
      if (!firstDeviceChart.value.ready) {
        void options.focusHomeWorkbenchSection('chart')
        return
      }
      void options.focusHomeWorkbenchSection('proof')
      return
    }
    if (step.id === 'automation') {
      await options.ensureFirstDeviceWorkbenchLoaded()
      router.push(
        options.hasSceneAutomation.value
          ? {
              path: '/automation/scene-linkage',
              query: buildFirstAutomationStarterQuery()
            }
          : {
              path: '/automation/linkage-edit',
              query: buildFirstAutomationStarterQuery()
            }
      )
      return
    }
    if (step.id === 'dashboard') {
      router.push({
        path: '/visualization/thingsvis',
        query: { onboarding: 'first-device' }
      })
      return
    }
    router.push(step.route)
  }

  const createFirstRunFirstDevice = async () => {
    if (firstRunCreate.loading.value) return
    if (!options.homeSetupReady.value) {
      firstRunCreate.tenantRequired.value = true
      window.$message?.warning(
        $t('custom.home.firstRun.setupRequired', { title: options.homeSetupGuideStep.value.title })
      )
      return
    }
    firstRunCreate.loading.value = true
    firstRunCreate.result.value = null
    firstRunCreate.tenantRequired.value = false
    try {
      const result = await createHomeFirstRunFirstDevice({
        userInfo: options.userInfo(),
        protocol: firstRunCreate.protocol.value
      })
      firstRunCreate.result.value = result
      const deviceStep = homeCustomerGuideProgress.value.find((step) => step.id === 'device')
      if (deviceStep) saveFirstRunGuideStep(deviceStep, result.deviceName)
      window.$message?.success($t('custom.home.firstRun.deviceCreated', { protocol: result.protocol }))
      await options.refreshFirstDeviceWorkbench()
      await options.focusHomeWorkbenchSection('test')
    } catch (error: any) {
      window.$message?.error(error?.message || $t('custom.home.firstRun.deviceCreateFailed'))
    } finally {
      firstRunCreate.loading.value = false
    }
  }

  watch(options.homeSetupReady, (ready) => {
    if (ready) {
      firstRunCreate.tenantRequired.value = false
    }
  })

  const runFirstDeviceQuickstartAction = (action: string) =>
    action === 'health'
      ? options.refreshDeploymentHealth()
      : options.ensureFirstDeviceWorkbenchLoaded().then(() =>
          options.workbench.runQuickstartAction(action, createFirstRunFirstDevice)
        )

  const openFirstDeviceAccessGuideAfterLoad = () =>
    options.ensureFirstDeviceWorkbenchLoaded().then(() => {
      options.workbench.openReadyCheck()
    })

  const openFirstDeviceFullGuideAfterLoad = () =>
    options.ensureFirstDeviceWorkbenchLoaded().then(() => {
      options.workbench.openFullGuide()
    })

  const copyFirstDevicePublishCommandAfterLoad = () =>
    options.ensureFirstDeviceWorkbenchLoaded().then(() => options.workbench.copyPublishCommand())

  const simulateFirstDeviceTelemetryAfterLoad = () =>
    options.ensureFirstDeviceWorkbenchLoaded().then(() => options.workbench.simulateTelemetry())

  return {
    firstRunGuideState,
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
  }
}
