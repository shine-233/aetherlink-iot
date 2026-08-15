/**
 * 文件用途: 配置管理服务。
 * 核心逻辑: 提供配置验证、模板管理、存储和深拷贝等编辑器配置辅助能力。
 * 关键注意事项: 验证结果和模板结构会影响用户保存配置前的反馈。
 * 重构建议: 将验证规则、模板注册和持久化存储拆分为可替换服务。
 */

import type { DataSourceConfiguration, ValidationResult } from '@/core/data-architecture/types'
import { smartDeepClone } from '@/utils/deep-clone'

export interface ConfigurationTemplate {
  id: string
  name: string
  description: string
  configuration: DataSourceConfiguration
  category: 'basic' | 'advanced' | 'starter'
  tags: string[]
}

/**
 * 配置管理器类
 */
export class ConfigurationManager {
  private templates: ConfigurationTemplate[] = []

  constructor() {
    this.initializeBuiltinTemplates()
  }

  /**
   * 初始化内置模板
   */
  private initializeBuiltinTemplates() {
    this.templates = [
      {
        id: 'json-basic',
        name: '设备遥测 JSON 模板',
        description: '用于首张图表和状态卡片的设备遥测静态数据模板',
        category: 'basic',
        tags: ['json', 'telemetry', 'device'],
        configuration: {
          componentId: 'device-telemetry-card',
          dataSources: [
            {
              sourceId: 'device_telemetry_json',
              dataItems: [
                {
                  item: {
                    type: 'json',
                    config: {
                      jsonString: JSON.stringify(
                        {
                          device_id: 'replace_with_device_id',
                          online: true,
                          temperature: 25,
                          humidity: 60,
                          rssi: -52,
                          alarm_count: 0,
                          status: 'online',
                          timestamp: new Date().toISOString()
                        },
                        null,
                        2
                      )
                    }
                  },
                  processing: {
                    filterPath: '$',
                    defaultValue: {}
                  }
                }
              ],
              mergeStrategy: { type: 'object' }
            }
          ],
          createdAt: Date.now(),
          updatedAt: Date.now()
        }
      },
      {
        id: 'http-api',
        name: '设备最新遥测 API',
        description: '从平台接口读取单台设备最新遥测数据',
        category: 'basic',
        tags: ['http', 'api', 'telemetry'],
        configuration: {
          componentId: 'device-current-telemetry',
          dataSources: [
            {
              sourceId: 'current_telemetry_api',
              dataItems: [
                {
                  item: {
                    type: 'http',
                    config: {
                      url: '/api/v1/telemetry/datas/current/replace_with_device_id',
                      method: 'GET'
                    }
                  },
                  processing: {
                    filterPath: '$.data',
                    defaultValue: []
                  }
                }
              ],
              mergeStrategy: { type: 'object' }
            }
          ],
          createdAt: Date.now(),
          updatedAt: Date.now()
        }
      },
      {
        id: 'script-generated',
        name: '设备遥测脚本模板',
        description: '通过 JavaScript 生成可用于预览的设备遥测数据',
        category: 'advanced',
        tags: ['script', 'telemetry', 'preview'],
        configuration: {
          componentId: 'device-telemetry-preview',
          dataSources: [
            {
              sourceId: 'telemetry_script_preview',
              dataItems: [
                {
                  item: {
                    type: 'script',
                    config: {
                      script: `
// 返回确定性的设备遥测预览数据，避免模板执行结果随机漂移。
const telemetry = {
  device_id: 'replace_with_device_id',
  online: true,
  timestamp: '2026-07-07T08:00:00.000Z',
  temperature: 25,
  humidity: 60,
  rssi: -52,
  alarm_count: 0,
  status: 'online',
  preview: true
}
return telemetry
                  `.trim()
                    }
                  },
                  processing: {
                    filterPath: '$',
                    defaultValue: {}
                  }
                }
              ],
              mergeStrategy: { type: 'object' }
            }
          ],
          createdAt: Date.now(),
          updatedAt: Date.now()
        }
      },
      {
        id: 'multi-source',
        name: '设备运维多源整合',
        description: '合并设备遥测、在线状态和交付元数据的起始配置',
        category: 'starter',
        tags: ['multi-source', 'telemetry', 'operations'],
        configuration: {
          componentId: 'device-operations-overview',
          dataSources: [
            {
              sourceId: 'telemetry_rows',
              dataItems: [
                {
                  item: {
                    type: 'json',
                    config: {
                      jsonString: JSON.stringify(
                        {
                          telemetryRows: [
                            { device_id: 'replace_with_device_id', metric: 'temperature', value: 25, unit: 'C' },
                            { device_id: 'replace_with_device_id', metric: 'humidity', value: 60, unit: '%' }
                          ]
                        },
                        null,
                        2
                      )
                    }
                  },
                  processing: {
                    filterPath: '$.telemetryRows',
                    defaultValue: []
                  }
                }
              ],
              mergeStrategy: { type: 'array' }
            },
            {
              sourceId: 'metadata',
              dataItems: [
                {
                  item: {
                    type: 'script',
                    config: {
                      script: `
return {
  timestamp: Date.now(),
  site: "first-device-lab",
  device_id: "replace_with_device_id",
  online: true,
  proof_stage: "latest-telemetry-visible",
  version: "1.0.0"
}
                    `.trim()
                    }
                  },
                  processing: {
                    filterPath: '$',
                    defaultValue: {}
                  }
                }
              ],
              mergeStrategy: { type: 'object' }
            }
          ],
          createdAt: Date.now(),
          updatedAt: Date.now()
        }
      }
    ]
  }

  /**
   * 获取所有内置模板
   */
  getBuiltinTemplates(): ConfigurationTemplate[] {
    return smartDeepClone(this.templates) as ConfigurationTemplate[]
  }

  /**
   * 根据ID获取模板
   */
  getTemplate(id: string): ConfigurationTemplate | undefined {
    const template = this.templates.find(t => t.id === id)
    return template ? (smartDeepClone(template) as ConfigurationTemplate) : undefined
  }

  /**
   * 根据分类获取模板
   */
  getTemplatesByCategory(category: string): ConfigurationTemplate[] {
    return smartDeepClone(this.templates.filter(t => t.category === category)) as ConfigurationTemplate[]
  }

  /**
   * 验证配置
   */
  validateConfiguration(config: DataSourceConfiguration): ValidationResult {
    const errors: string[] = []
    const warnings: string[] = []

    // 基础结构验证
    if (!config.componentId) {
      errors.push('组件ID不能为空')
    }

    if (!config.dataSources || config.dataSources.length === 0) {
      errors.push('至少需要配置一个数据源')
    }

    // 数据源验证
    config.dataSources?.forEach((dataSource, dsIndex) => {
      if (!dataSource.sourceId) {
        errors.push(`数据源 ${dsIndex + 1}: sourceId不能为空`)
      }

      if (!dataSource.dataItems || dataSource.dataItems.length === 0) {
        errors.push(`数据源 ${dsIndex + 1}: 至少需要一个数据项`)
      }

      // 数据项验证
      dataSource.dataItems?.forEach((dataItem, diIndex) => {
        if (!dataItem.item.type) {
          errors.push(`数据源 ${dsIndex + 1}, 数据项 ${diIndex + 1}: 数据类型不能为空`)
        }

        // 类型特定验证
        switch (dataItem.item.type) {
          case 'json':
            if (!dataItem.item.config.jsonString) {
              errors.push(`数据源 ${dsIndex + 1}, 数据项 ${diIndex + 1}: JSON内容不能为空`)
            } else {
              try {
                JSON.parse(dataItem.item.config.jsonString)
              } catch (e) {
                errors.push(`数据源 ${dsIndex + 1}, 数据项 ${diIndex + 1}: JSON格式错误`)
              }
            }
            break
          case 'http':
            if (!dataItem.item.config.url) {
              errors.push(`数据源 ${dsIndex + 1}, 数据项 ${diIndex + 1}: HTTP URL不能为空`)
            }
            if (!dataItem.item.config.method) {
              warnings.push(`数据源 ${dsIndex + 1}, 数据项 ${diIndex + 1}: 建议指定HTTP方法`)
            }
            break
          case 'script':
            if (!dataItem.item.config.script) {
              errors.push(`数据源 ${dsIndex + 1}, 数据项 ${diIndex + 1}: 脚本内容不能为空`)
            }
            break
        }

        // 处理配置验证
        if (!dataItem.processing.filterPath) {
          warnings.push(`数据源 ${dsIndex + 1}, 数据项 ${diIndex + 1}: 建议设置过滤路径`)
        }
      })

      // 合并策略验证
      if (!dataSource.mergeStrategy.type) {
        warnings.push(`数据源 ${dsIndex + 1}: 建议指定合并策略`)
      }
    })

    return {
      valid: errors.length === 0,
      errors,
      warnings
    }
  }

  /**
   * 导出配置为JSON字符串
   */
  exportConfiguration(config: DataSourceConfiguration): string {
    return JSON.stringify(config, null, 2)
  }

  /**
   * 从JSON字符串导入配置
   */
  importConfiguration(jsonString: string): DataSourceConfiguration {
    try {
      const config = JSON.parse(jsonString) as DataSourceConfiguration

      // 基础验证
      if (!config.dataSources || !Array.isArray(config.dataSources)) {
        throw new Error('配置格式错误: dataSources必须是数组')
      }

      // 添加时间戳
      config.updatedAt = Date.now()
      if (!config.createdAt) {
        config.createdAt = Date.now()
      }

      return config
    } catch (error) {
      throw new Error('配置导入失败: ' + (error.message || '格式错误'))
    }
  }

  /**
   * 导出配置为文件
   */
  exportConfigurationAsFile(config: DataSourceConfiguration, filename?: string) {
    const dataStr = this.exportConfiguration(config)
    const dataBlob = new Blob([dataStr], { type: 'application/json' })
    const url = URL.createObjectURL(dataBlob)

    try {
      const link = document.createElement('a')
      link.href = url
      link.download = filename || `${config.componentId}-config-${Date.now()}.json`
      link.click()
    } finally {
      URL.revokeObjectURL(url)
    }
  }

  /**
   * 从文件导入配置
   */
  async importConfigurationFromFile(file: File): Promise<DataSourceConfiguration> {
    const text = await file.text()
    return this.importConfiguration(text)
  }

  /**
   * 生成起始配置
   */
  generateStarterConfiguration(componentId: string): DataSourceConfiguration {
    const template = this.getTemplate('json-basic')
    if (template) {
      const configuration = smartDeepClone(template.configuration) as DataSourceConfiguration
      return {
        ...configuration,
        componentId,
        createdAt: Date.now(),
        updatedAt: Date.now()
      }
    }

    // 回退到简单起始配置
    return {
      componentId,
      dataSources: [
        {
          sourceId: 'starter_data',
          dataItems: [
            {
              item: {
                type: 'json',
                config: {
                  jsonString:
                    '{"device_id":"replace_with_device_id","online":true,"temperature":25,"timestamp":"' +
                    new Date().toISOString() +
                    '"}'
                }
              },
              processing: { filterPath: '$', defaultValue: {} }
            }
          ],
          mergeStrategy: { type: 'object' }
        }
      ],
      createdAt: Date.now(),
      updatedAt: Date.now()
    }
  }

  /**
   * 克隆配置
   */
  cloneConfiguration(config: DataSourceConfiguration, newComponentId?: string): DataSourceConfiguration {
    const cloned = smartDeepClone(config) as DataSourceConfiguration

    if (newComponentId) {
      cloned.componentId = newComponentId
    }

    cloned.createdAt = Date.now()
    cloned.updatedAt = Date.now()

    return cloned
  }

  /**
   * 合并配置
   */
  mergeConfigurations(
    baseConfig: DataSourceConfiguration,
    ...otherConfigs: DataSourceConfiguration[]
  ): DataSourceConfiguration {
    const merged = this.cloneConfiguration(baseConfig)

    otherConfigs.forEach(config => {
      // 克隆追加项，避免合并结果与输入配置共享嵌套状态。
      merged.dataSources.push(...(smartDeepClone(config.dataSources) as DataSourceConfiguration['dataSources']))
    })

    merged.updatedAt = Date.now()
    return merged
  }
}

// 创建单例实例
export const configurationManager = new ConfigurationManager()

// 默认导出
export default configurationManager
