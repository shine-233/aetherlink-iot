import { describe, expect, it, vi } from 'vitest'

vi.mock('@/utils/auth-user-avatar', () => ({
  resolveUserAvatarPath: (data: Record<string, unknown>) => String(data.avatar_url || '')
}))

import {
  COUNTRY_CODE_OPTIONS,
  TIMEZONE_OPTIONS,
  authorityLocaleKey,
  buildSubmitUserInfo,
  emptyPersonalCenterUserInfo,
  formatDisplayPhone,
  normalizeFetchedUserInfo,
  normalizeLocale,
  parsePhoneNumber
} from '../personal-center-profile'
import { isFrontendRsaEnabled } from '../usePersonalCenterPassword'

describe('personal-center-profile', () => {
  it('keeps the option catalogs intact', () => {
    expect(COUNTRY_CODE_OPTIONS).toHaveLength(43)
    expect(TIMEZONE_OPTIONS).toHaveLength(20)
    expect(TIMEZONE_OPTIONS[0]).toEqual({ label: 'Asia/Shanghai (Shanghai Time)', value: 'Asia/Shanghai' })
    expect(TIMEZONE_OPTIONS.at(-1)?.value).toBe('UTC')
  })

  it('parses phone numbers preferring the longest country code', () => {
    expect(parsePhoneNumber('')).toEqual({ country_code: '+86', phone_only: '' })
    expect(parsePhoneNumber('+852 9123 4567')).toEqual({ country_code: '+852', phone_only: '91234567' })
    expect(parsePhoneNumber('+1-415-555-0100')).toEqual({ country_code: '+1', phone_only: '4155550100' })
    expect(parsePhoneNumber('13800138000')).toEqual({ country_code: '+86', phone_only: '13800138000' })
  })

  it('normalizes loose locale values', () => {
    expect(normalizeLocale('zh_cn')).toBe('zh-CN')
    expect(normalizeLocale(' EN-US ')).toBe('en-US')
    expect(normalizeLocale('de-DE')).toBe('')
    expect(normalizeLocale(null)).toBe('')
  })

  it('normalizes fetched info and builds the submit payload without split phone fields', () => {
    const info = normalizeFetchedUserInfo({ phone_num: '+8613800138000', default_language: 'fr_fr', extra: 1 })
    expect(info).toMatchObject({
      country_code: '+86',
      phone_only: '13800138000',
      additional_info: '{}',
      default_language: 'fr-FR',
      extra: 1,
      address: { province: '', city: '', district: '', detailed_address: '' }
    })
    const payload = buildSubmitUserInfo(info)
    expect(payload).not.toHaveProperty('country_code')
    expect(payload).not.toHaveProperty('phone_only')
    expect(payload.phone_number).toBe('+8613800138000')
  })

  it('formats display phone and authority keys', () => {
    expect(formatDisplayPhone({ ...emptyPersonalCenterUserInfo(), phone_only: '123' })).toBe('+86 123')
    expect(formatDisplayPhone({ country_code: '', phone_only: '', phone_number: 'raw' })).toBe('raw')
    expect(authorityLocaleKey('TENANT_ADMIN')).toBe('generate.TENANT_ADMIN')
    expect(authorityLocaleKey('OTHER')).toBe('generate.user')
    expect(authorityLocaleKey('  ')).toBeNull()
  })

  it('detects the frontend RSA flag defensively', () => {
    const storage = (value: string | null) => ({ getItem: () => value })
    expect(isFrontendRsaEnabled(storage(null))).toBe(false)
    expect(isFrontendRsaEnabled(storage('not json'))).toBe(false)
    expect(isFrontendRsaEnabled(storage('{"a":1}'))).toBe(false)
    expect(isFrontendRsaEnabled(storage('[{"name":"frontend_res","enable_flag":"enable"}]'))).toBe(true)
  })
})
