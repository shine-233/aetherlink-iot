/**
 * 文件用途：定时报表计划的列表、搜索分页、创建/编辑表单与删除。
 * 核心逻辑：列表请求带序号与快照，过期响应直接丢弃；更新/删除遇到修订冲突（结构化错误码）时
 *   拉取最新计划回填并提示「已被他人修改」，而不是笼统报错。
 * 关键注意事项：卸载时调用 dispose() 使在途响应失效。
 */
import { reactive, ref } from 'vue'
import type { FormInst } from 'naive-ui'
import { $t } from '@/locales'
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

export function useReportSchedules(message: ReportMessenger) {
  const schedules = ref<ReportSchedule[]>([])
  const total = ref(0)
  const page = ref(1)
  const searchInput = ref('')
  const search = ref('')
  const listLoading = ref(false)
  const listFailed = ref(false)
  let sequence = 0

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

  async function loadSchedules() {
    const seq = ++sequence
    const snapshot = { page: page.value, search: search.value }
    listLoading.value = true
    listFailed.value = false
    try {
      const { data, error } = await listReportSchedules({
        page: snapshot.page,
        page_size: REPORT_PAGE_SIZE,
        search: snapshot.search
      })
      if (seq !== sequence || snapshot.page !== page.value || snapshot.search !== search.value) return
      if (error || !data) throw error || new Error('missing data')
      schedules.value = data.list || []
      total.value = data.total || 0
    } catch {
      if (seq !== sequence) return
      schedules.value = []
      total.value = 0
      listFailed.value = true
      message.error($t('report.message.loadFailed'))
    } finally {
      if (seq === sequence) listLoading.value = false
    }
  }

  /** 修订冲突后拉取最新计划：优先单条接口，失败则整页重载后在列表中查找。 */
  const refreshStaleSchedule = async (id: string) => {
    const { data, error } = await getReportSchedule(id)
    let refreshed: ReportSchedule | undefined = data || undefined
    if (error || !refreshed) {
      await loadSchedules()
      refreshed = schedules.value.find((schedule) => schedule.id === id)
    } else {
      const index = schedules.value.findIndex((schedule) => schedule.id === id)
      if (index >= 0) schedules.value.splice(index, 1, refreshed)
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
    search.value = searchInput.value.trim()
    page.value = 1
    void loadSchedules()
  }
  function changePage(next: number) {
    page.value = next
    void loadSchedules()
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
      if (schedules.value.length === 1 && page.value > 1) page.value -= 1
      await loadSchedules()
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
    sequence += 1
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
