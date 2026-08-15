import { describe, expect, it, vi } from 'vitest'

import type { FleetCommandJobListItem } from '@/service/api/device'
import { createCommandJobHistoryColumns } from '../commandCenterTableColumns'

describe('commandCenterTableColumns', () => {
  it('keeps the planned start visible in command job history', () => {
    const columns = createCommandJobHistoryColumns({
      t: key => ({
        'custom.commandCenter.scheduledAt': 'Scheduled start',
        'custom.commandCenter.updatedAt': 'Updated at'
      })[key] || key,
      openCommandJobDetail: vi.fn(),
      reuseCommandJobDraft: vi.fn(),
      saveCommandJobTemplate: vi.fn()
    }) as any[]
    const scheduledAtColumn = columns.find(column => column.key === 'scheduled_at')

    expect(scheduledAtColumn?.title).toBe('Scheduled start')
    expect(
      scheduledAtColumn?.render({ scheduled_at: '2026-07-20T01:00:00.000Z' } as FleetCommandJobListItem)
    ).toBe('2026-07-20 01:00:00 UTC')
    expect(scheduledAtColumn?.render({} as FleetCommandJobListItem)).toBe('--')
  })
})
