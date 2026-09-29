<!--
  文件用途：调度注册行（origin = scheduler_registry）列表弹窗：编辑/删除入口。
  核心逻辑：打开时拉取不带窗口的注册列表（含已停用/窗外登记行）；变更后由父组件调用 reload()。
-->
<script setup lang="ts">
import { ref, watch } from 'vue'
import { NButton, NEmpty, NModal, NPopconfirm, NSpace, NSpin, NTag, useMessage } from 'naive-ui'
import { deleteSchedulerEvent, getSchedulerEvents, type SchedulerEventItem } from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import { SOURCE_TAG_TYPE, sourceLabelKey } from '../calendar-model'

const show = defineModel<boolean>('show', { required: true })
const emit = defineEmits<{
  (event: 'create'): void
  (event: 'edit', row: SchedulerEventItem): void
  (event: 'deleted'): void
}>()

const message = useMessage()
const loading = ref(false)
const rows = ref<SchedulerEventItem[]>([])
let requestSeq = 0

async function reload() {
  const seq = ++requestSeq
  loading.value = true
  try {
    const response = await getSchedulerEvents({ page_size: 500 })
    // 丢弃过期响应：快速连续删除时，旧请求晚到不得覆盖新列表。
    if (seq !== requestSeq) return
    rows.value = (response.data?.list ?? []).filter((item) => item.origin === 'scheduler_registry')
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

async function remove(id: string) {
  await deleteSchedulerEvent(id)
  message.success($t('common.deleteSuccess'))
  await reload()
  emit('deleted')
}

watch(show, (open) => {
  if (open) void reload()
})

defineExpose({ reload })
</script>

<template>
  <NModal
    v-model:show="show"
    preset="card"
    :title="$t('page.schedulerCalendar.registryTitle')"
    class="scheduler-registry-modal"
  >
    <NSpace justify="end">
      <NButton size="small" type="primary" @click="emit('create')">
        {{ $t('page.schedulerCalendar.createEvent') }}
      </NButton>
    </NSpace>
    <NSpin :show="loading">
      <div v-if="rows.length === 0 && !loading">
        <NEmpty :description="$t('page.schedulerCalendar.noRegistryRows')" />
      </div>
      <table v-else class="scheduler-registry-table">
        <thead>
          <tr>
            <th scope="col">{{ $t('page.schedulerCalendar.formName') }}</th>
            <th scope="col">{{ $t('page.schedulerCalendar.formType') }}</th>
            <th scope="col">{{ $t('page.schedulerCalendar.formRefId') }}</th>
            <th scope="col">{{ $t('page.schedulerCalendar.formCron') }}</th>
            <th scope="col">{{ $t('page.schedulerCalendar.nextRun') }}</th>
            <th scope="col">{{ $t('page.schedulerCalendar.enabled') }}</th>
            <th scope="col">{{ $t('page.schedulerCalendar.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td>{{ row.name }}</td>
            <td>
              <NTag size="small" :type="SOURCE_TAG_TYPE[row.source_type]">
                {{ $t(sourceLabelKey(row.source_type)) }}
              </NTag>
            </td>
            <td class="scheduler-registry-table__mono">{{ row.ref_id || '-' }}</td>
            <td class="scheduler-registry-table__mono">{{ row.cron || '-' }}</td>
            <td>{{ row.next_run_at ? formatDateTime(row.next_run_at) : '-' }}</td>
            <td>{{ row.enabled ? $t('page.schedulerCalendar.enabledOn') : $t('page.schedulerCalendar.disabled') }}</td>
            <td>
              <NSpace size="small">
                <NButton size="tiny" @click="emit('edit', row)">{{ $t('page.schedulerCalendar.edit') }}</NButton>
                <NPopconfirm @positive-click="remove(row.id)">
                  <template #trigger>
                    <NButton size="tiny" type="error">{{ $t('page.schedulerCalendar.delete') }}</NButton>
                  </template>
                  {{ $t('page.schedulerCalendar.confirmDelete') }}
                </NPopconfirm>
              </NSpace>
            </td>
          </tr>
        </tbody>
      </table>
    </NSpin>
  </NModal>
</template>

<style scoped>
.scheduler-registry-modal {
  width: 720px;
}

.scheduler-registry-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.scheduler-registry-table th,
.scheduler-registry-table td {
  border-bottom: 1px solid var(--border-color);
  padding: 6px 8px;
  text-align: left;
}

.scheduler-registry-table__mono {
  font-family: monospace;
}
</style>
