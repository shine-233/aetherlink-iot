<!--
文件用途: 物模型新增和编辑弹窗。
核心逻辑: 按步骤加载基础信息、模型定义、图表配置和完成页，统一管理弹窗状态。
关键注意事项: 弹窗会复用物模型初始化数据，切换物模型或关闭弹窗时要避免残留上一次状态。
重构建议: 将步骤状态机和提交逻辑抽成组合函数，降低弹窗组件复杂度。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watchEffect } from 'vue'
import { $t } from '@/locales'
import { initTemplateInfoData, templateInfoData } from '../utils'
const AddInfo = defineAsyncComponent(() => import('./step/add-info.vue'))
const ModelDefinition = defineAsyncComponent(() => import('./step/model-definition.vue'))
const WebChartConfig = defineAsyncComponent(() => import('./step/web-chart-config.vue'))
const AppChartConfig = defineAsyncComponent(() => import('./step/app-chart-config.vue'))
const Complete = defineAsyncComponent(() => import('./step/complete.vue'))

export interface Props {
  visible: boolean
  type: 'add' | 'edit'
  templateId: string
  getTableData: () => void
}

const props = withDefaults(defineProps<Props>(), {
  type: 'add'
})

const stepCurrent = ref<number>(1)
const deviceTemplateId = ref<string>(props.type === 'add' ? '' : props.templateId)

const componentsList: { id: number; components: any }[] = [
  { id: 1, components: AddInfo },
  { id: 2, components: ModelDefinition },
  { id: 3, components: WebChartConfig },
  { id: 4, components: AppChartConfig },
  { id: 5, components: Complete }
]
const SwitchComponents = computed<any>(() => {
  return componentsList.find(item => item.id === stepCurrent.value)?.components
})

export type ModalType = NonNullable<Props['type']>

const emit = defineEmits<{
  'update:visible': [visible: boolean]
}>()

const modalVisible = computed({
  get() {
    // eslint-disable-next-line vue/no-side-effects-in-computed-properties
    stepCurrent.value = 1
    if (!props.visible) {
      templateInfoData.value = { ...initTemplateInfoData }
    }
    return props.visible
  },
  set(visible) {
    emit('update:visible', visible)
  }
})
const title = computed(() => {
  const titles: Record<ModalType, string> = {
    add: $t('device_template.addThingModel'),
    edit: $t('device_template.editThingModel')
  }
  return titles[props.type]
})

watchEffect(() => {
  deviceTemplateId.value = props.templateId
})

defineOptions({ name: 'TableActionModal' })
</script>

<template>
  <NModal
    v-model:show="modalVisible"
    preset="card"
    :title="title"
    class="w-80%"
    @after-leave="
      () => {
        deviceTemplateId = props.templateId
        props.getTableData()
      }
    "
  >
    <n-steps :current="stepCurrent" status="process">
      <n-step :title="$t('device_template.basicInfo')" :description="$t('device_template.addDeviceInfo')" />
      <n-step
        :title="$t('device_template.modelDefinition')"
        :description="$t('device_template.deviceParameterDescribe')"
      />
      <n-step
        :title="$t('device_template.webChartConfiguration')"
        :description="$t('device_template.bindTheCorrespondingChart')"
      />
      <n-step
        :title="$t('device_template.appChartConfiguration')"
        :description="$t('device_template.editAppDetailsPage')"
      />
      <n-step :title="$t('device_template.release')" :description="$t('device_template.releaseAppStore')" />
    </n-steps>

    <component
      :is="SwitchComponents"
      v-model:stepCurrent="stepCurrent"
      v-model:modalVisible="modalVisible"
      v-model:deviceTemplateId="deviceTemplateId"
    ></component>
  </NModal>
</template>

<style scoped></style>
