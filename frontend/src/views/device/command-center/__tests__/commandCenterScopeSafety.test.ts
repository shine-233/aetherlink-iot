import { buildCommandScopeSafety } from '../commandCenterScopeSafety'

const messages: Record<string, string> = {
  'custom.commandCenter.scopeSafetyMeta': 'Preview before submit',
  'custom.commandCenter.scopeSafetyNoScopeDesc': 'Select devices or a saved filter first.',
  'custom.commandCenter.scopeSafetyNoScopeTag': 'No scope',
  'custom.commandCenter.scopeSafetySelectedDesc': 'Selected {count} device(s).',
  'custom.commandCenter.scopeSafetySelectedTag': 'Selected devices',
  'custom.commandCenter.scopeSafetySavedFilterDesc':
    'Saved filter {name}: total {total}, current page {currentPage}, max {max}.',
  'custom.commandCenter.scopeSafetySavedFilterTag': 'Saved filter',
  'custom.commandCenter.scopeSafetyFilteredDesc':
    'Filtered by {filters} condition(s): total {total}, current page {currentPage}, max {max}.',
  'custom.commandCenter.scopeSafetyFilteredTag': 'Live filter'
}

const t = (key: string) => messages[key] || key

describe('buildCommandScopeSafety', () => {
  it('warns when no command scope is available', () => {
    expect(
      buildCommandScopeSafety({
        hasCommandJobScope: false,
        isDeviceFilterScope: false,
        selectedCount: 0,
        requestedTotal: null,
        currentPageCount: null,
        maxDevices: 200,
        filterCount: 0,
        t
      })
    ).toEqual({
      description: 'Select devices or a saved filter first.',
      meta: 'Preview before submit',
      tag: 'No scope',
      tagType: 'warning'
    })
  })

  it('marks explicit selected-device scope as safest', () => {
    expect(
      buildCommandScopeSafety({
        hasCommandJobScope: true,
        isDeviceFilterScope: false,
        selectedCount: 3,
        requestedTotal: null,
        currentPageCount: null,
        maxDevices: 200,
        filterCount: 0,
        t
      })
    ).toMatchObject({
      description: 'Selected 3 device(s).',
      tag: 'Selected devices',
      tagType: 'success'
    })
  })

  it('prefers the active saved-filter name over the route fallback', () => {
    expect(
      buildCommandScopeSafety({
        hasCommandJobScope: true,
        isDeviceFilterScope: true,
        selectedCount: 0,
        savedFilterName: 'Online chillers',
        routeSavedFilterName: 'Stale route name',
        requestedTotal: 120,
        currentPageCount: 20,
        maxDevices: 200,
        filterCount: 4,
        t
      })
    ).toMatchObject({
      description: 'Saved filter Online chillers: total 120, current page 20, max 200.',
      tag: 'Saved filter',
      tagType: 'info'
    })
  })

  it('keeps unknown filter counts visible for live filter scopes', () => {
    expect(
      buildCommandScopeSafety({
        hasCommandJobScope: true,
        isDeviceFilterScope: true,
        selectedCount: 0,
        requestedTotal: null,
        currentPageCount: null,
        maxDevices: 0,
        filterCount: 2,
        t
      })
    ).toMatchObject({
      description: 'Filtered by 2 condition(s): total --, current page --, max --.',
      tag: 'Live filter',
      tagType: 'warning'
    })
  })
})
