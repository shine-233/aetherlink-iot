<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NButton,
  NDivider,
  NDrawer,
  NDrawerContent,
  NIcon,
  NInput,
  NRadio,
  NRadioGroup,
  NSpace,
  NText,
  useMessage
} from 'naive-ui'
import { CreateOutline as EditOutline, LinkOutline, PhonePortraitOutline as DeviceIcon } from '@vicons/ionicons5'
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import ComponentPropertySelector from '@/core/data-architecture/components/common/ComponentPropertySelector.vue'
import UnifiedDeviceConfigSelector from '@/core/data-architecture/components/device-selectors/UnifiedDeviceConfigSelector.vue'
import { createNewParamConfig } from './dynamicParameterEditorState'
import type { NewParamConfig } from './dynamicParameterEditorState'
import { buildNewParamFromDrawerConfig, validateNewParameterKey } from './dynamicParameterEditorNewParam'

const props = defineProps<{
  show: boolean
  parameterType: 'header' | 'query' | 'path'
  existingParameters: EnhancedParameter[]
  currentComponentId?: string
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'add-parameter', parameter: EnhancedParameter): void
  (e: 'device-parameters-generated', parameters: EnhancedParameter[]): void
}>()

const message = useMessage()
const newParamConfig = ref<NewParamConfig>(createNewParamConfig())

const drawerVisible = computed({
  get: () => props.show,
  set: (value: boolean) => {
    emit('update:show', value)
  }
})

const resetNewParamConfig = () => {
  newParamConfig.value = createNewParamConfig()
}

const closeDrawer = () => {
  drawerVisible.value = false
  resetNewParamConfig()
}

watch(
  () => props.show,
  (show) => {
    if (show) {
      resetNewParamConfig()
    }
  }
)

const handlePropertyChange = (value: any) => {
  newParamConfig.value.propertyBinding = value
}

const handleDeviceConfigGenerated = (parameters: EnhancedParameter[]) => {
  if (!parameters || parameters.length === 0) {
    return
  }

  emit('device-parameters-generated', parameters)
  closeDrawer()
}

const validateConfigKey = () => {
  const result = validateNewParameterKey(newParamConfig.value.key, props.existingParameters)

  if (result.ok) {
    return result.key
  }

  if (result.reason === 'empty') {
    message.error('参数名(key)不能为空！')
    return null
  }

  message.error(`参数 key "${newParamConfig.value.key}" 已存在，不允许重复！`)
  return null
}

const confirmNewParam = () => {
  const key = validateConfigKey()
  if (!key) {
    return
  }

  const newParam = buildNewParamFromDrawerConfig(newParamConfig.value, key)
  emit('add-parameter', newParam)
  closeDrawer()
  message.success(`参数 "${newParam.key}" 添加成功！`)
}
</script>

<template>
  <n-drawer v-model:show="drawerVisible" :width="550" placement="right">
    <n-drawer-content title="添加参数" closable>
      <n-space vertical :size="16">
        <div>
          <n-text strong>参数名 (Key)</n-text>
          <n-text depth="3" style="font-size: 12px; margin-left: 8px">*必填</n-text>
          <n-input
            v-model:value="newParamConfig.key"
            placeholder="请输入参数名，如: device_id"
            size="medium"
            clearable
            style="margin-top: 8px"
          />
        </div>

        <div v-if="newParamConfig.key && newParamConfig.key.trim()">
          <n-divider />

          <div>
            <n-text strong>配置类型</n-text>
            <n-radio-group v-model:value="newParamConfig.configType" size="medium" style="margin-top: 8px">
              <n-space vertical>
                <n-radio value="manual">
                  <n-space align="center">
                    <n-icon size="18" color="#18a058"><EditOutline /></n-icon>
                    <span>手动输入</span>
                  </n-space>
                </n-radio>
                <n-radio value="property">
                  <n-space align="center">
                    <n-icon size="18" color="#2080f0"><LinkOutline /></n-icon>
                    <span>属性绑定</span>
                  </n-space>
                </n-radio>
                <n-radio value="device">
                  <n-space align="center">
                    <n-icon size="18" color="#f0a020"><DeviceIcon /></n-icon>
                    <span>设备配置</span>
                  </n-space>
                </n-radio>
              </n-space>
            </n-radio-group>
          </div>

          <n-divider />

          <div v-if="newParamConfig.configType === 'manual'">
            <n-space vertical :size="12">
              <div>
                <n-text strong>参数值</n-text>
                <n-input
                  v-model:value="newParamConfig.value"
                  placeholder="请输入参数值"
                  size="medium"
                  clearable
                  style="margin-top: 8px"
                />
              </div>

              <div>
                <n-text strong>描述（可选）</n-text>
                <n-input
                  v-model:value="newParamConfig.description"
                  placeholder="请输入参数描述"
                  size="medium"
                  type="textarea"
                  :rows="3"
                  clearable
                  style="margin-top: 8px"
                />
              </div>
            </n-space>
          </div>

          <div v-else-if="newParamConfig.configType === 'property'">
            <n-text strong>属性绑定配置</n-text>
            <div style="margin-top: 12px">
              <ComponentPropertySelector
                :value="newParamConfig.propertyBinding"
                :current-component-id="currentComponentId"
                @change="handlePropertyChange"
              />
            </div>

            <n-divider />

            <div>
              <n-text strong>描述（可选）</n-text>
              <n-input
                v-model:value="newParamConfig.description"
                placeholder="请输入参数描述"
                size="medium"
                type="textarea"
                :rows="2"
                clearable
                style="margin-top: 8px"
              />
            </div>
          </div>

          <div v-else-if="newParamConfig.configType === 'device'">
            <n-text strong>设备配置</n-text>
            <div style="margin-top: 12px">
              <UnifiedDeviceConfigSelector
                :parameter-type="parameterType"
                :existing-parameters="existingParameters"
                @parameters-generated="handleDeviceConfigGenerated"
              />
            </div>

            <n-divider />

            <div>
              <n-text strong>描述（可选）</n-text>
              <n-input
                v-model:value="newParamConfig.description"
                placeholder="请输入参数描述"
                size="medium"
                type="textarea"
                :rows="2"
                clearable
                style="margin-top: 8px"
              />
            </div>
          </div>
        </div>
      </n-space>

      <template #footer>
        <n-space justify="end">
          <n-button @click="closeDrawer">取消</n-button>
          <n-button
            type="primary"
            :disabled="!newParamConfig.key || !newParamConfig.key.trim()"
            @click="confirmNewParam"
          >
            确认添加
          </n-button>
        </n-space>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>
