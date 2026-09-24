<!--
文件用途：Data Converter（ThingsBoard 对标数据编解码器）管理控制台。
核心逻辑：
1. 转换器列表展示（上行 Uplink / 下行 Downlink、脚本 SCRIPT / 十六进制 HEX / JSONPath）；
2. 转换器增删改查；
3. 在线仿真测试沙箱（输入报文与元数据，即时执行解析并验证结果）。
-->
<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NDrawer,
  NDrawerContent,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NRadio,
  NRadioGroup,
  NSelect,
  NSpace,
  NSwitch,
  NTag,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import {
  createDataConverter,
  deleteDataConverter,
  getDataConvertersList,
  testDataConverter,
  updateDataConverter,
  type ConverterMode,
  type ConverterType,
  type CreateDataConverterParams,
  type DataConverterItem,
  type TestDataConverterResponse
} from '@/service/api/data-converter'
import { formatDateTime } from '@/utils/common/datetime'

const message = useMessage()

// 列表状态
const loading = ref(false)
const list = ref<DataConverterItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const searchQuery = ref('')
const selectedType = ref<ConverterType | null>(null)

// 模态弹窗
const modalVisible = ref(false)
const isEdit = ref(false)
const currentId = ref('')
const formRef = ref<FormInst | null>(null)
const submitting = ref(false)

// 表单模型
const formModel = reactive<{
  name: string
  type: ConverterType
  converter_mode: ConverterMode
  debug_mode: boolean
  configuration: string
  script: string
  description: string
}>({
  name: '',
  type: 'UPLINK',
  converter_mode: 'SCRIPT',
  debug_mode: false,
  configuration: '{\n  "version": "1.0"\n}',
  script: '-- ThingsBoard 兼容上行转换脚本\n-- input: payload, metadata\n-- output: telemetry, attributes\nlocal telemetry = {}\nlocal attributes = {}\n\nif payload.temperature then\n  telemetry.temp = payload.temperature\nend\nif payload.humidity then\n  telemetry.hum = payload.humidity\nend\n\nreturn {\n  telemetry = telemetry,\n  attributes = attributes\n}',
  description: ''
})

const formRules: FormRules = {
  name: [{ required: true, message: '请输入转换器名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择转换器方向', trigger: 'change' }],
  converter_mode: [{ required: true, message: '请选择编解码模式', trigger: 'change' }]
}

// 仿真测试抽屉
const testDrawerVisible = ref(false)
const testing = ref(false)
const testPayload = ref('{\n  "temperature": 25.6,\n  "humidity": 62\n}')
const testMetadata = ref('{\n  "device_name": "Sensor_01",\n  "protocol": "MQTT"\n}')
const activeTestConverter = ref<DataConverterItem | null>(null)
const testResult = ref<TestDataConverterResponse | null>(null)

const columns: DataTableColumns<DataConverterItem> = [
  {
    title: '转换器名称',
    key: 'name',
    render(row) {
      return h('span', { class: 'font-medium text-primary' }, row.name)
    }
  },
  {
    title: '传输方向',
    key: 'type',
    width: 120,
    render(row) {
      const isUplink = row.type === 'UPLINK'
      return h(
        NTag,
        { type: isUplink ? 'info' : 'warning', size: 'small', round: true },
        () => (isUplink ? '上行 (Uplink)' : '下行 (Downlink)')
      )
    }
  },
  {
    title: '转换模式',
    key: 'converter_mode',
    width: 140,
    render(row) {
      const modeMap: Record<ConverterMode, { label: string; type: 'default' | 'success' | 'info' }> = {
        SCRIPT: { label: '自定义脚本 (Script)', type: 'success' },
        HEX_BINARY: { label: '定长二进制 (Hex)', type: 'info' },
        JSON_PATH: { label: 'JSON 映射 (Path)', type: 'default' }
      }
      const meta = modeMap[row.converter_mode] || { label: row.converter_mode, type: 'default' }
      return h(NTag, { type: meta.type, size: 'small' }, () => meta.label)
    }
  },
  {
    title: '调试追踪',
    key: 'debug_mode',
    width: 100,
    render(row) {
      return h(
        NTag,
        { type: row.debug_mode ? 'success' : 'default', size: 'small' },
        () => (row.debug_mode ? '开启' : '关闭')
      )
    }
  },
  {
    title: '描述',
    key: 'description',
    ellipsis: { tooltip: true },
    render(row) {
      return row.description || '-'
    }
  },
  {
    title: '更新时间',
    key: 'updated_at',
    width: 180,
    render(row) {
      return row.updated_at ? formatDateTime(row.updated_at) : '-'
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 220,
    render(row) {
      return h(NSpace, { size: 'small' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            secondary: true,
            onClick: () => openTestDrawer(row)
          },
          () => '在线仿真'
        ),
        h(
          NButton,
          {
            size: 'small',
            secondary: true,
            onClick: () => openEditModal(row)
          },
          () => '编辑'
        ),
        h(
          NPopconfirm,
          {
            onPositiveClick: () => handleDelete(row.id)
          },
          {
            trigger: () =>
              h(NButton, { size: 'small', type: 'error', secondary: true }, () => '删除'),
            default: () => `确定删除数据转换器 "${row.name}" 吗？`
          }
        )
      ])
    }
  }
]

const loadData = async () => {
  loading.value = true
  try {
    const res = await getDataConvertersList({
      page: page.value,
      page_size: pageSize.value,
      search: searchQuery.value || undefined,
      type: selectedType.value || undefined
    })
    if (res.data) {
      list.value = res.data.list || []
      total.value = res.data.total || 0
    }
  } catch (err: any) {
    message.error(err.message || '加载转换器列表失败')
  } finally {
    loading.value = false
  }
}

const openCreateModal = () => {
  isEdit.value = false
  currentId.value = ''
  formModel.name = ''
  formModel.type = 'UPLINK'
  formModel.converter_mode = 'SCRIPT'
  formModel.debug_mode = false
  formModel.configuration = '{\n  "version": "1.0"\n}'
  formModel.script = '-- ThingsBoard 兼容上行转换脚本\n-- input: payload, metadata\n-- output: telemetry, attributes\nlocal telemetry = {}\nlocal attributes = {}\n\nif payload.temperature then\n  telemetry.temp = payload.temperature\nend\nif payload.humidity then\n  telemetry.hum = payload.humidity\nend\n\nreturn {\n  telemetry = telemetry,\n  attributes = attributes\n}'
  formModel.description = ''
  modalVisible.value = true
}

const openEditModal = (row: DataConverterItem) => {
  isEdit.value = true
  currentId.value = row.id
  formModel.name = row.name
  formModel.type = row.type
  formModel.converter_mode = row.converter_mode
  formModel.debug_mode = row.debug_mode
  formModel.configuration = row.configuration || '{\n  "version": "1.0"\n}'
  formModel.script = row.script || ''
  formModel.description = row.description || ''
  modalVisible.value = true
}

const handleSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate()

  submitting.value = true
  try {
    if (isEdit.value) {
      await updateDataConverter({
        id: currentId.value,
        name: formModel.name,
        type: formModel.type,
        converter_mode: formModel.converter_mode,
        debug_mode: formModel.debug_mode,
        configuration: formModel.configuration,
        script: formModel.script,
        description: formModel.description
      })
      message.success('更新转换器成功')
    } else {
      await createDataConverter({
        name: formModel.name,
        type: formModel.type,
        converter_mode: formModel.converter_mode,
        debug_mode: formModel.debug_mode,
        configuration: formModel.configuration,
        script: formModel.script,
        description: formModel.description
      })
      message.success('创建转换器成功')
    }
    modalVisible.value = false
    loadData()
  } catch (err: any) {
    message.error(err.message || '保存失败')
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (id: string) => {
  try {
    await deleteDataConverter(id)
    message.success('删除成功')
    loadData()
  } catch (err: any) {
    message.error(err.message || '删除失败')
  }
}

const openTestDrawer = (row: DataConverterItem) => {
  activeTestConverter.value = row
  testResult.value = null
  if (row.converter_mode === 'HEX_BINARY') {
    testPayload.value = '01030400010002'
  } else {
    testPayload.value = '{\n  "temperature": 28.5,\n  "humidity": 65,\n  "voltage": 220\n}'
  }
  testDrawerVisible.value = true
}

const runSimulation = async () => {
  if (!activeTestConverter.value) return
  testing.value = true
  testResult.value = null

  let metadataObj: Record<string, string> = {}
  try {
    if (testMetadata.value.trim()) {
      metadataObj = JSON.parse(testMetadata.value)
    }
  } catch {
    message.warning('元数据不是有效的 JSON 格式，将按空处理')
  }

  try {
    const res = await testDataConverter({
      converter_id: activeTestConverter.value.id,
      payload: testPayload.value,
      metadata: metadataObj
    })
    if (res.data) {
      testResult.value = res.data
      if (res.data.success) {
        message.success('仿真测试完成')
      } else {
        message.warning('仿真执行完成，但存在解析错误')
      }
    }
  } catch (err: any) {
    message.error(err.message || '仿真请求失败')
  } finally {
    testing.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>

<template>
  <div class="p-4 space-y-4">
    <NCard title="数据编解码转换器 (Data Converters)" size="small">
      <template #header-extra>
        <NButton type="primary" size="small" @click="openCreateModal">新建转换器</NButton>
      </template>

      <NAlert type="info" class="mb-4">
        对标 ThingsBoard Integrations 核心数据转换引擎。支持对 MQTT、HTTP、TCP 等非标准报文进行上行（Uplink）解码与下行（Downlink）编码转换，具备沙箱防护与在线仿真能力。
      </NAlert>

      <div class="flex items-center gap-4 mb-4">
        <NInput
          v-model:value="searchQuery"
          placeholder="搜索转换器名称..."
          clearable
          style="max-width: 260px"
          @update:value="loadData"
        />
        <NSelect
          v-model:value="selectedType"
          placeholder="全部传输方向"
          clearable
          :options="[
            { label: '上行 (Uplink)', value: 'UPLINK' },
            { label: '下行 (Downlink)', value: 'DOWNLINK' }
          ]"
          style="max-width: 180px"
          @update:value="loadData"
        />
        <NButton secondary size="small" @click="loadData">刷新</NButton>
      </div>

      <NDataTable
        :columns="columns"
        :data="list"
        :loading="loading"
        :pagination="{
          page,
          pageSize,
          itemCount: total,
          showSizePicker: true,
          pageSizes: [10, 20, 50],
          onChange: (p: number) => {
            page = p
            loadData()
          },
          onUpdatePageSize: (s: number) => {
            pageSize = s
            page = 1
            loadData()
          }
        }"
      />
    </NCard>

    <!-- 新建/编辑 Modal -->
    <NModal
      v-model:show="modalVisible"
      preset="card"
      :title="isEdit ? '编辑数据转换器' : '新建数据转换器'"
      style="width: 720px; max-width: 95vw"
    >
      <NForm ref="formRef" :model="formModel" :rules="formRules" label-placement="left" label-width="110">
        <NFormItem label="转换器名称" path="name">
          <NInput v-model:value="formModel.name" placeholder="例如：Modbus-TCP 温湿度解码器" />
        </NFormItem>

        <NFormItem label="传输方向" path="type">
          <NRadioGroup v-model:value="formModel.type">
            <NSpace>
              <NRadio value="UPLINK">上行解析 (Uplink Converter)</NRadio>
              <NRadio value="DOWNLINK">下行编码 (Downlink Converter)</NRadio>
            </NSpace>
          </NRadioGroup>
        </NFormItem>

        <NFormItem label="编解码模式" path="converter_mode">
          <NSelect
            v-model:value="formModel.converter_mode"
            :options="[
              { label: '自定义 Lua/JS 脚本 (SCRIPT)', value: 'SCRIPT' },
              { label: '定长二进制字节偏移 (HEX_BINARY)', value: 'HEX_BINARY' },
              { label: 'JSONPath 键值抽取 (JSON_PATH)', value: 'JSON_PATH' }
            ]"
          />
        </NFormItem>

        <NFormItem label="调试模式">
          <NSpace align="center">
            <NSwitch v-model:value="formModel.debug_mode" />
            <span class="text-xs text-gray-500">开启后将保留每一次转换的执行日志与原始载荷</span>
          </NSpace>
        </NFormItem>

        <NFormItem v-if="formModel.converter_mode === 'SCRIPT'" label="转换脚本">
          <NInput
            v-model:value="formModel.script"
            type="textarea"
            :rows="10"
            placeholder="输入转换脚本代码..."
            class="font-mono text-xs"
          />
        </NFormItem>

        <NFormItem v-else label="解析配置 (JSON)">
          <NInput
            v-model:value="formModel.configuration"
            type="textarea"
            :rows="8"
            placeholder="输入 JSON 配置结构..."
            class="font-mono text-xs"
          />
        </NFormItem>

        <NFormItem label="描述说明">
          <NInput v-model:value="formModel.description" type="textarea" placeholder="填写转换器的业务用途与支持协议" />
        </NFormItem>
      </NForm>

      <template #footer>
        <div class="flex justify-end gap-2">
          <NButton @click="modalVisible = false">取消</NButton>
          <NButton type="primary" :loading="submitting" @click="handleSubmit">保存</NButton>
        </div>
      </template>
    </NModal>

    <!-- 在线仿真测试抽屉 -->
    <NDrawer v-model:show="testDrawerVisible" :width="560">
      <NDrawerContent :title="`在线仿真调试: ${activeTestConverter?.name || ''}`">
        <div class="space-y-4">
          <NAlert type="info" size="small">
            在此输入模拟设备发送的真实报文，即时检验当前数据转换器的输出与日志追踪。
          </NAlert>

          <div>
            <div class="text-sm font-semibold mb-1">测试载荷 (Payload)</div>
            <NInput
              v-model:value="testPayload"
              type="textarea"
              :rows="5"
              placeholder="输入 16 进制字符串或 JSON 报文"
              class="font-mono text-xs"
            />
          </div>

          <div>
            <div class="text-sm font-semibold mb-1">元数据上下文 (Metadata JSON)</div>
            <NInput
              v-model:value="testMetadata"
              type="textarea"
              :rows="3"
              placeholder="{}"
              class="font-mono text-xs"
            />
          </div>

          <NButton type="primary" block :loading="testing" @click="runSimulation">
            执行仿真解析
          </NButton>

          <div v-if="testResult" class="mt-4 border rounded p-3 bg-gray-50 dark:bg-gray-800">
            <div class="text-sm font-semibold mb-2 flex items-center justify-between">
              <span>仿真结果</span>
              <NTag :type="testResult.success ? 'success' : 'error'" size="small">
                {{ testResult.success ? '执行成功' : '执行失败' }}
              </NTag>
            </div>

            <div v-if="testResult.error" class="text-red-500 text-xs font-mono mb-2">
              错误: {{ testResult.error }}
            </div>

            <div v-if="testResult.telemetry" class="mb-2">
              <span class="text-xs font-semibold text-gray-500">解析遥测 (Telemetry):</span>
              <pre class="bg-gray-100 dark:bg-black p-2 rounded text-xs overflow-x-auto">{{ JSON.stringify(testResult.telemetry, null, 2) }}</pre>
            </div>

            <div v-if="testResult.attributes" class="mb-2">
              <span class="text-xs font-semibold text-gray-500">解析属性 (Attributes):</span>
              <pre class="bg-gray-100 dark:bg-black p-2 rounded text-xs overflow-x-auto">{{ JSON.stringify(testResult.attributes, null, 2) }}</pre>
            </div>

            <div v-if="testResult.logs && testResult.logs.length" class="mt-2">
              <span class="text-xs font-semibold text-gray-500">执行追踪日志 (Logs):</span>
              <div class="bg-gray-900 text-green-400 p-2 rounded text-xs font-mono space-y-1">
                <div v-for="(log, idx) in testResult.logs" :key="idx">{{ log }}</div>
              </div>
            </div>
          </div>
        </div>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>
