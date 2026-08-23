// 文件用途：承载首页的空闲时段任务调度工具。
// 核心逻辑：优先使用 requestIdleCallback 调度低优先级任务，不支持时回退到 setTimeout。
// 关键注意事项：无 window 环境（如 SSR/测试）下直接同步执行，保证任务不丢失。
export function scheduleIdleHomeTask(task: () => void, fallbackDelay = 100) {
  if (typeof window === 'undefined') {
    task()
    return
  }
  if ('requestIdleCallback' in window) {
    window.requestIdleCallback(task, { timeout: 2000 })
    return
  }
  ;(window as Window).setTimeout(task, fallbackDelay)
}
