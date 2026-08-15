/*
 * 文件用途：提供 loading 与 empty 状态 Hook，统一页面和组件的加载/空态控制。
 * 核心逻辑：用 ref 保存状态，并返回 startLoading、endLoading、setEmpty 等简单操作。
 * 关键注意事项：该 Hook 不处理请求本身，调用方需要在异常分支中正确结束 loading。
 * 重构建议：如加入异步包装能力，应保证错误透传和 finally 清理。
 */
import { useBoolean } from '@aetherlink/hooks'
export default function useLoadingEmpty(initLoading = false, initEmpty = false) {
  const { bool: loading, setTrue: startLoading, setFalse: endLoading } = useBoolean(initLoading)
  const { bool: empty, setBool: setEmpty } = useBoolean(initEmpty)

  return {
    loading,
    startLoading,
    endLoading,
    empty,
    setEmpty
  }
}
