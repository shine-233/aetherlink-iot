<!--
设备接入信息面板：把设备详情、协议插件、连接参数和 voucher 编辑串成一条可执行的接入向导。
核心链路：加载设备详情 -> 拉取连接信息和动态凭证表单 -> 回显 voucher -> 生成可复制的 MQTT 测试命令 -> 保存凭证。
维护重点：
1. 连接信息和 voucher 都属于敏感配置，新增复制或展示能力时要先确认权限边界。
2. formElements、formData 和 deviceDataStore.deviceData.voucher 强耦合，后端 schema 漂移会直接影响回显和保存。
3. 服务接入型设备(access_way === 'B')仍跳转服务详情；普通设备展示本页接入向导。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { FormInst, FormRules } from 'naive-ui'
import { NAlert, NButton, NDescriptions, NDescriptionsItem, NForm, NFormItem, NInput, NSelect } from 'naive-ui'
import type { SelectMixedOption } from 'naive-ui/es/select/src/interface'
import {
  deviceConnectForm,
  getDeviceDebugLogs,
  getDeviceDebugStatus,
  getDeviceConnectionGuide,
  getDeviceConnectionDiagnostics,
  getDeviceConnectInfo,
  getPlugininfoByService,
  setDeviceDebug,
  updateDeviceVoucher
} from '@/service/api/device'
import type { DeviceDebugLogEntry, DeviceDebugLogsResponse, DeviceDebugStatus } from '@/service/api/device'
import { useDeviceDataStore } from '@/store/modules/device'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'
import DeviceAccessGuide from './DeviceAccessGuide.vue'
import { summarizeDeviceConnectionDiagnostics } from './device-connection-diagnostics-state'
import {
  buildDeviceAccessGuideStateFromConnectionGuide,
  isMaskedVoucherText,
  parseDeviceVoucherPayload
} from './device-access-guide-state'
import type { DeviceAccessGuideDiagnosticsSummary, DeviceConnectionGuideStateInput } from './device-access-guide-state'

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

interface PluginInfo {
  id?: string | number
  service_type?: string
  name?: string
  service_identifier?: string
}

const props = defineProps<{
  id: string
}>()

const route = useRoute()
const router = useRouter()
const deviceDataStore = useDeviceDataStore()
const formRef = ref<FormInst | null>(null)
const formRules = ref<FormRules>({})
const formElements = ref<FormElement[]>([])
const formData = reactive<Record<string, any>>({})
const savedVoucherData = ref<Record<string, unknown>>({})
const connectInfo = ref<Record<string, unknown>>({})
const connectionGuide = ref<DeviceConnectionGuideStateInput | null>(null)
const pluginInfo = ref<PluginInfo>({})
const connectionDiagnostics = ref<DeviceAccessGuideDiagnosticsSummary>({})
const debugStatus = ref<DeviceDebugStatus>({})
const debugLogs = ref<DeviceDebugLogEntry[]>([])
const debugEvidenceLoading = ref(false)
const debugActionLoading = ref(false)
let refreshDebugEvidencePromise: Promise<void> | null = null

const accessGuide = computed(() =>
  buildDeviceAccessGuideStateFromConnectionGuide(
    connectionGuide.value,
    String(deviceDataStore?.deviceData?.device_number || ''),
    formData,
    connectInfo.value,
    connectionDiagnostics.value,
    // 凭证哈希 Phase 2a：详情 voucher 已脱敏时，测试命令降级为占位提示命令。
    { credentialsUnavailable: voucherMasked.value }
  )
)

// voucherMasked：详情响应的凭证为掩码形态。
// 判定依据（Phase 2a 契约）：响应含 voucher_masked=true，或 voucher 值以"…"结尾。
const voucherMasked = computed(() => {
  const deviceData = deviceDataStore?.deviceData
  if (deviceData?.voucher_masked === true) return true
  return isMaskedVoucherText(deviceData?.voucher)
})

// parseVoucher 兼容脱敏输入：掩码形态返回显式 unavailable 而不是空对象，
// 这里只取 credentials 部分；脱敏态下表单保持空白且不可保存（见 handleSubmit 守卫）。
const parseVoucher = (raw: string | undefined) => {
  const availability = parseDeviceVoucherPayload(raw)
  return availability.status === 'ok' ? availability.credentials : {}
}

const credentialKeys = computed(() =>
  formElements.value.flatMap((element) =>
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

const snapshotCurrentCredentials = () =>
  credentialKeys.value.reduce<Record<string, unknown>>((snapshot, key) => {
    snapshot[key] = formData[key]
    return snapshot
  }, {})

const hasUnsavedCredentialChanges = computed(() =>
  credentialKeys.value.some(
    (key) => normalizeCredentialValue(formData[key]) !== normalizeCredentialValue(savedVoucherData.value[key])
  )
)

const getFormJson = async () => {
  const res = await deviceConnectForm({ device_id: props.id })
  formElements.value = Array.isArray(res.data) ? res.data : []
}

const feachConnectInfo = async () => {
  const res = await getDeviceConnectInfo({ device_id: props.id })
  connectInfo.value = res.data || {}
}

const fetchConnectionGuide = async () => {
  try {
    const response = await getDeviceConnectionGuide(props.id, { debug_log_limit: 5, command_log_limit: 3 })
    const guide =
      response && typeof response === 'object' && 'data' in response ? (response as any).data : response
    connectionGuide.value = guide || null
    connectionDiagnostics.value = {}
    return Boolean(connectionGuide.value)
  } catch {
    connectionGuide.value = null
    return false
  }
}

const fetchLatestDiagnosticFailure = async () => {
  try {
    const response = await getDeviceConnectionDiagnostics(props.id, { debug_log_limit: 5 })
    connectionDiagnostics.value = summarizeDeviceConnectionDiagnostics(response)
  } catch {
    connectionDiagnostics.value = {}
  }
}

const fetchDeviceDebugEvidence = async () => {
  debugEvidenceLoading.value = true
  try {
    const [statusResponse, logsResponse] = await Promise.all([
      getDeviceDebugStatus(props.id),
      getDeviceDebugLogs(props.id, { limit: 5 })
    ])
    debugStatus.value = (statusResponse?.data || statusResponse || {}) as DeviceDebugStatus
    const logsData = (logsResponse?.data || logsResponse || {}) as DeviceDebugLogsResponse
    debugLogs.value = Array.isArray(logsData.list) ? logsData.list : []
  } catch {
    debugStatus.value = {}
    debugLogs.value = []
  } finally {
    debugEvidenceLoading.value = false
  }
}

const refreshDebugEvidence = async (force = false) => {
  if (!force && refreshDebugEvidencePromise) {
    return refreshDebugEvidencePromise
  }

  refreshDebugEvidencePromise = (async () => {
    const [guideLoaded] = await Promise.all([fetchConnectionGuide(), fetchDeviceDebugEvidence()])
    if (!guideLoaded) {
      await fetchLatestDiagnosticFailure()
    }
  })()

  try {
    await refreshDebugEvidencePromise
  } finally {
    refreshDebugEvidencePromise = null
  }
}

const enableDebugForThirtyMinutes = async () => {
  debugActionLoading.value = true
  try {
    const response = await setDeviceDebug(props.id, {
      enabled: true,
      duration: 30 * 60,
      max_items: 1000,
      payload_max_bytes: 4096
    })
    debugStatus.value = (response?.data as DeviceDebugStatus) || {}
    await refreshDebugEvidence(true)
    window.$message?.success($t('custom.device_details.accessGuideDebugEnabled'))
  } finally {
    debugActionLoading.value = false
  }
}

const disableDebug = async () => {
  debugActionLoading.value = true
  try {
    const response = await setDeviceDebug(props.id, { enabled: false })
    debugStatus.value = (response?.data as DeviceDebugStatus) || {}
    await refreshDebugEvidence(true)
    window.$message?.success($t('custom.device_details.accessGuideDebugDisabled'))
  } finally {
    debugActionLoading.value = false
  }
}

const getPlugininfoByServiceReq = async (params: { service_identifier: string }) => {
  const { error, data } = await getPlugininfoByService(params)
  if (!error) {
    pluginInfo.value = data
  }
}

const warmAccessGuideData = () => {
  void Promise.allSettled([feachConnectInfo(), getFormJson(), refreshDebugEvidence()])
}

onMounted(async () => {
  const deviceDataPromise = deviceDataStore.fetchData(props.id)
  warmAccessGuideData()

  await deviceDataPromise
  let serviceIdentifier = deviceDataStore?.deviceData?.device_config?.protocol_type
  if (!serviceIdentifier || typeof serviceIdentifier === 'object') {
    serviceIdentifier = 'MQTT'
  }
  if (serviceIdentifier) {
    getPlugininfoByServiceReq({ service_identifier: String(serviceIdentifier) })
  }
})

watch(
  () => deviceDataStore?.deviceData?.voucher,
  (voucher) => {
    savedVoucherData.value = parseVoucher(voucher)
  },
  { immediate: true }
)

watchEffect(() => {
  const voucher = parseVoucher(deviceDataStore?.deviceData?.voucher)
  formElements.value.forEach((element) => {
    if (element.type === 'table' && Array.isArray(element.array)) {
      element.array.forEach((subElement) => {
        formRules.value[subElement.dataKey] = subElement.validate || {}
        formData[subElement.dataKey] ??= voucher[subElement.dataKey] ?? ''
      })
      return
    }

    formRules.value[element.dataKey] = element.validate || {}
    formData[element.dataKey] ??= voucher[element.dataKey] ?? ''
  })
})

const handleSubmit = async () => {
  // 脱敏态守卫：表单已隐藏且字段为空，提交会用空凭证覆盖真实凭证，必须拦截。
  if (voucherMasked.value) {
    window.$message?.warning($t('custom.device.accessGuide.maskedNotice'))
    return
  }
  await formRef.value?.validate()
  await updateDeviceVoucher({
    device_id: props.id,
    voucher: JSON.stringify(formData)
  })
  savedVoucherData.value = snapshotCurrentCredentials()
  await fetchConnectionGuide()
  window.$message?.success($t('common.updateSuccess'))
}

const copyText = async (text: unknown) => {
  if (hasUnsavedCredentialChanges.value) {
    window.$message?.warning($t('custom.device_details.accessGuideUnsavedVoucherCopyBlocked'))
    return
  }

  const copied = await writeClipboardText(String(text ?? ''))
  if (copied) {
    window.$message?.success($t('custom.device_details.accessGuideCopySuccessNextStep'))
  } else {
    window.$message?.error($t('common.copyFailed'))
  }
}

const openReadyCheck = () => {
  router.push({
    path: '/device/details',
    query: {
      ...route.query,
      d_id: props.id,
      tab: 'ready-check'
    }
  })
}

const openDeviceTwin = () => {
  router.push({
    path: '/device/details',
    query: {
      ...route.query,
      d_id: props.id,
      tab: 'device-twin'
    }
  })
}

const toServiceClick = () => {
  if (deviceDataStore?.deviceData?.access_way === 'B') {
    router.push(
      `/device/service-details?id=${pluginInfo.value.id}&service_type=${pluginInfo.value.service_type}&service_name=${pluginInfo.value.name}&service_identifier=${pluginInfo.value.service_identifier}`
    )
  }
}
</script>

<template>
  <div data-testid="device-join-section">
    <NDescriptions label-placement="left" :column="1" class="mt-6">
      <NDescriptionsItem :label="$t('generate.access-method-service')">
        <div :class="deviceDataStore?.deviceData?.access_way === 'B' ? 'blue-text' : ''" @click="toServiceClick">
          {{ deviceDataStore?.deviceData?.device_config?.protocol_type || '--' }}
        </div>
      </NDescriptionsItem>
    </NDescriptions>

    <DeviceAccessGuide
      v-if="deviceDataStore?.deviceData?.access_way !== 'B'"
      :device-id="props.id"
      :access-guide="accessGuide"
      :connect-info="connectInfo"
      :credentials-masked="voucherMasked"
      :debug-status="debugStatus"
      :debug-logs="debugLogs"
      :debug-loading="debugEvidenceLoading"
      :debug-action-loading="debugActionLoading"
      :has-unsaved-credentials="hasUnsavedCredentialChanges"
      @copy="copyText"
      @open-ready-check="openReadyCheck"
      @open-twin-evidence="openDeviceTwin"
      @enable-debug="enableDebugForThirtyMinutes"
      @disable-debug="disableDebug"
      @refresh-debug-evidence="refreshDebugEvidence"
    >
      <template #credential-form>
        <!-- 凭证已脱敏态：隐藏密码输入与复制入口，展示轮换指引文案（Phase 2a 降级）。 -->
        <NAlert v-if="voucherMasked" type="warning" data-testid="device-access-guide-masked-notice">
          {{ $t('custom.device.accessGuide.maskedNotice') }}
        </NAlert>
        <NForm v-else ref="formRef" :rules="formRules" :model="formData">
          <template v-for="element in formElements" :key="element.dataKey">
            <div v-if="element.type === 'input'" class="form-item">
              <NFormItem :label="element.label" :path="element.dataKey" style="height: 50px">
                <NInput v-model:value="formData[element.dataKey]" :placeholder="element.placeholder" />
              </NFormItem>
            </div>
            <div v-if="element.type === 'select'" class="form-item">
              <NFormItem :label="element.label" :path="element.dataKey">
                <NSelect v-model:value="formData[element.dataKey]" :options="element.options as SelectMixedOption[]" />
              </NFormItem>
            </div>
            <div v-if="element.type === 'table'">
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

    <div v-if="deviceDataStore?.deviceData?.access_way !== 'B' && !voucherMasked" class="mt-4 w-full flex-center">
      <NButton type="primary" data-testid="device-access-guide-save-credentials" @click="handleSubmit">
        {{ $t('common.save') }}
      </NButton>
    </div>
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

.blue-text {
  color: blue;
  cursor: pointer;
}
</style>
