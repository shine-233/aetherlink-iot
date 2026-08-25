<!--
  文件用途: 设备详情页里的子设备管理面板。
  实际职责:
  1. 拉取并分页展示当前父设备挂载的子设备列表；
  2. 提供新增子设备、设置子设备地址、删除子设备、跳转子设备详情等入口；
  3. 用弹窗承载表单交互，用表格承载当前挂载关系。
  阅读提示:
  1. 文件名是 `device-analysis`，但当前实现本质上是父子设备关系管理，并非分析指标展示；
  2. `props.id` 在这里始终代表父设备 ID，列表查询、地址更新和新增关系都依赖它；
  3. `showAddDialog`、`showSetDialog` 等状态分别对应不同编辑流程，维护时要注意不要串线。
  静态审查建议:
  1. 文件命名与业务语义明显不一致，后续应结合目录说明标记这一历史命名；
  2. 列表刷新分散在多个成功回调里，可逐步收敛为统一刷新 helper，降低遗漏重置分页/数据的风险；
  3. 多个 `ref<any>` 与未标注参数类型的函数降低了可读性，后续适合做轻量类型收敛，但不应在本轮改逻辑。
-->
<script setup lang="tsx">
import { computed, onMounted, ref } from 'vue'
import type { Ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NPopconfirm, NSpace } from 'naive-ui'
import {
  addChildDevice,
  childDeviceSelectList,
  childDeviceTableList,
  deviceUpdate,
  removeChildDevice
} from '@/service/api/device'
// import { useRouterPush } from '@/hooks/common/router';
import { $t } from '@/locales'

const router = useRouter()
// const { routerPushByKey } = useRouterPush();
const props = defineProps<{
  id: string
}>()

// 这个面板的核心不是“分析计算”，而是围绕父子设备关系做增删改查。
// 相关弹窗状态和表格状态集中放在顶部，方便维护者先看清交互流程。
const showAddDialog = ref(false)
const showSetDialog = ref(false)
const showDeleteDialog = ref(false)
const deviceSetName = ref()
const deviceSetId = ref()
const tableData = ref([])
const total = ref(0)
const log_page = ref(1)
const page_size = ref(10)
const selectChild = ref<string[]>([])
const sOptions = ref<any[]>([])

// 远程分页由当前组件完全接管；翻页和改页大小都会重新触发列表查询。
// 如果后续叠加筛选条件，需要确保这些参数和 `getData` 的查询对象同步演进。
const pagination = computed(() => ({
  page: log_page.value,
  pageSize: page_size.value,
  showSizePicker: true,
  pageSizes: [10, 15, 20, 25, 30],
  itemCount: total.value,
  onChange: (page: number) => {
    log_page.value = page
    getData()
  },
  onUpdatePageSize: (pageSize: number) => {
    page_size.value = pageSize
    log_page.value = 1
    getData()
  }
}))

// 主列表查询: 基于父设备 ID 拉取当前挂载的子设备关系。
const getData = async () => {
  const res = await childDeviceTableList({
    page: log_page.value,
    page_size: page_size.value,
    id: props.id
  })
  tableData.value = res.data.list || []
  total.value = res.data.total
}

// 多选器只存子设备 ID，真正建立关系时再按接口要求拼接成字符串。
const selectConfig = v => {
  selectChild.value = v
}

// 这里删除的是父设备与子设备的绑定关系，而不是删除子设备实体本身。
const deleteDevice = async id => {
  const { error } = await removeChildDevice({
    sub_device_id: id
  })
  if (!error) {
    showDeleteDialog.value = false
    log_page.value = 1
    tableData.value = []
    getData()
  }
}

// 查看动作会跳到子设备详情页，依赖路由里 `d_id` 这个查询参数契约。
const handleLook = (id: string) => {
  router.push({ path: 'details-child', query: { d_id: id } })
}

// 设置子设备地址前，先把目标 ID 和当前地址灌进弹窗态，交给用户二次确认。
const handleSetAddress = async (id, subDeviceAddr) => {
  deviceSetId.value = id
  showSetDialog.value = true
  deviceSetName.value = subDeviceAddr
}

// 动作列同时承载查看、设置地址、解除关联三个入口。
// 如果后续继续扩展按钮，适合拆出 action builder 或列配置 helper 提升可读性。
const columns: Ref<any> = ref([
  {
    title: $t('custom.devicePage.deviceName'),
    minWidth: '140px',
    key: 'name'
  },
  {
    title: $t('custom.devicePage.subDeviceAddress'),
    minWidth: '140px',
    key: 'subDeviceAddr'
  },
  {
    title: $t('common.actions'),
    key: '',
    minWidth: '140px',
    render: row => {
      return (
        <NSpace>
          <NButton type="primary" size="small" onClick={() => handleLook(row.id)}>
            {$t('generate.view')}
          </NButton>
          <NButton type="success" size="small" onClick={() => handleSetAddress(row.id, row.subDeviceAddr)}>
            {$t('generate.setSubDevices')}
          </NButton>
          <NPopconfirm
            negative-text={$t('common.cancel')}
            positive-text={$t('common.confirm')}
            onPositiveClick={() => deleteDevice(row.id)}
          >
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
]) as Ref<any>

// 地址更新成功后会清空本地表格缓存并重新拉取，确保界面显示以后端确认结果为准。
const setDeviceAddress = async () => {
  if (!deviceSetName.value) {
    window.$message?.error($t('generate.enter-sub-device-address'))
    return
  }
  const res = await deviceUpdate({
    id: deviceSetId.value,
    parent_id: props.id,
    sub_device_addr: deviceSetName.value
  })
  if (res) {
    tableData.value = []
    showSetDialog.value = false
    log_page.value = 1
    getData()
  }
}

// 新增关系时把多选结果合并为逗号分隔字符串提交。
// 当前前端只校验“是否有选项”，更细的重复挂载或非法状态校验仍依赖后端。
const addChildDeviceSure = () => {
  if (selectChild.value.length === 0) {
    window.$message?.error($t('generate.selectSubDevices'))
  } else {
    addChildDevice({
      id: props.id,
      son_id: selectChild.value.join(',')
    }).then(res => {
      if (!res.error) {
        showAddDialog.value = false
        selectChild.value = []
        sOptions.value = []
        tableData.value = []
        getData()
      }
    })
  }
}

// 候选子设备列表来自独立接口，与当前已挂载列表不是同一份数据源。
const getDeviceList = async () => {
  const res = await childDeviceSelectList()
  if (res.data.length !== 0) {
    sOptions.value = []
    const tempSOptions = res.data?.map(item => {
      return { label: item.name, value: item.id }
    })
    sOptions.value = sOptions.value.concat(tempSOptions)
  }
}

// 打开新增弹窗时再拉候选列表，避免详情页初次进入就额外请求全量候选数据。
const addDevice = () => {
  showAddDialog.value = true
  getDeviceList()
}

getData()

onMounted(() => {})
</script>

<template>
  <n-card class="w-full">
    <!--
      当前模板展示名称偏“分析”，但真实交互是子设备关系管理。
      若未来真的补设备分析能力，建议独立成新面板，而不是继续把两类职责叠在这个文件里。
    -->
    <NButton type="primary" @click="addDevice">{{ $t('generate.add-sub-device') }}</NButton>
    <n-modal
      v-model:show="showAddDialog"
      preset="dialog"
      :title="$t('generate.add-sub-device')"
      style="width: 520px"
      :showIcon="false"
      :mask-closable="false"
    >
      <n-form class="mt-6" label-placement="left" label-width="auto">
        <n-form-item :label="$t('generate.select-sub-device')">
          <n-select
            v-model:value="selectChild"
            multiple
            max-tag-count="responsive"
            :options="sOptions"
            :virtual-scroll="false"
            @update:value="selectConfig"
          >
            <template #header>{{ $t('custom.devicePage.deviceName') }}</template>
          </n-select>
        </n-form-item>
      </n-form>
      <template #action>
        <div class="modal-footer">
          <NButton @click="showAddDialog = false">{{ $t('generate.cancel') }}</NButton>
          <NButton type="primary" @click="addChildDeviceSure">{{ $t('page.login.common.confirm') }}</NButton>
        </div>
      </template>
    </n-modal>
    <n-modal v-model:show="showSetDialog" :title="$t('generate.issue-attribute')" class="w-[400px]">
      <n-card>
        <n-form>
          <n-form-item :label="$t('generate.sub-device-address-setting')">
            <n-input v-model:value="deviceSetName" type="text" :placeholder="$t('generate.enter-sub-device-address')" />
          </n-form-item>
          <NSpace style="display: flex; justify-content: flex-end">
            <NButton @click="showSetDialog = false">{{ $t('generate.cancel') }}</NButton>
            <NButton @click="setDeviceAddress">{{ $t('page.login.common.confirm') }}</NButton>
          </NSpace>
        </n-form>
      </n-card>
    </n-modal>

    <n-data-table :columns="columns" :data="tableData" class="mt-4" :pagination="pagination" :remote="true">
      <template #empty>
        <n-empty :description="$t('common.noData')" />
      </template>
    </n-data-table>
  </n-card>
</template>

<style scoped>
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
