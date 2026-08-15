/**
 * 文件用途：管理脚本模板的创建、更新、删除、查询和代码生成。
 * 核心逻辑：使用内存 Map 保存模板，并根据模板参数替换生成可执行脚本。
 * 关键注意事项：参数替换直接影响生成代码，必须保持默认值和必填校验一致。
 * 重构建议：可引入模板解析层和持久化适配器，支持更安全的参数插值与存储。
 */

import type {
  IScriptTemplateManager,
  ScriptTemplate,
  ScriptTemplateParameter,
  TemplateCategory
} from '@/core/script-engine/types'
import { nanoid } from 'nanoid'

/**
 * 脚本模板管理器实现类
 */
export class ScriptTemplateManager implements IScriptTemplateManager {
  private templates: Map<string, ScriptTemplate>

  constructor() {
    // 模板由统一 built-in registry 或调用方显式注册，避免维护两套重叠内置模板。
    this.templates = new Map()
  }

  /**
   * 获取所有模板
   */
  getAllTemplates(): ScriptTemplate[] {
    return Array.from(this.templates.values())
  }

  /**
   * 根据分类获取模板
   */
  getTemplatesByCategory(category: string): ScriptTemplate[] {
    return Array.from(this.templates.values()).filter(template => template.category === category)
  }

  /**
   * 获取指定模板
   */
  getTemplate(id: string): ScriptTemplate | null {
    return this.templates.get(id) || null
  }

  /**
   * 创建模板
   */
  createTemplate(template: Omit<ScriptTemplate, 'id' | 'createdAt' | 'updatedAt'>): ScriptTemplate {
    const now = Date.now()
    const newTemplate: ScriptTemplate = {
      ...template,
      id: nanoid(),
      createdAt: now,
      updatedAt: now
    }

    this.templates.set(newTemplate.id, newTemplate)
    return newTemplate
  }

  /**
   * 更新模板
   */
  updateTemplate(id: string, updates: Partial<ScriptTemplate>): boolean {
    const template = this.templates.get(id)
    if (!template) {
      return false
    }

    const updatedTemplate: ScriptTemplate = {
      ...template,
      ...updates,
      id, // 确保ID不被修改
      updatedAt: Date.now()
    }

    this.templates.set(id, updatedTemplate)
    return true
  }

  /**
   * 删除模板
   */
  deleteTemplate(id: string): boolean {
    const template = this.templates.get(id)
    if (!template || template.isSystem) {
      return false // 不能删除系统模板
    }

    return this.templates.delete(id)
  }

  /**
   * 根据模板生成代码
   */
  generateCode(templateId: string, parameters: Record<string, any>): string {
    const template = this.templates.get(templateId)
    if (!template) {
      throw new Error(`模板不存在: ${templateId}`)
    }

    let code = template.code

    // 替换模板参数
    template.parameters.forEach(param => {
      const value = parameters[param.name]
      const actualValue = value !== undefined ? value : param.defaultValue

      if (param.required && actualValue === undefined) {
        throw new Error(`缺少必需参数: ${param.name}`)
      }

      // 验证参数
      this.validateParameter(param, actualValue)

      // 替换代码中的占位符
      const placeholder = new RegExp(`\\{\\{${param.name}\\}\\}`, 'g')
      const replacement = this.formatParameterValue(actualValue, param.type)
      code = code.replace(placeholder, replacement)
    })

    return code
  }

  /**
   * 验证参数值
   */
  private validateParameter(param: ScriptTemplateParameter, value: any): void {
    if (value === undefined && !param.required) {
      return
    }

    // 类型检查
    switch (param.type) {
      case 'string':
        if (typeof value !== 'string') {
          throw new Error(`参数 ${param.name} 必须是字符串类型`)
        }
        break
      case 'number':
        if (typeof value !== 'number') {
          throw new Error(`参数 ${param.name} 必须是数字类型`)
        }
        break
      case 'boolean':
        if (typeof value !== 'boolean') {
          throw new Error(`参数 ${param.name} 必须是布尔类型`)
        }
        break
      case 'object':
        if (typeof value !== 'object' || value === null) {
          throw new Error(`参数 ${param.name} 必须是对象类型`)
        }
        break
      case 'array':
        if (!Array.isArray(value)) {
          throw new Error(`参数 ${param.name} 必须是数组类型`)
        }
        break
    }

    // 验证规则检查
    if (param.validation) {
      const validation = param.validation

      // 数值范围检查
      if (typeof value === 'number') {
        if (validation.min !== undefined && value < validation.min) {
          throw new Error(`参数 ${param.name} 不能小于 ${validation.min}`)
        }
        if (validation.max !== undefined && value > validation.max) {
          throw new Error(`参数 ${param.name} 不能大于 ${validation.max}`)
        }
      }

      // 字符串长度检查
      if (typeof value === 'string') {
        if (validation.min !== undefined && value.length < validation.min) {
          throw new Error(`参数 ${param.name} 长度不能小于 ${validation.min}`)
        }
        if (validation.max !== undefined && value.length > validation.max) {
          throw new Error(`参数 ${param.name} 长度不能大于 ${validation.max}`)
        }
        if (validation.pattern && !new RegExp(validation.pattern).test(value)) {
          throw new Error(`参数 ${param.name} 格式不正确`)
        }
      }

      // 枚举值检查
      if (validation.enum && !validation.enum.includes(value)) {
        throw new Error(`参数 ${param.name} 必须是以下值之一: ${validation.enum.join(', ')}`)
      }
    }
  }

  /**
   * 格式化参数值
   */
  private formatParameterValue(value: any, type: ScriptTemplateParameter['type']): string {
    switch (type) {
      case 'string':
        return JSON.stringify(value)
      case 'number':
      case 'boolean':
        return String(value)
      case 'object':
      case 'array':
        return JSON.stringify(value)
      case 'function':
        return typeof value === 'function' ? value.toString() : String(value)
      default:
        return JSON.stringify(value)
    }
  }
}
