<!--
后台用户管理页，负责用户列表查询、筛选、状态展示、删除、资料编辑和密码修改入口。
核心链路：组织查询条件 -> 拉取分页用户列表 -> 通过表格动作打开编辑弹窗或密码弹窗 -> 成功后统一回刷列表。
静态维护重点：
1. 页面同时管理身份信息、时区、语言和地址筛选，查询参数结构已经较重，后续扩展字段时建议优先拆 composable，而不是继续堆在单文件中。
2. 用户删除、状态展示和密码修改都属于高权限操作，后续增强时应优先补失败态、禁用态和角色边界，而不是只补 UI。
3. 省市区、时区、语言、国家区号这些静态选项当前直接写在页面里，后续若多页复用，建议抽成常量或字典服务。
-->
<script setup lang="tsx">
import { computed, reactive, getCurrentInstance, onMounted, ref } from 'vue'
import type { Ref } from 'vue'
import { NAlert, NButton, NPopconfirm, NSpace, NTag } from 'naive-ui'
import type { DataTableColumns, PaginationProps } from 'naive-ui'
import { useBoolean, useLoading } from '@aetherlink/hooks'
import dayjs from 'dayjs'
import { useRoute, useRouter } from 'vue-router'
import { userStatusOptions } from '@/constants/business'
import { delUser, fetchUserList } from '@/service/api/auth'
import { useAuthStore } from '@/store/modules/auth'
import { $t } from '@/locales'
import TableActionModal from './components/table-action-modal.vue'
import EditPasswordModal from './components/edit-password-modal.vue'
import type { ModalType } from './components/table-action-modal.vue'

const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()
const { loading, startLoading, endLoading } = useLoading(false)
const { bool: visible, setTrue: openModal } = useBoolean()
const { bool: editPwdVisible, setTrue: openEditPwdModal } = useBoolean()
const showEmpty = ref(false)
const tenantAdminSetupPromptDismissed = ref(false)
const tenantAdminSetupCompleted = ref(false)
const isTenantAdminSetupGuide = computed(() => route.query.setup === 'tenant-admin')
const showTenantAdminSetupPrompt = computed(
  () => isTenantAdminSetupGuide.value && !tenantAdminSetupPromptDismissed.value && !tenantAdminSetupCompleted.value
)

const customUserStatusOptions = computed(() => {
  return userStatusOptions.map(item => {
    const key = item.value === 'N' ? 'page.manage.user.status.normal' : 'page.manage.user.status.freeze'
    return {
      label: $t(key),
      value: item.value
    }
  })
})

// 时区选项直接服务于查询和编辑弹窗，当前由页面维护，后续若跨页复用应抽离。
// 城市描述走 i18n，避免在选项标签里硬编码中文。
const timezoneDefs: { value: string; cityKey: string }[] = [
  { value: 'Asia/Shanghai', cityKey: 'page.manage.user.tz.shanghai' },
  { value: 'Asia/Tokyo', cityKey: 'page.manage.user.tz.tokyo' },
  { value: 'Asia/Seoul', cityKey: 'page.manage.user.tz.seoul' },
  { value: 'Asia/Singapore', cityKey: 'page.manage.user.tz.singapore' },
  { value: 'Asia/Hong_Kong', cityKey: 'page.manage.user.tz.hongKong' },
  { value: 'Asia/Bangkok', cityKey: 'page.manage.user.tz.bangkok' },
  { value: 'Asia/Dubai', cityKey: 'page.manage.user.tz.dubai' },
  { value: 'Asia/Kolkata', cityKey: 'page.manage.user.tz.kolkata' },
  { value: 'Europe/London', cityKey: 'page.manage.user.tz.london' },
  { value: 'Europe/Paris', cityKey: 'page.manage.user.tz.paris' },
  { value: 'Europe/Berlin', cityKey: 'page.manage.user.tz.berlin' },
  { value: 'Europe/Moscow', cityKey: 'page.manage.user.tz.moscow' },
  { value: 'America/New_York', cityKey: 'page.manage.user.tz.newYork' },
  { value: 'America/Los_Angeles', cityKey: 'page.manage.user.tz.losAngeles' },
  { value: 'America/Chicago', cityKey: 'page.manage.user.tz.chicago' },
  { value: 'America/Toronto', cityKey: 'page.manage.user.tz.toronto' },
  { value: 'Australia/Sydney', cityKey: 'page.manage.user.tz.sydney' },
  { value: 'Australia/Melbourne', cityKey: 'page.manage.user.tz.melbourne' },
  { value: 'Pacific/Auckland', cityKey: 'page.manage.user.tz.auckland' },
  { value: 'UTC', cityKey: 'page.manage.user.tz.utc' }
]
const timezoneOptions = computed(() =>
  timezoneDefs.map(item => ({ label: `${item.value} (${$t(item.cityKey)})`, value: item.value }))
)

// 默认语言筛选同样属于后台静态枚举，建议后续与国际化配置统一来源。
const languageOptions = [
  { label: 'Chinese', value: 'zh-CN' },
  { label: 'English', value: 'en-US' },
  { label: 'Francais', value: 'fr-FR' },
  { label: 'Espanol', value: 'es-ES' }
]

// 将静态地区 JSON 转换成级联选择器所需格式，避免在模板层临时拼接。
const convertPwDataToCascader = (data: any): any[] => {
  return data.map((province: any) => ({
    value: province.name,
    label: province.name,
    children:
      province.children?.map((city: any) => ({
        value: city.name,
        label: city.name,
        children:
          city.children?.map((district: any) => ({
            value: district.name,
            label: district.name
          })) || []
      })) || []
  }))
}

// 页面查询和编辑弹窗共用同一套省市区级联数据。
const provinceCityData = reactive<any[]>([])

async function loadProvinceCityData() {
  if (provinceCityData.length) return

  const module = await import('@/assets/data/china-region.json')
  const data = module.default || module
  provinceCityData.splice(0, provinceCityData.length, ...convertPwDataToCascader(Array.isArray(data) ? data : []))
}

// 将级联选择器值回填到结构化地址字段，便于直接传给列表查询接口。
const handleAddressChange = (value: string[]) => {
  queryParams.address.cascaderValue = value
  if (value && value.length >= 3) {
    queryParams.address.province = value[0] // 省份
    queryParams.address.city = value[1] // 城市
    queryParams.address.district = value[2] // 区县
  } else {
    queryParams.address.province = null
    queryParams.address.city = null
    queryParams.address.district = null
  }
}

// 搜索按节点标签模糊匹配，降低地址筛选在大数据下的使用成本。
const filterCascader = (pattern: string, option: any) => {
  return option.label.toLowerCase().includes(pattern.toLowerCase())
}

type QueryFormModel = Pick<UserManagement.User, 'email' | 'name' | 'status'> & {
  page: number
  page_size: number
  organization: string | null
  timezone: string | null
  default_language: string | null
  address: {
    province: string | null
    city: string | null
    district: string | null
    detailed_address: string | null
    cascaderValue: string[] | null // 新增：存储级联选择器的值
  }
}

const queryParams = reactive<QueryFormModel>({
  email: null,
  name: null,
  status: null,
  page: 1,
  page_size: 10,
  organization: null,
  timezone: null,
  default_language: null,
  address: {
    province: null,
    city: null,
    district: null,
    detailed_address: null,
    cascaderValue: null
  }
})

const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  itemCount: 0,
  pageSizes: [10, 15, 20, 25, 30],
  onChange: (page: number) => {
    pagination.page = page
    queryParams.page = page
    getTableData()
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
    queryParams.page = 1
    queryParams.page_size = pageSize
    getTableData()
  }
})

const tableData = ref<UserManagement.User[]>([])

// 空列表与接口返回 null 的语义在这里分开处理，便于页面展示空态。
function setTableData(data: UserManagement.User[]) {
  if (data === null) {
    showEmpty.value = true
  } else {
    showEmpty.value = false
    tableData.value = data
  }
}

// 用户列表是页面真相源，筛选、分页和增删改成功后都统一走这一入口刷新。
async function getTableData() {
  startLoading()
  const { data } = await fetchUserList(queryParams)
  if (data) {
    const list: UserManagement.User[] = data.list
    pagination.itemCount = data.total
    setTableData(list)
    endLoading()
  }
}

const columns: Ref<DataTableColumns<UserManagement.User>> = ref([
  {
    key: 'email',
    minWidth: '140px',
    title: () => $t('page.manage.user.userEmail'),
    align: 'left'
  },
  {
    key: 'name',
    minWidth: '140px',
    title: () => $t('page.manage.user.userName'),
    align: 'left'
  },
  {
    key: 'phone_number',
    minWidth: '140px',
    title: () => $t('page.manage.user.userPhone'),
    align: 'left'
  },
  {
    key: 'created_at',
    minWidth: '140px',
    title: () => $t('common.creationTime'),
    align: 'left',
    render: row => dayjs(row.created_at).format('YYYY-MM-DD HH:mm:ss')
  },
  {
    key: 'status',
    minWidth: '140px',
    title: () => $t('page.manage.user.userStatus'),
    align: 'left',
    render: row => {
      if (row.status) {
        const tagTypes: Record<UserManagement.UserStatusKey, NaiveUI.ThemeColor> = {
          N: 'success',
          F: 'error'
        }
        const key = row.status === 'N' ? 'page.manage.user.status.normal' : 'page.manage.user.status.freeze'
        return <NTag type={tagTypes[row.status]}>{$t(key)}</NTag>
      }
      return <span></span>
    }
  },
  {
    key: 'lastVisitTime',
    minWidth: '140px',
    title: () => $t('custom.management.lastAccessTime'),
    align: 'left',
    render: row => dayjs(row.lastVisitTime || row.created_at).format('YYYY-MM-DD HH:mm:ss')
  },
  {
    key: 'remark',
    minWidth: '140px',
    title: () => $t('common.remark'),
    align: 'left'
  },
  {
    key: 'actions',
    width: '300px',
    fixed: 'right',
    title: () => $t('common.actions'),
    align: 'left',
    render: row => {
      return (
        <NSpace justify={'start'}>
          <NPopconfirm
            negative-text={$t('common.cancel')}
            positive-text={$t('common.confirm')}
            onPositiveClick={() => handleEnter(row.id)}
          >
            {{
              default: () => $t('common.confirm'),
              trigger: () => (
                <NButton type="warning" size={'small'}>
                  {$t('page.manage.user.enter')}
                </NButton>
              )
            }}
          </NPopconfirm>
          <NButton type="warning" size={'small'} onClick={() => handleEditPwd(row.id)}>
            {$t('page.login.resetPwd.title')}
          </NButton>
          <NButton type="primary" size={'small'} onClick={() => handleEditTable(row.id)}>
            {$t('common.edit')}
          </NButton>
          <NPopconfirm
            negative-text={$t('common.cancel')}
            positive-text={$t('common.confirm')}
            onPositiveClick={() => handleDeleteTable(row.id)}
          >
            {{
              default: () => $t('common.confirm'),
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
]) as Ref<DataTableColumns<UserManagement.User>>

const modalType = ref<ModalType>('add')
const tenantAdminSetupModalTitle = computed(() =>
  isTenantAdminSetupGuide.value && modalType.value === 'add'
    ? $t('page.manage.user.tenantAdminSetup.modalTitle')
    : undefined
)

function setModalType(type: ModalType) {
  modalType.value = type
}

const editData = ref<UserManagement.User | null>(null)

function setEditData(data: UserManagement.User | null) {
  editData.value = data
}

function handleAddTable() {
  setEditData(null)
  setModalType('add')
  openModal()
}

function handleOpenTenantAdminSetup() {
  tenantAdminSetupPromptDismissed.value = false
  handleAddTable()
}

function handleUserModalSuccess() {
  getTableData()
  if (isTenantAdminSetupGuide.value) {
    tenantAdminSetupCompleted.value = true
    tenantAdminSetupPromptDismissed.value = false
    window.$message?.success($t('page.manage.user.tenantAdminSetup.successMessage'))
  }
}

async function handleSwitchToTenantAdminLogin() {
  await authStore.resetStore()
}

function handleBackToHomeAfterTenantAdminSetup() {
  router.push('/')
}

/** 切换用户 */
async function handleEnter(rowId: string) {
  await authStore.enter(rowId)
}

function handleEditPwd(rowId: string) {
  const findItem = tableData.value.find(item => item.id === rowId)
  if (findItem) {
    setEditData(findItem)
  }
  openEditPwdModal()
}

function handleEditTable(rowId: string) {
  const findItem = tableData.value.find(item => item.id === rowId)
  if (findItem) {
    setEditData(findItem)
  }
  setModalType('edit')
  openModal()
}

async function handleDeleteTable(rowId: string) {
  const data = await delUser(rowId)
  if (!data.error) {
    window.$message?.success($t('common.deleteSuccess'))
    getTableData()
  }
}

function handleQuery() {
  queryParams.page = 1
  init()
}

function handleReset() {
  Object.assign(queryParams, {
    email: null,
    name: null,
    status: null,
    page: 1,
    organization: null,
    timezone: null,
    default_language: null,
    address: {
      province: null,
      city: null,
      district: null,
      detailed_address: null,
      cascaderValue: null
    }
  })
  handleQuery()
}

function init() {
  getTableData()
}

// 初始化
init()
onMounted(() => {
  loadProvinceCityData()
  if (isTenantAdminSetupGuide.value) {
    handleAddTable()
  }
})
const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
</script>

<template>
  <div>
    <NCard :title="$t('route.management_user')" :bordered="false" class="h-full rounded-8px shadow-sm">
      <div class="h-full flex-col">
        <NAlert
          v-if="showTenantAdminSetupPrompt"
          type="info"
          class="mb-12px"
          :title="$t('page.manage.user.tenantAdminSetup.promptTitle')"
          closable
          @close="tenantAdminSetupPromptDismissed = true"
        >
          {{ $t('page.manage.user.tenantAdminSetup.promptDesc') }}
          <!-- NAlert 只暴露 default/icon/header 三个插槽，没有 action；
               放在 #action 里的按钮不会被渲染，因此操作按钮必须留在默认插槽内。 -->
          <div class="mt-8px">
            <NButton size="small" type="primary" @click="handleOpenTenantAdminSetup">
              {{ $t('page.manage.user.tenantAdminSetup.createAction') }}
            </NButton>
          </div>
        </NAlert>
        <NAlert
          v-if="tenantAdminSetupCompleted"
          type="success"
          class="mb-12px"
          :title="$t('page.manage.user.tenantAdminSetup.doneTitle')"
          :show-icon="false"
        >
          {{ $t('page.manage.user.tenantAdminSetup.doneDesc') }}
          <!-- 同上：NAlert 没有 action 插槽，按钮放在默认插槽内才会渲染。 -->
          <div class="mt-8px">
            <NSpace>
              <NButton size="small" type="primary" @click="handleSwitchToTenantAdminLogin">
                {{ $t('page.manage.user.tenantAdminSetup.switchLogin') }}
              </NButton>
              <NButton size="small" secondary @click="handleBackToHomeAfterTenantAdminSetup">
                {{ $t('page.manage.user.tenantAdminSetup.backHome') }}
              </NButton>
            </NSpace>
          </div>
        </NAlert>
        <NForm :inline="!getPlatform" label-placement="left" :model="queryParams">
          <div class="flex flex-wrap">
            <NFormItem :label="$t('page.manage.user.userEmail')" path="email">
              <NInput v-model:value="queryParams.email" />
            </NFormItem>
            <NFormItem :label="$t('page.manage.user.userName')" path="name">
              <NInput v-model:value="queryParams.name" />
            </NFormItem>
            <NFormItem :label="$t('page.manage.user.userStatus')" path="status">
              <NSelect
                v-model:value="queryParams.status"
                clearable
                class="w-200px"
                :options="customUserStatusOptions"
              />
            </NFormItem>
            <NFormItem :label="$t('page.manage.user.organization')" path="organization">
              <NInput v-model:value="queryParams.organization" :placeholder="$t('page.manage.user.form.organization')" />
            </NFormItem>
            <NFormItem :label="$t('page.manage.user.address')" path="address.province">
              <NCascader
                v-model:value="queryParams.address.cascaderValue"
                :options="provinceCityData"
                :placeholder="$t('page.manage.user.form.address')"
                clearable
                class="w-300px"
                :show-path="true"
                :filterable="true"
                :filter="filterCascader"
                @update:value="handleAddressChange"
              />
            </NFormItem>
            <NFormItem :label="$t('page.manage.user.detailedAddress')" path="address.detailed_address">
              <NInput
                v-model:value="queryParams.address.detailed_address"
                :placeholder="$t('page.manage.user.form.detailedAddress')"
              />
            </NFormItem>
            <NFormItem :label="$t('page.manage.user.timezone')" path="timezone">
              <NSelect
                v-model:value="queryParams.timezone"
                clearable
                class="w-200px"
                :options="timezoneOptions"
                :placeholder="$t('page.manage.user.form.timezone')"
              />
            </NFormItem>
            <NFormItem :label="$t('page.manage.user.defaultLanguage')" path="default_language">
              <NSelect
                v-model:value="queryParams.default_language"
                clearable
                class="w-200px"
                :options="languageOptions"
                :placeholder="$t('page.manage.user.form.defaultLanguage')"
              />
            </NFormItem>
            <NFormItem>
              <NButton class="w-72px" type="primary" @click="handleQuery">{{ $t('common.search') }}</NButton>
              <NButton class="ml-20px w-72px" type="primary" @click="handleReset">{{ $t('common.reset') }}</NButton>
            </NFormItem>
          </div>
        </NForm>

        <NSpace class="pb-12px" justify="space-between">
          <NButton type="primary" @click="handleAddTable">
            <IconIcRoundPlus class="mr-4px text-20px" />
            {{ $t('common.add') }}
          </NButton>
        </NSpace>

        <NDataTable
          v-if="!showEmpty"
          :row-key="row => row.id"
          :remote="true"
          :columns="columns"
          :data="tableData"
          :loading="loading"
          :pagination="pagination"
          class="flex-1-hidden"
        />
        <div v-if="showEmpty" class="h-500px flex-center flex-col">
          <n-empty :description="$t('common.noData')"></n-empty>
        </div>

        <TableActionModal
          v-model:visible="visible"
          :type="modalType"
          :title-override="tenantAdminSetupModalTitle"
          :setup-tenant-admin-mode="isTenantAdminSetupGuide"
          :edit-data="editData"
          @success="handleUserModalSuccess"
        />
        <EditPasswordModal
          v-model:visible="editPwdVisible"
          :edit-data="editData"
          @success="getTableData"
        ></EditPasswordModal>
      </div>
    </NCard>
  </div>
</template>

<style scoped></style>
