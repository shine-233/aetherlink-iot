import { describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

import {
  hasInstallationInfo,
  snapshotInstallationEntries,
  snapshotStatusLabel,
  snapshotStatusTagType
} from '../rdiSnapshotPresentation'

const device = (overrides: Record<string, unknown> = {}) =>
  ({
    serialNumber: '--',
    installLocation: '--',
    installAddress: '--',
    installDate: '--',
    installerName: '--',
    installerContact: '--',
    adminName: '--',
    online: true,
    alarm: null,
    ...overrides
  }) as any

describe('rdiSnapshotPresentation', () => {
  it('prioritises alarm over offline over explicit normal over online', () => {
    expect(snapshotStatusTagType(device({ alarm: true, online: false }))).toBe('error')
    expect(snapshotStatusTagType(device({ online: false }))).toBe('default')
    expect(snapshotStatusTagType(device({ alarm: false }))).toBe('success')
    expect(snapshotStatusTagType(device())).toBe('info')
    expect(snapshotStatusLabel(device({ online: false }))).toBe('rdi.overview.offline')
  })

  it('lists only filled installation fields in display order', () => {
    expect(hasInstallationInfo(device())).toBe(false)
    const entries = snapshotInstallationEntries(device({ adminName: 'Ann', serialNumber: 'SN1' }))
    expect(entries.map((entry) => entry.field)).toEqual(['serialNumber', 'adminName'])
    expect(entries[1]).toEqual({ field: 'adminName', labelKey: 'rdi.overview.administrator', value: 'Ann' })
    expect(hasInstallationInfo(device({ installDate: '2026-01-01' }))).toBe(true)
  })
})
