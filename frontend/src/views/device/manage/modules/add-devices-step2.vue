<!-- Newly created device connection step: edit voucher fields and show the same copyable access guide used by device details. -->
<script setup lang="ts">
import { computed, defineProps, onMounted, reactive, ref, watchEffect } from 'vue'
import type { FormInst, FormRules } from 'naive-ui'
import { NButton, NForm, NFormItem, NInput, NSelect } from 'naive-ui'
import type { SelectMixedOption } from 'naive-ui/es/select/src/interface'
import { getDeviceConnectInfo, updateDeviceVoucher } from '@/service/api/device'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'
import DeviceAccessGuide from '@/views/device/details/modules/DeviceAccessGuide.vue'
import { buildDeviceAccessGuideState } from '@/views/device/details/modules/device-access-guide-state'

type FormElementType = 'input' | 'table' | 'select'

interface Option {
  label: string
  value: number | string
}

interface Validate {
  message?: string
  required?: boolean
  rules?: string
  type?: 'number' | 'string' | 'array' | 'boolean' | 'object'
}

interface FormElement {
  type: FormElementType
  dataKey: string
  label: string
  options?: Option[]
  placeholder?: string
  validate?: Validate
  array?: FormElement[]
}

const props = defineProps<{
  formElements: FormElement[]
  nextCallback: () => void
  device_id: string
  deviceNumber?: string
  formData: Record<string, unknown>
  setIsSuccess: (flag: boolean) => void
}>()

const formRef = ref<FormInst | null>(null)
const formRules = ref<FormRules>({})
const formData = reactive<Record<string, any>>({})
const connectInfo = ref<Record<string, unknown>>({})

const accessGuideDeviceNumber = computed(() => props.deviceNumber || props.device_id)
const accessGuide = computed(() => buildDeviceAccessGuideState(connectInfo.value, accessGuideDeviceNumber.value, formData))
const credentialKeys = computed(() =>
  props.formElements.flatMap((element) =>
    element.type === 'table' && Array.isArray(element.array)
      ? element.array.map((subElement) => subElement.dataKey)
      : [element.dataKey]
  )
)
const normalizeCredentialValue = (value: unknown) => {
  if (value === undefined || value === null) return ''
  if (typeof value === 'string') return value

  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}
const hasUnsavedCredentialChanges = computed(() =>
  credentialKeys.value.some(
    (key) => normalizeCredentialValue(formData[key]) !== normalizeCredentialValue(props.formData[key])
  )
)

const feachConnectInfo = async () => {
  const res = await getDeviceConnectInfo({ device_id: props.device_id })
  connectInfo.value = res.data || {}
}

const copyText = async (text: unknown) => {
  if (hasUnsavedCredentialChanges.value) {
    window.$message?.warning($t('custom.device_details.accessGuideUnsavedVoucherCopyBlocked'))
    return
  }

  const copied = await writeClipboardText(String(text ?? ''))
  if (copied) {
    window.$message?.success($t('theme.configOperation.copySuccess'))
  } else {
    window.$message?.error($t('common.copyFailed'))
  }
}

onMounted(() => {
  feachConnectInfo()
})

watchEffect(() => {
  props.formElements?.forEach(element => {
    if (element.type === 'table' && Array.isArray(element.array)) {
      element.array.forEach(subElement => {
        formRules.value[subElement.dataKey] = subElement.validate || {}
        formData[subElement.dataKey] ??= props.formData[subElement.dataKey] || ''
      })
      return
    }

    formRules.value[element.dataKey] = element.validate || {}
    formData[element.dataKey] ??= props.formData[element.dataKey] || ''
  })
})

const handleSubmit = async () => {
  await formRef.value?.validate()

  const res = await updateDeviceVoucher({
    device_id: props.device_id,
    voucher: JSON.stringify(formData) || '{}'
  })

  props.setIsSuccess(!res.error)
  props.nextCallback()
}
</script>

<template>
  <DeviceAccessGuide
    :access-guide="accessGuide"
    :connect-info="connectInfo"
    :has-unsaved-credentials="hasUnsavedCredentialChanges"
    @copy="copyText"
  >
    <template #credential-form>
      <NForm ref="formRef" :rules="formRules" :model="formData">
        <template v-for="element in formElements" :key="element.dataKey">
          <div v-if="element.type === 'input'" class="form-item">
            <NFormItem :label="element.label" :path="element.dataKey">
              <NInput v-model:value="formData[element.dataKey]" :placeholder="element.placeholder" />
            </NFormItem>
          </div>
          <div v-if="element.type === 'select'" class="form-item">
            <NFormItem :label="element.label" :path="element.dataKey">
              <NSelect v-model:value="formData[element.dataKey]" :options="element.options as SelectMixedOption[]" />
            </NFormItem>
          </div>
          <div v-if="element.type === 'table'">
            <div class="table-label">{{ element.label }}</div>
            <div class="table-content">
              <template v-for="subElement in element.array" :key="subElement.dataKey">
                <div v-if="subElement.type === 'input'" class="table-item">
                  <NFormItem :label="subElement.label" :path="subElement.dataKey">
                    <NInput v-model:value="formData[subElement.dataKey]" :placeholder="subElement.placeholder" />
                  </NFormItem>
                </div>
                <div v-if="subElement.type === 'select'" class="table-item">
                  <NFormItem :label="subElement.label" :path="subElement.dataKey">
                    <NSelect
                      v-model:value="formData[subElement.dataKey]"
                      :options="subElement.options as SelectMixedOption[]"
                    />
                  </NFormItem>
                </div>
              </template>
            </div>
          </div>
        </template>
      </NForm>
    </template>
  </DeviceAccessGuide>

  <div class="mt-4 w-full flex-center">
    <NButton type="primary" @click="handleSubmit">{{ $t('custom.devicePage.saveAndNext') }}</NButton>
  </div>
</template>

<style scoped>
.form-item {
  display: flex;
  flex-direction: column;
  margin-bottom: 12px;
}

.form-item > * {
  width: 100%;
}

.table-label {
  font-weight: bold;
  margin-bottom: 10px;
}

.table-content {
  margin-left: 20px;
}

.table-item {
  margin-bottom: 8px;
}
</style>
