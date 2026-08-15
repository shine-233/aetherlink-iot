export type CommandScopeSafetyTagType = 'success' | 'warning' | 'info'

export type CommandScopeSafetyViewModel = {
  description: string
  meta: string
  tag: string
  tagType: CommandScopeSafetyTagType
}

export type BuildCommandScopeSafetyOptions = {
  hasCommandJobScope: boolean
  isDeviceFilterScope: boolean
  selectedCount: number
  savedFilterName?: string
  routeSavedFilterName?: string
  requestedTotal: number | null
  currentPageCount: number | null
  maxDevices: number
  filterCount: number
  t: (key: string) => string
}

const emptyCountText = (value: number | null) => (value === null ? '--' : String(value))
const maxDevicesText = (value: number) => (value ? String(value) : '--')

export const buildCommandScopeSafety = ({
  hasCommandJobScope,
  isDeviceFilterScope,
  selectedCount,
  savedFilterName = '',
  routeSavedFilterName = '',
  requestedTotal,
  currentPageCount,
  maxDevices,
  filterCount,
  t
}: BuildCommandScopeSafetyOptions): CommandScopeSafetyViewModel => {
  const meta = t('custom.commandCenter.scopeSafetyMeta')

  if (!hasCommandJobScope) {
    return {
      description: t('custom.commandCenter.scopeSafetyNoScopeDesc'),
      meta,
      tag: t('custom.commandCenter.scopeSafetyNoScopeTag'),
      tagType: 'warning'
    }
  }

  if (!isDeviceFilterScope) {
    return {
      description: t('custom.commandCenter.scopeSafetySelectedDesc').replace('{count}', String(selectedCount)),
      meta,
      tag: t('custom.commandCenter.scopeSafetySelectedTag'),
      tagType: 'success'
    }
  }

  const resolvedSavedFilterName = savedFilterName || routeSavedFilterName
  const total = emptyCountText(requestedTotal)
  const currentPage = emptyCountText(currentPageCount)
  const max = maxDevicesText(maxDevices)

  if (resolvedSavedFilterName) {
    return {
      description: t('custom.commandCenter.scopeSafetySavedFilterDesc')
        .replace('{name}', resolvedSavedFilterName)
        .replace('{total}', total)
        .replace('{currentPage}', currentPage)
        .replace('{max}', max),
      meta,
      tag: t('custom.commandCenter.scopeSafetySavedFilterTag'),
      tagType: 'info'
    }
  }

  return {
    description: t('custom.commandCenter.scopeSafetyFilteredDesc')
      .replace('{filters}', String(filterCount))
      .replace('{total}', total)
      .replace('{currentPage}', currentPage)
      .replace('{max}', max),
    meta,
    tag: t('custom.commandCenter.scopeSafetyFilteredTag'),
    tagType: 'warning'
  }
}
