/*
 * "ready-check" deep-link context for the OTA task page.
 *
 * The device ready-check view links back with `source=ready-check&ota_task_id=...&ota_detail_id=...`;
 * this composable resolves those ids against the loaded task/detail lists and reports whether the
 * referenced task still exists, so the page can explain a stale link instead of silently ignoring it.
 */
import { computed, ref } from 'vue'
import type { Ref } from 'vue'
import { useRoute } from 'vue-router'
import { $t } from '@/locales'

export type ReadyCheckOtaContextStatus = 'idle' | 'matched' | 'not-found'

export function normalizeRouteQueryText(value: unknown) {
  const rawValue = Array.isArray(value) ? value[0] : value
  return typeof rawValue === 'string' ? rawValue.trim() : ''
}

interface ReadyCheckOptions {
  taskList: Ref<Array<{ id: string }>>
  detailList: Ref<Array<{ id: string }>>
  selectedTask: Ref<{ id: string } | null | undefined>
  detailModalVisible: Ref<boolean>
  openTaskDetail: (task: any) => Promise<void> | void
}

export function useOtaReadyCheckContext(options: ReadyCheckOptions) {
  const route = useRoute()

  const isReadyCheckOtaSource = computed(() => normalizeRouteQueryText(route.query.source) === 'ready-check')
  const readyCheckOtaTaskId = computed(() => normalizeRouteQueryText(route.query.ota_task_id))
  const readyCheckOtaDetailId = computed(() => normalizeRouteQueryText(route.query.ota_detail_id))
  const readyCheckOtaContextStatus = ref<ReadyCheckOtaContextStatus>('idle')

  const readyCheckOtaDetailMatched = computed(() => {
    if (!readyCheckOtaDetailId.value) return false
    return options.detailList.value.some(item => item.id === readyCheckOtaDetailId.value)
  })

  const readyCheckOtaContextVisible = computed(
    () => isReadyCheckOtaSource.value && Boolean(readyCheckOtaTaskId.value || readyCheckOtaDetailId.value)
  )

  const readyCheckOtaContextType = computed(() => {
    if (readyCheckOtaContextStatus.value === 'not-found') return 'warning'
    return readyCheckOtaContextStatus.value === 'matched' ? 'success' : 'info'
  })

  const readyCheckOtaContextMessage = computed(() => {
    if (!readyCheckOtaContextVisible.value) return ''
    if (readyCheckOtaContextStatus.value === 'not-found') {
      return $t('page.product.update-ota.readyCheckContextTaskMissing').replace(
        '{taskId}',
        readyCheckOtaTaskId.value || '--'
      )
    }
    if (readyCheckOtaContextStatus.value === 'matched') {
      return $t('page.product.update-ota.readyCheckContextTaskMatched')
        .replace('{taskId}', readyCheckOtaTaskId.value || '--')
        .replace('{detailId}', readyCheckOtaDetailId.value || '--')
    }
    return $t('page.product.update-ota.readyCheckContextPreserved')
      .replace('{taskId}', readyCheckOtaTaskId.value || '--')
      .replace('{detailId}', readyCheckOtaDetailId.value || '--')
  })

  const readyCheckOtaDetailContextMessage = computed(() => {
    if (!readyCheckOtaContextVisible.value || !readyCheckOtaDetailId.value) return ''
    return readyCheckOtaDetailMatched.value
      ? $t('page.product.update-ota.readyCheckDetailMatched').replace('{detailId}', readyCheckOtaDetailId.value)
      : $t('page.product.update-ota.readyCheckDetailPreserved').replace('{detailId}', readyCheckOtaDetailId.value)
  })

  async function applyReadyCheckOtaContext() {
    if (!isReadyCheckOtaSource.value || !readyCheckOtaTaskId.value) return
    const matchedTask = options.taskList.value.find(item => item.id === readyCheckOtaTaskId.value)
    if (!matchedTask) {
      readyCheckOtaContextStatus.value = 'not-found'
      return
    }
    readyCheckOtaContextStatus.value = 'matched'
    if (options.selectedTask.value?.id === matchedTask.id && options.detailModalVisible.value) return
    await options.openTaskDetail(matchedTask)
  }

  return {
    isReadyCheckOtaSource,
    readyCheckOtaContextVisible,
    readyCheckOtaContextType,
    readyCheckOtaContextMessage,
    readyCheckOtaDetailContextMessage,
    readyCheckOtaContextStatus,
    readyCheckOtaDetailMatched,
    applyReadyCheckOtaContext
  }
}
