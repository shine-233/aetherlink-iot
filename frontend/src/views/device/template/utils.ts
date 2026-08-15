/**
 * 文件用途: 提供物模型表单的默认数据、校验规则和辅助转换函数。
 * 核心逻辑: 维护物模型基础信息、附加信息解析和表单初始化数据。
 * 关键注意事项: 默认字段会影响新增/编辑物模型的提交载荷，修改前要同步检查接口字段。
 * 重构建议: 将表单默认值、校验规则和接口字段转换拆分，降低工具文件耦合。
 */
import { ref } from 'vue'
import { createRequiredFormRule } from '@/utils/form/rule'
import { $t } from '@/locales'

export const initTemplateInfoData = {
  name: '',
  tags: [],
  description: '',
  author: '',
  version: '',
  path: ''
}

export const templateInfoData = ref({ ...initTemplateInfoData })

export const templateInfoRules = {
  name: createRequiredFormRule($t('device_template.enterTemplateName'))
}

// device model
export const deviceModelTabs = [
  { name: 'telemetry', tab: $t('device_template.telemetry') },
  { name: 'property', tab: $t('device_template.attributes') },
  { name: 'event', tab: $t('device_template.events') },
  { name: 'command', tab: $t('device_template.command') }
]

export const initTelemetryModel = {}

export const telemetryModelData = ref({ ...initTelemetryModel })

export const telemetryModelDataTypeOptions = [
  {
    label: 'String',
    value: 'String'
  },
  {
    label: 'Boolean',
    value: 'Boolean'
  },
  {
    label: 'Number',
    value: 'Number'
  }
]

export const getAdditionalInfo = additionalInfoStr => {
  let additionalInfo = []
  if (typeof additionalInfoStr === 'string') {
    try {
      additionalInfo = JSON.parse(additionalInfoStr)
      if (!Array.isArray(additionalInfo)) {
        additionalInfo = []
      }
    } catch {
      additionalInfo = []
    }
  }

  return additionalInfo
}
