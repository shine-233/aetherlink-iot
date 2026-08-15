/**
 * 文件用途: 覆盖联动编辑保存流程的纯规则。
 * 核心逻辑: 验证提交阻断提示、保存后返回路由和新增/编辑 API 选择。
 * 关键注意事项: 这里只证明前端保存流程分支，不代表后端规则执行或设备动作成功。
 * 重构建议: 保存流程可继续保留在 helper，表单 ref 校验与确认弹窗仍由页面组件负责。
 */
import { describe, expect, it, vi } from 'vitest'
import {
  getAutomationDryRunSaveBlocker,
  getAutomationSubmitBlocker,
  normalizeAutomationDryRunBlockers,
  resolveAutomationPostSaveRoute,
  runAutomationDryRunSaveGate,
  saveAutomationDefinition
} from '../automationSaveFlow'

const t = (key: string) => key

describe('automationSaveFlow', () => {
  it('resolves post-save navigation without duplicating page branches', () => {
    expect(resolveAutomationPostSaveRoute('device', { device_id: 'device-1' })).toEqual({
      path: '/device/details',
      query: { d_id: 'device-1' }
    })
    expect(resolveAutomationPostSaveRoute('config', { device_config_id: 'config-1' })).toEqual({
      path: '/device/config-detail',
      query: { id: 'config-1' }
    })
    expect(resolveAutomationPostSaveRoute('', {})).toEqual({ path: '/automation/scene-linkage' })
  })

  it('chooses add or edit API and only reports success when the response has no error', async () => {
    const addAutomation = vi.fn().mockResolvedValue({ error: null })
    const editAutomation = vi.fn().mockResolvedValue({ error: { msg: 'failed' } })
    const payload = { trigger_condition_groups: [], actions: [] }

    await expect(saveAutomationDefinition({ isEdit: false, payload, addAutomation, editAutomation })).resolves.toBe(
      true
    )
    expect(addAutomation).toHaveBeenCalledWith(payload)
    expect(editAutomation).not.toHaveBeenCalled()

    await expect(saveAutomationDefinition({ isEdit: true, payload, addAutomation, editAutomation })).resolves.toBe(
      false
    )
    expect(editAutomation).toHaveBeenCalledWith(payload)
  })

  it('keeps submit blockers as warning text rather than pretending to save', () => {
    expect(
      getAutomationSubmitBlocker(
        {
          trigger_condition_groups: [
            [
              {
                trigger_conditions_type: '22'
              }
            ]
          ],
          actions: []
        },
        t
      )
    ).toBe('generate.timeRangeWarning')

    expect(
      getAutomationSubmitBlocker(
        {
          trigger_condition_groups: [
            [
              {
                trigger_conditions_type: '10',
                trigger_param_type: 'event',
                trigger_value: '{"match_mode":"field","conditions":[]}'
              }
            ]
          ],
          actions: []
        },
        t
      )
    ).toBe('generate.eventParamConditionRequired')
  })

  it('normalizes backend dry-run blockers before save', () => {
    expect(
      normalizeAutomationDryRunBlockers({
        blocking_errors: ['missing action', { message: 'missing condition' }, { code: 'bad-ref' }, null]
      })
    ).toEqual(['missing action', 'missing condition', 'bad-ref'])
    expect(normalizeAutomationDryRunBlockers({ blockers: ['blocked by backend'] })).toEqual(['blocked by backend'])
    expect(normalizeAutomationDryRunBlockers({ blockers: 'not-an-array' })).toEqual([])
  })

  it('keeps backend dry-run as a save gate instead of treating preview as success', async () => {
    const payload = { trigger_condition_groups: [], actions: [] }

    await expect(
      runAutomationDryRunSaveGate({
        payload,
        runBackendDryRunForPayload: vi.fn().mockResolvedValue(null),
        backendUnavailableMessage: 'dry-run unavailable',
        saveBlockedMessage: 'dry-run blocked'
      })
    ).resolves.toEqual({ canSave: false, message: 'dry-run unavailable' })

    await expect(
      runAutomationDryRunSaveGate({
        payload,
        runBackendDryRunForPayload: vi.fn().mockResolvedValue({ can_save: false, blockers: ['fix condition'] }),
        backendUnavailableMessage: 'dry-run unavailable',
        saveBlockedMessage: 'dry-run blocked'
      })
    ).resolves.toEqual({ canSave: false, message: 'fix condition' })

    await expect(
      runAutomationDryRunSaveGate({
        payload,
        runBackendDryRunForPayload: vi.fn().mockResolvedValue({ canSave: false }),
        backendUnavailableMessage: 'dry-run unavailable',
        saveBlockedMessage: 'dry-run blocked'
      })
    ).resolves.toEqual({ canSave: false, message: 'dry-run blocked' })

    await expect(
      runAutomationDryRunSaveGate({
        payload,
        runBackendDryRunForPayload: vi.fn().mockResolvedValue({ can_save: true, warnings: ['review telemetry'] }),
        backendUnavailableMessage: 'dry-run unavailable',
        saveBlockedMessage: 'dry-run blocked'
      })
    ).resolves.toEqual({ canSave: true, message: '' })
  })

  it('reports the first dry-run save blocker with fallback copy', () => {
    expect(getAutomationDryRunSaveBlocker({ can_save: false }, 'blocked')).toBe('blocked')
    expect(getAutomationDryRunSaveBlocker({ blocking_errors: [{ message: 'select target' }] }, 'blocked')).toBe(
      'select target'
    )
    expect(getAutomationDryRunSaveBlocker({ can_save: true }, 'blocked')).toBe('')
  })
})
