// 文件用途：初始化前端 Pinia 状态管理实例并挂载到 Vue 应用。
// 核心逻辑：创建 Pinia，注册 resetSetupStore 插件，然后通过 app.use 接入应用。
// 关键注意事项：该入口影响所有 store 模块，新增插件时需确认执行顺序和 SSR/测试环境兼容性。
// 重构建议：建议将插件注册清单显式化，并为重置插件行为保留 store 层回归测试。
import type { App } from 'vue'
import { createPinia } from 'pinia'
import { resetSetupStore } from './plugins'

/** Setup Vue store plugin pinia */
export function setupStore(app: App) {
  const store = createPinia()

  store.use(resetSetupStore)

  app.use(store)
}
