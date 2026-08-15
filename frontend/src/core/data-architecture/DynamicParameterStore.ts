/**
 * 文件用途: 独立管理 Data Warehouse 的动态参数。
 * 核心逻辑: 支持按直接键或 scope+name 组合键存储，并在读取时清理过期参数。
 * 关键注意事项: 参数仅在 Date.now() 严格大于 expiresAt 时过期。
 */

export interface DynamicParameterStorage {
  /** 参数名称 */
  name: string
  /** 参数值 */
  value: any
  /** 参数类型 */
  type: 'string' | 'number' | 'boolean' | 'object' | 'array'
  /** 作用域 */
  scope: 'global' | 'component' | 'session'
  /** 过期时间 */
  expiresAt?: number
  /** 依赖关系 */
  dependencies?: string[]
}

export class DynamicParameterStore {
  private storage = new Map<string, DynamicParameterStorage>()

  store(nameOrScope: string, parameterOrName: DynamicParameterStorage | string, value?: any): void {
    if (typeof parameterOrName === 'string') {
      const key = `${nameOrScope}:${parameterOrName}`
      this.storage.set(key, {
        name: parameterOrName,
        value,
        type: this.inferParameterType(value),
        scope: 'component'
      })
      return
    }

    this.storage.set(nameOrScope, parameterOrName)
  }

  get(nameOrScope: string, name?: string): DynamicParameterStorage | any | null {
    const key = name ? `${nameOrScope}:${name}` : nameOrScope
    const parameter = this.storage.get(key)

    if (parameter && parameter.expiresAt && Date.now() > parameter.expiresAt) {
      this.storage.delete(key)
      return null
    }
    if (!parameter) return null

    return name ? parameter.value : parameter
  }

  getAll(scope?: string): Record<string, any> {
    const parameters: Record<string, any> = {}

    for (const [key, parameter] of this.storage.entries()) {
      if (parameter.expiresAt && Date.now() > parameter.expiresAt) {
        this.storage.delete(key)
        continue
      }

      if (!scope) {
        parameters[key] = parameter
        continue
      }

      const prefix = `${scope}:`
      if (key.startsWith(prefix)) {
        parameters[key.slice(prefix.length)] = parameter.value
      }
    }

    return parameters
  }

  clear(): void {
    this.storage.clear()
  }

  private inferParameterType(value: any): DynamicParameterStorage['type'] {
    if (Array.isArray(value)) return 'array'
    if (value === null) return 'object'

    const valueType = typeof value
    if (valueType === 'string' || valueType === 'number' || valueType === 'boolean') {
      return valueType
    }

    return 'object'
  }
}
