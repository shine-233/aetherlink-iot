<!--
文件用途: 设备配置新增/编辑弹窗，负责收集配置基础字段并提交到设备配置接口。
核心链路: 外层控制弹窗显示 -> 组件按新增/编辑模式切换标题 -> 拉取物模型选项 -> 校验表单 -> 调用新增/编辑接口 -> 关闭弹窗并通知上层刷新。
维护边界:
1. 当前文件负责“弹窗内表单状态 + 提交流程”，不负责完整的页面列表刷新与编辑态来源管理。
2. 编辑态的回填来源不在本文件可见，后续调整编辑流程时要同步核对父层传值时机。
3. 物模型选项每次打开弹窗都会重新请求，若后续改为缓存/分页，要一起梳理下拉滚动与刷新策略。
-->
<script lang="ts" setup>
import { ref, watch } from 'vue'
import type { FormInst } from 'naive-ui'
import { $t } from '@/locales'
// import {useMessage} from 'naive-ui';
import { deviceConfigAdd, deviceConfigEdit, deviceTemplate } from '@/service/api/device'

// const message = useMessage();

interface Props {
  modalVisible?: boolean
  modalType?: string
}

const props = withDefaults(defineProps<Props>(), {
  modalVisible: false,
  modalType: 'add'
})
const modalTitle = ref($t('generate.add'))
const configForm = ref(defaultConfigForm())

// 统一维护新增态默认值，也作为关闭弹窗后的表单复位基线。
function defaultConfigForm() {
  return {
    additional_info: null,
    description: null,
    device_conn_type: null,
    device_template_id: null,
    device_type: null,
    name: null,
    protocol_config: null,
    protocol_type: null,
    remark: null,
    voucher_type: null
  }
}

// 当前仅覆盖弹窗内可见必填项；若后续增加更多协议字段，记得同步补校验说明。
const configFormRules = ref({
  name: {
    required: true,
    message: $t('common.deviceConfigName'),
    trigger: 'blur'
  },
  device_type: {
    required: true,
    message: $t('common.deviceAccessType'),
    trigger: 'change'
  },
  device_conn_type: {
    required: true,
    message: $t('common.deviceConnectionMethod'),
    trigger: 'change'
  }
})
type DeviceTemplateOption = { id: string; name: string; [key: string]: unknown }

const deviceTemplateOptions = ref<DeviceTemplateOption[]>([])
const templatePage = ref(1)
const templatePageSize = 20
const templateTotal = ref(0)
const templateLoading = ref(false)
let templateRequestGeneration = 0

const mergeTemplateOptions = (current: DeviceTemplateOption[], incoming: DeviceTemplateOption[]) => {
  const byID = new Map(current.map(option => [option.id, option]))
  incoming.forEach(option => {
    if (option?.id) byID.set(option.id, option)
  })
  return Array.from(byID.values())
}

// 每次打开从第一页重新加载；滚动加载时追加并按 ID 去重。
const getDeviceTemplate = async (reset = false) => {
  if (templateLoading.value && !reset) return

  if (reset) {
    templateRequestGeneration += 1
    templatePage.value = 1
    templateTotal.value = 0
    deviceTemplateOptions.value = []
  }

  const generation = templateRequestGeneration
  templateLoading.value = true
  try {
    const res = await deviceTemplate({ page: templatePage.value, page_size: templatePageSize })
    if (generation !== templateRequestGeneration || res.error) return

    const list = Array.isArray(res.data?.list) ? res.data.list : []
    deviceTemplateOptions.value = mergeTemplateOptions(deviceTemplateOptions.value, list)
    templateTotal.value = Number(res.data?.total ?? deviceTemplateOptions.value.length)
  } catch {
    // 保留已加载选项；网络恢复后可通过重新打开或继续滚动重试。
  } finally {
    if (generation === templateRequestGeneration) templateLoading.value = false
  }
}

interface Emits {
  (e: 'modalClose'): void

  (e: 'submitted'): void
}

const emit = defineEmits<Emits>()
const visible = ref(false)

// 这里把“是否显示”和“新增/编辑标题”绑在同一个观察点上，属于当前弹窗的生命周期入口。
watch(
  () => props.modalVisible,
  newValue => {
    visible.value = newValue
    if (props.modalType === 'add') {
      modalTitle.value = $t('generate.add')
    } else {
      modalTitle.value = $t('common.edit')
    }
    if (newValue) void getDeviceTemplate(true)
  }
)
const modalClose = () => {
  emit('modalClose')
}

const deviceTemplateScroll = (event: Event) => {
  const target = event.currentTarget as HTMLElement | null
  if (!target || templateLoading.value) return
  if (target.scrollTop + target.offsetHeight < target.scrollHeight - 2) return
  if (deviceTemplateOptions.value.length >= templateTotal.value) return

  templatePage.value += 1
  void getDeviceTemplate()
}
const configFormRef = ref<HTMLElement & FormInst>()

// 关闭时统一清理校验状态和表单默认值，避免下次打开时残留上一次输入。
const handleClose = () => {
  configFormRef.value?.restoreValidation?.()
  configForm.value = defaultConfigForm()
  visible.value = false
  modalClose()
}

// `submitted` 只表示后端已保存成功；失败时保留弹窗和用户输入。
const handleSubmit = async () => {
  await configFormRef?.value?.validate()
  try {
    const res = props.modalType === 'add'
      ? await deviceConfigAdd(configForm.value)
      : await deviceConfigEdit(configForm.value)
    if (res.error) return
  } catch {
    return
  }

  handleClose()
  emit('submitted')
}
</script>

<template>
  <div class="overflow-hidden">
    <NCard :title="`${modalTitle}${$t('custom.devicePage.deviceConfig')}`">
      <NForm ref="configFormRef" :model="configForm" :rules="configFormRules" label-placement="left" label-width="auto">
        <NFormItem :label="$t('generate.device-configuration-name')" path="name">
          <NInput v-model:value="configForm.name" :placeholder="$t('generate.enter-device-name')" />
        </NFormItem>
        <NFormItem :label="$t('generate.select-device-function-template')" path="device_template_id">
          <NSelect
            v-model:value="configForm.device_template_id"
            :options="deviceTemplateOptions"
            label-field="name"
            value-field="id"
            @scroll="deviceTemplateScroll"
          ></NSelect>
        </NFormItem>
        <NFormItem :label="$t('generate.device-access-type')" path="device_type">
          <n-radio-group v-model:value="configForm.device_type" name="device_type">
            <n-space>
              <n-radio value="1">{{ $t('generate.direct-connected-device') }}</n-radio>
              <n-radio value="2">{{ $t('generate.gateway') }}</n-radio>
              <n-radio value="3">{{ $t('generate.gateway-sub-device') }}</n-radio>
            </n-space>
          </n-radio-group>
        </NFormItem>
        <NFormItem :label="$t('generate.device-connection-method')" path="device_conn_type">
          <n-radio-group v-model:value="configForm.device_conn_type" name="device_conn_type">
            <n-space>
              <n-radio value="A">{{ $t('generate.device-connect-platform') }}</n-radio>
              <n-radio value="B">{{ $t('generate.platform-connect-device') }}</n-radio>
            </n-space>
          </n-radio-group>
        </NFormItem>
        <NFlex>
          <NButton @click="handleClose">{{ $t('generate.cancel') }}</NButton>
          <NButton type="primary" @click="handleSubmit">{{ $t('page.login.common.confirm') }}</NButton>
        </NFlex>
      </NForm>
    </NCard>
  </div>
</template>
