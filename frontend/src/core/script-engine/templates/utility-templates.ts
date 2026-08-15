import type { BuiltInTemplateDefinition } from './definition-types'

/**
 * 通用工具类模板：用于校验、诊断和性能观测等通用任务。
 */
export const UTILITY_TEMPLATES: BuiltInTemplateDefinition[] = [
  {
    name: '数据验证器',
    description: '验证数据格式和完整性',
    category: 'validation',
    code: `// 数据验证器
const rules = context?.rules || {}
const result = {
  valid: true,
  errors: [],
  warnings: [],
  summary: {}
}

// 基础类型检查
if (rules.type) {
  const actualType = Array.isArray(data) ? 'array' : typeof data
  if (actualType !== rules.type) {
    result.valid = false
    result.errors.push(\`类型错误: 期望 \${rules.type}, 实际 \${actualType}\`)
  }
}

// 必需字段检查
if (rules.required && Array.isArray(rules.required)) {
  rules.required.forEach(field => {
    if (data[field] === undefined || data[field] === null) {
      result.valid = false
      result.errors.push(\`缺少必需字段: \${field}\`)
    }
  })
}

// 数值范围检查
if (rules.ranges && typeof data === 'object') {
  Object.keys(rules.ranges).forEach(field => {
    if (data[field] !== undefined) {
      const range = rules.ranges[field]
      const value = data[field]

      if (typeof value === 'number') {
        if (range.min !== undefined && value < range.min) {
          result.errors.push(\`\${field} 值 \${value} 小于最小值 \${range.min}\`)
        }
        if (range.max !== undefined && value > range.max) {
          result.errors.push(\`\${field} 值 \${value} 大于最大值 \${range.max}\`)
        }
      }
    }
  })
}

// 格式检查
if (rules.formats && typeof data === 'object') {
  Object.keys(rules.formats).forEach(field => {
    if (data[field] !== undefined) {
      const pattern = new RegExp(rules.formats[field])
      if (!pattern.test(String(data[field]))) {
        result.warnings.push(\`\${field} 格式可能不正确\`)
      }
    }
  })
}

// 生成摘要
result.summary = {
  fieldsChecked: Object.keys(data || {}).length,
  errorsCount: result.errors.length,
  warningsCount: result.warnings.length,
  validationTime: Date.now()
}

return result`,
    parameters: [
      {
        name: 'rules',
        type: 'object',
        description: '验证规则配置',
        required: false,
        defaultValue: {}
      }
    ],
    usageSnippet: '// rules = { type: "object", required: ["device_id", "timestamp", "metrics"] }',
    isSystem: true
  },

  {
    name: '遥测脚本性能监控',
    description: '监控遥测处理脚本的执行耗时和资源使用',
    category: 'utility',
    code: `// 遥测脚本性能监控
const startTime = performance.now()
const memoryBefore = performance.memory ? performance.memory.usedJSHeapSize : 0

// 执行主要逻辑（这里放置实际的遥测处理代码）
const processedData = data

// 性能测量
const endTime = performance.now()
const memoryAfter = performance.memory ? performance.memory.usedJSHeapSize : 0

const metrics = {
  execution: {
    startTime: startTime,
    endTime: endTime,
    duration: endTime - startTime,
    durationText: \`\${(endTime - startTime).toFixed(2)}ms\`
  },
  memory: {
    before: memoryBefore,
    after: memoryAfter,
    used: memoryAfter - memoryBefore,
    usedText: \`\${((memoryAfter - memoryBefore) / 1024 / 1024).toFixed(2)}MB\`
  },
  data: {
    inputSize: JSON.stringify(data || {}).length,
    outputSize: JSON.stringify(processedData || {}).length,
    compressionRatio: JSON.stringify(data || {}).length > 0 ?
      (JSON.stringify(processedData || {}).length / JSON.stringify(data || {}).length).toFixed(2) : 1
  },
  performance: {
    rating: endTime - startTime < 100 ? 'excellent' :
            endTime - startTime < 500 ? 'good' :
            endTime - startTime < 1000 ? 'fair' : 'poor',
    recommendations: []
  }
}

// 性能建议
if (metrics.execution.duration > 1000) {
  metrics.performance.recommendations.push('执行时间过长，考虑优化算法')
}
if (metrics.memory.used > 10 * 1024 * 1024) {
  metrics.performance.recommendations.push('内存使用过多，考虑分批处理')
}
if (metrics.data.outputSize > metrics.data.inputSize * 2) {
  metrics.performance.recommendations.push('输出数据膨胀过多，考虑数据压缩')
}

return {
  result: processedData,
  metrics: metrics,
  timestamp: Date.now()
}`,
    parameters: [],
    usageSnippet: '// 自动测量遥测清洗、聚合或图表数据转换的性能',
    isSystem: true
  }
]
