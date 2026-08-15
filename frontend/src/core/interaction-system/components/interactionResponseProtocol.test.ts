import { describe, expect, it } from 'vitest'
import {
  buildJumpResponse,
  buildModifyResponse,
  getInteractionActionType,
  readCompatibilityJumpEditState,
  readJumpEditState,
  readModifyEditState
} from './interactionResponseProtocol'

describe('interactionResponseProtocol', () => {
  it('classifies current and compatibility response actions without reading component state', () => {
    expect(getInteractionActionType({ responses: [] })).toBe('none')
    expect(getInteractionActionType({ responses: [{ action: 'jump' }] })).toBe('jump')
    expect(getInteractionActionType({ responses: [{ action: 'navigateToUrl' }] })).toBe('jump')
    expect(getInteractionActionType({ responses: [{ action: 'modify' }] })).toBe('modify')
    expect(getInteractionActionType({ responses: [{ action: 'updateComponentData' }] })).toBe('modify')
    expect(getInteractionActionType({ responses: [{ action: 'notify' }] })).toBe('custom')
  })

  it('reads compatibility jump payloads into explicit external or internal edit state', () => {
    expect(readCompatibilityJumpEditState('https://example.com', '_blank')).toMatchObject({
      urlType: 'external',
      url: 'https://example.com',
      target: '_blank',
      selectedMenuPath: '',
      shouldLoadMenuOptions: false
    })

    expect(readCompatibilityJumpEditState('/device/list')).toMatchObject({
      urlType: 'internal',
      url: '/device/list',
      target: '_blank',
      selectedMenuPath: '/device/list',
      shouldLoadMenuOptions: true
    })
  })

  it('reads current jump and modify configs for editing', () => {
    expect(
      readJumpEditState({
        action: 'jump',
        jumpConfig: { jumpType: 'internal', internalPath: '/dashboard', target: '_self' }
      })
    ).toMatchObject({
      urlType: 'internal',
      url: '/dashboard',
      target: '_self',
      selectedMenuPath: '/dashboard',
      shouldLoadMenuOptions: true
    })

    expect(
      readModifyEditState({
        action: 'modify',
        modifyConfig: {
          targetComponentId: 'card-1',
          targetProperty: 'component.styles.color',
          updateValue: '#fff'
        }
      })
    ).toEqual({
      targetComponentId: 'card-1',
      targetProperty: 'component.styles.color',
      updateValue: '#fff'
    })
  })

  it('builds current responses while preserving compatibility fields', () => {
    expect(
      buildJumpResponse({
        urlType: 'internal',
        url: '/fallback',
        selectedMenuPath: '/device/list',
        target: '_self'
      })
    ).toEqual({
      action: 'jump',
      jumpConfig: {
        jumpType: 'internal',
        internalPath: '/device/list',
        target: '_self'
      },
      value: '/fallback',
      target: '_self'
    })

    expect(
      buildModifyResponse({
        targetComponentId: 'card-1',
        targetProperty: 'component.styles.color',
        updateValue: '#fff',
        bindingPath: 'card-1.component.styles.color'
      })
    ).toEqual({
      action: 'modify',
      modifyConfig: {
        targetComponentId: 'card-1',
        targetProperty: 'component.styles.color',
        updateValue: '#fff',
        updateMode: 'replace',
        bindingPath: 'card-1.component.styles.color'
      },
      targetComponentId: 'card-1',
      targetProperty: 'component.styles.color',
      updateValue: '#fff'
    })
  })
})
