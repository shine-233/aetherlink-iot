/**
 * 文件用途：定时报表计划的列表、搜索分页、创建/编辑表单与删除。
 * 核心逻辑：计划列表的分页/加载态/过期请求丢弃收口在 useListPage（@/components/data-table-page/useListPage）；
 *   失败态 fail-closed：请求异常时置 listFailed 并提示「加载失败」，行数据保留当前快照（useListPage 的 null 契约）；
 *   更新/删除遇到修订冲突（结构化错误码）时拉取最新计划回填并提示「已被他人修改」，而不是笼统报错。
 * 关键注意事项：卸载时 useListPage 通过 onScopeDispose 自动作废在途请求，dispose() 保留给显式触发；
 *   rows 为 shallowRef，行内回填采用不可变替换以触发重渲染。
 */
import { reactive, ref } from 'vue'
import type { FormInst } from 'naive-ui'
import { $t } from '@/locales'
import { useListPage } from '@/components/data-table-page/useListPage'
import {
  createReportSchedule,
  deleteReportSchedule,
  getReportSchedule,
  listReportSchedules,
  updateReportSchedule,
  type ReportSchedule,
  type ReportSchedulePayload,
  type UpdateReportSchedulePayload
} from '@/service/api/report'
import {
  REPORT_PAGE_SIZE,
  emptyReportForm,
  isReportRevisionConflict,
  reportFormFromSchedule,
  splitReportLines
} from './report-helpers'

export interface ReportMessenger {
  success: (text: string) => void
  warning: (text: string) => void
  error: (text: string) => void
}

interface ScheduleListQuery {
  search: string
}

export function useReportSchedules(message: ReportMessenger) {
  const searchInput = ref('')
  const listFailed = ref(false)

  // 列表状态机：分页/加载/过期请求丢弃交给 useListPage，search 为唯一过滤条件。
  const {
    rows: schedules,
    total,
    page,
    loading: listLoading,
    query: listQuery,
    load: loadSchedules,
    search: searchListPage,
    setPage,
    cancel: cancelListLoad
  } = useListPage<ReportSchedule, ScheduleListQuery>({
    initialQuery: () => ({ search: '' }),
    initialPageSize: REPORT_PAGE_SIZE,
    fetcher: async (params) => {
      listFailed.value = false
      try {
        const { data, error } = await listReportSchedules({
          page: params.page,
          page_size: params.page_size,
          search: params.search
        })
        if (error || !data) throw error || new Error('missing data')
        return { list: data.list || [], total: data.total || 0 }
      } catch {
        listFailed.value = true
        message.error($t('report.message.loadFailed'))
        return null
      }
    }
  })

  const showForm = ref(false)
  const editing = ref<ReportSchedule | null>(null)
  const saving = ref(false)
  const deletingId = ref('')
  const formRef = ref<Pick<FormInst, 'validate'> | null>(null)
  const form = reactive<ReportSchedulePayload>(emptyReportForm())
  const deviceIdsText = ref('')
  const keysText = ref('')

  const fillForm = (schedule?: ReportSchedule) => {
    const next = reportFormFromSchedule(schedule)
    if (!schedule) delete form.revision
    Object.assign(form, next)
    deviceIdsText.value = next.device_ids.join('\n')
    keysText.value = next.keys.join('\n')
  }

  /** 修订冲突后拉取最新计划：优先单条接口，失败则整页重载后在列表中查找。 */
  const refreshStaleSchedule = async (id: string) => {
    const { data, error } = await getReportSchedule(id)
    let refreshed: ReportSchedule | undefined = data || undefined
    if (error || !refreshed) {
      await loadSchedules()
      refreshed = schedules.value.find((schedule) => schedule.id === id)
    } else {
      // rows 为 shallowRef：单行回填用不可变替换触发重渲染。
      const next = refreshed
      schedules.value = schedules.value.map((schedule) => (schedule.id === id ? next : schedule))
    }
    if (refreshed && editing.value?.id === id) {
      editing.value = refreshed
      fillForm(refreshed)
    }
    return refreshed
  }

  /** 冲突处理：确实被他人改过（revision 变化）→ warning；否则按普通失败处理。 */
  const handleConflict = async (error: unknown, id: string, revision: number, staleKey: string, failKey: string) => {
    if (isReportRevisionConflict(error)) {
      const refreshed = await refreshStaleSchedule(id)
      if (refreshed && refreshed.revision !== revision) {
        message.warning($t(staleKey))
        return
      }
    }
    message.error($t(failKey))
  }

  function searchSchedules() {
    listQuery.search = searchInput.value.trim()
    return searchListPage()
  }
  function changePage(next: number) {
    return setPage(next)
  }
  function openCreate() {
    editing.value = null
    fillForm()
    showForm.value = true
  }
  function openEdit(row: ReportSchedule) {
    editing.value = row
    fillForm(row)
    showForm.value = true
  }

  async function saveSchedule() {
    form.device_ids = splitReportLines(deviceIdsText.value)
    form.keys = splitReportLines(keysText.value)
    try {
      await formRef.value?.validate()
    } catch {
      return
    }
    if (!form.device_ids.length || !form.keys.length) {
      message.error($t('report.message.targetsRequired'))
      return
    }
    const target = editing.value
    saving.value = true
    try {
      const result = target
        ? await updateReportSchedule(target.id, {
            ...form,
            revision: form.revision ?? target.revision
          } satisfies UpdateReportSchedulePayload)
        : await createReportSchedule({ ...form })
      if (result.error) throw result.error
      message.success($t(target ? 'report.message.updated' : 'report.message.created'))
      showForm.value = false
      await loadSchedules()
    } catch (error) {
      if (target) {
        await handleConflict(
          error,
          target.id,
          target.revision,
          'report.message.staleRevision',
          'report.message.saveFailed'
        )
      } else {
        message.error($t('report.message.saveFailed'))
      }
    } finally {
      saving.value = false
    }
  }

  async function removeSchedule(row: ReportSchedule) {
    deletingId.value = row.id
    try {
      const { error } = await deleteReportSchedule(row.id, row.revision)
      if (error) throw error
      message.success($t('report.message.deleted'))
      // 删掉当前页最后一条时回退一页（与旧实现一致）；useListPage 的空页回退仅兜底非预判场景。
      if (schedules.value.length === 1 && page.value > 1) {
        await setPage(page.value - 1)
      } else {
        await loadSchedules()
      }
    } catch (error) {
      await handleConflict(
        error,
        row.id,
        row.revision,
        'report.message.deleteStaleRevision',
        'report.message.deleteFailed'
      )
    } finally {
      deletingId.value = ''
    }
  }

  const dispose = () => {
    cancelListLoad()
  }

  return {
    schedules,
    total,
    page,
    searchInput,
    listLoading,
    listFailed,
    showForm,
    editing,
    saving,
    deletingId,
    formRef,
    form,
    deviceIdsText,
    keysText,
    loadSchedules,
    searchSchedules,
    changePage,
    openCreate,
    openEdit,
    saveSchedule,
    removeSchedule,
    dispose
  }
}
