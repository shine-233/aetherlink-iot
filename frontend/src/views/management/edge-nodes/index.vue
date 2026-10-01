<!--
文件用途：P1.5 边缘节点管理页——注册表列表、健康状态、证书签发与版本升级/回滚工作台。
核心逻辑：
1. 列表状态（rows/loading/分页/过期请求取消）收口在 useListPage；/edge/nodes 仅支持 limit
   拉全量（上限 200），fetcher 取回后按 page/page_size 本地切片，默认每页 200 与拆分前一致；
2. 注册、证书、升级/回滚三个工作台拆到 modules/ 子组件，注册与升级/回滚成功后回刷列表；
3. 节点注册走幂等更新；证书为 mTLS 凭证，私钥仅签发当次展示——细节见各子组件。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NCard, NDataTable } from 'naive-ui'
import { fetchEdgeNodes, heartbeatEdgeNode, type EdgeNodeEntry } from '@/service/api'
import { $t } from '@/locales'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'
import { createNodeColumns } from './node-columns'
import NodeCertModal from './modules/node-cert-modal.vue'
import NodeRegisterModal from './modules/node-register-modal.vue'
import NodeUpgradeDrawer from './modules/node-upgrade-drawer.vue'

/** 与拆分前一致：列表一次最多取 200 个节点（后端 /edge/nodes 的 limit 上限口径）。 */
const NODE_LIST_LIMIT = 200

const {
  rows: nodes,
  loading,
  pagination,
  rowKey,
  load: loadNodes
} = useListPage<EdgeNodeEntry>({
  initialPageSize: NODE_LIST_LIMIT,
  pageSizes: [20, 50, 100, NODE_LIST_LIMIT],
  rowKey: (row) => row.id,
  fetcher: async (params) => {
    const result = fromFlatResponse<EdgeNodeEntry>(await fetchEdgeNodes(NODE_LIST_LIMIT))
    if (!result) return null
    // 后端不支持 page/page_size：取回全量后在这里本地切片。
    const start = (params.page - 1) * params.page_size
    return { list: result.list.slice(start, start + params.page_size), total: result.total }
  }
})

// 注册 / 证书 / 升级三个工作台的打开状态；证书与升级共用当前选中节点。
const registerVisible = ref(false)
const certVisible = ref(false)
const upgradeVisible = ref(false)
const activeNode = ref<EdgeNodeEntry | null>(null)

const columns = createNodeColumns({
  onHeartbeat: handleHeartbeat,
  onCertificate: (row) => {
    activeNode.value = row
    certVisible.value = true
  },
  onUpgrade: (row) => {
    activeNode.value = row
    upgradeVisible.value = true
  }
})

async function handleHeartbeat(nodeId: string) {
  await heartbeatEdgeNode(nodeId)
  await loadNodes()
}

onMounted(() => {
  void loadNodes()
})
</script>

<template>
  <div class="min-h-500px">
    <NCard :title="$t('page.edgeNodes.title')" class="h-full">
      <template #header-extra>
        <NButton size="small" type="primary" @click="registerVisible = true">
          {{ $t('page.edgeNodes.register') }}
        </NButton>
      </template>

      <NDataTable
        remote
        :columns="columns"
        :data="nodes"
        :loading="loading"
        :pagination="pagination"
        :row-key="rowKey"
        :scroll-x="900"
      />
    </NCard>

    <!-- 注册节点模态框 -->
    <NodeRegisterModal v-model:show="registerVisible" @registered="loadNodes" />

    <!-- 证书管理模态框 -->
    <NodeCertModal v-model:show="certVisible" :node="activeNode" />

    <!-- 版本升级与历史抽屉 -->
    <NodeUpgradeDrawer v-model:show="upgradeVisible" :node="activeNode" @updated="loadNodes" />
  </div>
</template>
