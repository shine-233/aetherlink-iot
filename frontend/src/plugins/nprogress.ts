/**
 * 文件用途：配置页面切换进度条插件。
 * 核心逻辑：初始化 nprogress 默认参数和样式，供路由守卫或请求流程控制进度条。
 * 关键注意事项：进度条启动和结束必须成对维护，避免路由异常时残留加载状态。
 * 重构建议：可把路由进度控制封装到守卫层，插件文件只保留配置。
 */
import NProgress from 'nprogress'

/** Setup plugin NProgress */
export function setupNProgress() {
  NProgress.configure({ easing: 'ease', speed: 500 })

  // mount on window
  window.NProgress = NProgress
}
