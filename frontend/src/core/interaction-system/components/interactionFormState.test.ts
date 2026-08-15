import { describe, expect, it } from 'vitest'
import { buildInteractionDraft, readInteractionEditFormState } from './interactionFormState'

describe('interactionFormState', () => {
  it('hydrates data-change internal jump interactions into an edit form state', () => {
    const editState = readInteractionEditFormState({
      event: 'dataChange',
      enabled: true,
      priority: 2,
      watchedProperty: 'base.metricsList',
      condition: {
        type: 'comparison',
        operator: 'greaterThan',
        value: '80'
      },
      responses: [
        {
          action: 'jump',
          jumpConfig: {
            jumpType: 'internal',
            internalPath: '/dashboard/device',
            target: '_self'
          },
          value: '/dashboard/device',
          target: '_self'
        }
      ]
    })

    expect(editState).toMatchObject({
      interaction: {
        event: 'dataChange',
        enabled: true,
        priority: 2,
        url: '/dashboard/device',
        target: '_self'
      },
      condition: {
        watchedProperty: 'base.metricsList',
        type: 'comparison',
        operator: 'greaterThan',
        value: '80'
      },
      action: {
        actionType: 'jump',
        urlType: 'internal',
        selectedMenuPath: '/dashboard/device',
        shouldLoadMenuOptions: true
      }
    })
  })

  it('builds a data-change internal jump draft without changing compatibility fields', () => {
    expect(
      buildInteractionDraft({
        interaction: {
          event: 'dataChange',
          enabled: true,
          priority: 1,
          url: '/fallback',
          target: '_self',
          targetComponentId: '',
          targetProperty: '',
          updateValue: ''
        },
        condition: {
          watchedProperty: 'base.metricsList',
          type: 'range',
          operator: '',
          value: '10-20'
        },
        targetBinding: {
          bindingPath: '',
          propertyInfo: null
        },
        action: {
          actionType: 'jump',
          urlType: 'internal',
          selectedMenuPath: '/dashboard/device',
          shouldLoadMenuOptions: false
        }
      })
    ).toEqual({
      event: 'dataChange',
      enabled: true,
      priority: 1,
      watchedProperty: 'base.metricsList',
      condition: {
        type: 'range',
        value: '10-20'
      },
      responses: [
        {
          action: 'jump',
          jumpConfig: {
            jumpType: 'internal',
            internalPath: '/dashboard/device',
            target: '_self'
          },
          value: '/fallback',
          target: '_self'
        }
      ]
    })
  })

  it('builds modify drafts from either structured bindings or legacy hydrated targets', () => {
    expect(
      buildInteractionDraft({
        interaction: {
          event: 'click',
          enabled: true,
          priority: 1,
          url: '',
          target: '_blank',
          targetComponentId: 'legacy-card',
          targetProperty: 'component.styles.color',
          updateValue: '#111111'
        },
        condition: {
          watchedProperty: '',
          type: '',
          operator: '',
          value: ''
        },
        targetBinding: {
          bindingPath: '',
          propertyInfo: null
        },
        action: {
          actionType: 'modify',
          urlType: 'external',
          selectedMenuPath: '',
          shouldLoadMenuOptions: false
        }
      }).responses[0]
    ).toMatchObject({
      action: 'modify',
      targetComponentId: 'legacy-card',
      targetProperty: 'component.styles.color',
      updateValue: '#111111',
      modifyConfig: {
        targetComponentId: 'legacy-card',
        targetProperty: 'component.styles.color',
        bindingPath: ''
      }
    })

    expect(
      buildInteractionDraft({
        interaction: {
          event: 'click',
          enabled: true,
          priority: 1,
          url: '',
          target: '_blank',
          targetComponentId: 'legacy-card',
          targetProperty: 'component.styles.color',
          updateValue: '#222222'
        },
        condition: {
          watchedProperty: '',
          type: '',
          operator: '',
          value: ''
        },
        targetBinding: {
          bindingPath: 'target-card.component.styles.backgroundColor',
          propertyInfo: {
            componentId: 'target-card',
            layer: 'component',
            propertyName: 'styles.backgroundColor'
          }
        },
        action: {
          actionType: 'modify',
          urlType: 'external',
          selectedMenuPath: '',
          shouldLoadMenuOptions: false
        }
      }).responses[0]
    ).toMatchObject({
      action: 'modify',
      targetComponentId: 'target-card',
      targetProperty: 'component.styles.backgroundColor',
      updateValue: '#222222',
      modifyConfig: {
        targetComponentId: 'target-card',
        targetProperty: 'component.styles.backgroundColor',
        bindingPath: 'target-card.component.styles.backgroundColor'
      }
    })
  })
})
