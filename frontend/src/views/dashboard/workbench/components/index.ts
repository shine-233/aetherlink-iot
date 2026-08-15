/**
 * 文件用途：作为 frontend/src/views/dashboard/workbench/components 的统一导出入口。
 * 核心逻辑：集中转出相邻组件或模块，让上层页面保持稳定、简短的导入路径。
 * 关键注意事项：新增或重命名导出时要同步检查调用方和测试，避免运行时导入缺失。
 * 重构建议：如果导出项持续增多，可按业务语义拆分 barrel 文件，降低循环依赖风险。
 */
import WorkbenchHeader from './workbench-header/index.vue'
import WorkbenchMain from './workbench-main/index.vue'

export { WorkbenchHeader, WorkbenchMain }
