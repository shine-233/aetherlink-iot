/**
 * 文件用途: visual-editor 配置层类型定义。
 * 核心逻辑: 复用基础 WidgetConfiguration，并描述配置更新结果等配置服务返回结构。
 * 关键注意事项: WidgetConfiguration 是持久化配置契约，默认字段需与 bridge/state manager 保持一致。
 * 重构建议: 将配置 section 类型显式化，并为版本号和错误结构补类型级/运行时测试。
 */
import type { WidgetConfiguration as BaseWidgetConfiguration } from '@/components/visual-editor/types'

export type WidgetConfiguration = BaseWidgetConfiguration

export interface ConfigurationUpdateResult {
  success: boolean
  version?: number
  error?: string
}
