import { describe, expect, it } from 'vitest'
import { resolveDocumentTitle } from '../title-helper'

describe('resolveDocumentTitle', () => {
  const t = (key: string) => key

  it('builds route-title plus app-title for ordinary routes', () => {
    expect(
      resolveDocumentTitle(
        {
          path: '/management/setting',
          meta: { i18nKey: 'route.management_setting', title: 'ignored' }
        },
        'AetherLink IoT',
        t
      )
    ).toBe('route.management_setting-AetherLink IoT')
  })

  it('uses login child titles for login subroutes', () => {
    expect(resolveDocumentTitle({ path: '/login/register', meta: {} }, 'AetherLink IoT', t)).toBe(
      'page.login.register.title-AetherLink IoT'
    )
    expect(resolveDocumentTitle({ path: '/login/register-email', meta: {} }, 'AetherLink IoT', t)).toBe(
      'page.login.register.title-AetherLink IoT'
    )
    expect(resolveDocumentTitle({ path: '/login/register-super-admin', meta: {} }, 'AetherLink IoT', t)).toBe(
      'page.login.register.title-AetherLink IoT'
    )
    expect(resolveDocumentTitle({ path: '/login/reset-pwd', meta: {} }, 'AetherLink IoT', t)).toBe(
      'page.login.resetPwd.title-AetherLink IoT'
    )
    expect(resolveDocumentTitle({ path: '/login/bind-wechat', meta: {} }, 'AetherLink IoT', t)).toBe(
      'page.login.bindWeChat.title-AetherLink IoT'
    )
  })

  it('falls back to app title when the route has no title metadata', () => {
    expect(resolveDocumentTitle({ path: '/home', meta: {} }, 'AetherLink IoT', t)).toBe('AetherLink IoT')
  })
})
