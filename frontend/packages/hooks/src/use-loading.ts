/**
 * 文件用途：提供语义化 loading 状态 hook。
 * 核心逻辑：复用 useBoolean，将置真/置假方法命名为 startLoading 和 endLoading。
 * 关键注意事项：该 hook 只管理状态，不自动绑定异步任务生命周期。
 * 重构建议：可新增 withLoading 辅助函数，统一异步任务的 finally 收尾。
 */
import useBoolean from './use-boolean'

/**
 * Loading
 *
 * @param initValue Init value
 */
export default function useLoading(initValue = false) {
  const { bool: loading, setTrue: startLoading, setFalse: endLoading } = useBoolean(initValue)

  return {
    loading,
    startLoading,
    endLoading
  }
}
