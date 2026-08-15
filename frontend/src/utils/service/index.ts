/*
 * 文件用途：聚合导出 service 工具入口。
 * 核心逻辑：从 handler 模块重新导出接口结果处理能力。
 * 关键注意事项：新增导出会扩大公共工具 API，应保持命名清晰。
 * 重构建议：后续可按 handler、adapter、error-policy 拆分入口。
 */
export * from './handler'
