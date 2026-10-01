/**
 * Command Job 视图 barrel：保留历史导入路径，具体实现按关注点拆分在
 * - commandCenterJobFormat   共享类型、tone 分类、格式化
 * - commandCenterJobProgress 计数派生、进度健康、审计、治理、时间线、交接文本
 * - commandCenterJobOutcome  单设备下一步、结果分组、进度轨道
 * - commandCenterJobHistory  历史筛选项与关注聚合
 * 新代码可直接从具体模块导入。
 */
export * from './commandCenterJobFormat'
export * from './commandCenterJobProgress'
export * from './commandCenterJobOutcome'
export * from './commandCenterJobHistory'

export {
  buildCommandJobActionConsequenceRows,
  buildCommandJobOperatorNextAction,
  buildCommandJobTroubleshootingRows,
  type CommandJobActionConsequenceRow,
  type CommandJobOperatorNextAction,
  type CommandJobTroubleshootingRow
} from './commandCenterJobOperatorDecisionView'
export {
  buildCommandJobExecutionSummaryCard,
  buildCommandJobSupportBundlePreview,
  type CommandJobExecutionChecklistItem,
  type CommandJobExecutionSummaryCard,
  type CommandJobSupportBundlePreview,
  type CommandJobSupportFailedDeviceEvidence,
  type CommandJobSupportDiagnosticPreview
} from './commandCenterJobSupportBundlePreviewView'
