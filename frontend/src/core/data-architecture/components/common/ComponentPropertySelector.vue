<!--
  文件用途: 组件属性选择器。
  核心逻辑: 让用户选择组件及其可绑定属性，并提供 visual-editor 兼容的 fallback 字段。
  关键注意事项: 属性路径和组件 ID 会进入持久化绑定，字段变化会影响历史配置恢复。
  重构建议: 抽取组件/属性查询逻辑，组件只负责展示和选择。
-->
<template>
  <div class="component-property-selector">
    <!-- 第一级：组件选择 -->
    <div class="selector-level">
      <n-form-item label="选择组件">
        <n-select
          v-model:value="selectedComponentId"
          :options="componentOptions"
          placeholder="请选择要绑定的组件"
          clearable
          filterable
          @update:value="onComponentChange"
        />
      </n-form-item>
    </div>

    <!-- 第二级：属性选择 -->
    <div v-if="selectedComponentId" class="selector-level">
      <n-form-item label="选择属性">
        <n-select
          v-model:value="selectedPropertyPath"
          :options="propertyOptions"
          placeholder="请选择要绑定的属性"
          clearable
          filterable
          @update:value="onPropertyChange"
        />
      </n-form-item>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * 组件属性选择器（二级联动）
 * 基于组件元数据与运行配置列出可绑定属性
 */

import { ref, computed, watch, nextTick } from 'vue'
import { NFormItem, NSelect } from 'naive-ui'
import { useEditorStore } from '@/store/modules/editor'
import { configurationIntegrationBridge } from '@/components/visual-editor/configuration/ConfigurationIntegrationBridge'

// Props 和 Emits
interface Props {
  modelValue?: string
  placeholder?: string
  currentComponentId?: string // 当前组件ID，用于显示"当前组件"标识
  autoDetectComponentId?: boolean // 是否自动检测当前活跃组件ID
}

interface Emits {
  (e: 'update:modelValue', value: string): void
  (e: 'change', bindingPath: string, propertyInfo?: PropertyInfo): void
}

interface PropertyInfo {
  componentId: string
  componentName: string
  layer: 'base' | 'component' | 'system'
  propertyName: string
  propertyLabel: string
  type: string
  description?: string
  currentValue?: any
}

type ComponentPropertyLayer = PropertyInfo['layer']

interface BindablePropertyConfig {
  alias?: string
  type?: string
  label?: string
  description?: string
  level?: string
  [key: string]: any
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

// 内部状态
const selectedComponentId = ref<string>('')
const selectedPropertyPath = ref<string>('')

// Editor Store
const editorStore = useEditorStore()

// 监听外部 modelValue 变化
watch(
  () => props.modelValue,
  (newValue) => {
    if (newValue && newValue !== selectedPropertyPath.value) {
      parseBindingPath(newValue)
    } else if (!newValue) {
      selectedComponentId.value = ''
      selectedPropertyPath.value = ''
    }
  },
  { immediate: true }
)

/**
 * 解析绑定路径，设置对应的组件和属性选择
 */
const parseBindingPath = (bindingPath: string) => {
  if (!bindingPath || !bindingPath.includes('.')) return

  const parts = bindingPath.split('.')
  if (parts.length >= 3) {
    const componentId = parts[0]
    selectedComponentId.value = componentId
    selectedPropertyPath.value = bindingPath
  }
}

/**
 * 获取画布上的所有组件选项
 */
const componentOptions = computed(() => {
  const components = editorStore.nodes || []

  return components.map((comp) => {
    // 智能确定当前组件
    // 1. 优先使用明确传入的 currentComponentId
    // 2. 如果开启自动检测，使用选中的节点ID或第一个节点
    let effectiveCurrentComponentId = props.currentComponentId

    if (!effectiveCurrentComponentId && props.autoDetectComponentId) {
      // 自动检测：优先使用选中的节点，否则使用第一个节点
      effectiveCurrentComponentId = editorStore.selectedNodeId || components[0]?.id
    }

    const isCurrentComponent = comp.id === effectiveCurrentComponentId
    const componentLabel = isCurrentComponent
      ? `${comp.type || 'unknown'} (当前组件)`
      : `${comp.type || 'unknown'} (${comp.id.slice(0, 8)}...)`

    return {
      label: componentLabel,
      value: comp.id,
      componentType: comp.type
    }
  })
})

/**
 * 获取组件可绑定属性
 */
const getBindableProperties = async (componentId: string) => {
  if (!componentId) return []

  try {
    const targetComponent = resolveTargetComponent(componentId)
    if (!targetComponent?.type) {
      console.warn(`[ComponentPropertySelector] unable to determine component type for ${componentId}`)
      return []
    }

    const bindableProperties = readBindableProperties(componentId)

    if (Object.keys(bindableProperties).length === 0) {
      return []
    }

    const config = configurationIntegrationBridge.getConfiguration(componentId)
    const options: any[] = []

    for (const [propertyName, propConfig] of Object.entries(bindableProperties)) {
      const option = normalizeBindablePropertyOption({
        componentId,
        componentType: targetComponent.type,
        config,
        propertyName,
        propConfig
      })

      if (option) {
        options.push(option)
      }
    }

    return options
  } catch (error) {
    console.error('[ComponentPropertySelector] failed to load whitelisted properties:', error)
    return []
  }
}

/**
 * 🔍 解析目标组件
 */
const resolveTargetComponent = (componentId: string) => {
  const components = editorStore.nodes || []
  return components.find((comp) => comp.id === componentId) || null
}

/**
 * 读取组件声明的可绑定属性。
 * 优先使用正式组件能力元数据，兼容历史 exposedProperties 结构。
 */
const readBindableProperties = (componentId: string): Record<string, BindablePropertyConfig> => {
  const targetComponent = resolveTargetComponent(componentId)
  const metadata = targetComponent?.metadata || {}
  const watchableProperties =
    metadata.card2Definition?.interactionCapabilities?.watchableProperties ||
    metadata.exposedProperties?.card2Definition?.interactionCapabilities?.watchableProperties ||
    {}

  const normalizedProperties: Record<string, BindablePropertyConfig> = {}

  for (const [propertyName, propertyConfig] of Object.entries(watchableProperties)) {
    normalizedProperties[propertyName] = normalizeBindablePropertyConfig(propertyName, propertyConfig)
  }

  const exposedProperties = metadata.exposedProperties || {}
  for (const [propertyName, propertyValue] of Object.entries(exposedProperties)) {
    if (propertyName === 'card2Definition' || normalizedProperties[propertyName]) continue
    normalizedProperties[propertyName] = normalizeBindablePropertyConfig(propertyName, propertyValue)
  }

  return normalizedProperties
}

const normalizeBindablePropertyConfig = (propertyName: string, propertyValue: any): BindablePropertyConfig => {
  if (propertyValue && typeof propertyValue === 'object' && !Array.isArray(propertyValue)) {
    return {
      alias: propertyValue.alias,
      type: propertyValue.type || inferValueType(propertyValue.value),
      label: propertyValue.label,
      description: propertyValue.description || propertyValue.label || propertyName,
      level: propertyValue.level || 'public',
      ...propertyValue
    }
  }

  return {
    type: inferValueType(propertyValue),
    description: propertyName,
    level: 'public'
  }
}

/**
 * 从配置层级读取当前属性值
 */
const readCurrentPropertyValue = (config: any, propertyName: string) => {
  const isGlobalBaseProperty = isGlobalBasePropertyName(propertyName)

  if (isGlobalBaseProperty) {
    const baseValue = readPathValue(config?.base, propertyName)
    if (baseValue !== undefined) {
      return baseValue
    }

    const componentValue = readPathValue(config?.component, propertyName)
    if (componentValue !== undefined) {
      return componentValue
    }

    const customizeValue = readPathValue(config?.customize, propertyName)
    if (customizeValue !== undefined) {
      return customizeValue
    }

    const rootValue = readPathValue(config, propertyName)
    if (rootValue !== undefined) {
      return rootValue
    }

    return undefined
  }

  const componentValue = readPathValue(config?.component, propertyName)
  if (componentValue !== undefined) {
    return componentValue
  }

  const customizeValue = readPathValue(config?.customize, propertyName)
  if (customizeValue !== undefined) {
    return customizeValue
  }

  const rootValue = readPathValue(config, propertyName)
  if (rootValue !== undefined) {
    return rootValue
  }

  return undefined
}

const isGlobalBasePropertyName = (propertyName: string) => propertyName === 'deviceId' || propertyName === 'metricsList'

/**
 * 按点路径读取对象值，支持 styles.color 这类组件属性。
 */
const readPathValue = (source: any, path: string) => {
  if (!source || !path) return undefined

  if (source[path] !== undefined) {
    return source[path]
  }

  return path.split('.').reduce((current, key) => {
    if (current && typeof current === 'object' && key in current) {
      return current[key]
    }
    return undefined
  }, source)
}

const inferValueType = (value: any) => {
  if (Array.isArray(value)) return 'array'
  if (value === null || value === undefined) return 'unknown'
  return typeof value
}

/**
 * 归一化可绑定属性所在分组
 */
const resolveBindablePropertyLayer = (exposedName: string): ComponentPropertyLayer => {
  return isGlobalBasePropertyName(exposedName) ? 'base' : 'component'
}

/**
 * 归一化可绑定属性选项
 */
const normalizeBindablePropertyOption = ({
  componentId,
  config,
  propertyName,
  propConfig
}: {
  componentId: string
  componentType: string
  config: any
  propertyName: string
  propConfig: any
}) => {
  const exposedName = propConfig.alias || propertyName
  const currentValue = readCurrentPropertyValue(config, propertyName)
  const propertyLayer = resolveBindablePropertyLayer(exposedName)
  const isGlobalBaseProperty = propertyLayer === 'base'
  const propertyPath = `${componentId}.${propertyLayer}.${exposedName}`
  const propertyLabel = propConfig.description || propConfig.label || exposedName
  const propertyType = propConfig.type || inferValueType(currentValue)

  return {
    label: `${propertyLabel} (${propertyType})${isGlobalBaseProperty ? ' - 全局基础属性' : ''}`,
    value: propertyPath,
    propertyInfo: {
      componentId: componentId,
      componentName: getComponentName(componentId),
      layer: propertyLayer,
      propertyName: exposedName,
      propertyLabel,
      type: propertyType,
      description: propConfig.description || propConfig.label,
      currentValue,
      isBindable: true,
      accessLevel: propConfig.level,
      isGlobalBaseProperty
    }
  }
}

/**
 * 属性选项列表（使用ref支持异步更新）
 */
const propertyOptions = ref<any[]>([])

/**
 * 异步更新属性选项的函数
 */
const updatePropertyOptions = async () => {
  if (!selectedComponentId.value) {
    propertyOptions.value = []
    return
  }

  try {
    const bindableOptions = await getBindableProperties(selectedComponentId.value)

    // 获取组件配置，用于提取设备ID和指标
    const config = configurationIntegrationBridge.getConfiguration(selectedComponentId.value)

    // Always expose global base properties even when the selected component has no saved config yet.
    const mandatoryOptions: any[] = []

    const hasDeviceIdInBindableOptions = bindableOptions.some((opt) => opt.propertyInfo?.propertyName === 'deviceId')

    const hasMetricsListInBindableOptions = bindableOptions.some(
      (opt) => opt.propertyInfo?.propertyName === 'metricsList'
    )

    // If metadata does not expose deviceId, provide the global base property.
    if (!hasDeviceIdInBindableOptions) {
      const currentDeviceId = config?.base?.deviceId || config?.deviceId || ''
      mandatoryOptions.push({
        label: `设备ID (string) - 全局基础属性`,
        value: `${selectedComponentId.value}.base.deviceId`,
        propertyInfo: {
          componentId: selectedComponentId.value,
          componentName: getComponentName(selectedComponentId.value),
          layer: 'base',
          propertyName: 'deviceId',
          propertyLabel: '设备ID',
          type: 'string',
          description: '关联的设备唯一标识（全局基础属性）',
          currentValue: currentDeviceId,
          isBindable: true,
          isMandatory: true,
          userRequired: true
        }
      })
    }

    if (!hasMetricsListInBindableOptions) {
      const currentMetricsList = config?.base?.metricsList || config?.metricsList || []
      mandatoryOptions.push({
        label: `设备指标列表 (array) - 全局基础属性`,
        value: `${selectedComponentId.value}.base.metricsList`,
        propertyInfo: {
          componentId: selectedComponentId.value,
          componentName: getComponentName(selectedComponentId.value),
          layer: 'base',
          propertyName: 'metricsList',
          propertyLabel: '设备指标列表',
          type: 'array',
          description: '监控的设备指标列表（全局基础属性）',
          currentValue: currentMetricsList,
          isBindable: true,
          isMandatory: true,
          userRequired: true
        }
      })
    }

    // 合并所有选项：组件声明属性 + 必需属性（已去重）
    const allOptions = [...bindableOptions, ...mandatoryOptions]

    if (allOptions.length > 0) {
      propertyOptions.value = allOptions
      return
    }

    // 如果没有任何配置，提供基础属性
    console.warn(
      `[ComponentPropertySelector] component ${selectedComponentId.value} has no config; exposing base safe properties only`
    )

    const basicSafeOptions = [
      {
        label: `组件ID (string)`,
        value: `${selectedComponentId.value}.system.componentId`,
        propertyInfo: {
          componentId: selectedComponentId.value,
          componentName: getComponentName(selectedComponentId.value),
          layer: 'system',
          propertyName: 'componentId',
          propertyLabel: '组件ID',
          type: 'string',
          description: '组件的唯一标识符',
          currentValue: selectedComponentId.value,
          isBindable: true,
          isSafeDefault: true
        }
      }
    ]

    propertyOptions.value = basicSafeOptions
  } catch (error) {
    console.error('[ComponentPropertySelector] failed to load properties:', error)
    propertyOptions.value = []
  }
}

// 监听组件ID变化，自动更新属性选项
watch(
  () => selectedComponentId.value,
  () => {
    updatePropertyOptions()
  },
  { immediate: true }
)

/**
 * 获取组件名称
 */
const getComponentName = (componentId: string): string => {
  const components = editorStore.nodes || []
  const component = components.find((comp) => comp.id === componentId)
  return component?.name || component?.type || 'Unknown'
}

// 事件处理
const onComponentChange = (componentId: string | null) => {
  selectedComponentId.value = componentId || ''
  selectedPropertyPath.value = ''

  if (componentId) {
    // 组件选择变化时，属性选项会通过 watch 自动更新
    nextTick(() => {
      emit('change', '', undefined)
    })
  } else {
    emit('change', '', undefined)
  }
}

const onPropertyChange = (propertyPath: string | null) => {
  // 验证绑定路径格式，防止错误值传递
  if (propertyPath) {
    // 验证绑定路径格式：必须是 componentId.layer.propertyName 格式
    const isValidBindingPath =
      typeof propertyPath === 'string' &&
      propertyPath.includes('.') &&
      propertyPath.split('.').length >= 3 &&
      propertyPath.length > 10 && // 绑定路径通常较长
      !/^\d+$/.test(propertyPath) && // 不能是纯数字
      !propertyPath.includes('undefined') && // 不能包含undefined
      !propertyPath.includes('null') // 不能包含null

    if (!isValidBindingPath) {
      console.error('[ComponentPropertySelector] invalid binding path format:', {
        inputValue: propertyPath,
        valueType: typeof propertyPath,
        expectedFormat: 'componentId.layer.propertyName',
        actualLength: typeof propertyPath === 'string' ? propertyPath.length : 'non-string'
      })
      // 拒绝设置无效的绑定路径，保持当前选择不变
      return
    }
  }

  selectedPropertyPath.value = propertyPath || ''

  if (propertyPath) {
    // 从选项中找到对应的属性信息
    const selectedOption = propertyOptions.value.find((opt) => opt.value === propertyPath)
    const propertyInfo = selectedOption?.propertyInfo || null

    emit('change', propertyPath, propertyInfo ?? undefined)
  } else {
    emit('change', '', undefined)
  }
}
</script>

<style scoped>
.component-property-selector {
  width: 100%;
}

.selector-level {
  margin-bottom: 16px;
}

.selector-level:last-child {
  margin-bottom: 0;
}

.selector-level .n-form-item {
  margin-bottom: 0;
}

.selector-level .n-select {
  width: 100%;
}
</style>
