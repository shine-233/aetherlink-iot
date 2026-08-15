import { describe, expect, it } from 'vitest'
import { buildOtaFailureSupportBundle } from '../ota-task-failure-workbench'

describe('ota-task-failure-workbench', () => {
  it('builds a failed-rollout support package with task, package, statistics, samples, and boundaries', () => {
    const bundle = buildOtaFailureSupportBundle({
      task: {
        id: 'task-1',
        name: 'Factory rollout',
        target_mode: 'filter',
        preview_total: 18,
        created_at: '2026-07-06 09:00:00'
      },
      selectedPackage: {
        id: 'pkg-1',
        name: 'Gateway package',
        version: '2.0.0',
        target_version: '2.1.0'
      },
      rows: [
        {
          id: 'detail-1',
          device_id: 'device-1',
          name: 'Pump A',
          device_number: 'SN-001',
          current_version: '1.0.0',
          status: 5,
          status_description: 'download failed',
          updated_at: '2026-07-06 10:00:00'
        },
        {
          id: 'detail-2',
          device_id: 'device-2',
          name: 'Pump B',
          device_number: 'SN-002',
          current_version: '1.0.0',
          version: '2.1.1',
          status: 5,
          status_description: 'checksum mismatch',
          updated_at: '2026-07-06 10:01:00'
        },
        {
          id: 'detail-3',
          name: 'Pump C',
          status: 4
        }
      ],
      statistics: [
        { status: 4, count: 1 },
        { status: 5, count: 2 }
      ],
      fallbackReason: 'No failure reason returned',
      generatedAt: '2026-07-06T10:02:00.000Z',
      lastRefreshLabel: 'Last refreshed at 2026-07-06 10:02:00',
      maxDevices: 1
    })

    expect(bundle).toContain('# AetherLink OTA failed-rollout support package')
    expect(bundle).toContain('scope=loaded task detail rows in the current frontend state')
    expect(bundle).toContain('taskId=task-1')
    expect(bundle).toContain('target=filter expected=18')
    expect(bundle).toContain('packageVersion=2.0.0')
    expect(bundle).toContain('status_5=2')
    expect(bundle).toContain('loadedDetailRows=3')
    expect(bundle).toContain('- download failed: 1')
    expect(bundle).toContain('target=2.1.0')
    expect(bundle).toContain('deviceId=device-1')
    expect(bundle).toContain('/device/details?d_id=device-1&tab=ready-check&source=ota&ota_detail_id=detail-1')
    expect(bundle).toContain('reportedReason=download failed')
    expect(bundle).toContain('1 more failed device(s) omitted')
    expect(bundle).toContain('Evidence boundary')
    expect(bundle).toContain('currently loaded task-detail rows')
  })

  it('marks missing device ids as unsafe for diagnostic deep links', () => {
    const bundle = buildOtaFailureSupportBundle({
      rows: [{ id: 'detail-1', name: 'Pump A', status: 5 }],
      fallbackReason: 'No failure reason returned'
    })

    expect(bundle).toContain('deviceId=-')
    expect(bundle).toContain('diagnostics=<missing-device-id>')
  })
})
