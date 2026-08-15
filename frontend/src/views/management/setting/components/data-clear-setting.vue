<!--
数据清理策略组件，负责展示平台数据清理配置列表，并提供单条策略编辑入口。
核心链路：加载清理策略列表 -> 用表格展示保留天数、最近清理时间和启停状态 -> 打开弹窗修改 retention_days/enabled/remark -> 保存后重新拉取列表。
静态维护重点：
1. 这里配置的是系统级数据保留策略，虽不是立即执行的删除按钮，但会影响后续自动清理行为，属于高风险运维配置。
2. 当前页面只支持编辑已有策略，不支持新增或删除；若后续扩展为多策略管理，建议优先抽离表格列与编辑表单模型。
3. `row` 直接回填到 editData 的方式简单直接，但后续如果字段继续增多，建议改为显式白名单映射，避免把只读列误带入提交体。
-->
<script setup lang="tsx">
import { reactive, ref } from 'vue'
import type { Ref } from 'vue'
import { NButton, NSpace, NTag } from 'naive-ui'
import type { DataTableColumns, FormInst } from 'naive-ui'
import dayjs from 'dayjs'
import { useBoolean, useLoading } from '@aetherlink/hooks'
import { dataClearSettingEnabledTypeOptions } from '@/constants/business'
import { editDataClear, fetchDataClearList } from '@/service/api/setting'
import { smartDeepClone as deepClone } from '@/utils/deep-clone'
import { $t } from '@/locales'

const { loading, startLoading, endLoading } = useLoading(false)
const { bool: visible, setTrue: openModal, setFalse: closeModal } = useBoolean()

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
    render: row => {
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
    key: 'retention_days',
    title: () => $t('page.manage.setting.dataClearSetting.form.retentionDays'),
    align: 'left'
  },
  {
    key: 'last_cleanup_time',
    title: () => $t('page.manage.setting.dataClearSetting.form.lastCleanupTime'),
    align: 'left',
    render: row => {
      return <span>{dayjs(row.last_cleanup_time).format('YYYY-MM-DD HH:mm:ss')}</span>
    }
  },
  {
    key: 'last_cleanup_data_time',
    title: () => $t('page.manage.setting.dataClearSetting.form.lastCleanupDataTime'),
    align: 'left',
    render: row => {
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
    width: '100px',
    render: row => {
      return (
        <NSpace justify={'center'}>
          <NButton size={'small'} type="primary" onClick={() => handleEditTable(row)}>
            {$t('common.edit')}
          </NButton>
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

function init() {
  getTableData()
}

init()
</script>

<template>
  <div class="h-full flex-col">
    <NDataTable :columns="columns" :data="tableData" :loading="loading" flex-height min-height="150px" />

    <NModal v-model:show="visible" preset="card" :title="$t('common.edit')" class="w-700px">
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
  </div>
</template>

<style lang="scss"></style>
