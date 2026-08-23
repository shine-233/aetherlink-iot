// 文件用途：控制首设备 workbench 数据的一次性加载与强制刷新。
// 核心逻辑：合并并发的刷新请求；未加载时只加载一次，force 时始终触发刷新。
// 关键注意事项：shouldLoad 判定决定 onboarding 路由与错误态下的初始数据策略。
import { ref } from 'vue'

type UseFirstDeviceWorkbenchGateOptions = {
  shouldLoad: () => boolean
  refresh: () => Promise<void>
}

export function useFirstDeviceWorkbenchGate(options: UseFirstDeviceWorkbenchGateOptions) {
  const firstDeviceWorkbenchLoaded = ref(false)
  let firstDeviceWorkbenchLoadPromise: Promise<void> | null = null

  const runFirstDeviceWorkbenchRefresh = async (force = false) => {
    if (!options.shouldLoad()) return
    if (!force && firstDeviceWorkbenchLoaded.value) return
    if (firstDeviceWorkbenchLoadPromise) {
      await firstDeviceWorkbenchLoadPromise
      if (!force) return
    }

    const refreshPromise = options.refresh().then(() => {
      firstDeviceWorkbenchLoaded.value = true
    })
    firstDeviceWorkbenchLoadPromise = refreshPromise
    try {
      await refreshPromise
    } finally {
      if (firstDeviceWorkbenchLoadPromise === refreshPromise) {
        firstDeviceWorkbenchLoadPromise = null
      }
    }
  }

  const ensureFirstDeviceWorkbenchLoaded = () => runFirstDeviceWorkbenchRefresh(false)
  const refreshFirstDeviceWorkbench = () => runFirstDeviceWorkbenchRefresh(true)

  return {
    firstDeviceWorkbenchLoaded,
    ensureFirstDeviceWorkbenchLoaded,
    refreshFirstDeviceWorkbench
  }
}
