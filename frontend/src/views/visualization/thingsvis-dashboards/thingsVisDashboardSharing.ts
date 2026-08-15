import { NATIVE_BOARD_PROJECT_ID } from '@/service/visualization-provider/provider-ids'

type DashboardIdentity = { id: string; projectId?: string; shareToken?: string | null }

export function buildThingsVisDashboardViewerHref(dashboard: DashboardIdentity) {
  const params = [`id=${encodeURIComponent(dashboard.id)}`]
  if (dashboard.projectId === NATIVE_BOARD_PROJECT_ID) {
    params.push(`projectId=${encodeURIComponent(NATIVE_BOARD_PROJECT_ID)}`, 'provider=native')
    if (dashboard.shareToken) {
      params.push(`shareToken=${encodeURIComponent(dashboard.shareToken)}`)
    }
  }
  return `/tv-preview?${params.join('&')}`
}

export function buildThingsVisDashboardClipboardLink(dashboard: DashboardIdentity) {
  const href = buildThingsVisDashboardViewerHref(dashboard)

  if (typeof window === 'undefined') {
    return href
  }

  return new URL(href, window.location.origin).toString()
}
