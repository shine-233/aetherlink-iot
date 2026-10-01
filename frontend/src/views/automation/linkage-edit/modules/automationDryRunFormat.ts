/**
 * 预演视图共享的格式化原语：值展示、计数行、问题行。
 * 只处理已归一化的数据，不了解后端字段名。
 */
import type { AutomationDryRunIssueLine, AutomationDryRunLine } from './automationDryRunTypes'

export const formatPreviewValue = (value: unknown) => {
  if (value === null || value === undefined || value === '') return '--'
  if (typeof value === 'object') return JSON.stringify(value)

  return String(value)
}

export const buildCountLines = (prefix: string, record: Record<string, number>): AutomationDryRunLine[] =>
  Object.entries(record).map(([label, value]) => ({
    key: `${prefix}-${label}`,
    text: `${label}: ${value}`
  }))

export const buildIssueLines = (prefix: string, messages: string[]): AutomationDryRunIssueLine[] =>
  messages.map((text, index) => ({
    key: `${prefix}-${index}`,
    text
  }))

export const buildStepLines = (messages: string[]): AutomationDryRunLine[] =>
  messages.map((text, index) => ({
    key: `next-step-${index}`,
    text
  }))

export const firstLineText = (...lineGroups: AutomationDryRunLine[][]) => {
  for (const lines of lineGroups) {
    if (lines.length > 0) return lines[0].text
  }

  return ''
}
