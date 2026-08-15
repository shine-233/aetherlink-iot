/**
 * 文件说明：
 * - 提供 DynamicParameterEditor 的纯状态辅助函数和常量，包括新增参数配置、稳定 ID 和动态绑定状态推断。
 * - 将无 UI 副作用的参数状态逻辑从大组件中拆出，降低组件脚本区的维护压力。
 * 维护提示：
 * - 设备相关 key 和 group prefix 会影响设备参数分组展示与替换逻辑，改动时要同步检查设备参数导入流程。
 * - `inferParameterDynamicState` 需要兼容历史 component binding path，不能只依赖新的模板字段。
 */
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import { ParameterTemplateType } from '@/core/data-architecture/components/common/templates/index'

export type NewParamConfig = {
  key: string
  configType: 'manual' | 'property' | 'device'
  value: string
  description: string
  propertyBinding: Record<string, unknown> | null
  deviceConfig: Record<string, unknown> | null
}

export const DEVICE_RELATED_PARAMETER_KEYS = ['deviceId', 'metric', 'deviceLocation', 'deviceStatus']
export const DEVICE_RELATED_PARAMETER_KEY_SET = new Set(DEVICE_RELATED_PARAMETER_KEYS)
export const DEVICE_CONFIG_TEMPLATE_ID = 'device-metrics-selector'
export const DEVICE_PARAMETER_GROUP_PREFIX_BY_SOURCE: Record<string, string> = {
  'device-id': '设备',
  'device-metric': '指标',
  telemetry: '遥测'
}

export const createNewParamConfig = (): NewParamConfig => ({
  key: '',
  configType: 'manual',
  value: '',
  description: '',
  propertyBinding: null,
  deviceConfig: null
})

export const isDeviceRelatedParameter = (param: EnhancedParameter) => DEVICE_RELATED_PARAMETER_KEY_SET.has(param.key)

export const createStableParameterId = (index: number) =>
  `param_stable_${Date.now()}_${index}_${Math.random().toString(36).substr(2, 6)}`

export const looksLikeComponentBindingPath = (value: unknown) =>
  typeof value === 'string' && value.includes('.') && value.split('.').length >= 3 && value.length > 10

export const inferParameterDynamicState = (param: EnhancedParameter) => {
  if (param.isDynamic !== undefined) {
    return param.isDynamic
  }

  return (
    param.valueMode === ParameterTemplateType.COMPONENT ||
    param.selectedTemplate === 'component-property-binding' ||
    looksLikeComponentBindingPath(param.value)
  )
}
