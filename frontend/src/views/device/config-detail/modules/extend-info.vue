<!--
  文件名称: extend-info.vue
  文件用途: 设备配置详情页中的扩展信息面板，负责展示、编辑、启停和删除 additional_info 扩展字段。
  状态主线:
  1. onMounted 时从 props.configInfo.additional_info 解析出 extendInfoList，作为表格与弹窗的共享数据源。
  2. 新增/编辑时通过 visible、isEdit、editIndex、extendForm 维护弹窗状态与当前操作对象。
  3. 保存、开关、删除最终都会收敛到 handleSave，将 extendInfoList 序列化后回写到设备配置接口。
  关键注意事项:
  1. additional_info 来自后端 JSON 字段，可能为空串、'{}' 或非数组字符串，解析时必须兜底。
  2. 当前文件直接基于 props.configInfo 组装保存参数，注释补充时需明确“表单态 -> 列表态 -> 接口态”的串联关系。
  3. 编辑态直接复用行对象引用，阅读代码时要留意弹窗修改会先作用到当前列表对象。
-->
<script setup lang="tsx">
import type { Ref } from 'vue'
import { computed, getCurrentInstance, onMounted, ref } from 'vue'
import type { DataTableColumns, FormInst } from 'naive-ui'
import { NButton, NPopconfirm, NSpace, NSwitch, useMessage } from 'naive-ui'
import { deviceConfigEdit } from '@/service/api/device'
import { $t } from '@/locales'

// 弹窗显示态 + 编辑态索引，共同决定当前是“新增扩展项”还是“编辑已有扩展项”。
const visible = ref(false)
const isEdit = ref(false)
const editIndex = ref(-1)
const extendFormRef = ref<HTMLElement & FormInst>()
// 弹窗表单态：新增/编辑都先落在这里，再提交回 extendInfoList。
const extendForm = ref(defaultExtendForm())
const message = useMessage()

interface Emits {
  (e: 'upDateConfig'): void
}

const emit = defineEmits<Emits>()

interface Props {
  configInfo?: object | any
}

const props = withDefaults(defineProps<Props>(), {
  configInfo: null
})

function defaultExtendForm() {
  return {
    name: null,
    type: null,
    default_value: null,
    desc: null,
    enable: false
  }
}

const extendFormRules = ref({
  name: {
    required: true,
    message: $t('common.enterName'),
    trigger: 'blur'
  },
  type: {
    required: true,
    message: $t('generate.select-type'),
    trigger: 'change'
  }
})
// 表格主数据源，也是最终写回 additional_info 的唯一来源。
const extendInfoList = ref([] as any[])
const typeOptions = ref([
  {
    label: 'String',
    value: 'String'
  },
  {
    label: 'Number',
    value: 'Number'
  },
  {
    label: 'Boolean',
    value: 'Boolean'
  }
])

const parseExtensionInfo = (value: unknown): any[] => {
  if (!value || value === '{}') return []
  if (Array.isArray(value)) return value

  try {
    const parsed = JSON.parse(String(value))
    return Array.isArray(parsed) ? parsed : []
  } catch {
    // 查询链路兜底：
    // 后端 additional_info 是用户可扩展 JSON，解析失败时不阻断页面，
    // 直接回退为空数组，允许用户重新保存覆盖异常值。
    return []
  }
}

const addDevice = () => {
  visible.value = true
}
const modalClose = () => {}
const handleClose = () => {
  extendFormRef.value?.restoreValidation()
  extendForm.value = defaultExtendForm()
  visible.value = false
  isEdit.value = false
  editIndex.value = -1
}

const handleSave = async () => {
  // 保存链路收口：
  // 1. 先把当前 extendInfoList 序列化回 additional_info
  // 2. 再调用整份配置的编辑接口
  // 3. 成功后通知父层刷新配置详情，最后统一关闭弹窗/重置状态
  const postData = props.configInfo
  postData.additional_info = JSON.stringify(extendInfoList.value)
  const res = await deviceConfigEdit(postData)
  if (!res.error) {
    message.success($t('common.modifySuccess'))
    emit('upDateConfig')
  }
  handleClose()
}

const handleSubmit = async () => {
  await extendFormRef?.value?.validate()
  // 弹窗提交链路：
  // 编辑态按 editIndex 覆盖原项；新增态先补默认 enable，再追加到列表，
  // 最终统一交给 handleSave 做接口持久化。
  if (editIndex.value >= 0) {
    extendInfoList.value[editIndex.value] = extendForm.value
  } else {
    extendForm.value.enable = false
    extendInfoList.value.push(extendForm.value)
  }
  handleSave()
}

const handleSwitchChange = async (row) => {
  // 表格中的启停操作不直接请求单字段接口，而是先修改列表项，再复用保存链路整体提交。
  const index = (extendInfoList.value || []).findIndex((item) => {
    return (
      item.name === row.name &&
      item.type === row.type &&
      item.default_value === row.default_value &&
      item.desc === row.desc
    )
  })
  if (index >= 0) {
    extendInfoList.value[index].enable = !extendInfoList.value[index].enable
    handleSave()
  }
}

const handleDeleteTable = async (row) => {
  // 删除链路与启停保持一致：定位目标项 -> 更新 extendInfoList -> 统一保存。
  const index = (extendInfoList.value || []).findIndex((item) => {
    return (
      item.name === row.name &&
      item.type === row.type &&
      item.default_value === row.default_value &&
      item.desc === row.desc
    )
  })
  if (index >= 0) {
    extendInfoList.value.splice(index, 1)
    handleSave()
  }
  window.$message?.info($t('common.extensionInfoDeleted'))
}
const handleEditTable = async (row) => {
  // 选择编辑链路：
  // 先根据行内容回查索引，再把行对象放入表单态并打开弹窗。
  editIndex.value = (extendInfoList.value || []).findIndex((item) => {
    return (
      item.name === row.name &&
      item.type === row.type &&
      item.default_value === row.default_value &&
      item.desc === row.desc
    )
  })

  extendForm.value = row
  isEdit.value = true
  visible.value = true
}

const columns: Ref<DataTableColumns<ServiceManagement.Service>> = ref([
  {
    key: 'name',
    title: $t('page.manage.menu.form.name'),
    minWidth: '140px',
    align: 'left'
  },
  {
    key: 'type',
    minWidth: '140px',
    title: $t('page.manage.menu.form.type'),
    align: 'left'
  },
  {
    key: 'default_value',
    title: $t('generate.default-value'),
    minWidth: '140px',
    align: 'left'
  },
  {
    key: 'desc',
    title: $t('custom.groupPage.description'),
    minWidth: '140px',
    align: 'left'
  },
  {
    key: 'enable',
    minWidth: '140px',
    title: $t('page.manage.common.status.enable'),
    align: 'left',
    render: (row: any) => {
      return <NSwitch value={Boolean(row.enable)} onChange={() => handleSwitchChange(row)} />
    }
  },
  {
    key: 'operate',
    minWidth: '140px',
    title: $t('common.actions'),
    align: 'left',
    render: (row: any) => {
      return (
        <NSpace>
          <NButton size={'small'} type="primary" onClick={() => handleEditTable(row)}>
            {$t('common.edit')}
          </NButton>
          <NPopconfirm onPositiveClick={() => handleDeleteTable(row)}>
            {{
              default: () => $t('common.confirmDelete'),
              trigger: () => (
                <NButton type="error" size={'small'}>
                  {$t('common.delete')}
                </NButton>
              )
            }}
          </NPopconfirm>
        </NSpace>
      )
    }
  }
])
const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
onMounted(() => {
  // 初始查询链路：
  // 当前组件不单独发请求，而是消费父层传入的 configInfo，
  // 在挂载时把 additional_info 解析成可编辑列表。
  extendInfoList.value = parseExtensionInfo(props.configInfo.additional_info)
})
</script>

<template>
  <div class="extend-box">
    <NButton type="primary" @click="addDevice()">{{ $t('generate.add-extension-info') }}</NButton>
    <NDataTable :columns="columns" :data="extendInfoList" size="small" class="m-tb-10">
      <template #empty>
        <n-empty :description="$t('common.noData')" />
      </template>
    </NDataTable>
    <!--    <div class="pagination-box">-->
    <!--      &lt;!&ndash; Data table to display device groups &ndash;&gt;-->
    <!--      &lt;!&ndash; Pagination component &ndash;&gt;-->
    <!--      <NPagination v-model:page="associatedQuery.page" :item-count="associatedTotal" @update:page="getTableData"  />-->
    <!--    </div>-->
    <NModal
      v-model:show="visible"
      :mask-closable="false"
      :title="isEdit ? $t('common.editExtendedInfo') : $t('common.addExtendedInfo')"
      :class="getPlatform ? 'w-90%' : 'w-400px'"
      preset="card"
      @after-leave="modalClose"
    >
      <NForm ref="extendFormRef" :model="extendForm" :rules="extendFormRules" label-placement="left" label-width="auto">
        <NFormItem :label="$t('page.manage.menu.form.name')" path="name">
          <NInput v-model:value="extendForm.name" :placeholder="$t('generate.enter-device-name')" />
        </NFormItem>
        <NFormItem :label="$t('generate.type')" path="type">
          <NSelect
            v-model:value="extendForm.type"
            :options="typeOptions"
            :placeholder="$t('generate.select-type')"
          ></NSelect>
        </NFormItem>
        <NFormItem :label="$t('generate.default-value')" path="default_value">
          <NInput v-model:value="extendForm.default_value" :placeholder="$t('generate.enter-default-value')" />
        </NFormItem>
        <NFormItem :label="$t('device_template.table_header.description')" path="device_ids">
          <NInput v-model:value="extendForm.desc" :placeholder="$t('generate.enter-description')" type="textarea" />
        </NFormItem>
        <NFlex justify="flex-end">
          <NButton @click="handleClose">{{ $t('generate.cancel') }}</NButton>
          <NButton type="primary" @click="handleSubmit">{{ $t('page.login.common.confirm') }}</NButton>
        </NFlex>
      </NForm>
    </NModal>
  </div>
</template>

<style scoped lang="scss">
.extend-box {
  .pagination-box {
    display: flex;
    justify-content: flex-end;
  }

  .m-tb-10 {
    margin: 10px;
  }
}
</style>
