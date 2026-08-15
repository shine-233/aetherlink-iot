/*
 * 文件用途：提供倒计时 Hook，用于验证码、按钮冷却等需要秒级递减的业务场景。
 * 核心逻辑：用 ref/computed 管理剩余秒数、计时状态和 start/stop 控制。
 * 关键注意事项：调用方需要在组件卸载或流程结束时停止计时器，避免遗留 interval。
 * 重构建议：可进一步封装自动清理生命周期，并补充边界秒数测试。
 */
import { computed, onScopeDispose, ref } from 'vue'
import { useBoolean } from '@aetherlink/hooks'

/**
 * 倒计时
 *
 * @param second - 倒计时的时间(s)
 */
export default function useCountDown(second: number) {
  if (second <= 0 && second % 1 !== 0) {
    throw new Error('倒计时的时间应该为一个正整数！')
  }
  const { bool: isComplete, setTrue, setFalse } = useBoolean(false)

  const counts = ref(0)
  const isCounting = computed(() => Boolean(counts.value))

  let intervalId: any

  /**
   * 开始计时
   *
   * @param updateSecond - 更改初时传入的倒计时时间
   */
  function start(updateSecond: number = second) {
    if (!counts.value) {
      setFalse()
      counts.value = updateSecond
      intervalId = setInterval(() => {
        counts.value -= 1
        if (counts.value <= 0) {
          clearInterval(intervalId)
          setTrue()
        }
      }, 1000)
    }
  }

  /** 停止计时 */
  function stop() {
    intervalId = clearInterval(intervalId)
    counts.value = 0
  }

  onScopeDispose(stop)

  return {
    counts,
    isCounting,
    start,
    stop,
    isComplete
  }
}
