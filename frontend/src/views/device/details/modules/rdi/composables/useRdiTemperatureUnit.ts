/**
 * 文件用途：在 RDI 详情的实时状态与历史模块之间共享温标偏好。
 * 关键输入：首次消费时读取 localStorage 中的 rdi-temperature-unit。
 * 主要副作用：温标变化后写回 localStorage，使不同 RDI tab 保持同一 C/F 选择。
 * 维护注意：该状态是页面模块级单例；不要在各视图重新创建独立温标 ref。
 */
import { ref, watch, type Ref } from 'vue'

type RdiTemperatureUnit = 'C' | 'F'

const temperatureUnit = ref<RdiTemperatureUnit>('C')
let initialized = false

function readTemperatureUnitPreference(): RdiTemperatureUnit {
  try {
    return typeof window !== 'undefined' && window.localStorage.getItem('rdi-temperature-unit') === 'F' ? 'F' : 'C'
  } catch {
    return 'C'
  }
}

function persistTemperatureUnitPreference(value: RdiTemperatureUnit) {
  try {
    if (typeof window !== 'undefined') window.localStorage.setItem('rdi-temperature-unit', value)
  } catch {
    // The shared in-memory selection remains valid for the current session.
  }
}

export function useRdiTemperatureUnit(): Ref<RdiTemperatureUnit> {
  if (!initialized) {
    temperatureUnit.value = readTemperatureUnitPreference()
    watch(temperatureUnit, persistTemperatureUnitPreference)
    initialized = true
  }
  return temperatureUnit
}
