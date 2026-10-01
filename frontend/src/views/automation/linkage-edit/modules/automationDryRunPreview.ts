/**
 * 联动预演视图层的桶文件：保持既有导入路径与导出名不变。
 *
 * 模块划分：
 *   - automationDryRunTypes      公共视图类型
 *   - automationDryRunNormalize  后端响应 → NormalizedDryRun 的唯一归一化边界
 *   - automationDryRunFormat     共享格式化原语
 *   - automationDryRunLocal      不依赖后端响应的本地说明与表单摘要
 *   - automationDryRunGuide      操作员执行计划与新手引导卡片
 *   - automationDryRunViews      trace / 工程视图 / 客户视图
 */
export type {
  AutomationConditionSummaryGroup,
  AutomationDryRunBackendView,
  AutomationDryRunBeginnerGuideCard,
  AutomationDryRunCustomerStatus,
  AutomationDryRunCustomerView,
  AutomationDryRunDiagnosticItem,
  AutomationDryRunIssueLine,
  AutomationDryRunLine,
  AutomationDryRunOperatorPlan,
  AutomationDryRunQuickFixAction,
  AutomationDryRunTone,
  AutomationDryRunTraceStep,
  AutomationDryRunTraceView,
  BackendDryRunStatus
} from './automationDryRunTypes'

export { normalizeDryRunResponse } from './automationDryRunNormalize'
export type { DryRunResponseInput, DryRunWireResult, NormalizedDryRun } from './automationDryRunNormalize'

export {
  buildActionSummaryItems,
  buildConditionSummaryItems,
  getAutomationDryRunAlertType,
  getAutomationDryRunStatusText,
  getPreviewErrorText,
  stringifyDryRunResponse
} from './automationDryRunLocal'

export { buildAutomationDryRunBeginnerGuide, buildAutomationOperatorPlan } from './automationDryRunGuide'
export type { AutomationDryRunBeginnerGuideOptions } from './automationDryRunGuide'

export { buildAutomationDryRunCustomerView, buildBackendDryRunView, buildTraceView } from './automationDryRunViews'
