/**
 * 文件用途：增强类型配套的运行时工具类。
 * 关键注意事项：由 enhanced-types.ts 拆分而来，enhanced/index.ts 统一重导出，外部引用路径保持不变。
 */
export class DataTypeConverter {
  /**
   * 将字符串值转换为指定类型
   * @param value 原始字符串值
   * @param dataType 目标数据类型
   * @returns 转换后的值
   */
  static convertValue(value: string, dataType: 'string' | 'number' | 'boolean' | 'json'): unknown {
    if (value === null || value === undefined || value === '') {
      return value
    }

    switch (dataType) {
      case 'string':
        return String(value)

      case 'number': {
        const num = Number(value)
        if (isNaN(num)) {
          throw new Error(`无法将值 "${value}" 转换为数字类型`)
        }
        return num
      }

      case 'boolean': {
        const lowerValue = String(value).toLowerCase().trim()
        if (lowerValue === 'true' || lowerValue === '1') {
          return true
        }
        if (lowerValue === 'false' || lowerValue === '0') {
          return false
        }
        throw new Error(`无法将值 "${value}" 转换为布尔类型`)
      }

      case 'json': {
        try {
          return JSON.parse(value)
        } catch (error) {
          const errorMessage = error instanceof Error ? error.message : String(error)
          throw new Error(`无法将值 "${value}" 转换为JSON类型: ${errorMessage}`)
        }
      }

      default:
        return value
    }
  }

  /**
   * 验证值是否符合指定类型
   * @param value 要验证的值
   * @param dataType 期望的数据类型
   * @returns 验证结果
   */
  static validateType(value: unknown, dataType: 'string' | 'number' | 'boolean' | 'json'): boolean {
    switch (dataType) {
      case 'string':
        return typeof value === 'string'

      case 'number':
        return typeof value === 'number' && !isNaN(value)

      case 'boolean':
        return typeof value === 'boolean'

      case 'json':
        try {
          if (typeof value === 'string') {
            JSON.parse(value)
            return true
          }
          // 如果已经是对象，检查是否可序列化
          JSON.stringify(value)
          return true
        } catch {
          return false
        }

      default:
        return false
    }
  }

  /**
   * 获取值的实际数据类型
   * @param value 要检查的值
   * @returns 数据类型字符串
   */
  static getActualType(value: unknown): 'string' | 'number' | 'boolean' | 'json' | 'unknown' {
    if (typeof value === 'string') {
      return 'string'
    }
    if (typeof value === 'number' && !isNaN(value)) {
      return 'number'
    }
    if (typeof value === 'boolean') {
      return 'boolean'
    }
    if (typeof value === 'object' || Array.isArray(value)) {
      return 'json'
    }
    return 'unknown'
  }
}

/**
 * 占位符工具类
 * 用于处理{{variableName}}占位符
 */
export class PlaceholderUtils {
  /** 占位符正则表达式 */
  private static readonly PLACEHOLDER_REGEX = /\{\{([^}]+)\}\}/g

  /**
   * 提取字符串中的所有占位符
   * @param text 包含占位符的文本
   * @returns 占位符名称数组
   */
  static extractPlaceholders(text: string): string[] {
    if (typeof text !== 'string') {
      return []
    }

    const matches: string[] = []
    let match: RegExpExecArray | null

    // 重置正则表达式状态
    this.PLACEHOLDER_REGEX.lastIndex = 0

    while ((match = this.PLACEHOLDER_REGEX.exec(text)) !== null) {
      const placeholderName = match[1].trim()
      if (placeholderName && !matches.includes(placeholderName)) {
        matches.push(placeholderName)
      }
    }

    return matches
  }

  /**
   * 替换字符串中的占位符
   * @param text 包含占位符的文本
   * @param values 占位符值映射
   * @returns 替换后的文本
   */
  static replacePlaceholders(text: string, values: Map<string, unknown>): string {
    if (typeof text !== 'string') {
      return text
    }

    return text.replace(this.PLACEHOLDER_REGEX, (match, placeholderName) => {
      const trimmedName = placeholderName.trim()
      if (values.has(trimmedName)) {
        const value: unknown = values.get(trimmedName)
        return value !== null && value !== undefined ? String(value) : match
      }
      return match // 保持原占位符如果没有找到值
    })
  }

  /**
   * 验证占位符名称格式
   * @param name 占位符名称
   * @returns 是否为有效名称
   */
  static isValidPlaceholderName(name: string): boolean {
    if (!name || typeof name !== 'string') {
      return false
    }

    // 占位符名称规则：字母开头，可包含字母、数字、下划线
    const nameRegex = /^[a-zA-Z][a-zA-Z0-9_]*$/
    return nameRegex.test(name.trim())
  }

  /**
   * 生成HTTP参数的占位符名称
   * @param parameterKey 参数键名
   * @returns 标准化的占位符名称
   */
  static generateHttpPlaceholderName(parameterKey: string): string {
    if (!parameterKey || typeof parameterKey !== 'string') {
      throw new Error('参数键名不能为空')
    }

    // 清理参数键名，只保留字母数字和下划线
    const cleanKey = parameterKey
      .replace(/[^a-zA-Z0-9_]/g, '_')
      .replace(/_+/g, '_') // 合并连续的下划线
      .replace(/^_+|_+$/g, '') // 移除开头和结尾的下划线

    if (!cleanKey) {
      throw new Error(`无效的参数键名: ${parameterKey}`)
    }

    return `http_${cleanKey}`
  }
}

/**
 * 类型守卫：检查是否为增强版配置
 */
