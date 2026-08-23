// 文件用途：保存首台设备快速创建的交互状态。
// 核心逻辑：集中协议选择、创建进行中标记、创建结果与租户阻断标记四类响应式状态。
// 关键注意事项：该状态需先于 workbench 创建，供 onTenantRequired 回调与引导动作共享写入。
import { ref } from 'vue'
import type { HomeFirstRunProtocol, HomeFirstRunQuickCreateResult } from './homeFirstRunWizard'

export function createHomeFirstRunCreateState() {
  const loading = ref(false)
  const protocol = ref<HomeFirstRunProtocol>('MQTT')
  const result = ref<HomeFirstRunQuickCreateResult | null>(null)
  const tenantRequired = ref(false)

  return {
    loading,
    protocol,
    result,
    tenantRequired
  }
}

export type HomeFirstRunCreateState = ReturnType<typeof createHomeFirstRunCreateState>
