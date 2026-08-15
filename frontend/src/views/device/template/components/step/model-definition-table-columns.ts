/**
 * 文件用途: 提供物模型定义步骤使用的表格列和列表状态。
 * 核心逻辑: 维护不同模型类别的表格配置、分页状态和通用列定义。
 * 关键注意事项: 列字段要与接口返回和弹窗编辑字段保持一致，避免展示和保存错位。
 * 重构建议: 将各模型类别列配置拆分成独立工厂函数，降低单文件维护成本。
 */
import type { Ref } from 'vue'
import { ref } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { $t } from '@/locales'

export const telemetryColumns: Ref<DataTableColumns<AddDeviceModel.Device>> = ref([
  {
    key: 'data_name',
    minWidth: '100px',
    title: $t('device_template.table_header.dataName'),
    align: 'center'
  },
  {
    key: 'data_identifier',
    minWidth: '100px',
    title: $t('device_template.table_header.dataIdentifier'),
    align: 'center'
  },
  {
    key: 'read_write_flag',
    minWidth: '100px',
    title: $t('device_template.table_header.readAndWriteSign'),
    align: 'center'
  },
  {
    key: 'data_type',
    minWidth: '100px',
    title: $t('device_template.table_header.dataType'),
    align: 'center'
  },
  {
    key: 'unit',
    minWidth: '100px',
    title: $t('device_template.table_header.unit'),
    align: 'center'
  },
  {
    key: 'description',
    minWidth: '100px',
    maxWidth: '200px',
    title: $t('device_template.table_header.description'),
    align: 'center',
    ellipsis: { tooltip: true }
  }
])

export const attributeColumns: Ref<DataTableColumns<AddDeviceModel.Device>> = ref([
  {
    key: 'data_name',
    minWidth: '100px',
    title: $t('device_template.table_header.attributeName'),
    align: 'center'
  },
  {
    key: 'data_identifier',
    minWidth: '100px',
    title: $t('device_template.table_header.attributeIdentifier'),
    align: 'center'
  },
  {
    key: 'read_write_flag',
    minWidth: '100px',
    title: $t('device_template.table_header.readAndWriteSign'),
    align: 'center'
  },
  {
    key: 'data_type',
    minWidth: '100px',
    title: $t('device_template.table_header.dataType'),
    align: 'center'
  },
  {
    key: 'unit',
    minWidth: '100px',
    title: $t('device_template.table_header.unit'),
    align: 'center'
  },
  {
    key: 'description',
    minWidth: '100px',
    maxWidth: '200px',
    title: $t('device_template.table_header.description'),
    align: 'center',
    ellipsis: { tooltip: true }
  }
])

export const eventColumns: Ref<DataTableColumns<AddDeviceModel.Device>> = ref([
  {
    key: 'data_name',
    minWidth: '100px',
    title: $t('device_template.table_header.eventName'),
    align: 'center'
  },
  {
    key: 'data_identifier',
    minWidth: '100px',
    title: $t('device_template.table_header.eventIdentifier'),
    align: 'center'
  },
  {
    key: 'params',
    minWidth: '100px',
    title: $t('device_template.table_header.eventParameters'),
    align: 'center'
  },
  {
    key: 'description',
    minWidth: '100px',
    maxWidth: '200px',
    title: $t('device_template.table_header.description'),
    align: 'center',
    ellipsis: { tooltip: true }
  }
])

export const commandColumns: Ref<DataTableColumns<AddDeviceModel.Device>> = ref([
  {
    key: 'data_name',
    minWidth: '100px',
    title: $t('device_template.table_header.commandName'),
    align: 'center'
  },
  {
    key: 'data_identifier',
    minWidth: '100px',
    title: $t('device_template.table_header.commandIdentifier'),
    align: 'center'
  },
  {
    key: 'params',
    minWidth: '100px',
    title: $t('device_template.table_header.commandParameters'),
    align: 'center'
  },
  {
    key: 'description',
    minWidth: '100px',
    maxWidth: '200px',
    title: $t('device_template.table_header.description'),
    align: 'center',
    ellipsis: { tooltip: true }
  }
])
