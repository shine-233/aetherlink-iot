/**
 * 文件用途: 绑定路径校验与恢复工具。
 * 核心逻辑: 识别损坏的组件绑定路径，校验路径格式，并从变量名恢复候选路径。
 * 关键注意事项: 恢复规则影响历史配置兼容，过度宽松可能掩盖真实配置错误。
 * 重构建议: 把严格校验、宽松识别和恢复策略拆成可配置规则集。
 */

export interface BindingPathValidationOptions {
  allowEmpty?: boolean
  strict?: boolean
}

export interface BindingPathRecoveryResult {
  bindingPath: string | null
  isValid: boolean
  recovered: boolean
  wasDamaged: boolean
}

const STRICT_COMPONENT_BINDING_PATH_PATTERN = /^[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_.-]+$/

export function recoverComponentBindingPathFromVariableName(variableName?: string | null): string | null {
  if (!variableName || typeof variableName !== 'string') {
    return null
  }

  const lastUnderscoreIndex = variableName.lastIndexOf('_')

  if (lastUnderscoreIndex <= 0 || lastUnderscoreIndex === variableName.length - 1) {
    return null
  }

  const componentId = variableName.substring(0, lastUnderscoreIndex)
  const propertyName = variableName.substring(lastUnderscoreIndex + 1)

  return `${componentId}.base.${propertyName}`
}

export function isDamagedComponentBindingPath(bindingPath: unknown, variableName?: string | null): bindingPath is string {
  return (
    Boolean(bindingPath) &&
    typeof bindingPath === 'string' &&
    !bindingPath.includes('.') &&
    bindingPath.length < 10 &&
    recoverComponentBindingPathFromVariableName(variableName) !== null
  )
}

export function isValidComponentBindingPath(
  bindingPath: unknown,
  options: BindingPathValidationOptions = {}
): bindingPath is string {
  if (bindingPath === '' && options.allowEmpty) {
    return true
  }

  if (typeof bindingPath !== 'string' || !bindingPath.includes('.')) {
    return false
  }

  if (!options.strict) {
    return true
  }

  return (
    bindingPath.split('.').length >= 3 &&
    bindingPath.length > 10 &&
    !/^\d{1,4}$/.test(bindingPath) &&
    !bindingPath.includes('undefined') &&
    !bindingPath.includes('null') &&
    STRICT_COMPONENT_BINDING_PATH_PATTERN.test(bindingPath)
  )
}

export function resolveRecoverableComponentBindingPath(
  bindingPath: unknown,
  variableName?: string | null,
  options: BindingPathValidationOptions = {}
): BindingPathRecoveryResult {
  const wasDamaged = isDamagedComponentBindingPath(bindingPath, variableName)

  if (isValidComponentBindingPath(bindingPath, options)) {
    return {
      bindingPath,
      isValid: true,
      recovered: false,
      wasDamaged
    }
  }

  const recoveredPath = recoverComponentBindingPathFromVariableName(variableName)

  if (recoveredPath && isValidComponentBindingPath(recoveredPath, options)) {
    return {
      bindingPath: recoveredPath,
      isValid: true,
      recovered: true,
      wasDamaged
    }
  }

  return {
    bindingPath: typeof bindingPath === 'string' ? bindingPath : null,
    isValid: false,
    recovered: false,
    wasDamaged
  }
}
