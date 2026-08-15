/**
 * 文件说明：
 * - 集中维护 DynamicParameterEditor 参数列表行级操作的纯函数。
 * - 包括参数增删改、key 实时输入、value 更新、重复 key 校验、编辑索引计划和渲染前稳定 ID 补齐。
 * 维护提示：
 * - 这里只返回新的参数数组或校验结果，不直接调用 emit、message、logger、nextTick。
 * - key 自动重置规则会影响用户输入体验，修改前要同步检查 blur 校验和重复 key 提示。
 * 审查建议：
 * - 后续可补单元测试覆盖删除正在编辑项、追加后聚焦、重复 key 重置、旧配置缺少 `_id` 时的归一化。
 */
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import { createStableParameterId, inferParameterDynamicState } from './dynamicParameterEditorState'

export type ParameterKeyValidationResult =
  | { ok: true }
  | { ok: false; defaultKey: string; duplicateKey?: string }

export const removeParameterAt = (parameters: EnhancedParameter[], index: number) =>
  parameters.filter((_, paramIndex) => paramIndex !== index)

export const getEditingIndexAfterRemoval = (currentEditingIndex: number, removedIndex: number) => {
  if (currentEditingIndex === removedIndex) {
    return -1
  }

  if (removedIndex < currentEditingIndex) {
    return currentEditingIndex - 1
  }

  return currentEditingIndex
}

export const getFirstAppendedFocusIndex = (finalLength: number, appendedCount: number) => {
  if (appendedCount <= 0) {
    return null
  }

  return finalLength - appendedCount
}

export const updateParameterAt = (parameters: EnhancedParameter[], index: number, param: EnhancedParameter) => {
  const updatedParams = [...parameters]
  updatedParams[index] = { ...param }
  return updatedParams
}

export const updateParameterKeyAt = (
  parameters: EnhancedParameter[],
  index: number,
  param: EnhancedParameter,
  key: string
) => updateParameterAt(parameters, index, { ...param, key })

export const updateParameterValueAt = (
  parameters: EnhancedParameter[],
  index: number,
  param: EnhancedParameter,
  value: string
) => updateParameterAt(parameters, index, { ...param, value })

export const createDefaultParameterKey = (index: number) => `param${index + 1}`

export const isDuplicateParameterKey = (parameters: EnhancedParameter[], key: string, index: number) =>
  parameters.some((param, paramIndex) => paramIndex !== index && param.key === key)

export const validateExistingParameterKey = (
  parameters: EnhancedParameter[],
  param: EnhancedParameter,
  index: number
): ParameterKeyValidationResult => {
  const trimmedKey = param.key?.trim() || ''
  const defaultKey = createDefaultParameterKey(index)

  if (!trimmedKey) {
    return { ok: false, defaultKey }
  }

  if (isDuplicateParameterKey(parameters, trimmedKey, index)) {
    return { ok: false, defaultKey, duplicateKey: trimmedKey }
  }

  return { ok: true }
}

export const normalizeRenderedParameter = (param: EnhancedParameter, index: number): EnhancedParameter => ({
  ...param,
  isDynamic: inferParameterDynamicState(param),
  _id: param._id || createStableParameterId(index)
})

export const ensureParameterHasId = (param: EnhancedParameter, index: number): EnhancedParameter =>
  param._id ? param : normalizeRenderedParameter(param, index)
