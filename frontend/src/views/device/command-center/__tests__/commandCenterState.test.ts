import {
  buildCommandCenterFilterSummaryItems,
  buildCommandJobEligibilityImpactSummaryText,
  buildCommandJobEligibilityImpactPreview,
  buildCommandJobPreviewActionPlan,
  buildDeviceFilterFromQuery,
  buildFilteredFleetEligibilityPreview,
  buildFleetCommandPayload,
  buildRouteDecisionSummary,
  filterCommandJobPreviewRowsByImpactGroup,
  getFleetCommandPayloadValidationKey,
  getRecommendedPathLabelKey,
  normalizeQueryNumber,
  parseCommandCenterScopeContext,
  parseDeviceIds,
  serializeFleetCommandPayload
} from '../commandCenterState'

describe('commandCenterState', () => {
  it('normalizes route query devices and numeric counts', () => {
    expect(parseDeviceIds(' a, b,,c ')).toEqual(['a', 'b', 'c'])
    expect(parseDeviceIds(['a', 'b,c'])).toEqual(['a', 'b', 'c'])
    expect(normalizeQueryNumber('12')).toBe(12)
    expect(normalizeQueryNumber('bad')).toBeNull()
  })

  it('builds a stable selected-device command payload', () => {
    const payload = buildFleetCommandPayload({
      deviceIds: ['dev-1', 'dev-2'],
      identify: ' reboot ',
      value: ' {"delay":1} ',
      timeoutSeconds: 30
    })

    expect(payload).toEqual({
      device_ids: ['dev-1', 'dev-2'],
      scope_type: 'selected_devices',
      identify: 'reboot',
      value: '{"delay":1}',
      timeout_seconds: 30
    })
    expect(serializeFleetCommandPayload(payload)).toContain('"scope_type":"selected_devices"')
  })

  it('builds a filter-scope command payload from fleet handoff filters', () => {
    const context = parseCommandCenterScopeContext({
      fleet_source: 'device_manage',
      fleet_scope: 'current_page',
      fleet_requested_total: '42',
      fleet_current_page_count: '10',
      device_ids: 'subset-1,subset-2',
      group_id: 'group-1',
      is_online: '1'
    })
    const payload = buildFleetCommandPayload({
      deviceIds: context.deviceIds,
      scopeType: context.scopeType,
      deviceFilter: context.deviceFilter,
      expectedTotal: context.requestedTotal,
      currentPageCount: context.currentPageCount,
      source: context.source,
      identify: ' reboot '
    })

    expect(context.scopeType).toBe('device_filter')
    expect(payload).toEqual({
      scope_type: 'device_filter',
      device_filter: { group_id: 'group-1', is_online: 1 },
      expected_total: 42,
      current_page_count: 10,
      scope_source: 'device_manage',
      max_devices: 200,
      subset_limit: 20,
      sample_limit: 20,
      identify: 'reboot',
      timeout_seconds: 60
    })
    expect(serializeFleetCommandPayload(payload)).toContain('"scope_type":"device_filter"')
  })

  it('allows the filtered-job safety cap to be controlled by the operator', () => {
    const payload = buildFleetCommandPayload({
      deviceIds: [],
      scopeType: 'device_filter',
      deviceFilter: { search: 'pump' },
      identify: 'reboot',
      maxDevices: 25,
      subsetLimit: 10
    })

    expect(payload).toMatchObject({
      scope_type: 'device_filter',
      device_filter: { search: 'pump' },
      max_devices: 25,
      subset_limit: 10,
      sample_limit: 10
    })
  })

  it('converts the local datetime picker value to the scheduled_at ISO contract', () => {
    const payload = buildFleetCommandPayload({
      deviceIds: ['dev-1'],
      identify: 'reboot',
      scheduledAt: Date.parse('2026-07-20T01:00:00.000Z')
    })

    expect(payload.scheduled_at).toBe('2026-07-20T01:00:00.000Z')
  })

  it('parses explicit device_filter query and summarizes readable filters', () => {
    const filter = buildDeviceFilterFromQuery({
      fleet_scope: 'device_filter',
      device_filter: '{"group_id":"group-1","is_online":1,"is_enabled":"Y","lifecycle_status":"transmitted"}',
      search: 'pump',
      command_job_id: 'job-1'
    })

    expect(filter).toEqual({
      group_id: 'group-1',
      is_online: 1,
      is_enabled: 'Y',
      lifecycle_status: 'transmitted',
      search: 'pump'
    })
    expect(buildCommandCenterFilterSummaryItems(filter)).toEqual([
      { key: 'group_id', label: 'Device group', value: 'group-1' },
      { key: 'is_online', label: 'Online status', value: '1' },
      { key: 'is_enabled', label: 'Enabled state', value: 'Y' },
      { key: 'lifecycle_status', label: 'Lifecycle status', value: 'Transmission complete (reported)' },
      { key: 'search', label: 'Search', value: 'pump' }
    ])
  })

  it('keeps saved filter identity separate from device filter fields', () => {
    const context = parseCommandCenterScopeContext({
      fleet_scope: 'device_filter',
      device_filter: '{"group_id":"group-1"}',
      saved_filter_id: 'fleet-filter-1',
      saved_filter_name: 'Online pumps'
    })

    expect(context.savedFilterId).toBe('fleet-filter-1')
    expect(context.savedFilterName).toBe('Online pumps')
    expect(context.deviceFilter).toEqual({ group_id: 'group-1' })
  })

  it('preserves last-report timestamps, reported-history, and lifecycle filters', () => {
    expect(
      buildDeviceFilterFromQuery({
        fleet_scope: 'device_filter',
        device_filter:
          '{"last_reported_after":1752883200000,"last_reported_before":1752969600000,"never_reported":false,"lifecycle_status":"transmitted"}'
      })
    ).toEqual({
      last_reported_after: 1752883200000,
      last_reported_before: 1752969600000,
      never_reported: false,
      lifecycle_status: 'transmitted'
    })
  })

  it('returns validation and route-decision keys for operator guardrails', () => {
    expect(getFleetCommandPayloadValidationKey({ hasSelectedDevices: false, identify: 'reboot' })).toBe(
      'custom.commandCenter.noSelection'
    )
    expect(
      getFleetCommandPayloadValidationKey({
        hasSelectedDevices: false,
        hasDeviceFilter: false,
        scopeType: 'device_filter',
        identify: 'reboot'
      })
    ).toBe('custom.commandCenter.noFilterScope')
    expect(getFleetCommandPayloadValidationKey({ hasSelectedDevices: true, identify: '' })).toBe(
      'custom.commandCenter.commandIdentifierRequired'
    )
    expect(
      getFleetCommandPayloadValidationKey({
        hasSelectedDevices: true,
        identify: 'reboot',
        scheduledAt: Date.parse('2026-07-19T23:59:59.999Z'),
        nowMs: Date.parse('2026-07-20T00:00:00.000Z')
      })
    ).toBe('custom.commandCenter.scheduleMustBeFuture')
    expect(
      getFleetCommandPayloadValidationKey({
        hasSelectedDevices: true,
        identify: 'reboot',
        scheduledAt: Date.parse('2027-07-21T00:00:00.001Z'),
        nowMs: Date.parse('2026-07-20T00:00:00.000Z')
      })
    ).toBe('custom.commandCenter.scheduleTooFar')
    expect(
      getFleetCommandPayloadValidationKey({
        hasSelectedDevices: true,
        identify: 'reboot',
        scheduledAt: Date.parse('2026-07-20T00:01:00.000Z'),
        nowMs: Date.parse('2026-07-20T00:00:00.000Z')
      })
    ).toBe('')
    expect(getFleetCommandPayloadValidationKey({ hasSelectedDevices: true, identify: 'reboot' })).toBe('')

    expect(getRecommendedPathLabelKey('immediate')).toBe('custom.commandCenter.pathImmediate')
    expect(getRecommendedPathLabelKey('jobs')).toBe('custom.commandCenter.pathJobs')
    expect(getRecommendedPathLabelKey('blocked')).toBe('custom.commandCenter.pathBlocked')
  })

  it('summarizes preview route decisions and telemetry evidence', () => {
    expect(
      buildRouteDecisionSummary([
        { recommended_path: 'immediate', telemetry_current_count: 2 } as any,
        { recommended_path: 'jobs', telemetry_current_count: 0 } as any,
        { recommended_path: 'blocked', telemetry_current_count: 1 } as any
      ])
    ).toEqual({
      immediate: 1,
      jobs: 1,
      blocked: 1,
      telemetry: 2
    })
  })

  it('builds a customer action plan from backend preview guidance', () => {
    const plan = buildCommandJobPreviewActionPlan({
      previewResult: {
        path_counts: { immediate: 2, jobs: 3, blocked: 1, telemetry: 4 },
        blockers: [{ reason: 'device offline', advice: 'check connection', count: 1 }],
        next_action: 'Submit eligible devices only after reviewing blockers.',
        rows: []
      },
      fallbackNextAction: 'Preview before submitting'
    })

    expect(plan?.cards.map(card => [card.key, card.value])).toEqual([
      ['immediate', 2],
      ['jobs', 3],
      ['blocked', 1],
      ['telemetry', 4]
    ])
    expect(plan?.blockers).toEqual([{ reason: 'device offline', advice: 'check connection', count: 1 }])
    expect(plan?.nextAction).toBe('Submit eligible devices only after reviewing blockers.')
  })

  it('builds a customer action plan from preview rows when backend guidance is absent', () => {
    const plan = buildCommandJobPreviewActionPlan({
      previewResult: {
        rows: [
          { eligible: true, recommended_path: 'immediate', telemetry_current_count: 2 } as any,
          { eligible: false, recommended_path: 'blocked', reason: 'no permission', advice: 'ask admin' } as any,
          { eligible: false, recommended_path: 'blocked', reason: 'no permission', advice: 'ask admin' } as any
        ]
      },
      fallbackNextAction: 'Preview before submitting'
    })

    expect(plan?.cards.map(card => [card.key, card.value])).toEqual([
      ['immediate', 1],
      ['jobs', 0],
      ['blocked', 2],
      ['telemetry', 1]
    ])
    expect(plan?.blockers).toEqual([{ reason: 'no permission', advice: 'ask admin', count: 2 }])
    expect(plan?.nextAction).toBe('Preview before submitting')
  })

  it('builds a customer-readable eligibility and impact preview from real preview rows', () => {
    const preview = buildCommandJobEligibilityImpactPreview({
      isDeviceFilterScope: true,
      previewResult: {
        requested_count: 5,
        next_action: 'Submit eligible devices and fix blocked devices first.',
        rows: [
          {
            device_id: 'dev-1',
            device_number: 'pump-1',
            eligible: true,
            recommended_path: 'immediate',
            status: 'ready',
            advice: 'Send now'
          } as any,
          {
            device_id: 'dev-2',
            device_number: 'pump-2',
            eligible: true,
            recommended_path: 'jobs',
            status: 'queued',
            reason: 'offline but job-capable',
            advice: 'Track in job history'
          } as any,
          {
            device_id: 'dev-3',
            device_number: 'pump-3',
            eligible: false,
            recommended_path: 'blocked',
            status: 'blocked',
            reason: 'missing command permission',
            advice: 'Fix permission'
          } as any
        ]
      },
      fallbackNextAction: 'Preview before submitting'
    })

    expect(preview).toMatchObject({
      coverage: 'subset_only',
      coverageLabelKey: 'custom.commandCenter.impactPreviewSubsetCoverage',
      requestedCount: 5,
      shownCount: 3,
      nextAction: 'Submit eligible devices and fix blocked devices first.'
    })
    expect(preview?.groups.map(group => [group.key, group.count, group.type])).toEqual([
      ['eligible', 2, 'success'],
      ['immediate', 1, 'success'],
      ['jobs', 1, 'info'],
      ['blocked', 1, 'error']
    ])
    expect(preview?.groups.find(group => group.key === 'blocked')?.representativeRows).toEqual([
      {
        key: 'dev-3',
        device: 'pump-3',
        reason: 'missing command permission',
        advice: 'Fix permission'
      }
    ])
  })

  it('filters preview rows by customer impact group and builds copyable summary evidence', () => {
    const rows = [
      { device_id: 'dev-1', device_number: 'pump-1', eligible: true, recommended_path: 'immediate' },
      { device_id: 'dev-2', device_number: 'pump-2', eligible: true, recommended_path: 'jobs' },
      { device_id: 'dev-3', device_number: 'pump-3', eligible: false, recommended_path: 'blocked', reason: 'missing key' }
    ] as any[]
    const preview = buildCommandJobEligibilityImpactPreview({
      isDeviceFilterScope: false,
      previewResult: {
        requested_count: 3,
        next_action: 'Submit eligible devices.',
        rows
      },
      fallbackNextAction: 'Preview first'
    })

    expect(filterCommandJobPreviewRowsByImpactGroup(rows, 'all')).toHaveLength(3)
    expect(filterCommandJobPreviewRowsByImpactGroup(rows, 'eligible').map(row => row.device_id)).toEqual([
      'dev-1',
      'dev-2'
    ])
    expect(filterCommandJobPreviewRowsByImpactGroup(rows, 'immediate').map(row => row.device_id)).toEqual(['dev-1'])
    expect(filterCommandJobPreviewRowsByImpactGroup(rows, 'jobs').map(row => row.device_id)).toEqual(['dev-2'])
    expect(filterCommandJobPreviewRowsByImpactGroup(rows, 'blocked').map(row => row.device_id)).toEqual(['dev-3'])

    const summary = buildCommandJobEligibilityImpactSummaryText(preview, key => key)
    expect(summary).toContain('custom.commandCenter.impactPreviewTitle')
    expect(summary).toContain('custom.commandCenter.impactPreviewFullCoverage: 3/3')
    expect(summary).toContain('custom.commandCenter.impactPreviewBlocked: 1')
    expect(summary).toContain('- pump-3: missing key; -')
  })

  it('labels filtered fleet preview rows as subset-only until the preview covers the backend match', () => {
    const preview = buildFilteredFleetEligibilityPreview({
      isDeviceFilterScope: true,
      previewResult: {
        total_matched: 42,
        requested_count: 42,
        rows: [
          { eligible: true, recommended_path: 'immediate', telemetry_current_count: 1 } as any,
          { eligible: false, recommended_path: 'blocked', telemetry_current_count: 0 } as any
        ]
      }
    })

    expect(preview).toMatchObject({
      coverage: 'subset_only',
      alertType: 'warning',
      messageKey: 'custom.commandCenter.filteredPreviewSubsetOnlyScope',
      requestedCount: 42,
      shownCount: 2,
      totalMatched: 42,
      subsetEligibleCount: 1,
      subsetBlockedCount: 1,
      immediateCount: 1,
      jobsCount: 0,
      blockedPathCount: 1,
      telemetryCount: 1
    })
  })

  it('labels selected-device and complete filtered previews as full-scope evidence', () => {
    const selectedPreview = buildFilteredFleetEligibilityPreview({
      isDeviceFilterScope: false,
      previewResult: {
        requested_count: 10,
        rows: [{ eligible: true, recommended_path: 'jobs', telemetry_current_count: 0 } as any]
      }
    })
    const fullFilterPreview = buildFilteredFleetEligibilityPreview({
      isDeviceFilterScope: true,
      previewResult: {
        requested_count: 1,
        rows: [{ eligible: true, recommended_path: 'jobs', telemetry_current_count: 0 } as any]
      }
    })

    expect(selectedPreview?.coverage).toBe('full')
    expect(fullFilterPreview?.coverage).toBe('full')
  })
})
