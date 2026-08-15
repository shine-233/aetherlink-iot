import { describe, expect, it } from 'vitest'
import {
  buildOtaTaskPreflightItems,
  buildOtaTaskPreflightView,
  getOtaPackageTargetVersion
} from '../ota-task-preflight-view'

const t = (key: string) => key

describe('ota-task-preflight-view', () => {
  it('uses target_version before package version for rollout comparisons', () => {
    expect(getOtaPackageTargetVersion({ id: 'pkg-1', target_version: '2.0', version: '1.9' })).toBe('2.0')
    expect(getOtaPackageTargetVersion({ id: 'pkg-1', version: '1.9' })).toBe('1.9')
    expect(getOtaPackageTargetVersion(null)).toBeUndefined()
  })

  it('builds customer-visible preflight metrics with warning tags only when risk exists', () => {
    expect(
      buildOtaTaskPreflightItems(
        {
          eligible: 3,
          selected: 2,
          offline: 1,
          sameVersion: 0,
          missingVersion: 1,
          riskCount: 2
        },
        t
      )
    ).toEqual([
      { key: 'eligible', label: 'page.product.update-ota.preflightEligible', value: 3, type: 'info' },
      { key: 'selected', label: 'page.product.update-ota.preflightSelected', value: 2, type: 'success' },
      { key: 'offline', label: 'page.product.update-ota.preflightOffline', value: 1, type: 'warning' },
      { key: 'same-version', label: 'page.product.update-ota.preflightSameVersion', value: 0, type: 'default' },
      {
        key: 'missing-version',
        label: 'page.product.update-ota.preflightMissingVersion',
        value: 1,
        type: 'warning'
      }
    ])
  })

  it('builds preflight summary and risk rows from real device candidate fields', () => {
    const view = buildOtaTaskPreflightView(
      [
        { id: 'dev-1', name: 'Offline Device', current_version: '1.0', is_online: 0 },
        { id: 'dev-2', name: 'Current Device', current_version: '2.0', is_online: 1 },
        { id: 'dev-3', device_name: 'Unknown Device', online: true }
      ],
      ['dev-1', 'dev-2', 'dev-3'],
      { id: 'pkg-1', target_version: '2.0' },
      t
    )

    expect(view.summary).toEqual({
      eligible: 3,
      selected: 3,
      offline: 1,
      sameVersion: 1,
      missingVersion: 1,
      riskCount: 3
    })
    expect(view.riskDevices.map((item) => item.id)).toEqual(['dev-1', 'dev-2', 'dev-3'])
  })
})
