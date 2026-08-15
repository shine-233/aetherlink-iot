<script setup lang="ts">
import { defineAsyncComponent, ref, watch } from 'vue'
import { NButton, NDivider, NDrawer, NDrawerContent, NInput, NText } from 'naive-ui'
import DeviceMetricsSelector from '@/components/device-selectors/DeviceMetricsSelector.vue'
import DeviceDispatchSelector from '@/components/device-selectors/DeviceDispatchSelector.vue'
import ComponentPropertySelector from '@/core/data-architecture/components/common/ComponentPropertySelector.vue'
import { type EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import { getTemplateById } from '@/core/data-architecture/components/common/templates/index'
import { resolveRecoverableComponentBindingPath } from '@/core/data-architecture/utils/binding-path-recovery'
import { createLogger } from '@/utils/logger'

const logger = createLogger('DynamicParameterComponentDrawer')

// Keep the large icon registry out of the parameter editor's initial bundle.
const IconSelector = defineAsyncComponent(() => import('@/components/common/icon-selector.vue'))

const componentMap = {
  DeviceMetricsSelector,
  DeviceDispatchSelector,
  IconSelector,
  ComponentPropertySelector
}

interface Props {
  show: boolean
  parameter: EnhancedParameter | null
  currentComponentId?: string
}

interface Emits {
  (e: 'update:show', value: boolean): void
  (e: 'save', value: EnhancedParameter): void
  (e: 'closed'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const drawerParam = ref<EnhancedParameter | null>(null)

watch(
  () => [props.show, props.parameter] as const,
  ([show, parameter]) => {
    if (show && parameter) {
      drawerParam.value = { ...parameter }
    }
  },
  { immediate: true }
)

const closeDrawer = () => {
  emit('update:show', false)
}

const handleShowUpdate = (value: boolean) => {
  emit('update:show', value)
}

const handleAfterLeave = () => {
  drawerParam.value = null
  emit('closed')
}

const resolveDrawerBindingPath = (bindingPath: string) => {
  if (!drawerParam.value) {
    return null
  }

  const recoveredBindingPath = resolveRecoverableComponentBindingPath(bindingPath, drawerParam.value.variableName, {
    allowEmpty: true,
    strict: true
  })

  if ((recoveredBindingPath.recovered || !recoveredBindingPath.isValid) && bindingPath !== '') {
    logger.error('[DynamicParameterComponentDrawer] invalid bindingPath format; attempting automatic recovery', {
      inputValue: bindingPath,
      valueType: typeof bindingPath,
      valueLength: typeof bindingPath === 'string' ? bindingPath.length : 'non-string',
      expectedFormat: 'componentId.layer.propertyName',
      currentParameter: {
        key: drawerParam.value.key,
        currentValue: drawerParam.value.value,
        variableName: drawerParam.value.variableName
      }
    })

    if (!recoveredBindingPath.recovered || !recoveredBindingPath.bindingPath) {
      logger.error(
        '[DynamicParameterComponentDrawer] failed to recover bindingPath from variableName; rejecting update'
      )
      return null
    }

    return recoveredBindingPath.bindingPath
  }

  return bindingPath
}

const applyDrawerPropertyMetadata = (bindingPath: string, propertyInfo?: any) => {
  if (!drawerParam.value) {
    return
  }

  if (propertyInfo && bindingPath) {
    drawerParam.value.description = `绑定到组件属性: ${propertyInfo.componentName} -> ${propertyInfo.propertyLabel}`
    drawerParam.value.variableName = `${propertyInfo.componentId}_${propertyInfo.propertyName}`
    return
  }

  if (bindingPath === '') {
    drawerParam.value.description = ''
    drawerParam.value.variableName = ''
  }
}

const updateDrawerBinding = (bindingPath: string, propertyInfo?: any) => {
  if (!drawerParam.value) {
    return
  }

  drawerParam.value.value = bindingPath
  applyDrawerPropertyMetadata(bindingPath, propertyInfo)
}

const handleComponentPropertyChange = (bindingPath: string, propertyInfo?: any) => {
  if (!drawerParam.value) {
    logger.warn('[DynamicParameterComponentDrawer] drawerParam is empty; ignoring property change')
    return
  }

  const normalizedBindingPath = resolveDrawerBindingPath(bindingPath)
  if (normalizedBindingPath === null) {
    return
  }

  updateDrawerBinding(normalizedBindingPath, propertyInfo)
}

const saveDrawerChanges = () => {
  if (!drawerParam.value) {
    closeDrawer()
    return
  }

  emit('save', { ...drawerParam.value })
  closeDrawer()
}

const getComponentTemplate = (param: EnhancedParameter | null) => {
  if (!param || !param.selectedTemplate) return null
  const template = getTemplateById(param.selectedTemplate)
  const config = template?.componentConfig
  if (!config) return null

  const component =
    typeof config.component === 'string'
      ? componentMap[config.component as keyof typeof componentMap]
      : config.component

  let enhancedProps = { ...config.props }

  if (
    config.component === 'ComponentPropertySelector' ||
    (typeof config.component === 'string' && config.component === 'ComponentPropertySelector')
  ) {
    enhancedProps = {
      ...enhancedProps,
      currentComponentId: props.currentComponentId,
      autoDetectComponentId: true
    }
  }

  return {
    ...config,
    component,
    props: enhancedProps
  }
}
</script>

<template>
  <n-drawer :show="show" :width="500" :on-after-leave="handleAfterLeave" @update:show="handleShowUpdate">
    <n-drawer-content :title="`编辑 ${getComponentTemplate(drawerParam)?.name || '参数'}`" closable>
      <template v-if="drawerParam">
        <component
          :is="getComponentTemplate(drawerParam)?.component"
          v-if="getComponentTemplate(drawerParam)?.component"
          :value="drawerParam.value"
          v-bind="getComponentTemplate(drawerParam)?.props || {}"
          @change="handleComponentPropertyChange"
        />
        <div v-else>组件加载失败</div>

        <div v-if="drawerParam.selectedTemplate === 'component-property-binding'" style="margin-top: 16px">
          <n-divider />
          <div style="margin-bottom: 8px">
            <n-text strong>默认值设置</n-text>
            <n-text depth="3" style="font-size: 12px; margin-left: 8px">当绑定的组件属性为空时使用</n-text>
          </div>
          <n-input v-model:value="drawerParam.defaultValue" placeholder="请输入默认值（可选）" clearable />
          <n-text depth="3" style="font-size: 12px; margin-top: 4px; display: block">
            提示：如果组件属性值为空（null、undefined或空字符串），将使用此默认值
          </n-text>
        </div>
      </template>
      <template #footer>
        <n-button @click="closeDrawer">取消</n-button>
        <n-button type="primary" @click="saveDrawerChanges">确定</n-button>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>
