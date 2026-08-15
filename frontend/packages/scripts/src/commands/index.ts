/**
 * 文件用途：聚合导出 scripts 包的所有 CLI 子命令。
 * 核心逻辑：从各命令文件转发导出，供 CLI 入口统一注册。
 * 关键注意事项：新增导出会进入命令注册候选范围，需要同步检查 index.ts 中的 CLI 绑定。
 * 重构建议：可引入显式命令注册表，统一维护命令名称、说明和处理函数。
 */
export * from './git-commit'
export * from './cleanup'
export * from './update-pkg'
export * from './changelog'
export * from './release'
export * from './router'
