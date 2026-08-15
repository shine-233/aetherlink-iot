/**
 * 文件用途: 数据项处理器。
 * 核心逻辑: 对原始数据执行过滤和脚本处理，作为多层执行链的第二层。
 * 关键注意事项: 脚本执行依赖 script-engine，输入上下文和错误 fallback 需要保持安全边界。
 * 重构建议: 把过滤、脚本上下文构建和错误归一拆分，方便单独测试。
 */

import { defaultScriptEngine } from '@/core/script-engine'

export interface ProcessingConfig {
  /** JSONPath语法过滤路径，如: $.abc.bcd[0] */
  filterPath: string
  /** 自定义脚本处理 */
  customScript?: string
  /** 默认值配置 */
  defaultValue?: any
}

/**
 * 数据项处理器接口
 */
export interface IDataItemProcessor {
  /**
   * 处理原始数据：路径过滤 + 自定义脚本处理
   * @param rawData 原始数据
   * @param config 处理配置
   * @returns 处理后数据，出错时返回 {}
   */
  processData(rawData: any, config: ProcessingConfig): Promise<any>
}

/**
 * 数据项处理器实现类
 */
export class DataItemProcessor implements IDataItemProcessor {
  /**
   * 数据处理主方法
   */
  async processData(rawData: any, config: ProcessingConfig): Promise<any> {
    try {
      if (rawData === null || rawData === undefined) {
        return config.defaultValue ?? {}
      }

      // 允许空数组、空字符串等"falsy but valid"的值
      if (typeof rawData === 'object' && Object.keys(rawData).length === 0 && !Array.isArray(rawData)) {
        return config.defaultValue ?? {}
      }

      // 第一步：JSONPath路径过滤
      let filteredData = await this.applyPathFilter(rawData, config.filterPath)

      // 第二步：自定义脚本处理
      if (config.customScript) {
        filteredData = await this.applyCustomScript(filteredData, config.customScript)
      } else {
        /* intentionally empty */
      }

      // Preserve meaningful falsy values such as 0, false, [], and "".
      const finalResult = filteredData !== null && filteredData !== undefined ? filteredData : config.defaultValue ?? {}
      return finalResult
    } catch (error) {
      return config.defaultValue ?? {} // 统一错误处理：返回默认值或空对象
    }
  }

  /**
   * 应用简化 JSONPath 过滤，支持属性和数组索引，例如 $.items[0].name、$[0][1]。
   */
  private async applyPathFilter(data: any, filterPath: string): Promise<any> {
    const tokens = this.parseFilterPath(filterPath)
    if (tokens === null) {
      return null
    }

    let current = data
    for (const token of tokens) {
      if (current === null || current === undefined) {
        return null
      }

      if (typeof token === 'number') {
        if (!Array.isArray(current)) {
          return null
        }
        current = current[token]
      } else {
        current = current[token]
      }
    }

    return current
  }

  /**
   * 将受支持的 JSONPath 子集解析为属性名和数组索引。
   * 执行与校验共用该解析器，避免配置校验和运行时行为漂移。
   */
  private parseFilterPath(filterPath: string): Array<string | number> | null {
    if (!filterPath || filterPath === '$') {
      return []
    }

    const hasRootMarker = filterPath.startsWith('$')
    let path = hasRootMarker ? filterPath.slice(1) : filterPath
    if (hasRootMarker && path.startsWith('.')) {
      path = path.slice(1)
      if (!/^[a-zA-Z_]/.test(path)) {
        return null
      }
    }
    if (!path) {
      return null
    }

    const tokens: Array<string | number> = []
    let position = 0
    let requireProperty = false

    while (position < path.length) {
      if (path[position] === '.') {
        if (tokens.length === 0 || requireProperty) {
          return null
        }
        requireProperty = true
        position += 1
        continue
      }

      if (path[position] === '[') {
        if (requireProperty) {
          return null
        }
        const indexMatch = /^\[(\d+)\]/.exec(path.slice(position))
        if (!indexMatch) {
          return null
        }
        tokens.push(Number(indexMatch[1]))
        position += indexMatch[0].length
        continue
      }

      if (tokens.length > 0 && !requireProperty) {
        return null
      }
      const propertyMatch = /^[a-zA-Z_][a-zA-Z0-9_]*/.exec(path.slice(position))
      if (!propertyMatch) {
        return null
      }
      tokens.push(propertyMatch[0])
      position += propertyMatch[0].length
      requireProperty = false
    }

    return requireProperty ? null : tokens
  }

  /**
   * 应用自定义脚本处理 (使用 script-engine 安全执行)
   */
  private async applyCustomScript(data: any, script: string): Promise<any> {
    try {
      // 创建脚本执行上下文
      const scriptContext = {
        data
        // script-engine 已内置 JSON, console, Math, Date 等
      }

      // 使用 script-engine 安全执行脚本
      const result = await defaultScriptEngine.execute(script, scriptContext)

      if (result.success) {
        return result.data !== undefined ? result.data : data
      } else {
        return data // 脚本失败时返回原数据
      }
    } catch (error) {
      return data // 脚本失败时返回原数据
    }
  }

  /**
   * 校验运行时支持的简化 JSONPath 子集。
   */
  validateFilterPath(filterPath: string): boolean {
    return this.parseFilterPath(filterPath) !== null
  }
}
