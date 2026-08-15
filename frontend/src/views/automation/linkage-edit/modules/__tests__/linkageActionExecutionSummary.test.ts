/**
 * 文件用途: 覆盖联动编辑动作执行摘要的表单态展示规则。
 * 核心逻辑: 验证设备指令、场景激活、告警触发、空值和对象值的摘要格式。
 * 关键注意事项: 这里只证明本地表单摘要可读，不代表后端预演或真实执行结果。
 * 重构建议: 保持表单态 actionValue/actionType 与提交态 action_value/action_type helper 分离。
 */
import { describe, expect, it } from 'vitest'
import {
  buildActionExecutionSummaryItems,
  buildDeviceInstructionSummary,
  findOptionName,
  formatSummaryValue
} from '../linkageActionExecutionSummary'

const labels = {
  unset: 'Unset',
  singleDevice: 'Single device',
  singleClassDevice: 'Device profile',
  operateDevice: 'Operate device',
  activateScene: 'Activate scene',
  triggerAlarm: 'Trigger alarm',
  activate: 'Activate',
  trigger: 'Trigger'
}

const catalogs = {
  deviceOptions: [{ id: 'device-1', name: 'Pump A' }],
  deviceConfigOptions: [{ id: 'profile-1', name: 'Valve profile' }],
  sceneOptions: [{ id: 'scene-1', name: 'Night safety check' }],
  alarmOptions: [{ id: 'alarm-1', name: 'High temperature' }]
}

describe('linkageActionExecutionSummary', () => {
  it('formats option and value fallbacks without implying execution success', () => {
    expect(findOptionName(catalogs.deviceOptions, 'device-1', labels.unset)).toBe('Pump A')
    expect(findOptionName(catalogs.deviceOptions, 'missing-device', labels.unset)).toBe('missing-device')
    expect(findOptionName(catalogs.deviceOptions, '', labels.unset)).toBe(labels.unset)

    expect(formatSummaryValue(undefined, labels.unset)).toBe(labels.unset)
    expect(formatSummaryValue('', labels.unset)).toBe(labels.unset)
    expect(formatSummaryValue({ mode: 'auto' }, labels.unset)).toBe('{"mode":"auto"}')
    expect(formatSummaryValue(0, labels.unset)).toBe('0')
  })

  it('summarizes single-device and profile-device instruction form state', () => {
    expect(
      buildDeviceInstructionSummary(
        {
          action_type: '10',
          action_target: 'device-1',
          action_param: 'switch',
          actionParamData: { label: 'Main switch' },
          actionValue: true
        },
        0,
        catalogs,
        labels
      )
    ).toEqual({
      key: 'device-0',
      tag: 'Single device',
      text: 'Pump A / Main switch = true'
    })

    expect(
      buildDeviceInstructionSummary(
        {
          action_type: '11',
          action_target: 'profile-1',
          action_param: 'speed',
          actionValue: 3
        },
        1,
        catalogs,
        labels
      )
    ).toEqual({
      key: 'device-1',
      tag: 'Device profile',
      text: 'Valve profile / speed = 3'
    })
  })

  it('builds local action summary items from linkage form state only', () => {
    expect(
      buildActionExecutionSummaryItems(
        [
          {
            actionType: '1',
            actionInstructList: [
              {
                action_type: '10',
                action_target: 'device-1',
                action_param: 'temperature',
                actionValue: 28
              }
            ]
          },
          {
            actionType: '20',
            action_target: 'scene-1'
          },
          {
            actionType: '30',
            action_target: 'alarm-1'
          }
        ],
        catalogs,
        labels
      )
    ).toEqual([
      {
        key: 'group-0',
        tag: 'Operate device',
        lines: [
          {
            key: 'device-0',
            tag: 'Single device',
            text: 'Pump A / temperature = 28'
          }
        ]
      },
      {
        key: 'group-1',
        tag: 'Activate scene',
        lines: [
          {
            key: 'scene',
            tag: 'Activate',
            text: 'Night safety check'
          }
        ]
      },
      {
        key: 'group-2',
        tag: 'Trigger alarm',
        lines: [
          {
            key: 'alarm',
            tag: 'Trigger',
            text: 'High temperature'
          }
        ]
      }
    ])
  })
})
