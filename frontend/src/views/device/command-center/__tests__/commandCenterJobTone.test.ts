import { describe, expect, it } from 'vitest'
import type { FleetCommandJobSubmitResult, FleetCommandJobSubmitRow } from '@/service/api/device'
import {
  type CommandJobTone,
  commandJobToneSeverity,
  createCommandJobToneResolver,
  fillCommandJobTemplate,
  resolveCommandJobCountTone,
  resolveCommandJobDeviceStepTone,
  resolveCommandJobGovernanceLevelTone,
  resolveCommandJobGovernanceStateTone,
  resolveCommandJobProgressHealthTone,
  translateCommandJobOr
} from '../commandCenterJobFormat'
import {
  buildCommandJobDeviceProgressTracks,
  buildCommandJobOutcomeGroups,
  resolveCommandJobDeviceTrackTone,
  resolveCommandJobOutcomeGroupKey,
  resolveCommandJobOutcomeGroupTone
} from '../commandCenterJobOutcome'
import { buildCommandJobProgressHealthCard, resolveCommandJobProgressHealth } from '../commandCenterJobProgress'
import { buildCommandJobHistoryAttentionAggregateRows } from '../commandCenterJobHistory'
import * as barrel from '../commandCenterJobView'

const t = (key: string) => key

const row = (patch: Partial<FleetCommandJobSubmitRow> = {}): FleetCommandJobSubmitRow => ({
  device_id: 'dev-1',
  eligible: true,
  status: 'submitted',
  can_retry: false,
  log_recorded: true,
  ...patch
})

const tableCases = <T extends string>(cases: Record<T, CommandJobTone>) =>
  Object.entries(cases) as Array<[T, CommandJobTone]>

describe('command job tone tables', () => {
  it.each(
    tableCases({
      complete: 'success',
      scheduled: 'info',
      running: 'info',
      timeout_risk: 'warning',
      timed_out: 'error',
      needs_attention: 'error',
      canceled: 'error',
      unexpected_state: 'error'
    })
  )('progress health %s -> %s', (state, tone) => {
    expect(resolveCommandJobProgressHealthTone(state)).toBe(tone)
  })

  it.each(
    tableCases({
      success: 'success',
      warning: 'warning',
      error: 'error',
      blocked: 'error',
      info: 'info',
      ok: 'info',
      unknown: 'info'
    })
  )('governance level %s -> %s', (level, tone) => {
    expect(resolveCommandJobGovernanceLevelTone(level)).toBe(tone)
  })

  it.each(tableCases({ done: 'success', blocked: 'error', watch: 'warning', todo: 'warning', other: 'info' }))(
    'governance item state %s -> %s',
    (state, tone) => {
      expect(resolveCommandJobGovernanceStateTone(state)).toBe(tone)
    }
  )

  it.each(
    tableCases({
      done: 'success',
      blocked: 'error',
      failed: 'error',
      waiting: 'warning',
      missing: 'warning',
      pending: 'info'
    })
  )('device step state %s -> %s', (state, tone) => {
    expect(resolveCommandJobDeviceStepTone(state)).toBe(tone)
  })

  it.each(
    tableCases({
      retryable: 'error',
      device_failed: 'error',
      missing_logs: 'warning',
      blocked: 'error',
      in_progress: 'info',
      completed: 'success'
    })
  )('outcome group %s -> %s', (group, tone) => {
    expect(resolveCommandJobOutcomeGroupTone(group)).toBe(tone)
  })

  it('treats empty and prototype keys as unknown', () => {
    const resolve = createCommandJobToneResolver({ done: 'success' }, 'info')
    expect(resolve(undefined)).toBe('info')
    expect(resolve('')).toBe('info')
    expect(resolve('toString')).toBe('info')
    expect(resolve('constructor')).toBe('info')
  })

  it('maps counts to healthy or the metric tone', () => {
    expect(resolveCommandJobCountTone(undefined, 'error')).toBe('success')
    expect(resolveCommandJobCountTone(0, 'error')).toBe('success')
    expect(resolveCommandJobCountTone(2, 'info')).toBe('info')
  })

  it('orders severity error < warning < info < success', () => {
    const tones: CommandJobTone[] = ['success', 'info', 'error', 'warning']
    expect([...tones].sort((a, b) => commandJobToneSeverity(a) - commandJobToneSeverity(b))).toEqual([
      'error',
      'warning',
      'info',
      'success'
    ])
  })
})

describe('command job row classification', () => {
  const past = '2000-01-01T00:00:00Z'
  const future = '2999-01-01T00:00:00Z'

  it.each<[string, Partial<FleetCommandJobSubmitRow>, CommandJobTone, string]>([
    ['ack failed', { response_status_label: 'device_ack_failed' }, 'error', 'device_failed'],
    ['retry ready', { status: 'failed', can_retry: true }, 'error', 'retryable'],
    ['retry exhausted', { status: 'failed', dispatch_attempts: 3, max_dispatch_attempts: 3 }, 'error', 'in_progress'],
    ['preview blocked', { eligible: false, recommended_path: 'blocked' }, 'error', 'blocked'],
    ['ack success', { response_status_label: 'device_ack_success' }, 'success', 'completed'],
    ['completed status', { status: 'completed' }, 'success', 'completed'],
    ['log missing', { log_recorded: false }, 'warning', 'missing_logs'],
    [
      'retry waiting',
      { status: 'failed', retry_state: 'waiting_backoff', next_retry_after: future },
      'warning',
      'in_progress'
    ],
    ['plain in-flight', { next_retry_after: past }, 'info', 'in_progress'],
    // 两套规则刻意不同：轨道优先暴露失败信号，分组优先采信设备 ACK。
    [
      'ack success but retryable',
      { response_status_label: 'device_ack_success', can_retry: true },
      'error',
      'completed'
    ]
  ])('%s', (_name, patch, trackTone, groupKey) => {
    const subject = row(patch)
    expect(resolveCommandJobDeviceTrackTone(subject)).toBe(trackTone)
    expect(resolveCommandJobOutcomeGroupKey(subject)).toBe(groupKey)
  })

  it('sorts tracks by severity while keeping input order inside a tone', () => {
    const result = {
      rows: [
        row({ device_id: 'ok-1', status: 'completed' }),
        row({ device_id: 'warn-1', log_recorded: false }),
        row({ device_id: 'err-1', can_retry: true, status: 'failed' }),
        row({ device_id: 'ok-2', status: 'completed' }),
        row({ device_id: 'err-2', eligible: false })
      ]
    } as unknown as FleetCommandJobSubmitResult
    const tracks = buildCommandJobDeviceProgressTracks(result, t)
    expect(tracks.map((track) => track.deviceId)).toEqual(['err-1', 'err-2', 'warn-1', 'ok-1', 'ok-2'])
    expect(buildCommandJobDeviceProgressTracks(result, t, 2)).toHaveLength(2)
    expect(buildCommandJobDeviceProgressTracks(result, t, -1)).toHaveLength(0)
  })

  it('caps outcome group rows at five but keeps the full count', () => {
    const result = {
      rows: Array.from({ length: 7 }, (_, index) => row({ device_id: `d${index}`, status: 'completed' }))
    } as unknown as FleetCommandJobSubmitResult
    const [group] = buildCommandJobOutcomeGroups(result, t)
    expect(group).toMatchObject({ key: 'completed', count: 7, type: 'success' })
    expect(group.rows).toHaveLength(5)
  })
})

describe('command job fallbacks', () => {
  it.each([
    ['completed', 'complete', 'success'],
    ['scheduled', 'scheduled', 'info'],
    ['running', 'running', 'info'],
    ['canceled', 'canceled', 'error'],
    ['failed', 'needs_attention', 'error'],
    ['partially_failed', 'needs_attention', 'error']
  ])('derives health for legacy status %s', (status, state, tone) => {
    const result = {
      status,
      requested_count: 4,
      submitted_count: 1,
      failed_count: 1,
      blocked_count: 1
    } as unknown as FleetCommandJobSubmitResult
    expect(resolveCommandJobProgressHealth(result)).toMatchObject({ state, pending_count: 1, terminal_count: 3 })
    expect(buildCommandJobProgressHealthCard(result, t)?.type).toBe(tone)
  })

  it('assigns each aggregate attention row its own active tone', () => {
    const rows = buildCommandJobHistoryAttentionAggregateRows(
      {
        needs_operator_action_count: 1,
        retry_ready_count: 1,
        retry_waiting_count: 1,
        retry_exhausted_count: 1,
        device_ack_failed_count: 1,
        blocked_count: 1,
        log_missing_count: 1
      },
      t
    )
    expect(Object.fromEntries(rows.map((item) => [item.key, item.type]))).toEqual({
      needs_operator_action: 'warning',
      retry_ready: 'warning',
      retry_waiting: 'info',
      retry_exhausted: 'error',
      device_ack_failed: 'error',
      blocked: 'error',
      missing_log: 'warning'
    })
    expect(buildCommandJobHistoryAttentionAggregateRows(undefined, t).every((item) => item.type === 'success')).toBe(
      true
    )
  })

  it('fills templates literally and falls back on missing translations', () => {
    expect(fillCommandJobTemplate('{a} and {a}, {b}', { a: '$&', b: 2 })).toBe('$& and $&, 2')
    expect(translateCommandJobOr(t, 'missing.key', 'fallback')).toBe('fallback')
    expect(translateCommandJobOr(() => 'Hello', 'present.key', 'fallback')).toBe('Hello')
  })

  it('keeps the legacy barrel exporting the split modules', () => {
    expect(barrel.buildCommandJobOutcomeGroups).toBe(buildCommandJobOutcomeGroups)
    expect(barrel.buildCommandJobHistoryAttentionAggregateRows).toBe(buildCommandJobHistoryAttentionAggregateRows)
    expect(barrel.resolveCommandJobProgressHealthTone).toBe(resolveCommandJobProgressHealthTone)
    expect(typeof barrel.buildCommandJobOperatorNextAction).toBe('function')
  })
})
