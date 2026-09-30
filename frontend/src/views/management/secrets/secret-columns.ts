/**
 * 文件用途：密钥保管库表格列定义。
 * 核心逻辑：把 render 逻辑从页面抽离，页面只传入行操作回调，保持模板与列定义解耦。
 */
import { h } from 'vue'
import { NButton, NPopconfirm, NSpace, NTag } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { formatDateTime } from '@/utils/common/datetime'
import type { SecretItem, SecretType } from '@/service/api/secret'

/** 类型标签样式映射（key 缺失时回落到 default，避免未知枚举导致渲染空白）。 */
const TYPE_TAG_MAP: Record<
  SecretType,
  { type: 'default' | 'info' | 'success' | 'warning' | 'primary' | 'error'; label: string }
> = {
  GENERIC: { type: 'default', label: '通用' },
  API_KEY: { type: 'info', label: 'API Key' },
  TOKEN: { type: 'success', label: 'Token' },
  PASSWORD: { type: 'warning', label: 'Password' },
  CERTIFICATE: { type: 'primary', label: '证书' },
  OAUTH2: { type: 'error', label: 'OAuth2' }
}

export interface SecretColumnActions {
  onReveal: (row: SecretItem) => void
  onEdit: (row: SecretItem) => void
  onReseal: (row: SecretItem) => void
  onDelete: (row: SecretItem) => void
}

export function createSecretColumns(actions: SecretColumnActions): DataTableColumns<SecretItem> {
  return [
    {
      title: '密钥标识 (Key)',
      key: 'key',
      width: 180,
      render(row) {
        return h('span', { class: 'font-mono text-sm font-semibold text-primary' }, row.key)
      }
    },
    {
      title: '名称',
      key: 'name',
      width: 160,
      ellipsis: { tooltip: true }
    },
    {
      title: '类型',
      key: 'secret_type',
      width: 120,
      render(row) {
        const meta = TYPE_TAG_MAP[row.secret_type] || { type: 'default', label: row.secret_type }
        return h(NTag, { size: 'small', type: meta.type as any }, { default: () => meta.label })
      }
    },
    {
      title: '脱敏掩码',
      key: 'mask_preview',
      width: 120,
      render(row) {
        return h(
          'span',
          { class: 'font-mono text-xs text-gray-500 bg-gray-100 dark:bg-gray-800 px-2 py-0.5 rounded' },
          row.mask_preview
        )
      }
    },
    {
      title: '轮换状态',
      key: 'needs_reseal',
      width: 110,
      render(row) {
        if (row.needs_reseal) {
          return h(NTag, { size: 'small', type: 'warning', bordered: false }, { default: () => '需重新加密' })
        }
        return h(NTag, { size: 'small', type: 'success', bordered: false }, { default: () => '最新' })
      }
    },
    {
      title: '创建时间',
      key: 'created_at',
      width: 170,
      render(row) {
        return formatDateTime(row.created_at)
      }
    },
    {
      title: '操作',
      key: 'actions',
      width: 240,
      fixed: 'right',
      render(row) {
        return h(
          NSpace,
          { size: 'small' },
          {
            default: () =>
              [
                h(
                  NButton,
                  { size: 'tiny', type: 'info', quaternary: true, onClick: () => actions.onReveal(row) },
                  { default: () => '查看明文' }
                ),
                h(
                  NButton,
                  { size: 'tiny', type: 'primary', quaternary: true, onClick: () => actions.onEdit(row) },
                  { default: () => '编辑' }
                ),
                row.needs_reseal &&
                  h(
                    NButton,
                    { size: 'tiny', type: 'warning', quaternary: true, onClick: () => actions.onReseal(row) },
                    { default: () => '轮换' }
                  ),
                h(
                  NPopconfirm,
                  { onPositiveClick: () => actions.onDelete(row) },
                  {
                    trigger: () =>
                      h(
                        NButton,
                        { size: 'tiny', type: 'error', quaternary: true },
                        { default: () => '删除' }
                      ),
                    default: () => `确认删除密钥 ${row.key} 吗？下游引用的任务可能因此失败。`
                  }
                )
              ].filter(Boolean)
          }
        )
      }
    }
  ]
}
