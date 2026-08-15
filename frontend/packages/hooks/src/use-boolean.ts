/**
 * 文件用途：提供布尔状态的 Vue 组合式函数。
 * 核心逻辑：基于 ref 维护布尔值，并提供设置、置真、置假和切换方法。
 * 关键注意事项：返回的 bool 是 Ref，调用方需要按 Vue 响应式规则读取和传递。
 * 重构建议：可补充只读返回或命名别名，减少调用方误修改状态的可能。
 */
import { ref } from 'vue'

/**
 * Boolean
 *
 * @param initValue Init value
 */
export default function useBoolean(initValue = false) {
  const bool = ref(initValue)

  function setBool(value: boolean) {
    bool.value = value
  }
  function setTrue() {
    setBool(true)
  }
  function setFalse() {
    setBool(false)
  }
  function toggle() {
    setBool(!bool.value)
  }

  return {
    bool,
    setBool,
    setTrue,
    setFalse,
    toggle
  }
}
