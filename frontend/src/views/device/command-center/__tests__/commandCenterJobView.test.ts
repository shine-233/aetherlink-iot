import {
  buildCommandJobCapabilitySummary,
  buildCommandJobActionConsequenceRows,
  buildCommandJobAuditSummaryCard,
  buildCommandJobEvidenceSummary,
  buildCommandJobExecutionSummaryCard,
  buildCommandJobGovernanceSummaryCard,
  buildCommandJobCloseoutPacket,
  buildCommandJobDeviceProgressTracks,
  buildCommandJobHandoffSummary,
  buildCommandJobHistoryAttentionAggregateRows,
  buildCommandJobHistoryProgress,
  buildCommandJobHistoryAttentionOptions,
  buildCommandJobHistoryAttentionSummary,
  buildCommandJobHistoryAttentionTotalSummary,
  buildCommandJobHistoryStatusOptions,
  buildCommandJobOperatorNextAction,
  buildCommandJobOutcomeGroups,
  buildCommandJobProgressHealthCard,
  buildCommandJobProgressSummary,
  buildCommandJobStatusCountRows,
  buildCommandJobStatusRows,
  buildCommandJobSupportBundlePreview,
  buildCommandJobTimelineRows,
  buildCommandJobTroubleshootingRows,
  canRetryCommandJob,
  commandJobLogMissingCount,
  commandJobLogMissingRows,
  commandJobProgressPercent,
  commandJobRetryableCount,
  commandJobRetryableRows,
  formatCommandJobDateTime,
  formatCommandJobStatus
} from '../commandCenterJobView'

const translations: Record<string, string> = {
  'common.status': 'Status',
  'common.yesOrNo.yes': 'Yes',
  'common.yesOrNo.no': 'No',
  'custom.commandCenter.jobId': 'Job ID',
  'custom.commandCenter.jobProgress': 'Progress',
  'custom.commandCenter.createdAt': 'Created',
  'custom.commandCenter.updatedAt': 'Updated',
  'custom.commandCenter.scheduledAt': 'Scheduled start',
  'custom.commandCenter.nextDispatchAt': 'Next dispatch slot',
  'custom.commandCenter.timeoutAt': 'Timeout',
  'custom.commandCenter.jobProgressSummary': '{submitted}/{total} submitted, {failed} failed',
  'custom.commandCenter.progressHealthPending': 'Pending',
  'custom.commandCenter.progressHealthTerminal': 'Finished',
  'custom.commandCenter.progressHealthElapsed': 'Elapsed',
  'custom.commandCenter.progressHealthRemaining': 'Timeout left',
  'custom.commandCenter.progressHealthExpired': 'Expired',
  'custom.commandCenter.progressHealthSeconds': '{seconds}s',
  'custom.commandCenter.progressHealthMinutes': '{minutes}m',
  'custom.commandCenter.progressHealthState.timeout_risk': 'Near timeout',
  'custom.commandCenter.progressHealthState.needs_attention': 'Needs attention',
  'custom.commandCenter.progressHealthState.scheduled': 'Scheduled',
  'custom.commandCenter.progressHealthState.running': 'Running',
  'custom.commandCenter.progressHealthNext.scheduled': 'Wait for the scheduled start.',
  'custom.commandCenter.progressHealthNext.running': 'Keep waiting.',
  'custom.commandCenter.progressHealthNext.needs_attention': 'Review rows.',
  'custom.commandCenter.auditEventCount': 'Events',
  'custom.commandCenter.auditLatestEvent': 'Latest event',
  'custom.commandCenter.auditLatestAt': 'Latest time',
  'custom.commandCenter.auditLatestMessage': 'Latest message',
  'custom.commandCenter.governanceSummaryTitle': 'Governance summary',
  'custom.commandCenter.governanceLevel.warning': 'Review',
  'custom.commandCenter.governanceState.done': 'Done',
  'custom.commandCenter.governanceState.watch': 'Watch',
  'custom.commandCenter.governanceState.blocked': 'Blocked',
  'custom.commandCenter.executionDecision.retry': 'Retry ready devices',
  'custom.commandCenter.executionDecision.close': 'Close handoff',
  'custom.commandCenter.executionDecision.collect_evidence': 'Collect evidence',
  'custom.commandCenter.executionDecision.wait_schedule': 'Wait for scheduled start',
  'custom.commandCenter.closeReadinessBlocked': 'Closure blocked',
  'custom.commandCenter.closeReadinessReady': 'Ready to close',
  'custom.commandCenter.executionChecklistState.done': 'Done',
  'custom.commandCenter.executionChecklistState.todo': 'To do',
  'custom.commandCenter.executionChecklistState.watch': 'Watch',
  'custom.commandCenter.executionChecklistState.blocked': 'Blocked',
  'custom.commandCenter.submitCapabilitySummary': 'Cancel {cancel}, retry {retry}',
  'custom.commandCenter.submitEvidenceSummary': '{retryable} retryable, {missingLogs} missing logs',
  'custom.commandCenter.troubleshootingRetryTitle': 'Retry failed devices',
  'custom.commandCenter.troubleshootingRetryDesc': '{count} devices can be retried',
  'custom.commandCenter.troubleshootingRetryWaitingTitle': 'Retry window pending',
  'custom.commandCenter.troubleshootingRetryWaitingDesc': '{count} devices are waiting',
  'custom.commandCenter.troubleshootingRetryExhaustedTitle': 'Retry limit reached',
  'custom.commandCenter.troubleshootingRetryExhaustedDesc': '{count} devices reached the limit',
  'custom.commandCenter.troubleshootingCancelTitle': 'Cancel pending rows',
  'custom.commandCenter.troubleshootingCancelDesc': '{count} rows are pending',
  'custom.commandCenter.troubleshootingLogsTitle': 'Refresh logs',
  'custom.commandCenter.troubleshootingLogsDesc': '{count} devices need logs',
  'custom.commandCenter.troubleshootingBlockedTitle': 'Fix blocked devices',
  'custom.commandCenter.troubleshootingBlockedDesc': '{count} devices are blocked',
  'custom.commandCenter.troubleshootingSupportTitle': 'Keep support evidence',
  'custom.commandCenter.troubleshootingSupportDesc': 'Copy support bundle',
  'custom.commandCenter.operatorDecisionEvidence':
    '{retryable} retryable, {pending} pending, {missingLogs} missing logs, {blocked} blocked, {deviceFailed} device failed',
  'custom.commandCenter.operatorDecisionDeviceFailedTitle': 'Review device response failures',
  'custom.commandCenter.operatorDecisionDeviceFailedDesc': '{count} devices reported a failure response.',
  'custom.commandCenter.operatorDecisionRetryTitle': 'Retry failed devices',
  'custom.commandCenter.operatorDecisionRetryDesc': '{count} retryable devices are ready for a controlled retry.',
  'custom.commandCenter.operatorDecisionRetryWaitingTitle': 'Wait for retry window',
  'custom.commandCenter.operatorDecisionRetryWaitingDesc': '{count} devices are cooling down.',
  'custom.commandCenter.operatorDecisionRetryExhaustedTitle': 'Review retry-limit devices',
  'custom.commandCenter.operatorDecisionRetryExhaustedDesc': '{count} devices need support review.',
  'custom.commandCenter.operatorDecisionRefreshTitle': 'Keep watching the running job',
  'custom.commandCenter.operatorDecisionRefreshDesc': '{count} rows are still pending.',
  'custom.commandCenter.operatorDecisionLogsTitle': 'Refresh missing log evidence',
  'custom.commandCenter.operatorDecisionLogsDesc': '{count} rows need platform log evidence.',
  'custom.commandCenter.operatorDecisionBlockedTitle': 'Fix blockers before sending again',
  'custom.commandCenter.operatorDecisionBlockedDesc': '{count} devices were blocked before dispatch.',
  'custom.commandCenter.operatorDecisionDoneTitle': 'Job is ready to close',
  'custom.commandCenter.operatorDecisionDoneDesc': 'Keep the job link with the customer handoff.',
  'custom.commandCenter.operatorDecisionSupportTitle': 'Preserve support evidence',
  'custom.commandCenter.operatorDecisionSupportDesc': 'Preview the support bundle before handing off.',
  'custom.commandCenter.actionConsequenceCancel': 'Cancel {pending} pending rows while {status}.',
  'custom.commandCenter.actionConsequenceCancelUnavailable': 'Cancel unavailable while {status}.',
  'custom.commandCenter.actionConsequenceRetry': 'Retry {ready} ready, {waiting} waiting, {exhausted} exhausted.',
  'custom.commandCenter.actionConsequenceRetryUnavailable': 'Retry unavailable.',
  'custom.commandCenter.actionConsequenceWaiting': '{count} waiting.',
  'custom.commandCenter.actionConsequenceExhausted': '{count} exhausted.',
  'custom.commandCenter.actionConsequenceDeviceFailed': '{count} device failed.',
  'custom.commandCenter.actionConsequenceLogs': '{count} missing logs.',
  'custom.commandCenter.outcomeRetryTitle': 'Retryable failures',
  'custom.commandCenter.outcomeRetryDesc': 'Review and retry only these devices.',
  'custom.commandCenter.outcomeDeviceFailedTitle': 'Device response failures',
  'custom.commandCenter.outcomeDeviceFailedDesc': 'Review device-side error evidence before retrying.',
  'custom.commandCenter.outcomeMissingLogsTitle': 'Missing log evidence',
  'custom.commandCenter.outcomeMissingLogsDesc': 'Refresh before closing.',
  'custom.commandCenter.outcomeBlockedTitle': 'Blocked before dispatch',
  'custom.commandCenter.outcomeBlockedDesc': 'Fix blockers first.',
  'custom.commandCenter.outcomeInProgressTitle': 'Still in progress',
  'custom.commandCenter.outcomeInProgressDesc': 'Refresh until terminal.',
  'custom.commandCenter.outcomeCompletedTitle': 'Completed',
  'custom.commandCenter.outcomeCompletedDesc': 'No action needed.',
  'custom.commandCenter.nextActionRetry': 'Retry this device after reviewing the failure reason.',
  'custom.commandCenter.nextActionRetryAfter': 'Retry after {time}; review the failure reason before sending again.',
  'custom.commandCenter.nextActionRetryLimitReached': 'Retry limit reached.',
  'custom.commandCenter.nextActionRefreshLogs': 'Refresh the job or copy the support bundle.',
  'custom.commandCenter.nextActionFixBlocker': 'Fix the blocker shown in the reason before sending again.',
  'custom.commandCenter.nextActionCompleted': 'No action needed.',
  'custom.commandCenter.nextActionDeviceAckSuccess': 'Device returned success.',
  'custom.commandCenter.nextActionDeviceAckFailed': 'Device returned failure.',
  'custom.commandCenter.nextActionTrackMessage': 'Track the message ID and refresh until terminal.',
  'custom.commandCenter.nextActionSupportBundle': 'Copy the support bundle if unclear.',
  'custom.commandCenter.deviceProgressPreview': 'Preview',
  'custom.commandCenter.deviceProgressDispatch': 'Dispatch',
  'custom.commandCenter.deviceProgressAck': 'Device ACK',
  'custom.commandCenter.deviceProgressEvidence': 'Evidence',
  'custom.commandCenter.deviceProgressPreviewBlocked': 'Preview blocked.',
  'custom.commandCenter.deviceProgressDispatchWaiting': 'Waiting for dispatch evidence.',
  'custom.commandCenter.deviceProgressAckWaiting': 'Waiting for device response.',
  'custom.commandCenter.deviceProgressEvidenceMissing': 'Command log missing.',
  'custom.commandCenter.deviceProgressState.done': 'Done',
  'custom.commandCenter.deviceProgressState.waiting': 'Waiting',
  'custom.commandCenter.deviceProgressState.blocked': 'Blocked',
  'custom.commandCenter.deviceProgressState.failed': 'Failed',
  'custom.commandCenter.deviceProgressState.missing': 'Missing',
  'custom.commandCenter.supportBundleGeneratedAt': 'Generated at',
  'custom.commandCenter.supportBundleRetryableDevices': 'Retryable devices',
  'custom.commandCenter.supportBundleRetryReadyDevices': 'Ready to retry',
  'custom.commandCenter.supportBundleRetryWaitingDevices': 'Waiting for retry window',
  'custom.commandCenter.supportBundleRetryExhaustedDevices': 'Retry limit reached',
  'custom.commandCenter.supportBundleMissingLogDevices': 'Missing log devices',
  'custom.commandCenter.supportBundleFailedDevices': 'Failed devices',
  'custom.commandCenter.supportBundleEvents': 'Events',
  'custom.commandCenter.supportBundleShareHint': 'Share hint',
  'custom.commandCenter.supportBundleDevice': 'Device',
  'custom.commandCenter.readinessEvidence': 'Readiness evidence',
  'custom.commandCenter.reason': 'Reason',
  'custom.commandCenter.advice': 'Advice',
  'custom.commandCenter.messageId': 'Message ID',
  'custom.commandCenter.dispatchAttempts': 'Dispatch attempts',
  'custom.commandCenter.maxDispatchAttempts': 'Max dispatch attempts',
  'custom.commandCenter.retryState': 'Retry state',
  'custom.commandCenter.retryState.retryable': 'Retryable',
  'custom.commandCenter.retryState.waiting_backoff': 'Waiting for retry window',
  'custom.commandCenter.retryState.max_attempts_reached': 'Retry limit reached',
  'custom.commandCenter.retryState.not_retryable': 'Not retryable',
  'custom.commandCenter.nextRetryAfter': 'Next retry after',
  'custom.commandCenter.deviceResponseStatus': 'Device response',
  'custom.commandCenter.deviceResponseEvidence': 'Response evidence',
  'custom.commandCenter.responseStatus.awaiting': 'Awaiting response',
  'custom.commandCenter.responseStatus.device_ack_failed': 'Device failure',
  'custom.commandCenter.responseStatus.device_ack_success': 'Device success',
  'custom.commandCenter.timelineCreated': 'Created',
  'custom.commandCenter.timelineSubmitted': 'Submitted',
  'custom.commandCenter.timelineCompleted': 'Completed',
  'custom.commandCenter.timelineDeviceCount': '{count} devices',
  'custom.commandCenter.jobEvent.created': 'Job created',
  'custom.commandCenter.jobEvent.scheduled': 'Job scheduled',
  'custom.commandCenter.jobEvent.started': 'Scheduled job started',
  'custom.commandCenter.jobEvent.dispatch_started': 'Dispatch started',
  'custom.commandCenter.jobEvent.dispatch_failed': 'Dispatch failed',
  'custom.commandCenter.jobEvent.device_ack_ambiguous': 'Device ack ambiguous',
  'custom.commandCenter.jobStatusAll': 'All',
  'custom.commandCenter.jobStatus.scheduled': 'Scheduled',
  'custom.commandCenter.jobStatus.running': 'Running',
  'custom.commandCenter.jobStatus.completed': 'Completed',
  'custom.commandCenter.jobStatus.partially_failed': 'Partially failed',
  'custom.commandCenter.jobStatus.failed': 'Failed',
  'custom.commandCenter.jobStatus.canceled': 'Canceled',
  'custom.commandCenter.jobAttentionAll': 'All attention',
  'custom.commandCenter.jobAttentionNeedsOperatorAction': 'Needs operator action',
  'custom.commandCenter.jobAttentionRetryable': 'Retryable failures',
  'custom.commandCenter.jobAttentionDeviceFailed': 'Device response failures',
  'custom.commandCenter.jobAttentionMissingLog': 'Missing log evidence',
  'custom.commandCenter.jobAttentionBlocked': 'Blocked before dispatch',
  'custom.commandCenter.jobAttentionNone': 'No action needed',
  'custom.commandCenter.jobAttentionSummary':
    '{count} need action, {retryable} retryable, {deviceFailed} device failed, {missingLogs} missing logs',
  'custom.commandCenter.submittedShort': 'submitted',
  'custom.commandCenter.failedShort': 'failed'
}

const t = (key: string) => translations[key] ?? key

const result = {
  job_id: 'job-1',
  job_type: 'manual_command',
  scope_type: 'selected_devices',
  status: 'partially_failed',
  requested_count: 5,
  eligible_count: 4,
  blocked_count: 1,
  submitted_count: 3,
  failed_count: 1,
  timeout_seconds: 60,
  can_cancel: true,
  can_retry_failed: true,
  status_counts: {
    submitted: 3,
    failed: 1
  },
  created_at: '2026-07-05T10:00:00Z',
  updated_at: '2026-07-05T10:01:00.123Z',
  timeout_at: '',
  rows: [
    {
      device_id: 'dev-1',
      eligible: true,
      status: 'submitted',
      can_retry: false,
      log_recorded: true,
      submitted_at: '2026-07-05T10:00:10Z'
    },
    {
      device_id: 'dev-2',
      eligible: true,
      status: 'failed',
      can_retry: true,
      log_recorded: false,
      completed_at: '2026-07-05T10:00:20Z'
    },
    {
      device_id: 'dev-3',
      eligible: false,
      status: 'blocked',
      can_retry: false,
      log_recorded: false
    },
    {
      device_id: 'dev-4',
      eligible: true,
      status: 'submitted',
      response_status_label: 'device_ack_success',
      response_data: '{"result":0}',
      can_retry: false,
      log_recorded: true,
      submitted_at: '2026-07-05T10:00:30Z'
    }
  ]
} as any

describe('commandCenterJobView', () => {
  it('formats job status, dates, progress, and evidence summaries', () => {
    expect(formatCommandJobStatus('scheduled', t)).toBe('Scheduled')
    expect(formatCommandJobStatus('partially_failed', t)).toBe('Partially failed')
    expect(formatCommandJobStatus('new_status', t)).toBe('new_status')
    expect(formatCommandJobDateTime('2026-07-05T10:01:00.123Z')).toBe('2026-07-05 10:01:00 UTC')
    expect(formatCommandJobDateTime()).toBe('--')

    expect(commandJobProgressPercent(result)).toBe(80)
    expect(canRetryCommandJob(result)).toBe(true)
    expect(buildCommandJobProgressSummary(result, t)).toBe('3/5 submitted, 1 failed')
    expect(buildCommandJobCapabilitySummary(result, t)).toBe('Cancel Yes, retry Yes')
    expect(buildCommandJobEvidenceSummary(result, t)).toBe('1 retryable, 1 missing logs')
  })

  it('builds a customer-readable progress health card from backend evidence', () => {
    const card = buildCommandJobProgressHealthCard(
      {
        ...result,
        progress_health: {
          state: 'timeout_risk',
          pending_count: 2,
          terminal_count: 3,
          elapsed_seconds: 72,
          timeout_remaining_seconds: 10,
          next_action: 'Watch the pending rows closely.'
        }
      },
      t
    )

    expect(card).toMatchObject({
      stateLabel: 'Near timeout',
      type: 'warning',
      nextAction: 'Watch the pending rows closely.'
    })
    expect(card?.rows).toEqual([
      { label: 'Pending', value: '2' },
      { label: 'Finished', value: '3' },
      { label: 'Elapsed', value: '2m' },
      { label: 'Timeout left', value: '10s' }
    ])
  })

  it('builds a customer-readable governance summary card', () => {
    const card = buildCommandJobGovernanceSummaryCard(
      {
        level: 'warning',
        title: 'Job governance',
        summary: '5 requested, 3 submitted, 1 failed, 1 blocked.',
        next_action: 'Review affected rows before retrying.',
        items: [
          {
            key: 'scope',
            label: 'Target scope',
            value: '5 requested / 4 eligible',
            state: 'done',
            detail: '1 blocked device remains.'
          },
          {
            key: 'timeout',
            label: 'Timeout window',
            value: '10 seconds remaining',
            state: 'watch',
            detail: 'Watch pending rows.'
          },
          {
            key: 'retry_policy',
            label: 'Retry policy',
            value: '0 ready / 0 waiting / 1 exhausted',
            state: 'blocked',
            detail: 'Support review required.'
          }
        ]
      },
      t
    )

    expect(card).toMatchObject({
      title: 'Job governance',
      levelLabel: 'Review',
      type: 'warning',
      summary: '5 requested, 3 submitted, 1 failed, 1 blocked.',
      nextAction: 'Review affected rows before retrying.'
    })
    expect(card?.items).toEqual([
      {
        key: 'scope',
        label: 'Target scope',
        value: '5 requested / 4 eligible',
        detail: '1 blocked device remains.',
        stateLabel: 'Done',
        type: 'success'
      },
      {
        key: 'timeout',
        label: 'Timeout window',
        value: '10 seconds remaining',
        detail: 'Watch pending rows.',
        stateLabel: 'Watch',
        type: 'warning'
      },
      {
        key: 'retry_policy',
        label: 'Retry policy',
        value: '0 ready / 0 waiting / 1 exhausted',
        detail: 'Support review required.',
        stateLabel: 'Blocked',
        type: 'error'
      }
    ])
  })

  it('builds a copyable customer handoff summary with the job link', () => {
    expect(
      buildCommandJobHandoffSummary(
        {
          ...result,
          handoff_summary: 'Command Job job-1 is running: 3/5 submitted, 1 failed.',
          execution_summary: {
            path_type: 'fleet_job',
            path_label: 'Fleet filter job',
            decision: 'retry',
            can_close: false,
            close_blockers: ['1 devices are still pending.'],
            next_action: 'Review close blockers.',
            checklist: []
          }
        },
        'https://example.test/jobs?command_job_id=job-1'
      )
    ).toBe(
      'Command Job job-1 is running: 3/5 submitted, 1 failed.\nClose readiness: blocked - 1 devices are still pending.\nNext action: Review close blockers.\nhttps://example.test/jobs?command_job_id=job-1'
    )

    expect(buildCommandJobHandoffSummary({ ...result, handoff_summary: undefined })).toContain(
      'Command Job job-1 is partially_failed'
    )
  })

  it('builds a copyable closeout packet with support and closure evidence', () => {
    expect(
      buildCommandJobCloseoutPacket(
        {
          ...result,
          execution_summary: {
            path_type: 'fleet_job',
            path_label: 'Fleet filter job',
            decision: 'retry',
            can_close: false,
            close_blockers: ['1 devices are still pending.'],
            next_action: 'Review close blockers.',
            checklist: [
              {
                key: 'retry',
                label: 'Review retry-ready devices',
                state: 'todo',
                detail: '1 devices can be retried now'
              }
            ]
          },
          audit_summary: {
            event_count: 2,
            latest_event_type: 'dispatch_failed',
            latest_event_at: '2026-07-05T10:02:00Z',
            latest_message: 'publish failed',
            next_action: 'Review device rows before retry.'
          }
        },
        'https://example.test/jobs?command_job_id=job-1',
        {
          job_id: 'job-1',
          job_type: 'manual_command',
          scope_type: 'selected_devices',
          identify: 'reboot',
          status: 'partially_failed',
          requested_count: 5,
          eligible_count: 4,
          blocked_count: 1,
          submitted_count: 3,
          failed_count: 1,
          retryable_count: 1,
          retry_ready_count: 1,
          retry_waiting_count: 0,
          retry_exhausted_count: 0,
          log_missing_count: 1,
          missing_log_device_ids: ['dev-2'],
          failed_devices: [{ device_id: 'dev-2', status: 'failed' }],
          events: [{ id: 'event-1', event_type: 'dispatch_failed' } as any],
          next_actions: ['Retry dev-2', 'Check gateway'],
          generated_at: '2026-07-05T10:10:00Z',
          share_hint: 'Send to support'
        }
      )
    ).toBe(
      [
        'AetherLink Command Job closeout packet',
        'Job: job-1',
        'Status: partially_failed',
        'Progress: 3/5 submitted, 1 failed, 1 blocked',
        'Close readiness: blocked',
        'Close blockers: 1 devices are still pending.',
        'Next action: Review close blockers.',
        'Checklist:',
        '- [todo] Review retry-ready devices: 1 devices can be retried now',
        'Audit: 2 events, latest=dispatch_failed, at=2026-07-05 10:02:00 UTC, message=publish failed',
        'Support bundle generated: 2026-07-05 10:10:00 UTC',
        'Support next actions: Retry dev-2; Check gateway',
        'Retry ready/waiting/exhausted: 1/0/0',
        'Failed devices: 1',
        'Missing log devices: 1',
        'Support events: 1',
        'Job link: https://example.test/jobs?command_job_id=job-1'
      ].join('\n')
    )
  })

  it('builds a customer-readable audit receipt from backend event summary', () => {
    const card = buildCommandJobAuditSummaryCard(
      {
        ...result,
        audit_summary: {
          event_count: 2,
          latest_event_type: 'dispatch_failed',
          latest_event_at: '2026-07-05T10:02:00Z',
          latest_message: 'publish failed',
          next_action: 'Review device rows before retry.'
        }
      },
      t
    )

    expect(card).toMatchObject({
      latestLabel: 'Dispatch failed',
      nextAction: 'Review device rows before retry.'
    })
    expect(card?.rows).toEqual([
      { label: 'Events', value: '2' },
      { label: 'Latest event', value: 'dispatch_failed' },
      { label: 'Latest time', value: '2026-07-05 10:02:00 UTC' },
      { label: 'Latest message', value: 'publish failed' }
    ])
  })

  it('builds a customer-readable execution path card from backend evidence', () => {
    const card = buildCommandJobExecutionSummaryCard(
      {
        ...result,
        execution_summary: {
          path_type: 'fleet_job',
          path_label: 'Fleet filter job',
          decision: 'retry',
          can_close: false,
          close_blockers: ['1 devices are still pending.'],
          next_action: 'Review failed rows, then retry only ready devices.',
          evidence: ['5 requested, 4 eligible, 1 blocked', '1 retry-ready, 0 waiting, 0 exhausted'],
          checklist: [
            {
              key: 'retry',
              label: 'Review retry-ready devices',
              state: 'todo',
              detail: '1 devices can be retried now'
            }
          ]
        }
      },
      t
    )

    expect(card).toEqual({
      pathLabel: 'Fleet filter job',
      decisionLabel: 'Retry ready devices',
      canClose: false,
      closeBlockers: ['1 devices are still pending.'],
      nextAction: 'Review failed rows, then retry only ready devices.',
      type: 'error',
      evidence: ['5 requested, 4 eligible, 1 blocked', '1 retry-ready, 0 waiting, 0 exhausted'],
      checklist: [
        {
          key: 'retry',
          label: 'Review retry-ready devices',
          detail: '1 devices can be retried now',
          stateLabel: 'To do',
          type: 'error'
        }
      ]
    })
  })

  it('keeps ambiguous device ACK audit visible as a closeout blocker', () => {
    const closeBlocker =
      'Latest device response was not applied because multiple command-job rows matched; review duplicate message-id evidence.'
    const ambiguousResult = {
      ...result,
      audit_summary: {
        event_count: 3,
        latest_event_type: 'device_ack_ambiguous',
        latest_event_at: '2026-07-05T10:05:00Z',
        latest_message:
          'device response for message msg-1 with status 4 was not applied because multiple command job detail candidates matched this device and message',
        next_action: 'Review duplicate message-id and command-log evidence before closing.'
      },
      execution_summary: {
        path_type: 'fleet_job',
        path_label: 'Fleet filter job',
        decision: 'collect_evidence',
        can_close: false,
        close_blockers: [closeBlocker],
        next_action: 'Review duplicate message-id evidence before closing.',
        checklist: [
          {
            key: 'audit',
            label: 'Resolve ambiguous device response',
            state: 'blocked',
            detail: 'Latest event: device_ack_ambiguous'
          }
        ]
      }
    } as any

    const auditCard = buildCommandJobAuditSummaryCard(ambiguousResult, t)
    const executionCard = buildCommandJobExecutionSummaryCard(ambiguousResult, t)
    const closeoutPacket = buildCommandJobCloseoutPacket(
      ambiguousResult,
      'https://example.test/jobs?command_job_id=job-1'
    )

    expect(auditCard).toMatchObject({
      latestLabel: 'Device ack ambiguous',
      nextAction: 'Review duplicate message-id and command-log evidence before closing.'
    })
    expect(executionCard).toMatchObject({
      decisionLabel: 'Collect evidence',
      canClose: false,
      closeBlockers: [closeBlocker],
      checklist: [
        {
          key: 'audit',
          label: 'Resolve ambiguous device response',
          stateLabel: 'Blocked',
          type: 'error'
        }
      ]
    })
    expect(closeoutPacket).toContain('latest=device_ack_ambiguous')
    expect(closeoutPacket).toContain(closeBlocker)
    expect(closeoutPacket).toContain(
      '- [blocked] Resolve ambiguous device response: Latest event: device_ack_ambiguous'
    )
  })

  it('falls back to local progress health when backend health is absent', () => {
    const card = buildCommandJobProgressHealthCard({ ...result, status: 'running', progress_health: undefined }, t)

    expect(card).toMatchObject({
      stateLabel: 'Running',
      type: 'info',
      nextAction: 'Keep waiting.'
    })
    expect(card?.rows[0]).toEqual({ label: 'Pending', value: '0' })
  })

  it('keeps retry enabled from aggregate evidence when detail rows are summary-only', () => {
    const summaryOnlyResult = {
      ...result,
      can_retry_failed: true,
      retryable_count: 3,
      retry_ready_count: 3,
      rows: []
    }

    expect(commandJobRetryableRows(summaryOnlyResult).map((row) => row.device_id)).toEqual([])
    expect(commandJobRetryableCount(summaryOnlyResult)).toBe(3)
    expect(canRetryCommandJob(summaryOnlyResult)).toBe(true)

    expect(
      canRetryCommandJob({
        ...summaryOnlyResult,
        retry_ready_count: 0,
        retry_waiting_count: 3
      })
    ).toBe(false)

    expect(
      canRetryCommandJob({
        ...summaryOnlyResult,
        can_retry_failed: false
      })
    ).toBe(false)
  })

  it('shows a planned job as waiting instead of an execution failure', () => {
    const scheduledResult = {
      ...result,
      status: 'scheduled',
      scheduled_at: '2026-07-20T01:00:00Z',
      next_dispatch_at: '2026-07-20T01:00:00.500Z',
      progress_health: {
        state: 'scheduled',
        pending_count: 4,
        terminal_count: 1,
        elapsed_seconds: 0,
        timeout_remaining_seconds: 3660,
        next_action: ''
      }
    }

    expect(buildCommandJobStatusRows(scheduledResult, t)).toContainEqual({
      label: 'Scheduled start',
      value: '2026-07-20 01:00:00 UTC'
    })
    expect(buildCommandJobStatusRows(scheduledResult, t)).toContainEqual({
      label: 'Next dispatch slot',
      value: '2026-07-20 01:00:00 UTC'
    })
    expect(buildCommandJobProgressHealthCard(scheduledResult, t)).toMatchObject({
      stateLabel: 'Scheduled',
      type: 'info',
      nextAction: 'Wait for the scheduled start.'
    })
  })

  it('builds rows for retry, missing logs, status counts, and timeline', () => {
    expect(commandJobRetryableRows(result).map((row) => row.device_id)).toEqual(['dev-2'])
    expect(commandJobRetryableCount(result)).toBe(1)
    expect(commandJobLogMissingRows(result).map((row) => row.device_id)).toEqual(['dev-2'])
    expect(commandJobLogMissingCount(result)).toBe(1)

    expect(buildCommandJobStatusRows(result, t)).toEqual([
      { label: 'Job ID', value: 'job-1' },
      { label: 'Status', value: 'Partially failed' },
      { label: 'Progress', value: '3/5 submitted, 1 failed' },
      { label: 'Created', value: '2026-07-05 10:00:00 UTC' },
      { label: 'Updated', value: '2026-07-05 10:01:00 UTC' },
      { label: 'Timeout', value: '--' }
    ])
    expect(buildCommandJobStatusCountRows(result, t)).toEqual([
      { status: 'submitted', label: 'submitted', count: 3 },
      { status: 'failed', label: 'Failed', count: 1 }
    ])
    expect(buildCommandJobTroubleshootingRows(result, t)).toEqual([
      {
        key: 'retry',
        label: 'Retry failed devices',
        value: '1 devices can be retried',
        reviewRowsStatusFilter: 'retry_ready',
        type: 'error'
      },
      {
        key: 'cancel',
        label: 'Cancel pending rows',
        value: '1 rows are pending',
        reviewRowsStatusFilter: 'in_progress',
        type: 'warning'
      },
      {
        key: 'logs',
        label: 'Refresh logs',
        value: '1 devices need logs',
        reviewRowsStatusFilter: 'missing_log',
        type: 'warning'
      },
      {
        key: 'blocked',
        label: 'Fix blocked devices',
        value: '1 devices are blocked',
        reviewRowsStatusFilter: 'needs_attention',
        type: 'error'
      }
    ])
    expect(buildCommandJobTimelineRows(result, t)).toEqual([
      { key: 'created', label: 'Created', value: '2026-07-05 10:00:00 UTC' },
      { key: 'submitted', label: 'Submitted', value: '2 devices' },
      { key: 'completed', label: 'Completed', value: '1 devices' }
    ])
    expect(buildCommandJobOutcomeGroups(result, t)).toEqual([
      {
        key: 'retryable',
        title: 'Retryable failures',
        description: 'Review and retry only these devices.',
        count: 1,
        type: 'error',
        rows: [
          {
            key: 'dev-2',
            deviceId: 'dev-2',
            device: 'dev-2',
            status: 'Failed',
            readiness: '-',
            reason: '-',
            action: 'Retry this device after reviewing the failure reason.'
          }
        ]
      },
      {
        key: 'blocked',
        title: 'Blocked before dispatch',
        description: 'Fix blockers first.',
        count: 1,
        type: 'error',
        rows: [
          {
            key: 'dev-3',
            deviceId: 'dev-3',
            device: 'dev-3',
            status: 'blocked',
            readiness: '-',
            reason: '-',
            action: 'Fix the blocker shown in the reason before sending again.'
          }
        ]
      },
      {
        key: 'in_progress',
        title: 'Still in progress',
        description: 'Refresh until terminal.',
        count: 1,
        type: 'info',
        rows: [
          {
            key: 'dev-1',
            deviceId: 'dev-1',
            device: 'dev-1',
            status: 'submitted',
            readiness: '-',
            reason: '-',
            action: 'Copy the support bundle if unclear.'
          }
        ]
      },
      {
        key: 'completed',
        title: 'Completed',
        description: 'No action needed.',
        count: 1,
        type: 'success',
        rows: [
          {
            key: 'dev-4',
            deviceId: 'dev-4',
            device: 'dev-4',
            status: 'submitted',
            readiness: '-',
            reason: '-',
            action: 'Device returned success.'
          }
        ]
      }
    ])
  })

  it('builds one prioritized operator next action from job evidence', () => {
    expect(
      buildCommandJobOperatorNextAction(
        {
          ...result,
          rows: [
            {
              device_id: 'dev-ack-failed',
              eligible: true,
              status: 'submitted',
              readiness: ['ready_with_caution', 'mqtt_online'],
              response_status_label: 'device_ack_failed',
              can_retry: true,
              log_recorded: true
            }
          ]
        },
        t
      )
    ).toMatchObject({
      key: 'device-response',
      primaryAction: 'preview-support',
      reviewRowsStatusFilter: 'device_failed',
      type: 'error'
    })

    expect(buildCommandJobOperatorNextAction(result, t)).toMatchObject({
      key: 'retry',
      primaryAction: 'retry',
      reviewRowsStatusFilter: 'retry_ready',
      type: 'error'
    })

    expect(
      buildCommandJobOperatorNextAction(
        {
          ...result,
          status: 'running',
          can_retry_failed: false,
          retryable_count: 0,
          log_missing_count: 0,
          blocked_count: 0,
          submitted_count: 2,
          failed_count: 0,
          rows: []
        },
        t
      )
    ).toMatchObject({
      key: 'refresh',
      primaryAction: 'refresh',
      reviewRowsStatusFilter: 'in_progress',
      type: 'info'
    })

    expect(
      buildCommandJobOperatorNextAction(
        {
          ...result,
          status: 'partially_failed',
          can_retry_failed: false,
          retryable_count: 0,
          log_missing_count: 2,
          blocked_count: 0,
          submitted_count: 5,
          failed_count: 0,
          rows: []
        },
        t
      )
    ).toMatchObject({
      key: 'logs',
      primaryAction: 'refresh',
      reviewRowsStatusFilter: 'missing_log',
      type: 'warning'
    })

    expect(
      buildCommandJobOperatorNextAction(
        {
          ...result,
          status: 'completed',
          can_retry_failed: false,
          retryable_count: 0,
          log_missing_count: 0,
          blocked_count: 0,
          submitted_count: 5,
          failed_count: 0,
          rows: []
        },
        t
      )
    ).toMatchObject({
      key: 'done',
      primaryAction: 'copy-link',
      type: 'success'
    })
  })

  it('prefers backend audit events over derived timeline rows', () => {
    expect(
      buildCommandJobTimelineRows(
        {
          ...result,
          events: [
            {
              id: 'event-1',
              event_type: 'created',
              message: 'queued from preview',
              created_at: '2026-07-05T10:00:00Z'
            },
            {
              id: 'event-2',
              event_type: 'scheduled',
              message: 'waiting for planned start',
              created_at: '2026-07-05T10:00:01Z'
            },
            {
              id: 'event-3',
              event_type: 'started',
              message: 'planned start reached',
              created_at: '2026-07-05T10:00:04Z'
            },
            {
              id: 'event-4',
              event_type: 'dispatch_started',
              device_id: 'dev-1',
              message: 'worker claimed row',
              created_at: '2026-07-05T10:00:05Z'
            }
          ]
        },
        t
      )
    ).toEqual([
      { key: 'event-1', label: 'Job created', value: '2026-07-05 10:00:00 UTC - queued from preview' },
      { key: 'event-2', label: 'Job scheduled', value: '2026-07-05 10:00:01 UTC - waiting for planned start' },
      { key: 'event-3', label: 'Scheduled job started', value: '2026-07-05 10:00:04 UTC - planned start reached' },
      {
        key: 'event-4',
        label: 'Dispatch started',
        value: '2026-07-05 10:00:05 UTC - dev-1 - worker claimed row'
      }
    ])
  })

  it('builds cancel and retry consequence hints from aggregate job evidence', () => {
    expect(
      buildCommandJobActionConsequenceRows(
        {
          ...result,
          can_cancel: true,
          can_retry_failed: true,
          retry_ready_count: 2,
          retry_waiting_count: 1,
          retry_exhausted_count: 1,
          log_missing_count: 1,
          progress_health: {
            state: 'running',
            pending_count: 4,
            terminal_count: 1,
            elapsed_seconds: 20,
            timeout_remaining_seconds: 40,
            next_action: 'Keep watching.'
          }
        },
        t
      )
    ).toEqual([
      {
        key: 'cancel',
        label: 'custom.commandCenter.cancelJob',
        value: 'Cancel 4 pending rows while partially_failed.',
        reviewRowsStatusFilter: 'in_progress',
        type: 'warning'
      },
      {
        key: 'retry',
        label: 'custom.commandCenter.retryFailedJob',
        value: 'Retry 2 ready, 1 waiting, 1 exhausted.',
        reviewRowsStatusFilter: 'retry_ready',
        type: 'error'
      },
      {
        key: 'waiting',
        label: 'Waiting for retry window',
        value: '1 waiting.',
        reviewRowsStatusFilter: 'retry_waiting',
        type: 'warning'
      },
      {
        key: 'exhausted',
        label: 'Retry limit reached',
        value: '1 exhausted.',
        reviewRowsStatusFilter: 'retry_exhausted',
        type: 'error'
      },
      {
        key: 'logs',
        label: 'Missing log evidence',
        value: '1 missing logs.',
        reviewRowsStatusFilter: 'missing_log',
        type: 'warning'
      }
    ])
  })

  it('builds per-device progress tracks from preview, dispatch, ACK, and evidence fields', () => {
    const tracks = buildCommandJobDeviceProgressTracks(
      {
        ...result,
        rows: [
          {
            device_id: 'dev-ok',
            device_number: 'pump-1',
            eligible: true,
            status: 'completed',
            readiness: ['mqtt_online'],
            message_id: 'msg-1',
            response_status_label: 'device_ack_success',
            command_log_created_at: '2026-07-05T10:00:31Z',
            log_recorded: true,
            can_retry: false,
            submitted_at: '2026-07-05T10:00:30Z',
            completed_at: '2026-07-05T10:00:31Z'
          },
          {
            device_id: 'dev-failed',
            eligible: true,
            status: 'failed',
            dispatch_attempts: 2,
            max_dispatch_attempts: 3,
            retry_state: 'waiting_backoff',
            next_retry_after: '2026-07-05T10:03:00Z',
            reason: 'broker timeout',
            can_retry: true,
            log_recorded: false
          },
          {
            device_id: 'dev-blocked',
            eligible: false,
            status: 'blocked',
            reason: 'permission missing',
            can_retry: false,
            log_recorded: false
          }
        ]
      },
      t
    )

    expect(tracks.map((track) => [track.device, track.type])).toEqual([
      ['dev-failed', 'error'],
      ['dev-blocked', 'error'],
      ['pump-1', 'success']
    ])
    expect(tracks[0].steps.map((step) => [step.key, step.state])).toEqual([
      ['preview', 'Done'],
      ['dispatch', 'Failed 2/3'],
      ['ack', 'Waiting'],
      ['evidence', 'Missing']
    ])
    expect(tracks[1].steps[0]).toMatchObject({
      key: 'preview',
      state: 'Blocked',
      detail: 'permission missing',
      type: 'error'
    })
    expect(tracks[2].steps.map((step) => step.state)).toEqual(['Done', 'Done', 'Done', 'Done'])
  })

  it('treats device failure responses as retryable evidence', () => {
    expect(
      buildCommandJobOutcomeGroups(
        {
          ...result,
          requested_count: 1,
          eligible_count: 1,
          blocked_count: 0,
          submitted_count: 1,
          failed_count: 0,
          status_counts: {
            submitted: 1
          },
          rows: [
            {
              device_id: 'dev-ack-failed',
              eligible: true,
              status: 'submitted',
              response_status_label: 'device_ack_failed',
              response_error: 'motor refused command',
              can_retry: false,
              log_recorded: true,
              submitted_at: '2026-07-05T10:00:30Z'
            }
          ]
        },
        t
      )
    ).toEqual([
      {
        key: 'device_failed',
        title: 'Device response failures',
        description: 'Review device-side error evidence before retrying.',
        count: 1,
        type: 'error',
        rows: [
          {
            key: 'dev-ack-failed',
            device: 'dev-ack-failed',
            status: 'submitted',
            deviceId: 'dev-ack-failed',
            readiness: '-',
            reason: '-',
            action: 'motor refused command'
          }
        ]
      }
    ])
  })

  it('builds history status options and compact row progress', () => {
    expect(buildCommandJobHistoryStatusOptions(t).map((option) => option.value)).toEqual([
      '',
      'scheduled',
      'running',
      'completed',
      'partially_failed',
      'failed',
      'canceled'
    ])
    expect(buildCommandJobHistoryProgress(result, t)).toBe('3/5 submitted, 1 failed')
    expect(buildCommandJobHistoryAttentionOptions(t).map((option) => option.value)).toEqual([
      '',
      'needs_operator_action',
      'retryable',
      'retry_ready',
      'retry_waiting',
      'retry_exhausted',
      'device_failed',
      'missing_log',
      'blocked'
    ])
    expect(
      buildCommandJobHistoryAttentionSummary(
        {
          ...result,
          needs_operator_action_count: 5,
          retryable_count: 2,
          device_ack_failed_count: 1,
          log_missing_count: 2
        },
        t
      )
    ).toBe('5 need action, 2 retryable, 1 device failed, 2 missing logs')
    expect(buildCommandJobHistoryAttentionSummary(result, t)).toBe('No action needed')
    expect(
      buildCommandJobHistoryAttentionOptions(t, {
        needs_operator_action_count: 6,
        retryable_count: 2,
        device_ack_failed_count: 1,
        log_missing_count: 3,
        blocked_count: 1
      }).map((option) => option.label)
    ).toEqual([
      'All attention',
      'Needs operator action (6)',
      'Retryable failures (2)',
      'Ready to retry',
      'Waiting for retry window',
      'Retry limit reached',
      'Device response failures (1)',
      'Missing log evidence (3)',
      'Blocked before dispatch (1)'
    ])
    expect(
      buildCommandJobHistoryAttentionTotalSummary(
        {
          needs_operator_action_count: 6,
          retryable_count: 2,
          device_ack_failed_count: 1,
          log_missing_count: 3
        },
        t
      )
    ).toBe('6 need action, 2 retryable, 1 device failed, 3 missing logs')
  })

  it('builds aggregate attention rows from the full filtered job history result', () => {
    const rows = buildCommandJobHistoryAttentionAggregateRows(
      {
        needs_operator_action_count: 6,
        retry_ready_count: 2,
        retry_waiting_count: 1,
        retry_exhausted_count: 1,
        device_ack_failed_count: 3,
        blocked_count: 1,
        log_missing_count: 4
      },
      t
    )

    expect(rows.map((row) => [row.key, row.count, row.type, row.filter])).toEqual([
      ['needs_operator_action', 6, 'warning', 'needs_operator_action'],
      ['retry_ready', 2, 'warning', 'retry_ready'],
      ['retry_waiting', 1, 'info', 'retry_waiting'],
      ['retry_exhausted', 1, 'error', 'retry_exhausted'],
      ['device_ack_failed', 3, 'error', 'device_failed'],
      ['blocked', 1, 'error', 'blocked'],
      ['missing_log', 4, 'warning', 'missing_log']
    ])
  })

  it('falls back to support evidence when no troubleshooting action is urgent', () => {
    expect(
      buildCommandJobTroubleshootingRows(
        {
          ...result,
          status: 'completed',
          blocked_count: 0,
          failed_count: 0,
          submitted_count: 4,
          can_cancel: false,
          can_retry_failed: false,
          retryable_count: 0,
          log_missing_count: 0,
          rows: result.rows.map((row: any) => ({
            ...row,
            eligible: true,
            can_retry: false,
            log_recorded: true
          }))
        },
        t
      )
    ).toEqual([
      {
        key: 'support',
        label: 'Keep support evidence',
        value: 'Copy support bundle',
        type: 'success'
      }
    ])
  })

  it('builds a readable support bundle preview from support evidence', () => {
    expect(
      buildCommandJobSupportBundlePreview(
        {
          job_id: 'job-1',
          job_type: 'manual_command',
          scope_type: 'selected_devices',
          identify: 'reboot',
          status: 'partially_failed',
          scheduled_at: '2026-07-05T10:05:00Z',
          next_dispatch_at: '2026-07-05T10:05:00.500Z',
          requested_count: 4,
          eligible_count: 3,
          blocked_count: 1,
          submitted_count: 2,
          failed_count: 1,
          retryable_count: 1,
          retry_ready_count: 0,
          retry_waiting_count: 1,
          retry_exhausted_count: 0,
          log_missing_count: 1,
          retryable_device_ids: ['dev-2'],
          missing_log_device_ids: ['dev-2'],
          failed_devices: [
            {
              device_id: 'dev-2',
              device_number: 'SN-002',
              status: 'failed',
              dispatch_attempts: 2,
              max_dispatch_attempts: 3,
              retry_state: 'waiting_backoff',
              next_retry_after: '2026-07-05T10:15:00Z',
              response_status_label: 'device_ack_failed',
              response_error: 'motor refused command',
              reason: 'offline',
              readiness: ['offline', 'job_path']
            }
          ],
          events: [
            {
              id: 'event-1',
              event_type: 'failed'
            } as any
          ],
          execution_summary: {
            path_type: 'fleet_job',
            path_label: 'Fleet filter job',
            decision: 'wait_schedule',
            can_close: false,
            close_blockers: ['1 devices are missing platform log evidence.'],
            next_action: 'Review failed rows, then retry only ready devices.',
            evidence: ['4 requested, 3 eligible, 1 blocked'],
            checklist: [
              {
                key: 'logs',
                label: 'Collect command log evidence',
                state: 'todo',
                detail: '1 devices are missing platform log evidence'
              }
            ]
          },
          governance_summary: {
            level: 'warning',
            title: 'Job governance',
            summary: '4 requested, 2 submitted, 1 failed, 1 blocked.',
            next_action: 'Review the waiting retry window.',
            items: [
              {
                key: 'retry_policy',
                label: 'Retry policy',
                value: '0 ready / 1 waiting / 0 exhausted',
                state: 'watch',
                detail: 'Wait for the retry window.'
              }
            ]
          },
          next_actions: ['Retry dev-2', 'Check gateway'],
          generated_at: '2026-07-05T10:10:00Z',
          share_hint: 'Send to support'
        },
        t
      )
    ).toEqual({
      summaryRows: [
        { label: 'Generated at', value: '2026-07-05 10:10:00 UTC' },
        { label: 'Scheduled start', value: '2026-07-05 10:05:00 UTC' },
        { label: 'Next dispatch slot', value: '2026-07-05 10:05:00 UTC' },
        { label: 'Retryable devices', value: '1' },
        { label: 'Ready to retry', value: '0' },
        { label: 'Waiting for retry window', value: '1' },
        { label: 'Retry limit reached', value: '0' },
        { label: 'Missing log devices', value: '1' },
        { label: 'Failed devices', value: '1' },
        { label: 'Events', value: '1' },
        { label: 'Share hint', value: 'Send to support' }
      ],
      executionSummary: {
        pathLabel: 'Fleet filter job',
        decisionLabel: 'Wait for scheduled start',
        canClose: false,
        closeBlockers: ['1 devices are missing platform log evidence.'],
        nextAction: 'Review failed rows, then retry only ready devices.',
        type: 'error',
        evidence: ['4 requested, 3 eligible, 1 blocked'],
        checklist: [
          {
            key: 'logs',
            label: 'Collect command log evidence',
            detail: '1 devices are missing platform log evidence',
            stateLabel: 'To do',
            type: 'error'
          }
        ]
      },
      governanceSummary: {
        title: 'Job governance',
        levelLabel: 'Review',
        summary: '4 requested, 2 submitted, 1 failed, 1 blocked.',
        nextAction: 'Review the waiting retry window.',
        type: 'warning',
        items: [
          {
            key: 'retry_policy',
            label: 'Retry policy',
            value: '0 ready / 1 waiting / 0 exhausted',
            detail: 'Wait for the retry window.',
            stateLabel: 'Watch',
            type: 'warning'
          }
        ]
      },
      nextActions: ['Retry dev-2', 'Check gateway'],
      failedDeviceEvidence: [
        {
          key: 'dev-2',
          deviceId: 'dev-2',
          rows: [
            { label: 'Device', value: 'SN-002' },
            { label: 'Status', value: 'Failed' },
            { label: 'Readiness evidence', value: 'offline, job_path' },
            { label: 'Reason', value: 'offline' },
            { label: 'Advice', value: '-' },
            { label: 'Dispatch attempts', value: '2/3' },
            { label: 'Retry state', value: 'Waiting for retry window' },
            { label: 'Next retry after', value: '2026-07-05 10:15:00 UTC' },
            { label: 'Message ID', value: '-' },
            { label: 'Device response', value: 'Device failure' },
            { label: 'Response evidence', value: 'motor refused command' }
          ]
        }
      ]
    })
  })
})
