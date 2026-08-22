<!--
设备详情设置区域，负责设备配置绑定、设备分组关系维护、固件/编码展示与设备删除入口。
核心链路：读取设备详情回填当前设备配置和设备编码 -> 读取设备分组树与当前设备分组关系 -> 支持切换设备配置并通知父层刷新详情 -> 支持勾选/取消设备分组关系 -> 删除设备成功后关闭当前标签页。
静态维护重点：
1. 配置切换、分组关系和删除设备都属于高影响动作，当前集中在一个区域中，后续继续扩展更适合拆成配置、分组、危险操作三个区块。
2. 分组解绑同时在 `onUpdateCheckedKeys` 和 `watch(valueRef)` 里处理，静态上存在重复调用解绑接口的风险，后续应收口为单一出口。
3. 配置远程搜索直接向 `sOptions` 追加结果，不同关键词反复搜索后可能出现重复项或旧结果残留。
4. 这里几乎所有动作都缺少 loading、失败回滚和按钮禁用态，弱网或接口失败时用户难以判断真实落库状态。
-->
<script setup lang="tsx">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import type { TransferRenderSourceList } from 'naive-ui'
import { NTree } from 'naive-ui'
import {
  deleteDeviceGroupRelation,
  deleteDevice,
  deviceDetail,
  deviceGroupRelation,
  deviceGroupTree,
  deviceUpdateConfig,
  getDeviceConfigList,
  getDeviceGroupRelation
} from '@/service/api'
import { useDeviceDataStore } from '@/store/modules/device'
import { useTabStore } from '@/store/modules/tab'
import { getTabIdByRoute } from '@/store/modules/tab/shared'
import { $t } from '@/locales'

const props = defineProps<{
  id: string
  online: string
}>()
const valueRef = ref<Array<string | number>>([])
const device_coding = ref<string>('')
const emit = defineEmits(['change'])
const treeData = ref()
type Option = {
  label: string
  value: string
  children?: Option[]
}
const options = ref<Option[]>()
const unbindConfigOption = { label: $t('generate.unbind'), value: '' }
const sOptions = ref<any[]>([unbindConfigOption])
const configOptionsLoading = ref(false)
let configSearchTimer: ReturnType<typeof setTimeout> | null = null
let configSearchSeq = 0
const route = useRoute()
const { query } = route
const { removeTab } = useTabStore()
const currentTabId = getTabIdByRoute(route)

function normalizeDeviceConfigOptions(list: any[] = []) {
  const optionMap = new Map<string, any>()
  optionMap.set(unbindConfigOption.value, unbindConfigOption)
  const sourceList = list ?? []
  sourceList.forEach(item => {
    if (!item?.id) return
    optionMap.set(item.id, {
      label: item.name,
      value: item.id
    })
  })
  return Array.from(optionMap.values())
}

// 配置下拉总是保留“解绑”入口，后端会把空值解释为解除当前设备与设备配置的绑定关系。
const deviceConfigList = async (name = '') => {
  const searchSeq = ++configSearchSeq
  configOptionsLoading.value = true
  await loadDeviceConfigList(name, searchSeq)
}

const loadDeviceConfigList = async (name: string, searchSeq: number) => {
  try {
    const { data, error } = await getDeviceConfigList({
      page: 1,
      page_size: 99,
      name
    })
    if (searchSeq !== configSearchSeq) return
    if (!error && data) {
      sOptions.value = normalizeDeviceConfigOptions(data?.list)
    }
  } finally {
    if (searchSeq === configSearchSeq) {
      configOptionsLoading.value = false
    }
  }
}

const searchDeviceConfigList = (name = '') => {
  const searchSeq = ++configSearchSeq
  configOptionsLoading.value = true
  if (configSearchTimer) clearTimeout(configSearchTimer)
  configSearchTimer = setTimeout(() => {
    configSearchTimer = null
    loadDeviceConfigList(name, searchSeq)
  }, 250)
}

// 把后端分组树中的 group 包装结构转换成 NTree / NTransfer 可消费的 option 结构。
function transformDataToOptions(data) {
  // 定义转换函数
  const transform = item => {
    // 基本转换
    const option = {
      label: item.group.name,
      value: item.group.id,
      children: undefined
    }

    // 如果存在子项，则递归转换
    if (item.children && item.children.length > 0) {
      option.children = item.children.map(transform)
    }

    return option
  }

  // 对输入的数据应用转换函数
  return data.map(transform)
}

// 加载设备分组树后，同时保留树形结构与拍平结构，分别供 NTree 和 NTransfer 共享。
const getTreeData = async () => {
  const { data, error } = await deviceGroupTree({})
  if (!error && data) {
    treeData.value = transformDataToOptions(data)
    options.value = flattenTree(treeData.value)
  }
}

// 回显当前设备已加入的分组关系。
const getTreeRelationData = async () => {
  const { data, error } = await getDeviceGroupRelation({ device_id: props.id })
  if (!error && data) {
    valueRef.value = data?.map(item => item.group_id)
  }
}
const deviceDataStore = useDeviceDataStore()
const selectedValues = ref('')

function flattenTree(list: undefined | Option[]): Option[] {
  const result: Option[] = []

  function flatten(_list: Option[] = []) {
    _list.forEach(item => {
      result.push(item)
      flatten(item.children)
    })
  }

  flatten(list)
  return result
}

// 设备分组勾选变化时立即调用新增/删除关系接口，当前没有额外的“保存”步骤。
function syncDeviceGroupRelation(nextKeys: Array<string | number>, previousKeys: Array<string | number>) {
  const addedKeys = nextKeys.filter(key => !previousKeys.includes(key))
  const removedKeys = previousKeys.filter(key => !nextKeys.includes(key))

  addedKeys.forEach(groupId => {
    deviceGroupRelation({
      group_id: groupId,
      device_id_list: [props.id]
    })
  })

  removedKeys.forEach(groupId => {
    deleteDeviceGroupRelation({
      group_id: groupId,
      device_id: props.id
    })
  })
}

const renderSourceList: TransferRenderSourceList = ({ pattern }) => {
  return (
    <NTree
      data={treeData.value}
      style="margin: 0 4px;"
      checkedKeys={valueRef.value}
      keyField="value"
      defaultExpandAll
      checkable
      checkOnClick
      blockLine
      selectable={false}
      onUpdateCheckedKeys={keys => {
        const previousKeys = valueRef.value
        valueRef.value = keys
        syncDeviceGroupRelation(keys, previousKeys)
      }}
      pattern={pattern}
    />
  )
}

// 初始化链路:
// 1. 读取设备详情回填设备编码与当前设备配置。
// 2. 读取分组树与设备分组关系，组装 transfer/tree 勾选状态。
// 3. 外层通过 change 事件感知配置切换后需要联动刷新的详情区域。
const initData = async () => {
  const result = await deviceDetail(query.d_id as string)
  device_coding.value = result?.data?.device_number || ''
  selectedValues.value = result?.data?.device_config_id || ''
  getTreeData()
  getTreeRelationData()
}

onMounted(() => {
  initData()
  deviceConfigList('')
})

onBeforeUnmount(() => {
  if (configSearchTimer) clearTimeout(configSearchTimer)
})

// 切换设备配置后刷新 store 与当前页，再通知父层详情壳层做级联更新。
// 切换设备配置不是纯本地状态更新，而是直接改真实设备绑定配置，并回刷 store 与当前区域。
const selectConfig = async v => {
  selectedValues.value = v
  await deviceUpdateConfig({ device_id: props.id, device_config_id: v })
  await deviceDataStore.fetchData(props.id)
  await initData()
  emit('change')
}

// 删除设备前要求二次确认，避免在详情设置页误删当前设备。
// 删除设备保留二次确认，避免在详情页误触导致真实设备被立即删除。
const handleDeleteDevice = () => {
  window.$dialog?.warning({
    title: $t('common.delete'),
    content: $t('common.confirmDelete'),
    positiveText: $t('common.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: () => {
      deleteD(props.id)
    }
  })
}

// 删除成功后关闭当前标签页，防止用户停留在已经无效的详情上下文。
const deleteD = async (id: string) => {
  try {
    await deleteDevice({ id })
    window.$message?.success($t('common.deleteSuccess'))
    // 删除成功后主动关闭当前详情标签，避免用户继续停留在已失效的设备上下文。
    removeTab(currentTabId)
  } catch (error) {
    console.error('删除设备失败:', error)
  }
}
</script>

<template>
  <div class="flex-col gap-16px p-t-10px">
    <div class="flex items-center">
      <div>{{ $t('card.configTemplate') }}：</div>
      <NSelect
        v-model:value="selectedValues"
        filterable
        remote
        class="w-200px"
        :options="sOptions"
        :loading="configOptionsLoading"
        @update:value="selectConfig"
        @search="searchDeviceConfigList"
      />
    </div>
    <div class="flex items-center gap-13px">
      <span>{{ $t('generate.deviceCode') }}</span>
      <span>{{ device_coding }}</span>
    </div>

    <div class="flex items-center">
      {{ $t('generate.device-firmware') }}
      <span class="ml-4">{{ deviceDataStore?.deviceData?.current_version || '--' }}</span>
    </div>

    <div class="flex items-center">
      <n-button type="error" size="small" @click="handleDeleteDevice">
        {{ $t('common.delete') }}
      </n-button>
    </div>

    <div class="flex-1">
      <div class="mb-4">{{ $t('generate.device-group') }}</div>
      <n-transfer
        v-model:value="valueRef"
        :options="options"
        :render-source-list="renderSourceList"
        source-filterable
      />
    </div>
  </div>
</template>

<style scoped></style>
