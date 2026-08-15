/**
 * 文件用途：集中导出前端运行配置和静态开关。
 * 核心逻辑：聚合主题、路由、权限或项目级配置供入口和模块读取。
 * 关键注意事项：配置项常被构建期和运行期共同消费，命名变更需检查引用链。
 * 重构建议：可按权限、布局、运行时三类拆分，减少单文件配置膨胀。
 */
/**
 * 文件：前端配置统一入口。
 * 作用：集中导出配置模块，并暴露配置模块元信息。
 * 依赖：依赖当前目录下的 security 安全配置模块。
 * 维护：新增配置子模块时同步扩展导出列表和 configInfo.modules。
 */

/**
 * 配置模块统一导出
 * Configuration Module Unified Exports
 */

// 导出安全配置
export * from './security'

/**
 * 配置模块信息
 * Configuration Module Information
 */
export const configInfo = {
  version: '1.0.0',
  description: 'AetherLink IoT Frontend Configuration Module',
  modules: ['security'],
  lastUpdated: new Date().toISOString()
} as const
