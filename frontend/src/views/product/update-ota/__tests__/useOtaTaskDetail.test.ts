import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { formatOtaTaskTime, useOtaTaskDetail } from '../useOtaTaskDetail'

const t = (key: string) => key

describe('useOtaTaskDetail', () => {
  it('builds status options and rollout summary bindings for the task detail modal', () => {
    const detail = useOtaTaskDetail({
      selectedPackage: ref({ id: 'pkg-1', version: '2.0.0' }),
      selectedTask: ref(null),
      detailLoading: ref(false),
      detailList: ref([]),
      detailStatistics: ref([
        { status: 4, count: 3 },
        { status: 5, count: 1 }
      ]),
      loadTaskDetail: vi.fn(),
      fetchTaskDetails: vi.fn(),
      editTaskDetail: vi.fn(),
      t,
      message: {},
      dialog: {}
    })

    expect(detail.statusOptions.value[0]).toEqual({
      label: 'page.product.update-ota.allStatus',
      value: 0
    })
    expect(detail.rolloutFailedCount.value).toBe(1)
    expect(detail.rolloutSuccessRate.value).toBe('75%')
    expect(detail.rolloutSummaryItems.value.map((item) => item.key)).toEqual([
      'total',
      'pending',
      'pushed',
      'upgrading',
      'success',
      'failed',
      'canceled'
    ])
    expect(detail.detailColumns.length).toBeGreaterThan(0)
  })

  it('formats empty and real OTA timestamps consistently', () => {
    expect(formatOtaTaskTime()).toBe('-')
    expect(formatOtaTaskTime('2026-07-05T01:02:03Z')).toContain('2026-07-05')
  })

  it('copies an OTA failed-rollout support package from loaded detail state', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText }
    })
    const message = {
      success: vi.fn(),
      warning: vi.fn()
    }
    const detail = useOtaTaskDetail({
      selectedPackage: ref({ id: 'pkg-1', name: 'Pkg', version: '2.0.0' }),
      selectedTask: ref({ id: 'task-1', name: 'Factory rollout', device_count: 2 }),
      detailLoading: ref(false),
      detailList: ref([
        {
          id: 'detail-1',
          name: 'Pump A',
          device_number: 'SN-001',
          current_version: '1.0.0',
          version: '2.0.0',
          status: 5,
          status_description: 'download failed'
        }
      ]),
      detailStatistics: ref([{ status: 5, count: 1 }]),
      loadTaskDetail: vi.fn(),
      fetchTaskDetails: vi.fn(),
      editTaskDetail: vi.fn(),
      t,
      message,
      dialog: {}
    })

    await detail.copyFailureSupportBundle()

    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('taskId=task-1'))
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('download failed'))
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('Evidence boundary'))
    expect(message.success).toHaveBeenCalledWith('page.product.update-ota.failureSupportBundleCopied')
  })

  it('maps OTA task status codes to labels and tag types', () => {
    const detail = useOtaTaskDetail({
      selectedPackage: ref(null),
      selectedTask: ref(null),
      detailLoading: ref(false),
      detailList: ref([]),
      detailStatistics: ref([]),
      loadTaskDetail: vi.fn(),
      fetchTaskDetails: vi.fn(),
      editTaskDetail: vi.fn(),
      t,
      message: {},
      dialog: {}
    })

    expect(detail.statusLabel(1)).toBe('page.product.update-ota.pendingTask')
    expect(detail.statusLabel(2)).toBe('page.product.update-ota.pushTask')
    expect(detail.statusLabel(3)).toBe('page.product.update-ota.upgradingTask')
    expect(detail.statusLabel(4)).toBe('page.product.update-ota.completeTask')
    expect(detail.statusLabel(5)).toBe('page.product.update-ota.failTask')
    expect(detail.statusLabel(6)).toBe('page.product.update-ota.cancelTask')
    // 未知状态回落成原始码，未传状态回落成占位符，避免表格出现空单元格
    expect(detail.statusLabel(99)).toBe('99')
    expect(detail.statusLabel(undefined)).toBe('-')

    expect(detail.statusTagType(4)).toBe('success')
    expect(detail.statusTagType(5)).toBe('error')
    expect(detail.statusTagType(3)).toBe('warning')
    expect(detail.statusTagType(6)).toBe('default')
    expect(detail.statusTagType(1)).toBe('info')
  })

  it('confirms cancel and retry detail actions before calling the edit API', async () => {
    const editTaskDetail = vi.fn().mockResolvedValue({ error: null })
    const fetchTaskDetails = vi.fn().mockResolvedValue(undefined)
    const dialogWarning = vi.fn()
    const message = { success: vi.fn(), warning: vi.fn() }

    const detail = useOtaTaskDetail({
      selectedPackage: ref({ id: 'pkg-1', version: '2.0.0' }),
      selectedTask: ref({ id: 'task-1', name: 'Task 1' }),
      detailLoading: ref(false),
      detailList: ref([]),
      detailStatistics: ref([]),
      loadTaskDetail: vi.fn(),
      fetchTaskDetails,
      editTaskDetail,
      t,
      message,
      dialog: { warning: dialogWarning }
    })

    // 取消动作：必须先弹确认框，确认后才调用后端
    detail.updateTaskDetailStatus({ id: 'detail-1', name: 'Device A', status: 3 } as any, 6 as any)
    expect(editTaskDetail).not.toHaveBeenCalled()
    await dialogWarning.mock.calls[0][0].onPositiveClick()
    expect(editTaskDetail).toHaveBeenNthCalledWith(1, { id: 'detail-1', action: 6 })

    // 重试动作走同一条确认链路
    detail.updateTaskDetailStatus({ id: 'detail-1', name: 'Device A', status: 6 } as any, 1 as any)
    await dialogWarning.mock.calls[1][0].onPositiveClick()
    expect(editTaskDetail).toHaveBeenNthCalledWith(2, { id: 'detail-1', action: 1 })

    expect(message.success).toHaveBeenCalledWith('common.operationSuccess')
  })

  it('does not refresh detail rows when the edit API reports an error', async () => {
    const editTaskDetail = vi.fn().mockResolvedValue({ error: { message: 'boom' } })
    const fetchTaskDetails = vi.fn().mockResolvedValue(undefined)
    const dialogWarning = vi.fn()
    const message = { success: vi.fn(), warning: vi.fn() }

    const detail = useOtaTaskDetail({
      selectedPackage: ref({ id: 'pkg-1', version: '2.0.0' }),
      selectedTask: ref({ id: 'task-1', name: 'Task 1' }),
      detailLoading: ref(false),
      detailList: ref([]),
      detailStatistics: ref([]),
      loadTaskDetail: vi.fn(),
      fetchTaskDetails,
      editTaskDetail,
      t,
      message,
      dialog: { warning: dialogWarning }
    })

    detail.updateTaskDetailStatus({ id: 'detail-1', name: 'Device A', status: 3 } as any, 6 as any)
    await dialogWarning.mock.calls[0][0].onPositiveClick()

    expect(editTaskDetail).toHaveBeenCalledTimes(1)
    expect(message.success).not.toHaveBeenCalled()
  })
})
