import { describe, expect, it, vi } from 'vitest'

vi.mock('@/utils/common/tool', () => ({
  getPlatformApiBaseUrl: vi.fn(() => 'http://localhost/api/v1')
}))

import { mergeUserAvatarIntoAdditionalInfo, resolvePlatformAssetUrl, resolveUserAvatarPath } from '../auth-user-avatar'

describe('auth-user-avatar helpers', () => {
  it('prefers additional_info.user_icon over avatar_url', () => {
    expect(
      resolveUserAvatarPath({
        additional_info: '{"user_icon":"/uploads/from-additional.png"}',
        avatar_url: '/uploads/from-avatar.png'
      })
    ).toBe('/uploads/from-additional.png')
  })

  it('falls back to avatar_url when additional_info has no user_icon', () => {
    expect(
      resolveUserAvatarPath({
        additional_info: '{}',
        avatar_url: '/uploads/from-avatar.png'
      })
    ).toBe('/uploads/from-avatar.png')
  })

  it('resolves platform-relative asset paths against the platform host', () => {
    expect(resolvePlatformAssetUrl('/uploads/avatar.png')).toBe('http://localhost/uploads/avatar.png')
  })

  it('preserves already absolute asset urls', () => {
    expect(resolvePlatformAssetUrl('https://cdn.example.com/avatar.png')).toBe('https://cdn.example.com/avatar.png')
  })

  it('merges avatar path into malformed additional_info without throwing', () => {
    expect(mergeUserAvatarIntoAdditionalInfo('{bad json', '/uploads/avatar.png')).toBe(
      '{"user_icon":"/uploads/avatar.png"}'
    )
  })
})
