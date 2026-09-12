import { describe, expect, it } from 'vitest'
import {
  canEditDocument,
  commandRequiresConfirmation,
  isArchivedError,
  isTelemetryStale,
  isVersionConflictError,
  parseCanvas,
  resolveWidgets,
  serializeCanvas,
  type ScadaWidgetDefinition,
  type ScadaWidgetInstance
} from './scada-model'

describe('parseCanvas', () => {
  it('空载荷视为空画布', () => {
    expect(parseCanvas(null)).toEqual({ ok: true, canvas: { widgets: [] } })
    expect(parseCanvas('')).toEqual({ ok: true, canvas: { widgets: [] } })
    expect(parseCanvas('   ')).toEqual({ ok: true, canvas: { widgets: [] } })
  })

  // 本用例是整份模型存在的理由：解析失败若退化成空画布，
  // 编辑器会显示空白，用户一保存就把真实内容覆盖成空。
  it('损坏的 JSON 必须报错，绝不退化成空画布', () => {
    const result = parseCanvas('{"widgets":[')
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.reason).toBe('invalid-json')
  })

  it('非对象载荷必须报错', () => {
    expect(parseCanvas('[1,2,3]')).toMatchObject({ ok: false, reason: 'not-object' })
    expect(parseCanvas('"a string"')).toMatchObject({ ok: false, reason: 'not-object' })
  })

  it('widgets 字段非数组必须报错', () => {
    expect(parseCanvas('{"widgets":{"a":1}}')).toMatchObject({ ok: false, reason: 'bad-widgets' })
  })

  it('widget 缺少 id 或 widget_type 必须报错', () => {
    expect(parseCanvas('{"widgets":[{"id":"w1"}]}')).toMatchObject({ ok: false, reason: 'bad-widgets' })
    expect(parseCanvas('{"widgets":[{"widget_type":"gauge"}]}')).toMatchObject({ ok: false, reason: 'bad-widgets' })
  })

  it('正常画布解析出结构与默认布局', () => {
    const result = parseCanvas('{"widgets":[{"id":"w1","widget_type":"gauge","version":"1"}]}')
    expect(result.ok).toBe(true)
    if (result.ok) {
      expect(result.canvas.widgets).toHaveLength(1)
      expect(result.canvas.widgets[0].layout).toEqual({ x: 0, y: 0, w: 2, h: 2 })
    }
  })

  it('保留变量与绑定', () => {
    const result = parseCanvas('{"widgets":[],"variables":{"a":1},"bindings":[{"source":"s","target_widget_id":"w","target_property":"p"}]}')
    expect(result.ok).toBe(true)
    if (result.ok) {
      expect(result.canvas.variables).toEqual({ a: 1 })
      expect(result.canvas.bindings).toHaveLength(1)
    }
  })
})

describe('serializeCanvas / 往返', () => {
  it('空画布序列化为 {}', () => {
    expect(serializeCanvas({ widgets: [] })).toBe('{}')
  })

  it('解析再序列化可往返', () => {
    const raw = '{"widgets":[{"id":"w1","widget_type":"gauge","version":"1","layout":{"x":1,"y":2,"w":3,"h":4}}]}'
    const parsed = parseCanvas(raw)
    expect(parsed.ok).toBe(true)
    if (parsed.ok) {
      const again = parseCanvas(serializeCanvas(parsed.canvas))
      expect(again).toEqual(parsed)
    }
  })
})

describe('isVersionConflictError', () => {
  it('识别后端版本冲突', () => {
    expect(
      isVersionConflictError({ response: { data: { code: 201002, message: 'scada document version conflict; reload before saving' } } })
    ).toBe(true)
  })

  it('仅有文案时也能识别', () => {
    expect(isVersionConflictError({ response: { data: { message: 'version conflict' } } })).toBe(true)
  })

  // 不能把别的拒绝当成冲突：否则用户会以为重载一下就好，实际是权限问题。
  it('不把其他拒绝误判为冲突', () => {
    expect(isVersionConflictError({ response: { data: { code: 201002, message: 'no permission to send control commands' } } })).toBe(false)
    expect(isVersionConflictError({ response: { data: { code: 100404, message: 'scada document not found' } } })).toBe(false)
  })

  it('无响应体的错误返回 false', () => {
    expect(isVersionConflictError(null)).toBe(false)
    expect(isVersionConflictError(new Error('network'))).toBe(false)
  })
})

describe('isArchivedError', () => {
  it('识别归档终态拒绝', () => {
    expect(isArchivedError({ response: { data: { message: 'archived scada document cannot be modified' } } })).toBe(true)
    expect(isArchivedError({ response: { data: { message: 'version conflict' } } })).toBe(false)
  })
})

describe('isTelemetryStale', () => {
  it('已连接且消息新鲜时不陈旧', () => {
    expect(isTelemetryStale('connected', 10_000, 11_000)).toBe(false)
  })

  it('已连接但静默超时即陈旧', () => {
    expect(isTelemetryStale('connected', 0, 31_000)).toBe(true)
  })

  it('已连接但从未收到消息时陈旧', () => {
    expect(isTelemetryStale('connected', null, 0)).toBe(true)
  })

  // 本用例是陈旧判定的核心：断开后最后一帧再新也不是实时数据。
  it('断开状态下即使消息刚到也判为陈旧', () => {
    expect(isTelemetryStale('disconnected', 10_000, 10_001)).toBe(true)
    expect(isTelemetryStale('reconnecting', 10_000, 10_001)).toBe(true)
    expect(isTelemetryStale('error', 10_000, 10_001)).toBe(true)
    expect(isTelemetryStale('idle', 10_000, 10_001)).toBe(true)
  })
})

describe('resolveWidgets', () => {
  const registry: ScadaWidgetDefinition[] = [
    { type: 'gauge', version: '1', schema: '{}', capabilities: ['2d'] },
    { type: 'chart', version: '1', schema: '{}', capabilities: ['2d'] },
    { type: 'twin3d', version: '1', schema: '{}', capabilities: ['3d'] }
  ]
  const instances: ScadaWidgetInstance[] = [
    { id: 'w1', widget_type: 'gauge', version: '1', layout: { x: 0, y: 0, w: 2, h: 2 } },
    { id: 'w2', widget_type: 'chart', version: '1', layout: { x: 2, y: 0, w: 2, h: 2 } },
    { id: 'w3', widget_type: 'twin3d', version: '1', layout: { x: 4, y: 0, w: 2, h: 2 } },
    { id: 'w4', widget_type: 'retired', version: '9', layout: { x: 6, y: 0, w: 2, h: 2 } }
  ]

  it('有 WebGL 时仅未注册项降级', () => {
    const res = resolveWidgets(instances, registry, true)
    expect(res.available).toHaveLength(3)
    expect(res.degraded).toHaveLength(0)
    expect(res.unknown.map(i => i.widget_type)).toEqual(['retired'])
  })

  // 门禁：3D/WebGL 降级不得影响 2D 看板。
  it('无 WebGL 时 3D 降级但 2D 全部照常可用', () => {
    const res = resolveWidgets(instances, registry, false)
    expect(res.available.map(d => d.type)).toEqual(['gauge', 'chart'])
    expect(res.degraded.map(d => d.type)).toEqual(['twin3d'])
    expect(res.unknown).toHaveLength(1)
  })

  it('版本未精确命中时回落到该类型最新版本', () => {
    const res = resolveWidgets(
      [{ id: 'w', widget_type: 'gauge', version: '0.9', layout: { x: 0, y: 0, w: 2, h: 2 } }],
      registry,
      false
    )
    expect(res.available).toHaveLength(1)
  })
})

describe('canEditDocument', () => {
  it('归档为终态', () => {
    expect(canEditDocument('ARCHIVED')).toBe(false)
    expect(canEditDocument('DRAFT')).toBe(true)
    expect(canEditDocument('PUBLISHED')).toBe(true)
  })
})

describe('commandRequiresConfirmation', () => {
  const registry: ScadaWidgetDefinition[] = [
    {
      type: 'valve',
      version: '1',
      schema: '{}',
      capabilities: ['2d'],
      commands: [
        { name: 'refresh', requires_confirmation: false },
        { name: 'open_valve', requires_confirmation: true }
      ]
    }
  ]

  it('按注册声明决定是否需确认', () => {
    expect(commandRequiresConfirmation(registry, 'valve', 'open_valve')).toBe(true)
    expect(commandRequiresConfirmation(registry, 'valve', 'refresh')).toBe(false)
  })

  // 未知命令必须默认要求确认（fail closed），否则危险操作可能被一键触发。
  it('未知命令默认要求确认', () => {
    expect(commandRequiresConfirmation(registry, 'valve', 'self_destruct')).toBe(true)
    expect(commandRequiresConfirmation(registry, 'unknown-widget', 'open_valve')).toBe(true)
  })
})
