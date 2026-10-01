/*
 * 文件用途：计算字段列表的表格列定义（从 index.vue 拆出）。
 * 核心逻辑：纯展示列 + 启停开关/动作列；模板名映射与动作回调由页面注入，列定义本身无副作用。
 */
import { computed } from 'vue'
import { h } from 'vue'
import { NButton, NPopconfirm, NSpace, NSwitch, NTag, NText } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import dayjs from 'dayjs'
import type { CalculatedFieldRow } from '@/service/api/calculated_field'
import { $t } from '@/locales'

export interface CalcFieldColumnDeps {
  /** 模板 id -> 名称（依赖已加载选项，缺失时回退显示 id）。 */
  templateName: (templateId: string) => string
  /** 进行中的行级动作 id（启停/删除按钮的 loading）。 */
  actingId: () => string
  onToggle: (row: CalculatedFieldRow, nextEnabled: boolean) => void
  onEdit: (row: CalculatedFieldRow) => void
  onDelete: (row: CalculatedFieldRow) => void
}

export function useCalcFieldColumns(deps: CalcFieldColumnDeps) {
  function formatTime(value?: string | number | null) {
    if (!value) return '-'
    const time = dayjs(value)
    return time.isValid() ? time.format('YYYY-MM-DD HH:mm:ss') : String(value)
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
      render: (row) => <NText>{deps.templateName(row.device_template_id)}</NText>
    },
    {
      key: 'output_key',
      title: $t('custom.management.calcField.outputKey'),
      minWidth: 130,
      render: (row) => <NTag size="small">{row.output_key}</NTag>
    },
    {
      key: 'expression',
      title: $t('custom.management.calcField.expression'),
      minWidth: 200,
      ellipsis: { tooltip: true },
      render: (row) => <NText code>{row.expression}</NText>
    },
    {
      key: 'enabled',
      title: $t('custom.management.calcField.enabled'),
      width: 90,
      render: (row) => (
        <NSwitch
          size="small"
          value={row.enabled}
          loading={deps.actingId() === row.id}
          onUpdateValue={(value) => deps.onToggle(row, value)}
        />
      )
    },
    {
      key: 'updated_at',
      title: $t('custom.management.calcField.updatedAt'),
      minWidth: 170,
      render: (row) => formatTime(row.updated_at)
    },
    {
      key: 'actions',
      title: $t('custom.management.calcField.actions'),
      width: 160,
      fixed: 'right',
      render: (row) => (
        <NSpace size={8}>
          <NButton size="small" onClick={() => deps.onEdit(row)}>
            {$t('common.edit')}
          </NButton>
          <NPopconfirm
            negative-text={$t('common.cancel')}
            positive-text={$t('common.confirm')}
            onPositiveClick={() => deps.onDelete(row)}
          >
            {{
              default: () => $t('custom.management.calcField.confirmDelete'),
              trigger: () => (
                <NButton size="small" type="error" ghost loading={deps.actingId() === row.id}>
                  {$t('common.delete')}
                </NButton>
              )
            }}
          </NPopconfirm>
        </NSpace>
      )
    }
  ])

  return { columns }
}
