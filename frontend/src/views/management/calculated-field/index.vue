<!--
计算字段管理页（系统设置 tab 内嵌）：从设备遥测按表达式派生新遥测键。
核心链路：分页拉取当前租户的计算字段 → 表格展示/启停开关 → 新建编辑弹窗提交到 /calculated_fields。
静态维护重点：
1. output_key 前后端双重校验：字母开头，仅字母/数字/下划线，与后端正则保持一致。
2. 模板下拉复用 device/template 分页接口；列内模板名依赖已加载选项做 id→name 映射，缺失时回退显示 id。
3. 表达式合法性以后端 govaluate 解析为准，前端只做非空拦截；解析失败会回显后端 100002 提示。
-->
<script setup lang="tsx">
import { computed, reactive, ref } from 'vue'
import { NButton, NPopconfirm, NSpace, NSwitch, NTag, NText } from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules, PaginationProps } from 'naive-ui'
import dayjs from 'dayjs'
import { deviceTemplate } from '@/service/api/device-template-model'
import type { DeviceTemplateOption } from './types'
import {
  createCalculatedField,
  deleteCalculatedField,
  getCalculatedFields,
  toggleCalculatedField,
  updateCalculatedField
} from '@/service/api/calculated_field'
import type {
  CalculatedFieldRow,
  CalculatedFieldUpsertParams
} from '@/service/api/calculated_field'
import { $t } from '@/locales'
import { useLoading } from '~/packages/hooks'

const { loading, startLoading, endLoading } = useLoading(false)
const saving = ref(false)
const actingId = ref('')
const tableData = ref<CalculatedFieldRow[]>([])

const outputKeyPattern = /^[a-zA-Z][a-zA-Z0-9_]*$/

type ListResponse = Awaited<ReturnType<typeof getCalculatedFields>>

function extractListResult(response: ListResponse) {
  const data = response?.data
  return {
    list: Array.isArray(data?.list) ? data.list : ([] as CalculatedFieldRow[]),
    total: Number(data?.total ?? 0)
  }
}

const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  onChange: page => {
    pagination.page = page
    void getTableData()
  },
  onUpdatePageSize: pageSize => {
    pagination.pageSize = pageSize
    pagination.page = 1
    void getTableData()
  }
})

// ===== 设备模板下拉与名称映射 =====
const templateOptions = ref<DeviceTemplateOption[]>([])
const templateLoading = ref(false)

const templateNameMap = computed(() => new Map(templateOptions.value.map(option => [option.id, option.name])))

async function loadTemplateOptions() {
  templateLoading.value = true
  try {
    const response = await deviceTemplate({ page: 1, page_size: 200 })
    const list = Array.isArray(response.data?.list) ? (response.data.list as DeviceTemplateOption[]) : []
    templateOptions.value = list.filter(option => Boolean(option?.id))
  } catch {
    templateOptions.value = []
  } finally {
    templateLoading.value = false
  }
}

function templateName(templateId: string) {
  return templateNameMap.value.get(templateId) || templateId
}

function formatTime(value?: string | number | null) {
  if (!value) return '-'
  const time = dayjs(value)
  return time.isValid() ? time.format('YYYY-MM-DD HH:mm:ss') : String(value)
}

// ===== 列表查询 =====
async function getTableData() {
  startLoading()
  try {
    const response = await getCalculatedFields({
      page: pagination.page ?? 1,
      page_size: pagination.pageSize ?? 10
    })
    const result = extractListResult(response)
    tableData.value = result.list
    pagination.itemCount = result.total
  } catch {
    tableData.value = []
    pagination.itemCount = 0
  } finally {
    endLoading()
  }
}

// ===== 启停开关 =====
async function handleToggle(row: CalculatedFieldRow, nextEnabled: boolean) {
  actingId.value = row.id
  try {
    await toggleCalculatedField(row.id, nextEnabled)
    row.enabled = nextEnabled
    window.$message?.success($t('custom.management.calcField.statusUpdated'))
  } finally {
    actingId.value = ''
  }
}

// ===== 删除 =====
async function handleDelete(row: CalculatedFieldRow) {
  actingId.value = row.id
  try {
    await deleteCalculatedField(row.id)
    window.$message?.success($t('custom.management.calcField.deleteSuccess'))
    await getTableData()
  } finally {
    actingId.value = ''
  }
}

// ===== 弹窗状态 =====
const modalVisible = ref(false)
const editingId = ref('')
const formRef = ref<FormInst | null>(null)

type CalcFieldFormModel = {
  name: string
  device_template_id: string
  output_key: string
  expression: string
  remark: string
}

const formModel = reactive<CalcFieldFormModel>({
  name: '',
  device_template_id: '',
  output_key: '',
  expression: '',
  remark: ''
})

const formRules: FormRules = {
  name: [{ required: true, message: $t('custom.management.calcField.nameRequired'), trigger: 'blur' }],
  device_template_id: [
    { required: true, message: $t('custom.management.calcField.templateRequired'), trigger: 'change' }
  ],
  output_key: [
    { required: true, message: $t('custom.management.calcField.outputKeyRequired'), trigger: 'blur' },
    {
      validator: (_rule, value: string) => outputKeyPattern.test(value),
      message: $t('custom.management.calcField.outputKeyRequired'),
      trigger: 'blur'
    }
  ],
  expression: [{ required: true, message: $t('custom.management.calcField.expressionRequired'), trigger: 'blur' }]
}

const modalTitle = computed(() =>
  editingId.value ? $t('custom.management.calcField.edit') : $t('custom.management.calcField.add')
)

function openCreateModal() {
  editingId.value = ''
  formModel.name = ''
  formModel.device_template_id = ''
  formModel.output_key = ''
  formModel.expression = ''
  formModel.remark = ''
  modalVisible.value = true
}

function openEditModal(row: CalculatedFieldRow) {
  editingId.value = row.id
  formModel.name = row.name
  formModel.device_template_id = row.device_template_id
  formModel.output_key = row.output_key
  formModel.expression = row.expression
  formModel.remark = row.remark || ''
  modalVisible.value = true
}

function buildUpsertParams(): CalculatedFieldUpsertParams {
  return {
    name: formModel.name.trim(),
    device_template_id: formModel.device_template_id,
    output_key: formModel.output_key.trim(),
    expression: formModel.expression,
    remark: formModel.remark.trim() ? formModel.remark.trim() : null
  }
}

async function handleSubmit() {
  await formRef.value?.validate()
  saving.value = true
  try {
    const params = buildUpsertParams()
    if (editingId.value) {
      await updateCalculatedField(editingId.value, params)
    } else {
      await createCalculatedField(params)
    }
    window.$message?.success($t('custom.management.calcField.saveSuccess'))
    modalVisible.value = false
    await getTableData()
  } catch {
    // 参数错误(100002)/不存在(100404)提示由全局请求拦截器统一弹出。
  } finally {
    saving.value = false
  }
}

const columns = computed<DataTableColumns<CalculatedFieldRow>>(() => [
  {
    key: 'name',
    title: $t('custom.management.calcField.name'),
    minWidth: 140,
    ellipsis: { tooltip: true }
  },
  {
    key: 'device_template_id',
    title: $t('custom.management.calcField.template'),
    minWidth: 150,
    ellipsis: { tooltip: true },
    render: row => <NText>{templateName(row.device_template_id)}</NText>
  },
  {
    key: 'output_key',
    title: $t('custom.management.calcField.outputKey'),
    minWidth: 130,
    render: row => <NTag size="small">{row.output_key}</NTag>
  },
  {
    key: 'expression',
    title: $t('custom.management.calcField.expression'),
    minWidth: 200,
    ellipsis: { tooltip: true },
    render: row => (
      <NText code>
        {row.expression}
      </NText>
    )
  },
  {
    key: 'enabled',
    title: $t('custom.management.calcField.enabled'),
    width: 90,
    render: row => (
      <NSwitch
        size="small"
        value={row.enabled}
        loading={actingId.value === row.id}
        onUpdateValue={value => handleToggle(row, value)}
      />
    )
  },
  {
    key: 'updated_at',
    title: $t('custom.management.calcField.updatedAt'),
    minWidth: 170,
    render: row => formatTime(row.updated_at)
  },
  {
    key: 'actions',
    title: $t('custom.management.calcField.actions'),
    width: 160,
    fixed: 'right',
    render: row => (
      <NSpace size={8}>
        <NButton size="small" onClick={() => openEditModal(row)}>
          {$t('common.edit')}
        </NButton>
        <NPopconfirm
          negative-text={$t('common.cancel')}
          positive-text={$t('common.confirm')}
          onPositiveClick={() => handleDelete(row)}
        >
          {{
            default: () => $t('custom.management.calcField.confirmDelete'),
            trigger: () => (
              <NButton size="small" type="error" ghost loading={actingId.value === row.id}>
                {$t('common.delete')}
              </NButton>
            )
          }}
        </NPopconfirm>
      </NSpace>
    )
  }
])

void loadTemplateOptions()
void getTableData()
</script>

<template>
  <div class="h-full flex-col gap-12px">
    <NSpace justify="end" align="center">
      <NButton :loading="loading" @click="getTableData">
        {{ $t('custom.management.calcField.refresh') }}
      </NButton>
      <NButton type="primary" @click="openCreateModal">
        {{ $t('custom.management.calcField.add') }}
      </NButton>
    </NSpace>

    <NDataTable
      remote
      :columns="columns"
      :data="tableData"
      :loading="loading"
      :pagination="pagination"
      :scroll-x="1100"
      flex-height
      min-height="360px"
    />

    <NModal v-model:show="modalVisible" preset="card" :title="modalTitle" class="w-560px">
      <NForm ref="formRef" :model="formModel" :rules="formRules" label-placement="left" :label-width="90">
        <NFormItem :label="$t('custom.management.calcField.name')" path="name">
          <NInput v-model:value="formModel.name" :maxlength="128" />
        </NFormItem>
        <NFormItem :label="$t('custom.management.calcField.template')" path="device_template_id">
          <NSelect
            v-model:value="formModel.device_template_id"
            :options="templateOptions.map(option => ({ label: option.name, value: option.id }))"
            :loading="templateLoading"
            filterable
            clearable
          />
        </NFormItem>
        <NFormItem :label="$t('custom.management.calcField.outputKey')" path="output_key">
          <NInput v-model:value="formModel.output_key" :maxlength="128" placeholder="power_w" />
        </NFormItem>
        <NFormItem :label="$t('custom.management.calcField.expression')" path="expression">
          <NInput
            v-model:value="formModel.expression"
            type="textarea"
            :rows="3"
            placeholder="(voltage * current) / 1000"
          />
        </NFormItem>
        <NFormItem :label="$t('custom.management.calcField.remark')" path="remark">
          <NInput v-model:value="formModel.remark" type="textarea" :rows="2" :maxlength="500" />
        </NFormItem>
        <NSpace justify="end">
          <NButton @click="modalVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="saving" @click="handleSubmit">
            {{ $t('common.confirm') }}
          </NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>

<style lang="scss"></style>
