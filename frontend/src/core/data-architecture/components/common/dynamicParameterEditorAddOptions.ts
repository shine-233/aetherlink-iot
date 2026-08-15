/**
 * 文件说明：
 * - 集中维护 DynamicParameterEditor 的添加入口选项和推荐模板读取。
 * - 将无 UI 副作用的下拉配置从大组件中拆出，便于后续统一调整入口文案和模板策略。
 * 维护提示：
 * - `api-template` 与 `apply-interface-template` 是接口模板导入流程的兼容 key，改名会影响下拉事件分发。
 * - 这里只返回配置数组或动作计划，不触发抽屉、message、emit 或 nextTick。
 * 审查建议：
 * - 后续如果新增批量导入、复制参数或 AI 推荐入口，应先扩展本模块，再让组件决定具体 UI 动作。
 */
import {
  getRecommendedTemplates,
  type ParameterTemplate
} from '@/core/data-architecture/components/common/templates/index'

export type AddParameterOptionKey = 'manual' | 'property' | 'device' | 'api-template' | 'apply-interface-template'

export type AddParameterOptionAction =
  | 'import-template'
  | 'add-manual'
  | 'add-property'
  | 'open-device-config'
  | 'blocked-by-limit'

export type AddParameterOption = {
  label: string
  key: AddParameterOptionKey
  description: string
}

type CurrentApiInfo = {
  commonParams?: unknown[]
} | null | undefined

type ParameterType = 'header' | 'query' | 'path'

export const isTemplateImportOption = (key: string) => key === 'api-template' || key === 'apply-interface-template'

const createBaseAddParameterOptions = (): AddParameterOption[] => [
  {
    label: '手动输入',
    key: 'manual',
    description: '直接输入固定参数值'
  },
  {
    label: '组件属性绑定',
    key: 'property',
    description: '绑定到组件属性（运行时获取值）'
  },
  {
    label: '设备配置',
    key: 'device',
    description: '选择设备和对应的指标数据'
  }
]

const getApiTemplateParamCount = (currentApiInfo: CurrentApiInfo) => {
  const commonParams = currentApiInfo?.commonParams
  return Array.isArray(commonParams) ? commonParams.length : 0
}

export const buildAddParameterOptions = (currentApiInfo: CurrentApiInfo): AddParameterOption[] => {
  const baseOptions = createBaseAddParameterOptions()
  const commonParamCount = getApiTemplateParamCount(currentApiInfo)

  if (commonParamCount > 0) {
    baseOptions.unshift({
      label: `✨ 应用接口模板 (${commonParamCount}个参数)`,
      key: 'api-template',
      description: '自动导入内部接口的预制参数'
    })
  }

  return baseOptions
}

export const loadRecommendedTemplates = (parameterType: ParameterType): ParameterTemplate[] =>
  getRecommendedTemplates(parameterType)

export const resolveAddParameterOptionAction = (key: string, canAddMore: boolean): AddParameterOptionAction => {
  // 接口模板导入历史上不受单个新增参数入口的数量限制，这里保留原有优先级。
  if (isTemplateImportOption(key)) {
    return 'import-template'
  }

  if (!canAddMore) {
    return 'blocked-by-limit'
  }

  switch (key) {
    case 'property':
      return 'add-property'
    case 'device':
      return 'open-device-config'
    case 'manual':
    default:
      return 'add-manual'
  }
}
