// 文件用途：边缘节点管理页主表格的列定义（从 index.vue 拆出）。
// 核心逻辑：健康状态映射为 NTag 语义色；操作列按节点状态分流——active 节点提供
//   心跳 / 证书 / 升级三个动作，非 active 节点直接展示状态标签。
// 关键注意事项：i18n key 与列宽和拆分前保持一致；动作回调由页面注入。
import { NButton, NPopconfirm, NSpace, NTag } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import type { EdgeNodeEntry } from '@/service/api'

export const nodeHealthTagType: Record<EdgeNodeEntry['health'], 'success' | 'warning' | 'error' | 'default'> = {
  online: 'success',
  degraded: 'warning',
  offline: 'error',
  unknown: 'default'
}

export interface NodeColumnHandlers {
  /** 心跳确认后触发；由页面负责刷新列表。 */
  onHeartbeat: (nodeId: string) => void | Promise<void>
  /** 打开证书工作台模态框。 */
  onCertificate: (row: EdgeNodeEntry) => void
  /** 打开升级/回滚抽屉。 */
  onUpgrade: (row: EdgeNodeEntry) => void
}

export function createNodeColumns({
  onHeartbeat,
  onCertificate,
  onUpgrade
}: NodeColumnHandlers): DataTableColumns<EdgeNodeEntry> {
  return [
    { title: $t('page.edgeNodes.nodeId'), key: 'id', width: 200, ellipsis: { tooltip: true } },
    { title: $t('page.edgeNodes.version'), key: 'version', width: 100 },
    {
      title: $t('page.edgeNodes.health'),
      key: 'health',
      width: 100,
      render: row => (
        <NTag type={nodeHealthTagType[row.health] ?? 'default'} size="small">
          {row.health}
        </NTag>
      )
    },
    {
      title: $t('page.edgeNodes.capabilities'),
      key: 'capabilities',
      render: row => (row.capabilities?.length ? row.capabilities.join(', ') : '-')
    },
    {
      title: $t('page.edgeNodes.lastSeen'),
      key: 'last_seen_at',
      width: 170,
      render: row => (row.last_seen_at ? formatDateTime(row.last_seen_at) : '-')
    },
    {
      title: $t('page.edgeNodes.actions'),
      key: 'actions',
      width: 240,
      render: row =>
        row.status === 'active' ? (
          <NSpace size="small">
            <NPopconfirm onPositiveClick={() => onHeartbeat(row.id)}>
              {{
                trigger: () => (
                  <NButton size="tiny" quaternary type="primary">
                    {$t('page.edgeNodes.heartbeat')}
                  </NButton>
                ),
                default: () => $t('page.edgeNodes.heartbeatConfirm')
              }}
            </NPopconfirm>
            <NButton size="tiny" quaternary type="info" onClick={() => onCertificate(row)}>
              {$t('page.edgeNodes.certificate')}
            </NButton>
            <NButton size="tiny" quaternary type="warning" onClick={() => onUpgrade(row)}>
              {$t('page.edgeNodes.upgrade')}
            </NButton>
          </NSpace>
        ) : (
          <NTag type="error" size="small">
            {row.status}
          </NTag>
        )
    }
  ]
}
