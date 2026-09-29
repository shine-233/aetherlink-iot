<!--
文件用途：Customer（ThingsBoard 对标客户实体）管理页——客户档案增删改查与名下设备分配。
核心逻辑：
1. 客户列表分页检索（按名称模糊搜索）；
2. 客户创建/编辑表单（联系信息与地址）；
3. 设备分配工作台：分配即移动（一台设备只属一个客户），支持解绑。
-->
<script setup lang="tsx">
import { onMounted, reactive, ref } from 'vue'
import {
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
  NSelect,
  NSpace,
  NTag,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules, SelectOption } from 'naive-ui'
import {
  assignCustomerDevices,
  deleteCustomer,
  getCustomerDevices,
  getCustomersList,
  getDeviceListForSelect,
  saveCustomer,
  unassignCustomerDevice,
  type CustomerItem
} from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'

const message = useMessage()
const searchValue = ref('')

// 分页（含每页条数变化后回拉）、加载态与过期请求丢弃由 useListPage 统一处理。
const {
  rows: customers,
  loading,
  pagination,
  load: fetchCustomers,
  search: handleSearch
} = useListPage<CustomerItem, { search?: string }>({
  pageSizes: [10, 20, 50],
  serialize: () => ({ search: searchValue.value || undefined }),
  fetcher: async (params) => fromFlatResponse<CustomerItem>(await getCustomersList(params))
})

const modalVisible = ref(false)
const submitting = ref(false)
const editingId = ref('')
const formRef = ref<FormInst | null>(null)
const formModel = reactive({
  name: '',
  phone: '',
  email: '',
  country: '',
  state: '',
  city: '',
  address: '',
  address2: '',
  zip: ''
})

const formRules: FormRules = {
  name: { required: true, message: $t('page.customer.nameRequired'), trigger: 'blur' }
}

// 设备分配抽屉
const assignVisible = ref(false)
const assignCustomer = ref<CustomerItem | null>(null)
const assignedDeviceIds = ref<string[]>([])
const assignSelectedIds = ref<string[]>([])
const deviceOptions = ref<SelectOption[]>([])
const assignSubmitting = ref(false)

/** 设备 ID -> 展示名：优先用选择器选项名，选项未加载时回退为 ID。 */
const deviceLabel = (deviceId: string) => {
  const option = deviceOptions.value.find(item => item.value === deviceId)
  return (option?.label as string) || deviceId
}

const columns: DataTableColumns<CustomerItem> = [
  { title: $t('page.customer.name'), key: 'name', minWidth: 140 },
  { title: $t('page.customer.phone'), key: 'phone', minWidth: 120 },
  { title: $t('page.customer.email'), key: 'email', minWidth: 160 },
  {
    title: $t('page.customer.address'),
    key: 'address',
    minWidth: 180,
    render: (row) => [row.country, row.state, row.city, row.address].filter(Boolean).join(' / ') || '-'
  },
  {
    title: $t('page.customer.createdAt'),
    key: 'created_at',
    width: 170,
    render: (row) => (row.created_at ? formatDateTime(row.created_at) : '-')
  },
  {
    title: $t('page.customer.actions'),
    key: 'actions',
    width: 260,
    render: (row) => (
      <NSpace>
        <NButton size="small" onClick={() => openEdit(row)}>
          {$t('common.edit')}
        </NButton>
        <NButton size="small" type="primary" ghost onClick={() => openAssign(row)}>
          {$t('page.customer.assignDevices')}
        </NButton>
        <NPopconfirm onPositiveClick={() => handleDelete(row)}>
          {{
            trigger: () => (
              <NButton size="small" type="error" ghost>
                {$t('common.delete')}
              </NButton>
            ),
            default: () => $t('page.customer.deleteConfirm')
          }}
        </NPopconfirm>
      </NSpace>
    )
  }
]

const openCreate = () => {
  editingId.value = ''
  Object.assign(formModel, {
    name: '',
    phone: '',
    email: '',
    country: '',
    state: '',
    city: '',
    address: '',
    address2: '',
    zip: ''
  })
  modalVisible.value = true
}

const openEdit = (row: CustomerItem) => {
  editingId.value = row.id
  Object.assign(formModel, {
    name: row.name,
    phone: row.phone ?? '',
    email: row.email ?? '',
    country: row.country ?? '',
    state: row.state ?? '',
    city: row.city ?? '',
    address: row.address ?? '',
    address2: row.address2 ?? '',
    zip: row.zip ?? ''
  })
  modalVisible.value = true
}

const handleSubmit = async () => {
  await formRef.value?.validate()
  submitting.value = true
  try {
    const { error } = await saveCustomer({ id: editingId.value || undefined, ...formModel })
    if (!error) {
      message.success($t('page.customer.saveSuccess'))
      modalVisible.value = false
      await fetchCustomers()
    }
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (row: CustomerItem) => {
  const { error } = await deleteCustomer(row.id)
  if (!error) {
    message.success($t('page.customer.deleteSuccess'))
    await fetchCustomers()
  }
}

const openAssign = async (row: CustomerItem) => {
  assignCustomer.value = row
  assignSelectedIds.value = []
  assignedDeviceIds.value = []
  assignVisible.value = true
  const [assigned, options] = await Promise.all([
    getCustomerDevices(row.id),
    getDeviceListForSelect({ page: '1', page_size: '100' })
  ])
  assignedDeviceIds.value = assigned.data?.device_ids ?? []
  deviceOptions.value = (options.data?.list ?? []).map(item => ({
    label: item.device_name || item.device_id,
    value: item.device_id
  }))
}

const unassign = async (deviceId: string) => {
  if (!assignCustomer.value) return
  const { error } = await unassignCustomerDevice(assignCustomer.value.id, deviceId)
  if (!error) {
    assignedDeviceIds.value = assignedDeviceIds.value.filter(id => id !== deviceId)
    message.success($t('page.customer.unassignSuccess'))
  }
}

const handleAssign = async () => {
  if (!assignCustomer.value || assignSelectedIds.value.length === 0) return
  assignSubmitting.value = true
  try {
    const { error } = await assignCustomerDevices(assignCustomer.value.id, assignSelectedIds.value)
    if (!error) {
      message.success($t('page.customer.assignSuccess'))
      const assigned = await getCustomerDevices(assignCustomer.value.id)
      assignedDeviceIds.value = assigned.data?.device_ids ?? []
      assignSelectedIds.value = []
    }
  } finally {
    assignSubmitting.value = false
  }
}

onMounted(() => {
  void fetchCustomers()
})
</script>

<template>
  <div class="min-h-500px">
    <NCard :title="$t('route.customer_list')" :bordered="false">
      <template #header-extra>
        <NSpace>
          <NInput
            v-model:value="searchValue"
            :placeholder="$t('page.customer.searchPlaceholder')"
            clearable
            style="width: 220px"
            @keyup.enter="handleSearch"
          />
          <NButton type="primary" @click="handleSearch">{{ $t('common.search') }}</NButton>
          <NButton type="primary" @click="openCreate">{{ $t('page.customer.create') }}</NButton>
        </NSpace>
      </template>

      <NDataTable
        remote
        :columns="columns"
        :data="customers"
        :loading="loading"
        :pagination="pagination"
        :scroll-x="900"
        :row-key="(row: CustomerItem) => row.id"
      />
    </NCard>

    <NModal
      v-model:show="modalVisible"
      preset="card"
      :title="editingId ? $t('page.customer.editTitle') : $t('page.customer.createTitle')"
      class="w-640px"
    >
      <NForm ref="formRef" :model="formModel" :rules="formRules" label-placement="left" label-width="100">
        <NFormItem :label="$t('page.customer.name')" path="name">
          <NInput v-model:value="formModel.name" :placeholder="$t('page.customer.namePlaceholder')" />
        </NFormItem>
        <NFormItem :label="$t('page.customer.phone')" path="phone">
          <NInput v-model:value="formModel.phone" />
        </NFormItem>
        <NFormItem :label="$t('page.customer.email')" path="email">
          <NInput v-model:value="formModel.email" />
        </NFormItem>
        <NFormItem :label="$t('page.customer.country')" path="country">
          <NInput v-model:value="formModel.country" />
        </NFormItem>
        <NFormItem :label="$t('page.customer.state')" path="state">
          <NInput v-model:value="formModel.state" />
        </NFormItem>
        <NFormItem :label="$t('page.customer.city')" path="city">
          <NInput v-model:value="formModel.city" />
        </NFormItem>
        <NFormItem :label="$t('page.customer.address')" path="address">
          <NInput v-model:value="formModel.address" />
        </NFormItem>
        <NFormItem :label="$t('page.customer.address2')" path="address2">
          <NInput v-model:value="formModel.address2" />
        </NFormItem>
        <NFormItem :label="$t('page.customer.zip')" path="zip">
          <NInput v-model:value="formModel.zip" />
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="modalVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="submitting" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
        </NSpace>
      </template>
    </NModal>

    <NDrawer v-model:show="assignVisible" :width="480">
      <NDrawerContent :title="$t('page.customer.assignTitle') + (assignCustomer?.name ?? '')" closable>
        <div class="mb-12px">{{ $t('page.customer.assignedDevices') }}</div>
        <NSpace vertical>
          <NTag v-for="deviceId in assignedDeviceIds" :key="deviceId" closable @close="unassign(deviceId)">
            {{ deviceLabel(deviceId) }}
          </NTag>
          <span v-if="assignedDeviceIds.length === 0" class="text-gray-400">
            {{ $t('page.customer.noAssignedDevices') }}
          </span>
        </NSpace>

        <div class="mt-16px mb-8px">{{ $t('page.customer.pickDevices') }}</div>
        <NSelect v-model:value="assignSelectedIds" multiple filterable :options="deviceOptions" clearable />
        <template #footer>
          <NSpace justify="end">
            <NButton @click="assignVisible = false">{{ $t('common.cancel') }}</NButton>
            <NButton
              type="primary"
              :loading="assignSubmitting"
              :disabled="assignSelectedIds.length === 0"
              @click="handleAssign"
            >
              {{ $t('page.customer.assignConfirm') }}
            </NButton>
          </NSpace>
        </template>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<style scoped></style>
