/**
 * Write `document.title` directly.
 *
 * Use this instead of `@vueuse/core`'s `useTitle(value)` outside component
 * setup (router guards, app bootstrap): `useTitle` creates a `watch` each call,
 * and without an active effect scope that watcher is never disposed, so every
 * navigation leaked one more watcher.
 */
export function setDocumentTitle(title: string) {
  if (typeof document === 'undefined') return
  if (document.title !== title) document.title = title
}

type TitleRouteMeta = {
  i18nKey?: string
  title?: string
}

type TitleRouteInput = {
  path?: string
  meta?: TitleRouteMeta
}

export function resolveDocumentTitle(
  route: TitleRouteInput,
  appTitle: string,
  translate: (key: string) => string
): string {
  const path = String(route.path || '')
  const meta = route.meta || {}
  let routeTitle = ''

  if (path.startsWith('/login/')) {
    const loginChild = path.split('/').pop()?.toLowerCase()
    switch (loginChild) {
      case 'register':
      case 'register-email':
      case 'register-super-admin':
        routeTitle = translate('page.login.register.title')
        break
      case 'reset-pwd':
        routeTitle = translate('page.login.resetPwd.title')
        break
      case 'bind-wechat':
        routeTitle = translate('page.login.bindWeChat.title')
        break
      default:
        routeTitle = String(meta.title || translate('page.login.pwdLogin.title'))
        break
    }
  } else {
    routeTitle = meta.i18nKey ? translate(meta.i18nKey) : String(meta.title || '')
  }

  return routeTitle ? `${routeTitle}-${appTitle}` : appTitle
}
