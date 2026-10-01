<!--
数据清理策略组件，负责展示平台数据清理配置列表，并提供单条策略编辑入口。
核心链路：加载清理策略列表 -> 用表格展示保留天数、最近清理时间和启停状态 -> 打开弹窗修改 retention_days/enabled/remark -> 保存后重新拉取列表。
TB-15R 扩展（138.sql）：支持行级（租户/档案粒度）保留策略的添加与删除——
新增"添加行级策略"弹窗（租户必选、设备档案可选：空=租户级覆盖全部设备），
表格新增作用域列区分 全局/租户级/档案级，行级行提供删除入口（全局默认行不可删）。
静态维护重点：
1. 这里配置的是系统级数据保留策略，虽不是立即执行的删除按钮，但会影响后续自动清理行为，属于高风险运维配置。
2. 行级策略仅支持设备数据（data_type=1），创建请求固定携带 data_type='1'；重复 (租户,档案) 由后端唯一索引拒绝并回显参数错误。
3. `row` 直接回填到 editData 的方式简单直接，但后续如果字段继续增多，建议改为显式白名单映射，避免把只读列误带入提交体。
-->
<script setup lang="tsx">
import { computed, reactive, ref } from 'vue'
import type { Ref } from 'vue'
import { NButton, NEmpty, NSpace, NTag } from 'naive-ui'
import type { DataTableColumns, FormInst } from 'naive-ui'
import dayjs from 'dayjs'
import { useBoolean, useLoading } from '@aetherlink/hooks'
import { dataClearSettingEnabledTypeOptions } from '@/constants/business'
import {
  createDataClear,
  deleteDataClear,
  editDataClear,
  fetchDataClearList,
  fetchDeviceConfigOptions,
  fetchTenantOptions
} from '@/service/api/setting'
import { smartDeepClone as deepClone } from '@/utils/deep-clone'
import { $t } from '@/locales'

const { loading, startLoading, endLoading } = useLoading(false)
const { bool: visible, setTrue: openModal, setFalse: closeModal } = useBoolean()
const { bool: createVisible, setTrue: openCreateModal, setFalse: closeCreateModal } = useBoolean()

const tableData = ref<GeneralSetting.DataClearSetting[]>([])

// 表格数据只保留后端返回的最新策略列表，不在前端做额外缓存拼接。
function setTableData(data: GeneralSetting.DataClearSetting[]) {
  tableData.value = data
}

type QueryFormModel = {
  page: number
  page_size: number
}

const queryParams = reactive<QueryFormModel>({
  page: 1,
  page_size: 10
})

// 数据清理列表是整个组件的真相源，编辑成功后必须重新回读，避免保留旧的保留天数和最近执行时间。
async function getTableData() {
  startLoading()
  try {
    const { data } = await fetchDataClearList(queryParams)
    if (data) {
      const list: Api.GeneralSetting.DataClearSetting[] = data.list || []
      setTableData(list)
      return
    }
    setTableData([])
  } catch {
    setTableData([])
  } finally {
    endLoading()
  }
}

// ---- 行级策略作用域（TB-15R）：档案级(租户+档案) > 租户级(租户) > 全局 ----
type PolicyScopeKey = 'global' | 'tenant' | 'profile'

function scopeOfPolicy(row: GeneralSetting.DataClearSetting): PolicyScopeKey {
  if (row.tenant_id) {
    return row.device_config_id ? 'profile' : 'tenant'
  }
  return 'global'
}

const scopeTagTypes: Record<PolicyScopeKey, NaiveUI.ThemeColor> = {
  global: 'default',
  tenant: 'info',
  profile: 'warning'
}

function scopeTagKey(scope: PolicyScopeKey) {
  return `page.manage.setting.dataClearSetting.scope.${scope}` as const
}

// 作用域列展示：标签 + 行级细节（租户/档案 id），全局行只显示"全局"。
function renderScopeCell(row: GeneralSetting.DataClearSetting) {
  const scope = scopeOfPolicy(row)
  const detail =
    scope === 'global' ? '' : `${row.tenant_id}${row.device_config_id ? ` · ${row.device_config_id}` : ''}`
  return (
    <div class="flex-col">
      <NTag type={scopeTagTypes[scope]}>{$t(scopeTagKey(scope))}</NTag>
      {detail ? <span class="text-12px text-gray-400">{detail}</span> : null}
    </div>
  )
}

const columns: Ref<DataTableColumns<GeneralSetting.DataClearSetting>> = ref([
  {
    key: 'id',
    title: 'ID',
    align: 'center',
    width: '100px'
  },
  {
    key: 'data_type',
    title: () => $t('page.manage.setting.dataClearSetting.form.cleanupType'),
    align: 'left',
    render: (row) => {
      if (row.data_type) {
        const tagTypes: Record<GeneralSetting.CleanupTypeKey, NaiveUI.ThemeColor> = {
          '1': 'success',
          '2': 'warning'
        }
        const key =
          row.data_type === '1'
            ? 'page.manage.setting.dataClearSetting.type.equipmentData'
            : 'page.manage.setting.dataClearSetting.type.operationLog'
        return <NTag type={tagTypes[row.data_type]}>{$t(key)}</NTag>
      }
      return <span></span>
    }
  },
  {
    key: 'scope',
    title: () => $t('page.manage.setting.dataClearSetting.form.scope'),
    align: 'left',
    render: (row) => renderScopeCell(row)
  },
  {
    key: 'retention_days',
    title: () => $t('page.manage.setting.dataClearSetting.form.retentionDays'),
    align: 'left'
  },
  {
    key: 'last_cleanup_time',
    title: () => $t('page.manage.setting.dataClearSetting.form.lastCleanupTime'),
    align: 'left',
    render: (row) => {
      return <span>{dayjs(row.last_cleanup_time).format('YYYY-MM-DD HH:mm:ss')}</span>
    }
  },
  {
    key: 'last_cleanup_data_time',
    title: () => $t('page.manage.setting.dataClearSetting.form.lastCleanupDataTime'),
    align: 'left',
    render: (row) => {
      return <span>{dayjs(row.last_cleanup_data_time).format('YYYY-MM-DD HH:mm:ss')}</span>
    }
  },
  {
    key: 'remark',
    title: () => $t('common.remark'),
    align: 'left'
  },
  {
    key: 'actions',
    title: () => $t('common.actions'),
    align: 'center',
    width: '140px',
    render: (row) => {
      return (
        <NSpace justify={'center'}>
          <NButton size={'small'} type="primary" onClick={() => handleEditTable(row)}>
            {$t('common.edit')}
          </NButton>
          {row.tenant_id ? (
            <NButton size={'small'} type="error" onClick={() => handleDeletePolicy(row)}>
              {$t('common.delete')}
            </NButton>
          ) : null}
        </NSpace>
      )
    }
  }
]) as Ref<DataTableColumns<GeneralSetting.DataClearSetting>>

const formRef = ref<HTMLElement & FormInst>()

type FormModel = Pick<GeneralSetting.DataClearSetting, 'retention_days' | 'enabled' | 'remark'>

const editData = reactive<FormModel>(createDefaultFormModel())

function createDefaultFormModel(): FormModel {
  return {
    retention_days: 0,
    enabled: '1',
    remark: null
  }
}

// 编辑弹窗直接回填当前行策略数据，保持“所见即所改”的后台配置体验。
function setEditData(data: GeneralSetting.DataClearSetting | null) {
  Object.assign(editData, data)
}

function handleEditTable(row: any) {
  setEditData(row)
  openModal()
}

// 提交时复用当前编辑模型，保存成功后统一回刷列表，而不是本地乐观改表格。
async function handleSubmit() {
  await formRef.value?.validate()
  try {
    const formData = deepClone(editData)
    const data: any = await editDataClear(formData)
    if (!data.error) {
      window.$message?.success(data.msg)
      await getTableData()
    }
  } catch {
    // request layer already surfaces the user-facing error message
  }
  closeModal()
}

// ---- 行级策略创建（TB-15R） ----
type CreateFormModel = {
  data_type: string
  tenant_id: string | null
  device_config_id: string | null
  retention_days: number
  enabled: string
  remark: string | null
}

function createDefaultCreateFormModel(): CreateFormModel {
  return {
    data_type: '1',
    tenant_id: null,
    device_config_id: null,
    retention_days: 30,
    enabled: '1',
    remark: null
  }
}

const createForm = reactive<CreateFormModel>(createDefaultCreateFormModel())
const createFormRef = ref<HTMLElement & FormInst>()

const tenantOptions = ref<{ id: string; name: string }[]>([])
const deviceConfigOptions = ref<{ id: string; name: string; tenant_id: string }[]>([])

// 档案下拉按所选租户过滤（后端 device_config 列表返回各行 tenant_id）。
const deviceConfigOptionsForTenant = computed(() =>
  deviceConfigOptions.value.filter(item => item.tenant_id === createForm.tenant_id)
)

// 打开创建弹窗时一次性拉取租户与设备档案选项（管理面数量级有限，分页拉大即可）。
async function loadPolicyScopeOptions() {
  try {
    const [tenantRes, configRes] = await Promise.all([
      fetchTenantOptions({ page: 1, page_size: 500 }),
      fetchDeviceConfigOptions({ page: 1, page_size: 500 })
    ])
    tenantOptions.value = (tenantRes?.data?.list || []).map((item: any) => ({
      id: item.id,
      name: item.name
    }))
    deviceConfigOptions.value = (configRes?.data?.list || []).map((item: any) => ({
      id: item.id,
      name: item.name,
      tenant_id: item.tenant_id
    }))
  } catch {
    // request layer already surfaces the user-facing error message
  }
}

async function handleOpenCreate() {
  openCreateModal()
  await loadPolicyScopeOptions()
}

// 切换租户后重置档案选择，避免残留其他租户的档案 id。
function handleCreateTenantChange() {
  createForm.device_config_id = null
}

// 创建行级策略：data_type 固定设备数据（后端拒绝其它类型的行级行）。
async function handleCreateSubmit() {
  if (!createForm.tenant_id) {
    window.$message?.error($t('page.manage.setting.dataClearSetting.message.tenantRequired'))
    return
  }
  try {
    const data: any = await createDataClear({
      data_type: '1',
      tenant_id: createForm.tenant_id,
      device_config_id: createForm.device_config_id || null,
      retention_days: createForm.retention_days,
      enabled: createForm.enabled,
      remark: createForm.remark
    })
    if (!data.error) {
      window.$message?.success(data.msg)
      closeCreateModal()
      await getTableData()
    }
  } catch {
    // request layer already surfaces the user-facing error message
  }
}

// 删除行级策略：全局默认行不提供入口（后端同样拒绝，双重防线）。
async function handleDeletePolicy(row: GeneralSetting.DataClearSetting) {
  window.$dialog?.warning({
    title: $t('common.delete'),
    content: row.remark || row.id,
    positiveText: $t('common.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: async () => {
      try {
        const data: any = await deleteDataClear(row.id)
        if (!data.error) {
          window.$message?.success(data.msg)
          await getTableData()
        }
      } catch {
        // request layer already surfaces the user-facing error message
      }
    }
  })
}

function init() {
  getTableData()
}

init()
</script>

<template>
  <div class="h-full flex-col">
    <div class="mb-12px flex justify-end">
      <NButton type="primary" @click="handleOpenCreate">
        {{ $t('page.manage.setting.dataClearSetting.action.addPolicy') }}
      </NButton>
    </div>
    <NDataTable :columns="columns" :data="tableData" :loading="loading" flex-height min-height="150px">
      <template #empty>
        <NEmpty :description="$t('common.noData')" class="py-24px" />
      </template>
    </NDataTable>

    <NModal
      v-model:show="visible"
      preset="card"
      :title="$t('common.edit')"
      :aria-label="$t('common.edit')"
      class="w-700px"
    >
      <NForm ref="formRef" label-placement="left" :label-width="120" :model="editData">
        <NGrid :cols="24" :x-gap="18">
          <NFormItemGridItem :span="24" :label="$t('page.manage.setting.dataClearSetting.form.retentionDays')">
            <NInputNumber v-model:value="editData.retention_days" class="flex-1" />
          </NFormItemGridItem>
          <NFormItemGridItem :span="24" :label="$t('page.manage.setting.dataClearSetting.form.enabled')" path="enabled">
            <NRadioGroup v-model:value="editData.enabled">
              <NRadio v-for="item in dataClearSettingEnabledTypeOptions" :key="item.value" :value="item.value">
                {{ item.label }}
              </NRadio>
            </NRadioGroup>
          </NFormItemGridItem>
          <NFormItemGridItem :span="24" :label="$t('common.remark')">
            <NInput v-model:value="editData.remark" type="textarea" />
          </NFormItemGridItem>
        </NGrid>
        <NSpace class="w-full pt-16px" :size="24" justify="center">
          <NButton class="w-72px" type="primary" @click="handleSubmit">{{ $t('common.edit') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>

    <NModal
      v-model:show="createVisible"
      preset="card"
      :title="$t('page.manage.setting.dataClearSetting.action.addPolicy')"
      :aria-label="$t('page.manage.setting.dataClearSetting.action.addPolicy')"
      class="w-700px"
    >
      <NForm ref="createFormRef" label-placement="left" :label-width="120" :model="createForm">
        <NGrid :cols="24" :x-gap="18">
          <NFormItemGridItem :span="24" :label="$t('page.manage.setting.dataClearSetting.form.tenant')" required>
            <NSelect
              v-model:value="createForm.tenant_id"
              :options="tenantOptions.map(item => ({ label: item.name, value: item.id }))"
              :placeholder="$t('page.manage.setting.dataClearSetting.form.tenant')"
              filterable
              clearable
              @update:value="handleCreateTenantChange"
            />
          </NFormItemGridItem>
          <NFormItemGridItem :span="24" :label="$t('page.manage.setting.dataClearSetting.form.deviceConfig')">
            <NSelect
              v-model:value="createForm.device_config_id"
              :options="deviceConfigOptionsForTenant.map(item => ({ label: item.name, value: item.id }))"
              :placeholder="$t('page.manage.setting.dataClearSetting.form.deviceConfigAll')"
              filterable
              clearable
              :disabled="!createForm.tenant_id"
            />
          </NFormItemGridItem>
          <NFormItemGridItem :span="24" :label="$t('page.manage.setting.dataClearSetting.form.retentionDays')">
            <NInputNumber v-model:value="createForm.retention_days" class="flex-1" :min="1" />
          </NFormItemGridItem>
          <NFormItemGridItem :span="24" :label="$t('page.manage.setting.dataClearSetting.form.enabled')">
            <NRadioGroup v-model:value="createForm.enabled">
              <NRadio v-for="item in dataClearSettingEnabledTypeOptions" :key="item.value" :value="item.value">
                {{ item.label }}
              </NRadio>
            </NRadioGroup>
          </NFormItemGridItem>
          <NFormItemGridItem :span="24" :label="$t('common.remark')">
            <NInput v-model:value="createForm.remark" type="textarea" />
          </NFormItemGridItem>
        </NGrid>
        <NSpace class="w-full pt-16px" :size="24" justify="center">
          <NButton class="w-72px" type="primary" @click="handleCreateSubmit">{{ $t('common.confirm') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>

<style lang="scss"></style>
