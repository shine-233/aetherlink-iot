/**
 * 脚本引擎内置模板的聚合与注册 facade。
 *
 * 模板定义按场景拆分到独立模块；这里仅负责统一导出、固定顺序聚合、注册和初始化统计。
 */

import { TemplateCategory } from '@/core/script-engine/types'
import { DATA_FETCHER_TEMPLATES } from './data-fetcher-templates'
import { DATA_MERGER_TEMPLATES } from './data-merger-templates'
import { DATA_PROCESSOR_TEMPLATES } from './data-processor-templates'
import type { BuiltInTemplateDefinition } from './definition-types'
import { UTILITY_TEMPLATES } from './utility-templates'

export {
  DATA_FETCHER_TEMPLATES,
  DATA_PROCESSOR_TEMPLATES,
  DATA_MERGER_TEMPLATES,
  UTILITY_TEMPLATES
}

type BuiltInTemplateManager = {
  createTemplate: (template: BuiltInTemplateDefinition) => unknown
}

type BuiltInTemplateCategory =
  | TemplateCategory.DATA_GENERATION
  | TemplateCategory.DATA_PROCESSING
  | TemplateCategory.API_INTEGRATION
  | TemplateCategory.TIME_SERIES
  | TemplateCategory.TRANSFORMATION
  | TemplateCategory.VALIDATION
  | TemplateCategory.UTILITY

// 初始化统计只关心这几类内置模板，既用于校验，也用于汇总分类数量。
type BuiltInTemplateInitializationStats = {
  total: number
  success: number
  error: number
  categories: Record<BuiltInTemplateCategory, number>
}

// 模板分类白名单，保持与 ScriptTemplate.category 的枚举范围一致。
const BUILT_IN_TEMPLATE_CATEGORIES: BuiltInTemplateCategory[] = [
  TemplateCategory.DATA_GENERATION,
  TemplateCategory.DATA_PROCESSING,
  TemplateCategory.API_INTEGRATION,
  TemplateCategory.TIME_SERIES,
  TemplateCategory.TRANSFORMATION,
  TemplateCategory.VALIDATION,
  TemplateCategory.UTILITY
]

const BUILT_IN_TEMPLATE_CATEGORY_SET = new Set<BuiltInTemplateCategory>(BUILT_IN_TEMPLATE_CATEGORIES)

function createEmptyBuiltInTemplateCategoryStats(): Record<BuiltInTemplateCategory, number> {
  return {
    [TemplateCategory.DATA_GENERATION]: 0,
    [TemplateCategory.DATA_PROCESSING]: 0,
    [TemplateCategory.API_INTEGRATION]: 0,
    [TemplateCategory.TIME_SERIES]: 0,
    [TemplateCategory.TRANSFORMATION]: 0,
    [TemplateCategory.VALIDATION]: 0,
    [TemplateCategory.UTILITY]: 0
  }
}

function isBuiltInTemplateCategory(category: string): category is BuiltInTemplateCategory {
  return BUILT_IN_TEMPLATE_CATEGORY_SET.has(category as BuiltInTemplateCategory)
}

function collectBuiltInTemplateCategoryStats(
  templates: BuiltInTemplateDefinition[]
): Record<BuiltInTemplateCategory, number> {
  const stats = createEmptyBuiltInTemplateCategoryStats()

  for (const template of templates) {
    if (isBuiltInTemplateCategory(template.category)) {
      stats[template.category] += 1
    }
  }

  return stats
}

// 单个模板注册失败时只记失败，不打断其他模板的初始化。
function registerBuiltInTemplate(
  templateManager: BuiltInTemplateManager,
  template: BuiltInTemplateDefinition
): boolean {
  try {
    templateManager.createTemplate(template)
    return true
  } catch {
    return false
  }
}

// 聚合后的模板清单，供外部直接消费或做统一注册。
const BUILT_IN_TEMPLATE_GROUPS: BuiltInTemplateDefinition[][] = [
  DATA_FETCHER_TEMPLATES,
  DATA_PROCESSOR_TEMPLATES,
  DATA_MERGER_TEMPLATES,
  UTILITY_TEMPLATES
]

export const ALL_BUILT_IN_TEMPLATES: BuiltInTemplateDefinition[] = BUILT_IN_TEMPLATE_GROUPS.flat()

const BUILT_IN_TEMPLATE_CATEGORY_STATS = collectBuiltInTemplateCategoryStats(ALL_BUILT_IN_TEMPLATES)

/**
 * 将所有内置模板注册到模板管理器，并返回初始化统计。
 *
 * 这个函数只负责批量注册与统计，不负责模板生成，也不修改模板内容本身。
 */
export function initializeBuiltInTemplates(
  templateManager: BuiltInTemplateManager
): BuiltInTemplateInitializationStats {
  return ALL_BUILT_IN_TEMPLATES.reduce<BuiltInTemplateInitializationStats>(
    (summary, template) => {
      if (registerBuiltInTemplate(templateManager, template)) {
        summary.success += 1
      } else {
        summary.error += 1
      }

      return summary
    },

    // 返回统计信息
    {
      total: ALL_BUILT_IN_TEMPLATES.length,
      success: 0,
      error: 0,
      categories: { ...BUILT_IN_TEMPLATE_CATEGORY_STATS }
    }
  )
}
