/**
 * 文件用途：脚本执行的来源信任策略门禁。
 * 核心逻辑：按脚本来源（编辑器内建/导入配置）决定是否允许进入沙箱执行。
 * 关键注意事项：沙箱是同线程护栏而非隔离边界；来自外部导入看板配置的脚本默认不可信，
 * 必须显式放行才可执行，用于阻断"导入看板配置 → 客户端任意脚本执行"攻击链。
 * 重构建议：随 Worker 化迁移补齐配置级 trustOrigin 标记，实现按看板粒度授权与审计。
 */

export type ScriptExecutionSource = 'editor' | 'imported-config'

/** 稳定错误前缀：供 UI 与自动化测试识别该策略拒绝。 */
export const SCRIPT_IMPORTED_CONFIG_BLOCKED = 'SCRIPT_IMPORTED_CONFIG_BLOCKED'

let allowImportedConfigScripts = false

/** 查询当前是否允许执行来自导入看板配置的脚本；默认禁止。 */
export function importedConfigScriptsAllowed(): boolean {
  return allowImportedConfigScripts
}

/**
 * 显式放行/收回对导入配置脚本的执行许可。
 * 只应由受控设置入口（未来按看板 trustOrigin 授权）调用，不暴露给脚本自身可达的作用域。
 */
export function setImportedConfigScriptsAllowed(allowed: boolean): void {
  allowImportedConfigScripts = allowed
}

/** 断言给定来源的脚本允许执行；被策略拒绝时抛出带稳定错误前缀的异常。 */
export function assertScriptExecutionAllowed(source: ScriptExecutionSource): void {
  if (source !== 'imported-config') {
    return
  }
  if (!allowImportedConfigScripts) {
    throw new Error(
      `${SCRIPT_IMPORTED_CONFIG_BLOCKED}: scripts from imported board configs are disabled by policy`
    )
  }
}
