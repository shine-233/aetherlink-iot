/**
 * 文件用途: 覆盖联动编辑详情回填状态构建。
 * 核心逻辑: 验证后端详情可以转换为页面表单、条件组件和动作组件需要的编辑态数据。
 * 关键注意事项: 这里只证明编辑态回填规则，不代表保存、dry-run 或真实自动化执行。
 * 重构建议: 后续接口字段变化时，先补这里的回填用例再改页面。
 */
import { describe, expect, it } from 'vitest'
import { buildAutomationEditEchoState } from '../automationEditEchoState'

describe('automationEditEchoState', () => {
  it('returns null when no detail payload is available', () => {
    expect(buildAutomationEditEchoState(null)).toBeNull()
    expect(buildAutomationEditEchoState(undefined)).toBeNull()
  })

  it('builds editor echo state and falls back missing arrays to empty lists', () => {
    const detail = {
      id: 'scene-1',
      name: 'Temperature alarm'
    }

    expect(buildAutomationEditEchoState(detail)).toEqual({
      automationsInfo: detail,
      configForm: detail,
      conditionData: [],
      actionData: []
    })
  })

  it('groups device instructions for the action editor', () => {
    const detail = {
      id: 'scene-1',
      trigger_condition_groups: [],
      actions: [
        {
          action_type: '10',
          action_param_type: 'telemetry',
          action_param: 'temperature',
          action_value: '{"temperature":30}'
        },
        {
          action_type: '30',
          action_target: 'alarm-1'
        }
      ]
    }

    expect(buildAutomationEditEchoState(detail)?.actionData).toEqual([
      {
        action_type: '30',
        action_target: 'alarm-1',
        actionType: '30'
      },
      {
        actionType: '1',
        actionInstructList: [
          {
            action_type: '10',
            action_param_type: 'telemetry',
            action_param: 'temperature',
            action_value: '{"temperature":30}',
            actionParamOptions: [],
            actionValue: 30
          }
        ]
      }
    ])
  })
})
