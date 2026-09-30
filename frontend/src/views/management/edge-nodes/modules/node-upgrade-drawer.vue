<!--
文件用途：边缘节点版本升级与回滚抽屉（从 index.vue 拆出）。
核心逻辑：
1. 打开抽屉时重置升级表单并拉取升级历史流水；
2. 在线升级：target_version 必填，package_url/checksum/description 可选（空串不提交）；
3. 一键回滚到历史版本；升级/回滚成功后回刷历史并通知父页面刷新节点列表。
关键注意事项：版本严格递增校验由后端执行，前端只保证必填与去空白。
-->
<script setup lang="tsx">
import { computed, reactive, ref, watch } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NDrawer,
  NDrawerContent,
  NForm,
  NFormItem,
  NInput,
  NPopconfirm,
  NSpace,
  NTag
} from 'naive-ui'
import type { DataTableColumns, FormRules } from 'naive-ui'
import {
  fetchEdgeNodeUpgradeHistory,
  rollbackEdgeNode,
  upgradeEdgeNode,
  type EdgeNodeEntry,
  type EdgeNodeUpgradeHistoryEntry
} from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'

const props = defineProps<{
  show: boolean
  node: EdgeNodeEntry | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  /** 升级或回滚成功后触发，父页面据此刷新节点列表。 */
  updated: []
}>()

const visible = computed({
  get: () => props.show,
  set: (value) => emit('update:show', value)
})

const upgradeLoading = ref(false)
const upgradeSubmitting = ref(false)
const upgradeHistory = ref<EdgeNodeUpgradeHistoryEntry[]>([])

const upgradeForm = reactive({
  target_version: '',
  package_url: '',
  checksum: '',
  description: ''
})

const upgradeRules: FormRules = {
  target_version: { required: true, message: $t('page.edgeNodes.targetVersionPlaceholder'), trigger: 'blur' }
}

watch(
  () => props.show,
  (show) => {
    if (!show || !props.node) return
    upgradeForm.target_version = ''
    upgradeForm.package_url = ''
    upgradeForm.checksum = ''
    upgradeForm.description = ''
    void loadUpgradeHistory(props.node.id)
  }
)

async function loadUpgradeHistory(nodeId: string) {
  upgradeLoading.value = true
  try {
    const { data } = await fetchEdgeNodeUpgradeHistory(nodeId, 50)
    upgradeHistory.value = data ?? []
  } finally {
    upgradeLoading.value = false
  }
}

async function handleUpgrade() {
  if (!props.node || !upgradeForm.target_version.trim()) return
  upgradeSubmitting.value = true
  try {
    const { error } = await upgradeEdgeNode(props.node.id, {
      target_version: upgradeForm.target_version.trim(),
      package_url: upgradeForm.package_url.trim() || undefined,
      checksum: upgradeForm.checksum.trim() || undefined,
      description: upgradeForm.description.trim() || undefined
    })
    if (!error) {
      upgradeForm.target_version = ''
      upgradeForm.package_url = ''
      upgradeForm.checksum = ''
      upgradeForm.description = ''
      await loadUpgradeHistory(props.node.id)
      emit('updated')
    }
  } finally {
    upgradeSubmitting.value = false
  }
}

async function handleRollback(historyId: string) {
  if (!props.node) return
  upgradeLoading.value = true
  try {
    const { error } = await rollbackEdgeNode(props.node.id, historyId)
    if (!error) {
      await loadUpgradeHistory(props.node.id)
      emit('updated')
    }
  } finally {
    upgradeLoading.value = false
  }
}

const historyColumns: DataTableColumns<EdgeNodeUpgradeHistoryEntry> = [
  { title: $t('page.edgeNodes.fromVersion'), key: 'from_version', width: 90 },
  { title: $t('page.edgeNodes.targetVersion'), key: 'target_version', width: 90 },
  {
    title: $t('page.edgeNodes.health'),
    key: 'status',
    width: 100,
    render: (row) => (
      <NTag
        type={row.status === 'rolled_back' ? 'warning' : row.status === 'dispatched' ? 'info' : 'default'}
        size="small"
      >
        {row.status}
      </NTag>
    )
  },
  {
    title: $t('page.edgeNodes.lastSeen'),
    key: 'created_at',
    width: 160,
    render: (row) => (row.created_at ? formatDateTime(row.created_at) : '-')
  },
  {
    title: $t('page.edgeNodes.actions'),
    key: 'actions',
    width: 80,
    render: (row) => (
      <NPopconfirm onPositiveClick={() => handleRollback(row.id)}>
        {{
          trigger: () => (
            <NButton size="tiny" quaternary type="warning">
              {$t('page.edgeNodes.rollback')}
            </NButton>
          ),
          default: () => $t('page.edgeNodes.rollbackConfirm')
        }}
      </NPopconfirm>
    )
  }
]

defineExpose({ handleUpgrade, handleRollback, upgradeForm, upgradeHistory, loadUpgradeHistory })
</script>

<template>
  <NDrawer v-model:show="visible" :width="600" placement="right">
    <NDrawerContent :title="`${$t('page.edgeNodes.upgradeTitle')} - ${node?.id}`">
      <NSpace vertical :size="20">
        <NCard size="small" :title="$t('page.edgeNodes.executeUpgrade')">
          <NForm :model="upgradeForm" :rules="upgradeRules" label-placement="top">
            <NFormItem label="当前版本">
              <NTag type="info" size="medium">{{ node?.version }}</NTag>
            </NFormItem>
            <NFormItem :label="$t('page.edgeNodes.targetVersion')" path="target_version">
              <NInput
                v-model:value="upgradeForm.target_version"
                :placeholder="$t('page.edgeNodes.targetVersionPlaceholder')"
              />
            </NFormItem>
            <NFormItem label="升级包 URL (可选)">
              <NInput v-model:value="upgradeForm.package_url" placeholder="https://..." />
            </NFormItem>
            <NFormItem label="校验和 Checksum (可选)">
              <NInput v-model:value="upgradeForm.checksum" placeholder="sha256:..." />
            </NFormItem>
            <NFormItem label="升级说明 (可选)">
              <NInput v-model:value="upgradeForm.description" placeholder="升级说明..." />
            </NFormItem>
            <NButton type="primary" block :loading="upgradeSubmitting" @click="handleUpgrade">
              {{ $t('page.edgeNodes.executeUpgrade') }}
            </NButton>
          </NForm>
        </NCard>

        <div>
          <div class="text-base font-semibold mb-3">{{ $t('page.edgeNodes.upgradeHistory') }}</div>
          <NDataTable
            remote
            :columns="historyColumns"
            :data="upgradeHistory"
            :loading="upgradeLoading"
            :pagination="false"
            :row-key="(row: EdgeNodeUpgradeHistoryEntry) => row.id"
            size="small"
          />
        </div>
      </NSpace>
    </NDrawerContent>
  </NDrawer>
</template>
