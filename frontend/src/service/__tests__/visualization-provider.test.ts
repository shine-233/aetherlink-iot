import { describe, expect, it } from 'vitest'
import {
  externalVisualizationProvider,
  getVisualizationProvider,
  legacyThingsVisProvider,
  localVisualizationProvider,
  nativeBoardProvider,
  VISUALIZATION_PROVIDER_KINDS
} from '../visualization-provider'

describe('visualization provider compatibility boundary', () => {
  it('exposes stable local-default and opt-in external identities', () => {
    expect(VISUALIZATION_PROVIDER_KINDS).toEqual({ local: 'local', external: 'external' })
    expect(localVisualizationProvider).toBe(nativeBoardProvider)
    expect(externalVisualizationProvider).toBe(legacyThingsVisProvider)
    expect(localVisualizationProvider).toMatchObject({
      id: 'native-board',
      kind: 'local',
      deploymentMode: 'local-default'
    })
    expect(externalVisualizationProvider).toMatchObject({
      id: 'legacy-thingsvis',
      kind: 'third-party',
      deploymentMode: 'optional-external'
    })
  })

  it('defaults to the self-hosted provider without probing ThingsVis', () => {
    expect(getVisualizationProvider()).toBe(localVisualizationProvider)
    expect(getVisualizationProvider(undefined)).toBe(localVisualizationProvider)
    expect(getVisualizationProvider('local')).toBe(localVisualizationProvider)
    expect(getVisualizationProvider('external')).toBe(externalVisualizationProvider)
  })

  it.each(['native', 'thingsvis', '', null, false, 1, {}, []])('fails closed for unknown provider config %j', kind => {
    expect(getVisualizationProvider(kind)).toBeNull()
  })
})
