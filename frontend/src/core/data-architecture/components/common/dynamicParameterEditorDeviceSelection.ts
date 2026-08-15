/**
 * 文件说明：
 * - 集中维护 DynamicParameterEditor 从设备选择结果生成参数的纯转换逻辑。
 * - 包括可用参数槽位计算、设备选择项到 EnhancedParameter 的字段映射，以及 maxParameters 截断。
 * 维护提示：
 * - 这里只处理数据转换，不关闭抽屉、不触发 emit，也不弹出 message。
 * - `selectedTemplate: device-dispatch-selector` 是历史兼容字段，改动会影响已保存设备参数的回显。
 * 审查建议：
 * - 后续可补单元测试覆盖无剩余槽位、source 缺失、metricsId fallback 和 maxParameters 截断。
 */
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import { generateVariableName } from '@/core/data-architecture/types/http-config'
import { ParameterTemplateType } from '@/core/data-architecture/components/common/templates/index'

type DeviceSelectionSource = {
  deviceName?: string
  metricsName?: string
}

type DeviceSelectionParam = {
  key?: string
  metricsId?: string
  value?: string
  source?: DeviceSelectionSource
}

export const getAvailableDeviceParameterSlots = (currentCount: number, maxParameters?: number) =>
  maxParameters !== undefined ? maxParameters - currentCount : Infinity

export const buildDeviceParameterFromSelection = (param: DeviceSelectionParam): EnhancedParameter => {
  const key = param.key || param.metricsId || ''
  const source = param.source

  return {
    key,
    value: source ? `${source.deviceName}.${source.metricsName}` : param.value || '',
    enabled: true,
    valueMode: ParameterTemplateType.COMPONENT,
    selectedTemplate: 'device-dispatch-selector',
    dataType: 'string',
    variableName: source ? generateVariableName(key) : '',
    description: source ? `设备: ${source.deviceName}, 指标: ${source.metricsName}` : ''
  }
}

export const buildDeviceParametersForAvailableSlots = (
  params: DeviceSelectionParam[],
  currentCount: number,
  maxParameters?: number
) => {
  const availableSlots = getAvailableDeviceParameterSlots(currentCount, maxParameters)

  if (availableSlots <= 0) {
    return null
  }

  return params.slice(0, availableSlots).map(buildDeviceParameterFromSelection)
}
