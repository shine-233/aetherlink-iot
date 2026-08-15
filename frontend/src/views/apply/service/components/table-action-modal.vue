<!--
文件用途：提供 协议服务管理 页面内的 table-action-modal 子组件。
核心逻辑：封装局部表单、弹窗、列表或展示模块，通过 props、emit 与父页面协作。
关键注意事项：保持组件边界清晰，避免在子组件中绕过父页面的数据刷新与权限控制。
重构建议：后续可把重复表单规则、选项转换和弹窗状态管理抽成可复用组合函数。
-->
<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type { FormInst } from 'naive-ui'
import { smartDeepClone as deepClone } from '@/utils/deep-clone'
import { addProtocolPlugin, editProtocolPlugin } from '@/service/api'
import { createRequiredFormRule } from '@/utils/form/rule'
import { $t } from '@/locales'

export interface Props {
  /** 弹窗可见性 */
  visible: boolean
  /** 弹窗类型 add: 新增 edit: 编辑 */
  type?: 'add' | 'edit'
  /** 编辑的表格行数据 */
  editData?: ServiceManagement.Service | null
}

export type ModalType = NonNullable<Props['type']>

defineOptions({ name: 'TableActionModal' })

const props = withDefaults(defineProps<Props>(), {
  type: 'add',
  editData: null
})

interface Emits {
  (e: 'update:visible', visible: boolean): void
  (e: 'success'): void
}

const emit = defineEmits<Emits>()

const modalVisible = computed({
  get() {
    return props.visible
  },
  set(visible) {
    emit('update:visible', visible)
  }
})
const closeModal = () => {
  modalVisible.value = false
}

const title = computed(() => {
  const titles: Record<ModalType, string> = {
    add: $t('common.add'),
    edit: $t('common.edit')
  }
  return titles[props.type]
})

const formRef = ref<HTMLElement & FormInst>()

const deviceOptions = ref<any[]>([
  { label: $t('generate.direct-connected-device'), value: 1 },
  { label: $t('generate.gatewayDevice'), value: 2 }
])

type FormModel = Pick<
  ServiceManagement.Service,
  | 'name'
  | 'device_type'
  | 'protocol_type'
  | 'access_address'
  | 'http_address'
  | 'sub_topic_prefix'
  | 'description'
  | 'additional_info'
  | 'language_code'
> & {
  additional_info_list: any[]
}

const formModel = reactive<FormModel>(createDefaultFormModel())

const rules = {
  name: createRequiredFormRule($t('common.pleaseCheckValue')),
  device_type: createRequiredFormRule($t('common.pleaseCheckValue')),
  protocol_type: createRequiredFormRule($t('common.pleaseCheckValue')),
  access_address: createRequiredFormRule($t('common.pleaseCheckValue')),
  http_address: createRequiredFormRule($t('common.pleaseCheckValue')),
  sub_topic_prefix: createRequiredFormRule($t('common.pleaseCheckValue'))
}

function parseAdditionalInfo(value: string | null | undefined) {
  if (!value) return {}
  try {
    const parsed = JSON.parse(value)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

function createDefaultFormModel(): FormModel {
  return {
    name: '',
    device_type: '',
    protocol_type: '',
    access_address: null,
    http_address: null,
    sub_topic_prefix: null,
    description: null,
    language_code: 'zh',
    additional_info: '',
    additional_info_list: []
  }
}

function handleUpdateFormModel(model: Partial<FormModel>) {
  Object.assign(formModel, model)
  const additional_info_list: any = []
  const additional_info = parseAdditionalInfo(formModel.additional_info)
  for (const key in additional_info) {
    if (Object.hasOwn(additional_info, key)) {
      const value = additional_info[key]
      additional_info_list.push({ key, value })
    }
  }
  formModel.additional_info_list = additional_info_list
}

function handleUpdateFormModelByModalType() {
  const handlers: Record<ModalType, () => void> = {
    add: () => {
      const defaultFormModel = createDefaultFormModel()
      handleUpdateFormModel(defaultFormModel)
    },
    edit: () => {
      if (props.editData) {
        handleUpdateFormModel(props.editData)
      }
    }
  }

  handlers[props.type]()
}

async function handleSubmit() {
  await formRef.value?.validate()
  const formData = deepClone(formModel)
  formData.device_type = Number(formData.device_type)

  const additional_info = {}
  formData.additional_info_list.map((item: any) => {
    if (item.key && item.value) {
      return (additional_info[item.key] = item.value)
    }
    return additional_info[item.key]
  })
  formData.additional_info = JSON.stringify(additional_info)
  delete (formData as Record<string, unknown>).additional_info_list
  let data: any
  if (props.type === 'add') {
    data = await addProtocolPlugin(formData)
  } else if (props.type === 'edit') {
    data = await editProtocolPlugin(formData)
  }
  if (!data.error) {
    emit('success')
  }
  closeModal()
}

function handleAddAdditionalInfo() {
  formModel.additional_info_list.push({
    key: '',
    value: ''
  })
}

watch(
  () => props.visible,
  newValue => {
    if (newValue) {
      handleUpdateFormModelByModalType()
    }
  }
)
</script>

<template>
  <NModal v-model:show="modalVisible" preset="card" :title="title" class="w-700px">
    <NForm ref="formRef" label-placement="left" :label-width="120" :model="formModel" :rules="rules">
      <NGrid :cols="24" :x-gap="18">
        <NFormItemGridItem :span="24" :label="$t('page.apply.service.form.serviceName')" path="name">
          <NInput v-model:value="formModel.name" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('page.apply.service.form.deviceType')" path="device_type">
          <NSelect v-model:value="formModel.device_type" :options="deviceOptions" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('page.apply.service.form.protocolType')" path="protocol_type">
          <NInput v-model:value="formModel.protocol_type" />
          <!-- <NSelect v-model:value="formModel.protocol_type" :options="serviceManagementProtocolTypeOptions" /> -->
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('page.apply.service.form.accessAddress')" path="access_address">
          <NInput v-model:value="formModel.access_address" placeholder="broker-host:1883" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('page.apply.service.form.httpAddress')" path="http_address">
          <NInput v-model:value="formModel.http_address" placeholder="http://protocol-service:8080" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('page.apply.service.form.subTopicPrefix')" path="sub_topic_prefix">
          <NInput v-model:value="formModel.sub_topic_prefix" placeholder="tenant/{tenantId}/service/{service}/" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('common.description')">
          <NInput v-model:value="formModel.description" type="textarea" :placeholder="$t('common.description')" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('page.apply.service.form.additionalInfo')">
          <NButton class="w-72px" type="primary" @click="handleAddAdditionalInfo">
            {{ $t('common.add') }}
          </NButton>
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" label=" ">
          <div>
            <div v-for="(item, index) in formModel.additional_info_list" :key="index" class="mt-10px flex">
              <NInput v-model:value="item.key" :placeholder="$t('generate.fieldKey')" />
              <NInput v-model:value="item.value" class="ml-20px" :placeholder="$t('generate.fieldValue')" />
            </div>
          </div>
        </NFormItemGridItem>
      </NGrid>
      <NSpace class="w-full pt-16px" :size="24" justify="end">
        <NButton class="w-72px" @click="closeModal">
          {{ $t('common.cancel') }}
        </NButton>
        <NButton class="w-72px" type="primary" @click="handleSubmit">
          {{ $t('common.confirm') }}
        </NButton>
      </NSpace>
    </NForm>
  </NModal>
</template>

<style scoped></style>
