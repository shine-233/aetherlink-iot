/**
 * 文件用途：声明 naive-ui 相关的前端类型边界。
 * 核心逻辑：通过 namespace、interface 或模块声明约束跨文件调用形态。
 * 关键注意事项：声明文件可能影响全局类型推断或生成类型边界，修改需检查全量 TS 引用。
 * 重构建议：可逐步把宽泛全局声明收敛为模块化导出，并标注生成来源。
 */
declare namespace NaiveUI {
  type ThemeColor = 'default' | 'error' | 'primary' | 'info' | 'success' | 'warning'
}
