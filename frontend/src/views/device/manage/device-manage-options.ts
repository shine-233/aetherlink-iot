/*
 * Async option loaders shared by the device-management filters and the add-device drawer.
 */
import type { TreeSelectOption } from 'naive-ui/es/tree-select/src/interface'
import { deviceGroupTree, getDeviceConfigList } from '@/service/api/device'
import { $t } from '@/locales'

function convertTreeNodeToTarget(treeNode: DeviceManagement.TreeNode): TreeSelectOption {
  const { group, children } = treeNode
  const targetNode: TreeSelectOption = {
    label: group.name,
    key: group.id
  }

  if (children && children.length > 0) {
    targetNode.children = children.map(convertTreeNodeToTarget)
  }

  return targetNode
}

export async function loadDeviceGroupOptions(): Promise<TreeSelectOption[]> {
  const res = await deviceGroupTree({})
  if (!res.data) return []
  return res.data.map(convertTreeNodeToTarget)
}

/** Device-config options prefixed with an explicit "no config" choice. */
export async function loadDeviceConfigOptions(): Promise<any[]> {
  const res = await getDeviceConfigList({ page: 1, page_size: 99 })
  const options = res.data?.list ?? []
  return [{ name: $t('custom.devicePage.unlimitedDeviceConfig'), id: '' }, ...options]
}
