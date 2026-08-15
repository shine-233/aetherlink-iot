<!--
文件用途: 物模型定义步骤。
核心逻辑: 汇总管理遥测、属性、事件、命令和自定义控制等模型定义。
关键注意事项: 该步骤是模板能力的核心入口，删除或编辑模型项会影响设备详情和自动化配置。
重构建议: 将各模型类别的表格配置、弹窗状态和 API 调用拆分为独立模块。
-->
<script setup lang="tsx">
import { computed, defineAsyncComponent, reactive, ref } from 'vue'
import type { PaginationProps } from 'naive-ui'
import { NButton, NPopconfirm, NSpace } from 'naive-ui'
import { useLoading } from '@aetherlink/hooks'
import { $t } from '@/locales'
import {
  attributesApi,
  commandsApi,
  delAttributes,
  delCommands,
  delEvents,
  delTelemetry,
  eventsApi,
  telemetryApi
} from '@/service/api/system-data'
import {
  attributeColumns,
  commandColumns,
  eventColumns,
  telemetryColumns
} from './model-definition-table-columns'
import AddEditTest from './add-edit-test.vue'
import AddEditAttributes from './add-edit-attributes.vue'
import AddEditEvents from './add-edit-events.vue'
import AddEditCommands from './add-edit-commands.vue'
import WidgetPresetConfig from './widget-preset-config.vue'

const CustomCommands = defineAsyncComponent(() => import('./custom-commands.vue'))
const CustomControls = defineAsyncComponent(() => import('./custom-controls.vue'))

const emit = defineEmits(['update:stepCurrent', 'update:modalVisible'])
const { loading, startLoading, endLoading } = useLoading(false)

const props = defineProps({
  stepCurrent: {
    type: Number,
    required: true
  },
  modalVisible: {
    type: Boolean,
    required: true
  },
  deviceTemplateId: {
    type: String,
    required: true
  }
})

const deviceTemplateId = ref<string>(props.deviceTemplateId)
type ModelDefinitionTabName = 'telemetry' | 'attributes' | 'events' | 'command'

const tabsCurrent = ref<ModelDefinitionTabName>('telemetry')
const loadedTabs = reactive<Record<ModelDefinitionTabName, boolean>>({
  telemetry: false,
  attributes: false,
  events: false,
  command: false
})
const addAndEditModalVisible = ref<boolean>(false)
const presetModalVisible = ref<boolean>(false)
const presetProperty = ref<any>({})
const presetType = ref<'telemetry' | 'attributes'>('telemetry')
const addAndEditTitle = ref<string>($t('device_template.addAndEditTelemetry'))

const comList: { id: string; components: any; title: string }[] = [
  { id: 'telemetry', components: AddEditTest, title: $t('device_template.addAndEditTelemetry') },
  { id: 'attributes', components: AddEditAttributes, title: $t('device_template.addAndEditAttributes') },
  { id: 'events', components: AddEditEvents, title: $t('device_template.addAndEditEvents') },
  { id: 'command', components: AddEditCommands, title: $t('device_template.addAndEditCommand') }
]
const SwitchCom = computed<any>(() => {
  // eslint-disable-next-line array-callback-return,consistent-return
  return comList.find(item => {
    if (item.id === tabsCurrent.value) {
      const objItem: any = item
      addAndEditTitle.value = objItem.title
      return objItem
    }
  })?.components
})

const queryParams: any = reactive([
  {
    page: 1,
    page_size: 10,
    device_template_id: props.deviceTemplateId
  },
  {
    page: 1,
    page_size: 10,
    device_template_id: props.deviceTemplateId
  },
  {
    page: 1,
    page_size: 10,
    device_template_id: props.deviceTemplateId
  },
  {
    page: 1,
    page_size: 10,
    device_template_id: props.deviceTemplateId
  }
])

const checkedTabs: (value: string | number) => void = value => {
  const tabName = String(value) as ModelDefinitionTabName
  tabsCurrent.value = tabName
  if (!loadedTabs[tabName]) {
    getTableData(tabName)
  }
}

const getPagination = (index: number) => {
  return {
    page: queryParams[index].page,
    pageSize: queryParams[index].page_size,
    itemCount: columnsList[index].total,
    showSizePicker: true,
    pageSizes: [10, 15, 20, 25, 30],
    onChange: (page: number) => {
      queryParams[index].page = page
      getTableData(columnsList[index].name)
    },
    onUpdatePageSize: (pageSize: number) => {
      queryParams[index].page_size = pageSize
      queryParams[index].page = 1
      getTableData(columnsList[index].name)
    }
  }
}

// 编辑
let objItem = reactive<any>({})
const edit: (row: any) => void = row => {
  addAndEditModalVisible.value = true
  objItem = row
}

// 预设配置
const configPreset = (row: any, type: 'telemetry' | 'attributes') => {
  presetProperty.value = {
    id: row.id,
    name: row.data_name,
    identifier: row.data_identifier,
    dataType: row.data_type,
    unit: row.unit
  }
  presetType.value = type
  presetModalVisible.value = true
}

// 新增或者编辑成功后的回调函数
const determine: () => void = () => {
  // eslint-disable-next-line @typescript-eslint/no-use-before-define
  getTableData(tabsCurrent.value)
}

// 删除
const del: (id: string) => void = async id => {
  if (tabsCurrent.value === 'telemetry') {
    await delTelemetry(id)
  } else if (tabsCurrent.value === 'attributes') {
    await delAttributes(id)
  } else if (tabsCurrent.value === 'events') {
    await delEvents(id)
  } else {
    await delCommands(id)
  }
  // eslint-disable-next-line @typescript-eslint/no-use-before-define
  getTableData(tabsCurrent.value)
}
// 上一步
const next: () => void = async () => {
  emit('update:stepCurrent', 3)
}
// 下一步
const back: () => void = async () => {
  emit('update:stepCurrent', 1)
}
// 取消
const cancellation: () => void = () => {
  emit('update:modalVisible', false)
}
const cloneaddAndEditVisible: () => void = () => {
  objItem = {}
}
const columnsList: any = reactive([
  {
    addBtn: () => {
      addAndEditModalVisible.value = true
    },
    total: 0,
    data: [{ data_name: $t('common.test') }],
    name: 'telemetry',
    text: $t('device_template.telemetry'),
    col: [
      ...telemetryColumns.value,
      {
        key: 'actions',
        width: 350,
        title: () => $t('common.actions'),
        align: 'center',
        render: row => {
          return (
            <NSpace justify={'center'}>
              <NButton quaternary type="primary" size={'small'} onClick={() => edit(row)}>
                {$t('common.edit')}
              </NButton>
              <NPopconfirm onPositiveClick={() => del(row.id)}>
                {{
                  default: () => $t('common.confirmDelete'),
                  trigger: () => (
                    <NButton quaternary type="primary" size={'small'}>
                      {$t('common.delete')}
                    </NButton>
                  )
                }}
              </NPopconfirm>
            </NSpace>
          )
        }
      }
    ]
  },
  {
    addBtn: () => {
      addAndEditModalVisible.value = true
    },
    total: 0,
    data: [],
    name: 'attributes',
    text: $t('device_template.attributes'),
    col: [
      ...attributeColumns.value,
      {
        key: 'actions',
        width: 350,
        title: () => $t('common.actions'),
        align: 'center',
        render: row => {
          return (
            <NSpace justify={'center'}>
              <NButton quaternary type="primary" size={'small'} onClick={() => edit(row)}>
                {$t('common.edit')}
              </NButton>
              <NPopconfirm onPositiveClick={() => del(row.id)}>
                {{
                  default: () => $t('common.confirmDelete'),
                  trigger: () => (
                    <NButton quaternary type="primary" size={'small'}>
                      {$t('common.delete')}
                    </NButton>
                  )
                }}
              </NPopconfirm>
            </NSpace>
          )
        }
      }
    ]
  },
  {
    addBtn: () => {
      addAndEditModalVisible.value = true
    },
    total: 0,
    data: [],
    name: 'events',
    text: $t('device_template.events'),
    col: [
      ...eventColumns.value,
      {
        key: 'actions',
        width: 350,
        title: () => $t('common.actions'),
        align: 'center',
        render: row => {
          return (
            <NSpace justify={'center'}>
              <NButton quaternary type="primary" size={'small'} onClick={() => edit(row)}>
                {$t('common.edit')}
              </NButton>
              <NPopconfirm onPositiveClick={() => del(row.id)}>
                {{
                  default: () => $t('common.confirmDelete'),
                  trigger: () => (
                    <NButton quaternary type="primary" size={'small'}>
                      {$t('common.delete')}
                    </NButton>
                  )
                }}
              </NPopconfirm>
            </NSpace>
          )
        }
      }
    ]
  },
  {
    addBtn: () => {
      addAndEditModalVisible.value = true
    },
    total: 0,
    data: [],
    name: 'command',
    text: $t('device_template.command'),
    col: [
      ...commandColumns.value,
      {
        key: 'actions',
        width: 350,
        title: () => $t('common.actions'),
        align: 'center',
        render: row => {
          return (
            <NSpace justify={'center'}>
              {/* eslint-disable-next-line @typescript-eslint/no-use-before-define */}
              <NButton quaternary type="primary" size={'small'} onClick={() => edit(row)}>
                {$t('common.edit')}
              </NButton>
              {/* eslint-disable-next-line @typescript-eslint/no-use-before-define */}
              <NPopconfirm onPositiveClick={() => del(row.id)}>
                {{
                  default: () => $t('common.confirmDelete'),
                  trigger: () => (
                    <NButton quaternary type="primary" size={'small'}>
                      {$t('common.delete')}
                    </NButton>
                  )
                }}
              </NPopconfirm>
            </NSpace>
          )
        }
      }
    ]
  }
])

const updateAttributesData = (data: any) => {
  columnsList[1].data = data?.list ?? []
  columnsList[1].total = data?.total || 0
  columnsList[1].data?.forEach((item: any) => {
    item.read_write_flag = formatReadWriteFlag(item.read_write_flag)
  })
}

function formatReadWriteFlag(flag: string) {
  if (flag === 'R' || flag === 'R-只读') {
    return $t('device_template.table_header.readOnly')
  }
  if (flag === 'W' || flag === 'W-只写') {
    return $t('device_template.table_header.writeOnly')
  }
  if (flag === 'RW' || flag === 'RW-读/写') {
    return $t('device_template.table_header.readAndWrite')
  }
  return flag
}

const handleParamsOfEventsAndcommands = data => {
  if (!data || !Array.isArray(data)) {
    return data
  }
  return data.map(item => {
    const paramsArr = JSON.parse(item.params) || []
    return {
      ...item,
      paramsOrigin: item.params,
      params: paramsArr.map(param => param.data_name).join(', ')
    }
  })
}

// Helper functions to update data
const updateTelemetryData = (data: any) => {
  columnsList[0].data = data?.list ?? []
  columnsList[0].total = data?.total || 0
  columnsList[0].data.forEach((item: any) => {
    item.read_write_flag = formatReadWriteFlag(item.read_write_flag)
  })
}

const updateEventsData = (data: any) => {
  columnsList[2].data = handleParamsOfEventsAndcommands(data?.list ?? [])
  columnsList[2].total = data?.total || 0
}

const updateCommandsData = (data: any) => {
  columnsList[3].data = handleParamsOfEventsAndcommands(data?.list ?? [])
  columnsList[3].total = data?.total || 0
}
const getTableData: (value?: string) => Promise<void> = async value => {
  const tabName = (value || tabsCurrent.value) as ModelDefinitionTabName
  startLoading()
  try {
    if (tabName === 'telemetry') {
      const { data: data0 }: any = await telemetryApi(queryParams[0])
      updateTelemetryData(data0)
    } else if (tabName === 'attributes') {
      const { data: data1 }: any = await attributesApi(queryParams[1])
      updateAttributesData(data1)
    } else if (tabName === 'events') {
      const { data: data2 }: any = await eventsApi(queryParams[2])
      updateEventsData(data2)
    } else {
      const { data: data3 }: any = await commandsApi(queryParams[3])
      updateCommandsData(data3)
    }
    loadedTabs[tabName] = true
  } catch (error) {
    console.error('Error fetching data:', error)
  } finally {
    endLoading()
  }
}

getTableData()
</script>

<template>
  <div>
    <n-tabs v-model:value="tabsCurrent" type="line" animated @update:value="checkedTabs">
      <n-tab-pane v-for="(item, index) in columnsList" :key="item.name" :name="item.name" :tab="item.text">
        <NButton type="primary" class="addBtn" @click="item.addBtn">
          <template #icon>
            <SvgIcon local-icon="add" class="more" />
          </template>
          {{ $t('device_template.add') }}
        </NButton>
        <n-data-table
          :columns="item.col"
          :data="item.data"
          :loading="loading"
          :pagination="getPagination(index)"
          :remote="true"
          class="m-t9 flex-1-hidden"
        />

        <CustomControls
          v-if="item.name === 'telemetry' && tabsCurrent === 'telemetry'"
          :id="deviceTemplateId"
        ></CustomControls>
        <CustomCommands v-if="item.name === 'command' && tabsCurrent === 'command'" :id="deviceTemplateId"></CustomCommands>
      </n-tab-pane>
    </n-tabs>
  </div>
  <div class="box1 m-t2">
    <NButton type="primary" @click="next">{{ $t('device_template.nextStep') }}</NButton>
    <NButton class="m-r3" ghost type="primary" @click="back">{{ $t('device_template.back') }}</NButton>
    <NButton class="m-r3" @click="cancellation">{{ $t('generate.cancel') }}</NButton>
  </div>
  <NModal
    v-model:show="addAndEditModalVisible"
    preset="card"
    :title="addAndEditTitle"
    class="mw-600px w-50%"
    @after-leave="cloneaddAndEditVisible"
  >
    <component
      :is="SwitchCom"
      v-model:addAndEditModalVisible="addAndEditModalVisible"
      v-model:deviceTemplateId="deviceTemplateId"
      v-model:objItem="objItem"
      @determine="determine"
    ></component>
  </NModal>
  <!-- 预设组件配置弹窗 -->
  <WidgetPresetConfig
    v-if="presetModalVisible"
    v-model:presetModalVisible="presetModalVisible"
    :device-template-id="deviceTemplateId"
    :property="presetProperty"
    :property-type="presetType"
  />
</template>

<style lang="scss" scoped>
.addBtn {
  position: absolute;
  right: 0;
  top: 0.5rem;
}
.mw-600px {
  min-width: 600px !important;
}
.box1 {
  display: flex;
  flex-direction: row-reverse;
}
</style>
