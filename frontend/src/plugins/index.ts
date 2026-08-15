/**
 * 文件用途：导出前端插件初始化入口集合。
 * 核心逻辑：统一转发 loading、nprogress、iconify 和 dayjs 等插件模块，供应用启动流程引用。
 * 关键注意事项：这里是插件 import 契约，删除导出前需确认启动链路和测试入口不再依赖。
 * 重构建议：新增插件时保持副作用初始化和显式 install 函数边界清晰。
 */
export * from './loading'
export * from './nprogress'
export * from './dayjs'
