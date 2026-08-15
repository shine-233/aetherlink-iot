/**
 * 文件用途：声明 global 相关的前端类型边界。
 * 核心逻辑：通过 namespace、interface 或模块声明约束跨文件调用形态。
 * 关键注意事项：声明文件可能影响全局类型推断或生成类型边界，修改需检查全量 TS 引用。
 * 重构建议：可逐步把宽泛全局声明收敛为模块化导出，并标注生成来源。
 */
interface Window {
  /** NProgress instance */
  NProgress?: import('nprogress').NProgress
  /** Loading bar instance */
  $loadingBar?: import('naive-ui').LoadingBarProviderInst
  /** Dialog instance */
  $dialog?: import('naive-ui').DialogProviderInst
  /** Message instance */
  $message?: import('naive-ui').MessageProviderInst
  /** Notification instance */
  $notification?: import('naive-ui').NotificationProviderInst
}

// ViewTransition 和 Document.startViewTransition 已由 TypeScript 5.x DOM lib 内建提供
// 不再需要本地声明，避免与内建类型修饰符冲突
type OptionTypes = {
  label: string
  value: any
}
interface ImportMeta {
  readonly env: Env.ImportMeta
}

declare namespace Common2 {
  /** 策略模式 [状态, 为true时执行的回调函数] */
  type StrategyAction = [boolean, () => void]

  /** 选项数据 */
  type OptionWithKey<K> = { value: K; label: string }
}

/** Build time of the project */
declare const BUILD_TIME: string

// eslint-disable-next-line no-redeclare
declare interface Window {
  NMessage: any
}
