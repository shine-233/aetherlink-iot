/**
 * 文件用途：定义 设备状态模块 的 Pinia 状态模块。
 * 核心逻辑：维护模块状态、计算属性和动作，并把状态变化暴露给页面、组件和路由流程。
 * 关键注意事项：状态字段、持久化键和跨模块调用属于前端契约，调整时需要同步测试与调用方。
 * 重构建议：可将副作用、接口访问和纯状态推导拆分，降低 store 文件复杂度。
 */
import { ref } from 'vue'
import { defineStore } from 'pinia'
import { SetupStoreId } from '@/enum'
import { createLatestQueryRunner } from '@/service/request/abortable'
import { deviceDetail } from '@/service/api'

export const useDeviceDataStore = defineStore(SetupStoreId.Device, () => {
  const deviceData = ref<DeviceManagement.DeviceDetail | any>({}) // 更具体的类型替换 any
  // 快速切换设备时，取消/丢弃未归的旧详情请求，避免旧设备数据覆盖新设备。
  const detailRunner = createLatestQueryRunner<Awaited<ReturnType<typeof deviceDetail>>>()

  async function fetchData(id: string) {
    const result = await detailRunner.run((signal) => deviceDetail(id, { signal }))

    // null = 被后一次查询取代或被主动取消：过期结果直接丢弃，不清空也不覆盖当前数据。
    if (result === null) return

    const { data, error } = result
    if (!error) {
      deviceData.value = data
    } else {
      deviceData.value = {}
    }
  }

  return { deviceData, fetchData }
})
