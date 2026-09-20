<!--
文件用途: 物模型版本历史与升级/回滚抽屉组件 (ROADMAP P1.6 模板市场产品化)。
核心逻辑:
  1. 展示指定物模型的版本升级历史记录（回滚点列表）。
  2. 支持上传/导入新版本物模型载荷执行升级。
  3. 支持针对历史版本执行一键幂等回滚。
关键注意事项: 升级必须使用新于当前版本的点分版本；回滚重放历史载荷不删历史行。
-->
<script setup lang="ts">
import { ref, watch, h } from 'vue'
import {
  NDrawer,
  NDrawerContent,
  NButton,
  NDataTable,
  NEmpty,
  NSpace,
  NTag,
  NPopconfirm,
  NUpload,
  NCard,
  NAlert,
  type UploadFileInfo
} from 'naive-ui'
import { $t } from '@/locales'
import {
  getDeviceTemplateUpgradeHistory,
  upgradeDeviceTemplate,
  rollbackDeviceTemplateUpgrade
} from '@/service/api/device-template-model'

export interface Props {
  visible: boolean
  templateName: string
  currentVersion?: string
}

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  templateName: '',
  currentVersion: ''
})

const emit = defineEmits<{
  'update:visible': [visible: boolean]
  success: []
}>()

const loading = ref(false)
const historyList = ref<any[]>([])
const upgrading = ref(false)

const fetchHistory = async () => {
  if (!props.templateName) return
  loading.value = true
  try {
    const res = await getDeviceTemplateUpgradeHistory({ template_name: props.templateName })
    if (!res.error) {
      historyList.value = Array.isArray(res.data) ? res.data : res.data?.list || []
    }
  } catch (err) {
    console.error('Failed to load template upgrade history:', err)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      fetchHistory()
    }
  }
)

const handleRollback = async (historyId: string) => {
  loading.value = true
  try {
    const res = await rollbackDeviceTemplateUpgrade(historyId)
    if (!res.error) {
      window.$message?.success('物模型版本回滚成功')
      emit('success')
      await fetchHistory()
    } else {
      window.$message?.error(res.error.message || '回滚失败')
    }
  } catch (err: any) {
    window.$message?.error(err.message || '回滚请求失败')
  } finally {
    loading.value = false
  }
}

const handleUploadChange = async (options: { file: UploadFileInfo }) => {
  const file = options.file.file
  if (!file) return
  const reader = new FileReader()
  reader.onload = async (e) => {
    try {
      const content = e.target?.result as string
      const payload = JSON.parse(content)
      upgrading.value = true
      const res = await upgradeDeviceTemplate({ payload })
      if (!res.error) {
        window.$message?.success('物模型升级成功')
        emit('success')
        await fetchHistory()
      } else {
        window.$message?.error(res.error.message || '升级失败')
      }
    } catch (parseErr: any) {
      window.$message?.error('解析物模型文件失败: ' + (parseErr.message || '非合法 JSON'))
    } finally {
      upgrading.value = false
    }
  }
  reader.readAsText(file)
}

const columns = [
  {
    title: '目标版本',
    key: 'to_version',
    render(row: any) {
      return h(NTag, { type: 'primary', size: 'small' }, { default: () => row.to_version || row.version || '--' })
    }
  },
  {
    title: '源版本',
    key: 'from_version',
    render(row: any) {
      return row.from_version ? h(NTag, { size: 'small' }, { default: () => row.from_version }) : '初始版本'
    }
  },
  {
    title: '升级时间',
    key: 'created_at',
    render(row: any) {
      return row.created_at ? new Date(row.created_at).toLocaleString() : '--'
    }
  },
  {
    title: '操作人',
    key: 'operator_user_id',
    render(row: any) {
      return row.operator_user_id ? String(row.operator_user_id).slice(0, 8) + '...' : '--'
    }
  },
  {
    title: '操作',
    key: 'actions',
    render(row: any) {
      return h(
        NPopconfirm,
        {
          onPositiveClick: () => handleRollback(row.id)
        },
        {
          default: () => `确认将物模型回滚到版本 ${row.from_version || '前置版本'} 吗？`,
          trigger: () =>
            h(
              NButton,
              {
                size: 'tiny',
                type: 'warning',
                disabled: loading.value
              },
              { default: () => '回滚到此点' }
            )
        }
      )
    }
  }
]
</script>

<template>
  <NDrawer :show="visible" :width="700" placement="right" @update:show="(val: boolean) => emit('update:visible', val)">
    <NDrawerContent :title="`物模型版本管理与升级回滚 - ${templateName}`" closable>
      <NSpace vertical size="large">
        <NAlert type="info" title="版本升级说明">
          物模型升级需提供点分数字版本（例如 1.2.0）的物模型导出 JSON
          文件。目标版本必须严格新于当前版本。回滚操作为安全幂等重放，不删除任何历史审计点。
        </NAlert>

        <NCard title="快速升级版本" size="small">
          <NSpace align="center">
            <NUpload :show-file-list="false" accept=".json" :custom-request="() => {}" @change="handleUploadChange">
              <NButton type="primary" :loading="upgrading">上传新版本 JSON 升级</NButton>
            </NUpload>
            <span v-if="currentVersion" class="text-xs text-gray-500">当前版本: {{ currentVersion }}</span>
          </NSpace>
        </NCard>

        <NCard title="历史升级点与回滚" size="small">
          <!--
            naive-ui 的 NDataTable 只有 `empty` 插槽，**没有** `empty-text` prop
            （见 naive-ui/es/data-table/src/interface.d.ts 的 DataTableSlots）。
            此前写成 `empty-text="暂无版本升级历史"` 会退化成 fallthrough 属性被静默丢弃，
            空态既不显示自定义文案也不报错——2026-09-16 由浏览器 E2E 抓出。
          -->
          <NDataTable :loading="loading" :data="historyList" :columns="columns" :pagination="false">
            <template #empty>
              <NEmpty description="暂无版本升级历史" />
            </template>
          </NDataTable>
        </NCard>
      </NSpace>
    </NDrawerContent>
  </NDrawer>
</template>
