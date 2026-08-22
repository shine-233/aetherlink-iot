// 文件用途：承载首设备引导的焦点定位与 onboarding 路由查询消费。
// 核心逻辑：从 home/index.vue 抽出的交互编排，负责 workbench 视图等待、分区聚焦与 URL 焦点参数清理。
// 关键注意事项：focus 映射表是页面对外契约，调整前需同步检查 onboarding 链接生成方。
import { nextTick, ref, type Ref } from 'vue'
import { router } from '@/router'
import type { RouteLocationNormalizedLoaded } from 'vue-router'

export type HomeFirstDeviceWorkbenchViewExpose = {
  focusDeploymentHealth: () => Promise<void>
  focusSection: (key: string) => Promise<void>
}

type UseFirstDeviceFocusRouterOptions = {
  workbenchViewRef: Ref<HomeFirstDeviceWorkbenchViewExpose | null>
  ensureWorkbenchLoaded: () => Promise<void> | void
  route: Pick<RouteLocationNormalizedLoaded, 'query' | 'hash' | 'path'>
}

const FIRST_DEVICE_FOCUS_REF_RETRY_LIMIT = 40
const FIRST_DEVICE_FOCUS_REF_RETRY_DELAY_MS = 50

const FIRST_DEVICE_FOCUS_SECTION_MAP: Record<string, string> = {
  deployment: 'deployment',
  health: 'deployment',
  device: 'device',
  identity: 'device',
  connection: 'connection',
  command: 'connection',
  test: 'test',
  sample: 'test',
  tester: 'test',
  browser_test: 'test',
  online: 'proof',
  telemetry: 'chart',
  chart: 'chart',
  proof: 'proof',
  support: 'support',
  quickstart: 'quickstart',
  'first-device-proof': 'proof',
  'first-device-chart': 'chart'
}

export function useFirstDeviceFocusRouter(options: UseFirstDeviceFocusRouterOptions) {
  const { workbenchViewRef, ensureWorkbenchLoaded, route } = options

  const skipNextDefaultFirstDeviceFocus = ref(false)

  const waitForFirstDeviceWorkbenchView = async () => {
    if (workbenchViewRef.value) return workbenchViewRef.value

    for (let attempt = 0; attempt < FIRST_DEVICE_FOCUS_REF_RETRY_LIMIT; attempt += 1) {
      await nextTick()
      if (workbenchViewRef.value) return workbenchViewRef.value
      await new Promise((resolve) => window.setTimeout(resolve, FIRST_DEVICE_FOCUS_REF_RETRY_DELAY_MS))
      if (workbenchViewRef.value) return workbenchViewRef.value
    }

    return workbenchViewRef.value
  }

  const focusHomeWorkbenchSection = async (key: string) => {
    await ensureWorkbenchLoaded()
    const workbenchView = await waitForFirstDeviceWorkbenchView()
    await (workbenchView?.focusSection(key) || Promise.resolve())
  }

  const consumeFirstDeviceFocusQuery = () => {
    const routeHash = String(route.hash || '').replace(/^#/, '')
    if (route.query.onboarding !== 'first-device' && !routeHash.startsWith('first-device')) return
    const queryFocus = Array.isArray(route.query.focus) ? route.query.focus[0] : route.query.focus
    if (!queryFocus && !routeHash && skipNextDefaultFirstDeviceFocus.value) {
      skipNextDefaultFirstDeviceFocus.value = false
      return
    }
    const focus = queryFocus || routeHash || 'quickstart'
    const section = FIRST_DEVICE_FOCUS_SECTION_MAP[String(focus || 'quickstart')] || 'quickstart'

    window.setTimeout(() => {
      void focusHomeWorkbenchSection(section)
    }, 0)
    const shouldClearFocus = Boolean(queryFocus || routeHash)
    if (!shouldClearFocus) return
    skipNextDefaultFirstDeviceFocus.value = true
    const { focus: _focus, ...query } = route.query
    router.replace({
      path: route.path,
      query,
      hash: ''
    })
  }

  return {
    focusHomeWorkbenchSection,
    consumeFirstDeviceFocusQuery
  }
}
