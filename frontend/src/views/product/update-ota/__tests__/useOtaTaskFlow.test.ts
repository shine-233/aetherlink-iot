import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { useOtaTaskFlow } from '../useOtaTaskFlow'
import {
  FLEET_CURRENT_PAGE_SCOPE,
  FLEET_DEVICE_FILTER_SCOPE,
  FLEET_FILTER_RESULT_SCOPE
} from '../../../device/modules/fleet-rollout-context'

const t = (key: string) => key

function createFlowHarness(fleetRolloutContext = ref(null as any)) {
  const fetchDevices = vi.fn().mockResolvedValue(undefined)
  const fetchTasks = vi.fn().mockResolvedValue(undefined)
  const clearDeviceCandidates = vi.fn()
  const addTask = vi.fn().mockResolvedValue({ error: null })
  const previewTask = vi.fn().mockResolvedValue({
    data: { selected_count: 42, total_matched: 42, over_limit: false, max_devices: 5000 },
    error: null
  })
  const message = {
    success: vi.fn(),
    warning: vi.fn()
  }

  const flow = useOtaTaskFlow({
    data: {
      selectedPackageId: ref<string | null>('pkg-1'),
      selectedPackage: ref({ id: 'pkg-1', target_version: '2.0' }),
      deviceCandidates: ref([
        { id: 'dev-1', name: 'Offline Device', current_version: '1.0', is_online: 0 },
        { id: 'dev-2', name: 'Current Device', current_version: '2.0' }
      ]),
      deviceOptions: ref([{ label: 'Offline Device', value: 'dev-1' }]),
      fetchDevices,
      fetchTasks,
      clearDeviceCandidates
    },
    services: {
      addTask,
      previewTask
    },
    t,
    message,
    fleetRolloutContext
  })

  return {
    flow,
    fetchDevices,
    fetchTasks,
    clearDeviceCandidates,
    addTask,
    previewTask,
    message
  }
}

describe('useOtaTaskFlow', () => {
  it('opens the task modal after resetting form state and loading eligible devices', async () => {
    const harness = createFlowHarness()

    await harness.flow.openTaskModal()

    expect(harness.clearDeviceCandidates).toHaveBeenCalledTimes(1)
    expect(harness.fetchDevices).toHaveBeenCalledTimes(1)
    expect(harness.flow.taskModalVisible.value).toBe(true)
    expect(harness.message.warning).not.toHaveBeenCalled()
  })

  it('saves the current task form without changing the backend create-task payload', async () => {
    const harness = createFlowHarness()
    harness.flow.taskForm.name = '  Batch 1  '
    harness.flow.taskForm.description = '  staged rollout  '
    harness.flow.taskForm.device_id_list = ['dev-1']

    await harness.flow.saveTask()

    expect(harness.addTask).toHaveBeenCalledWith({
      name: 'Batch 1',
      ota_upgrade_package_id: 'pkg-1',
      description: 'staged rollout',
      device_id_list: ['dev-1']
    })
    expect(harness.message.success).toHaveBeenCalledWith('common.saveSuccess')
    expect(harness.fetchTasks).toHaveBeenCalledTimes(1)
    expect(harness.flow.taskModalVisible.value).toBe(false)
  })

  it('saves a fleet filter rollout as a backend device_filter contract instead of a current-page id list', async () => {
    const fleetRolloutContext = ref({
      source: 'device_manage',
      scope: FLEET_FILTER_RESULT_SCOPE,
      deviceIds: ['dev-1'],
      requestedTotal: 42,
      currentPageCount: 1,
      deviceFilter: { group_id: 'group-1', is_online: 1 }
    })
    const harness = createFlowHarness(fleetRolloutContext)
    harness.flow.taskForm.name = '  Filter rollout  '
    harness.flow.taskForm.device_id_list = ['dev-1']

    await harness.flow.saveTask()

    expect(harness.previewTask).toHaveBeenCalledWith({
      ota_upgrade_package_id: 'pkg-1',
      device_filter: { group_id: 'group-1', is_online: 1 },
      exclude_device_id_list: [],
      max_devices: 5000
    })
    expect(harness.addTask).not.toHaveBeenCalled()
    expect(harness.flow.filterPreviewResult.value).toEqual(
      expect.objectContaining({ selected_count: 42, total_matched: 42, max_devices: 5000 })
    )

    await harness.flow.saveTask()

    expect(harness.addTask).toHaveBeenCalledWith({
      name: 'Filter rollout',
      ota_upgrade_package_id: 'pkg-1',
      description: undefined,
      device_filter: { group_id: 'group-1', is_online: 1 },
      exclude_device_id_list: [],
      expected_total: 42,
      max_devices: 5000
    })
  })

  it('keeps compatible current-page fleet links working when they include a backend device filter', async () => {
    const fleetRolloutContext = ref({
      source: 'device_manage',
      scope: FLEET_CURRENT_PAGE_SCOPE,
      deviceIds: ['dev-1'],
      requestedTotal: 42,
      currentPageCount: 1,
      deviceFilter: { group_id: 'group-1' }
    })
    const harness = createFlowHarness(fleetRolloutContext)
    harness.flow.taskForm.name = 'Compatible filter rollout'
    harness.flow.taskForm.device_id_list = ['dev-1']

    await harness.flow.saveTask()

    expect(harness.previewTask).toHaveBeenCalledWith({
      ota_upgrade_package_id: 'pkg-1',
      device_filter: { group_id: 'group-1' },
      exclude_device_id_list: [],
      max_devices: 5000
    })
    expect(harness.addTask).not.toHaveBeenCalled()
  })

  it('treats device_filter scope as a full backend-filter OTA rollout', async () => {
    const fleetRolloutContext = ref({
      source: 'device_manage',
      scope: FLEET_DEVICE_FILTER_SCOPE,
      deviceIds: ['dev-1', 'dev-2'],
      requestedTotal: 42,
      currentPageCount: 2,
      deviceFilter: { search: 'pump', is_online: 1 }
    })
    const harness = createFlowHarness(fleetRolloutContext)
    harness.flow.taskForm.name = 'Command center filter rollout'

    await harness.flow.saveTask()

    expect(harness.previewTask).toHaveBeenCalledWith({
      ota_upgrade_package_id: 'pkg-1',
      device_filter: { search: 'pump', is_online: 1 },
      exclude_device_id_list: [],
      max_devices: 5000
    })
    expect(harness.addTask).not.toHaveBeenCalled()
    expect(harness.flow.filterPreviewResult.value).toMatchObject({
      selected_count: 42,
      total_matched: 42,
      max_devices: 5000
    })
  })

  it('blocks a fleet filter rollout when backend preview finds no devices', async () => {
    const fleetRolloutContext = ref({
      source: 'device_manage',
      scope: FLEET_FILTER_RESULT_SCOPE,
      deviceIds: ['dev-1'],
      requestedTotal: 1,
      currentPageCount: 1,
      deviceFilter: { group_id: 'group-1' }
    })
    const harness = createFlowHarness(fleetRolloutContext)
    harness.previewTask.mockResolvedValueOnce({ data: { selected_count: 0, over_limit: false }, error: null })
    harness.flow.taskForm.name = 'Filter rollout'

    await harness.flow.saveTask()

    expect(harness.addTask).not.toHaveBeenCalled()
    expect(harness.message.warning).toHaveBeenCalledWith('page.product.update-ota.filterPreviewNoDevices')
  })

  it('blocks a fleet filter rollout when backend preview exceeds the task limit', async () => {
    const fleetRolloutContext = ref({
      source: 'device_manage',
      scope: FLEET_FILTER_RESULT_SCOPE,
      deviceIds: ['dev-1'],
      requestedTotal: 6000,
      currentPageCount: 1,
      deviceFilter: { group_id: 'group-1' }
    })
    const harness = createFlowHarness(fleetRolloutContext)
    harness.previewTask.mockResolvedValueOnce({
      data: { selected_count: 6000, over_limit: true, max_devices: 5000 },
      error: null
    })
    harness.flow.taskForm.name = 'Filter rollout'

    await harness.flow.saveTask()

    expect(harness.addTask).not.toHaveBeenCalled()
    expect(harness.message.warning).toHaveBeenCalledWith(
      'page.product.update-ota.filterPreviewOverLimit'.replace('{selected}', '6000').replace('{max}', '5000')
    )
  })

  it('uses local preflight data to explain selected rollout risks', () => {
    const harness = createFlowHarness()
    harness.flow.taskForm.device_id_list = ['dev-1', 'dev-2']

    expect(harness.flow.taskPreflight.value).toEqual({
      eligible: 2,
      selected: 2,
      offline: 1,
      sameVersion: 1,
      missingVersion: 0,
      riskCount: 2
    })
    expect(harness.flow.taskRiskDevices.value.map((item) => item.id)).toEqual(['dev-1', 'dev-2'])
  })

})
