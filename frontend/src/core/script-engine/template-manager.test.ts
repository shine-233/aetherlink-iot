import { describe, expect, it } from 'vitest'
import { ScriptEngine } from './script-engine'
import { ScriptTemplateManager } from './template-manager'
import {
  ALL_BUILT_IN_TEMPLATES,
  DATA_FETCHER_TEMPLATES as FACADE_DATA_FETCHER_TEMPLATES,
  DATA_MERGER_TEMPLATES as FACADE_DATA_MERGER_TEMPLATES,
  DATA_PROCESSOR_TEMPLATES as FACADE_DATA_PROCESSOR_TEMPLATES,
  UTILITY_TEMPLATES as FACADE_UTILITY_TEMPLATES
} from './templates/built-in-templates'
import { DATA_FETCHER_TEMPLATES } from './templates/data-fetcher-templates'
import { DATA_MERGER_TEMPLATES } from './templates/data-merger-templates'
import { DATA_PROCESSOR_TEMPLATES } from './templates/data-processor-templates'
import { UTILITY_TEMPLATES } from './templates/utility-templates'

describe('ScriptTemplateManager', () => {
  it('keeps template modules identical to facade exports and aggregation order', () => {
    expect(FACADE_DATA_FETCHER_TEMPLATES).toBe(DATA_FETCHER_TEMPLATES)
    expect(FACADE_DATA_PROCESSOR_TEMPLATES).toBe(DATA_PROCESSOR_TEMPLATES)
    expect(FACADE_DATA_MERGER_TEMPLATES).toBe(DATA_MERGER_TEMPLATES)
    expect(FACADE_UTILITY_TEMPLATES).toBe(UTILITY_TEMPLATES)
    expect([
      ...DATA_FETCHER_TEMPLATES,
      ...DATA_PROCESSOR_TEMPLATES,
      ...DATA_MERGER_TEMPLATES,
      ...UTILITY_TEMPLATES
    ]).toEqual(ALL_BUILT_IN_TEMPLATES)
  })

  it('starts empty so built-in templates have one explicit registry', () => {
    const manager = new ScriptTemplateManager()
    expect(manager.getAllTemplates()).toEqual([])
  })

  it('registers exactly the canonical built-in template set in ScriptEngine', () => {
    const engine = new ScriptEngine()
    const templates = engine.templateManager.getAllTemplates()

    expect(templates).toHaveLength(ALL_BUILT_IN_TEMPLATES.length)
    expect(templates.map(template => template.name)).toEqual(
      ALL_BUILT_IN_TEMPLATES.map(template => template.name)
    )
    expect(new Set(templates.map(template => template.name)).size).toBe(templates.length)
  })

  it('keeps HTTP integration behind the explicit audited network adapter contract', () => {
    const template = ALL_BUILT_IN_TEMPLATES.find(candidate => candidate.name === 'HTTP API 数据获取')

    expect(template).toBeDefined()
    expect(template?.code).toContain('_utils.networkUtils')
    expect(template?.code).not.toContain('fetch(')
    expect(template?.description).toContain('SCRIPT_NETWORK_EXTERNAL_BLOCKED')
    expect(template?.usageSnippet).toContain('SCRIPT_NETWORK_EXTERNAL_BLOCKED')
  })

  it('preserves parameter validation and interpolation for explicit templates', () => {
    const manager = new ScriptTemplateManager()
    const template = manager.createTemplate({
      name: 'threshold',
      description: 'threshold expression',
      category: 'validation',
      code: 'return value > {{threshold}} && source === {{source}}',
      parameters: [
        { name: 'threshold', type: 'number', required: true },
        { name: 'source', type: 'string', required: true }
      ],
      isSystem: false
    })

    expect(manager.generateCode(template.id, { threshold: 20, source: 'device-1' })).toBe(
      'return value > 20 && source === "device-1"'
    )
    expect(() => manager.generateCode(template.id, { source: 'device-1' })).toThrow('缺少必需参数: threshold')
  })

  it('does not delete canonical system templates', () => {
    const engine = new ScriptEngine()
    const systemTemplate = engine.templateManager.getAllTemplates()[0]

    expect(systemTemplate.isSystem).toBe(true)
    expect(engine.templateManager.deleteTemplate(systemTemplate.id)).toBe(false)
    expect(engine.templateManager.getTemplate(systemTemplate.id)).toBe(systemTemplate)
  })
})
