/**
 * 文件用途：个人中心资料的纯数据规则（选项常量、手机号拆分、语言归一、后端资料归一与提交载荷）。
 * 关键注意事项：手机号按「最长区号优先」匹配，避免 +852 被 +8 之类的短前缀误吞。
 */
import { resolveUserAvatarPath } from '@/utils/auth-user-avatar'

export type LocaleValue = App.I18n.LangType

export interface PersonalCenterUserInfo {
  additional_info: string
  name: string
  email: string
  phone_number: string
  country_code: string
  phone_only: string
  authority: string
  organization: string
  timezone: string
  default_language: string
  avatar_url: string
  address: { province: string; city: string; district: string; detailed_address: string }
  [key: string]: unknown
}

export const DEFAULT_COUNTRY_CODE = '+86'

export const emptyPersonalCenterUserInfo = (): PersonalCenterUserInfo => ({
  additional_info: '',
  name: '',
  email: '',
  phone_number: '',
  country_code: DEFAULT_COUNTRY_CODE,
  phone_only: '',
  authority: '',
  organization: '',
  timezone: '',
  default_language: '',
  avatar_url: '',
  address: { province: '', city: '', district: '', detailed_address: '' }
})

const TIMEZONES: Array<[string, string]> = [
  ['Asia/Shanghai', 'Shanghai'],
  ['Asia/Tokyo', 'Tokyo'],
  ['Asia/Seoul', 'Seoul'],
  ['Asia/Singapore', 'Singapore'],
  ['Asia/Hong_Kong', 'Hong Kong'],
  ['Asia/Bangkok', 'Bangkok'],
  ['Asia/Dubai', 'Dubai'],
  ['Asia/Kolkata', 'India'],
  ['Europe/London', 'London'],
  ['Europe/Paris', 'Paris'],
  ['Europe/Berlin', 'Berlin'],
  ['Europe/Moscow', 'Moscow'],
  ['America/New_York', 'New York'],
  ['America/Los_Angeles', 'Los Angeles'],
  ['America/Chicago', 'Chicago'],
  ['America/Toronto', 'Toronto'],
  ['Australia/Sydney', 'Sydney'],
  ['Australia/Melbourne', 'Melbourne'],
  ['Pacific/Auckland', 'Auckland']
]

export const TIMEZONE_OPTIONS = [
  ...TIMEZONES.map(([value, city]) => ({ label: `${value} (${city} Time)`, value })),
  { label: 'UTC (Coordinated Universal Time)', value: 'UTC' }
]

const COUNTRY_CODES = [
  '+86',
  '+1',
  '+44',
  '+33',
  '+49',
  '+39',
  '+34',
  '+7',
  '+81',
  '+82',
  '+65',
  '+60',
  '+66',
  '+84',
  '+62',
  '+63',
  '+91',
  '+61',
  '+64',
  '+55',
  '+52',
  '+54',
  '+27',
  '+20',
  '+971',
  '+966',
  '+90',
  '+31',
  '+46',
  '+47',
  '+45',
  '+41',
  '+43',
  '+32',
  '+351',
  '+30',
  '+48',
  '+420',
  '+36',
  '+385',
  '+852',
  '+853',
  '+886'
]

export const COUNTRY_CODE_OPTIONS = COUNTRY_CODES.map((value) => ({ label: value, value }))
const COUNTRY_CODES_LONGEST_FIRST = [...COUNTRY_CODES].sort((a, b) => b.length - a.length)

/** 完整手机号 → 区号 + 本地号码；无法匹配时回落默认区号。 */
export function parsePhoneNumber(phoneNumber: string) {
  if (!phoneNumber) return { country_code: DEFAULT_COUNTRY_CODE, phone_only: '' }
  const cleanPhone = phoneNumber.replace(/[^\d+]/g, '')
  const code = COUNTRY_CODES_LONGEST_FIRST.find((candidate) => cleanPhone.startsWith(candidate))
  return code
    ? { country_code: code, phone_only: cleanPhone.substring(code.length) }
    : { country_code: DEFAULT_COUNTRY_CODE, phone_only: cleanPhone }
}

const LOCALE_MAP: Record<string, LocaleValue> = {
  'zh-cn': 'zh-CN',
  'en-us': 'en-US',
  'fr-fr': 'fr-FR',
  'es-es': 'es-ES'
}

/** 宽松语言值（zh_cn / EN-US 等）→ 受支持的 LangType；不支持返回空串。 */
export function normalizeLocale(value: unknown): LocaleValue | '' {
  const key = String(value || '')
    .trim()
    .toLowerCase()
    .replace(/_/g, '-')
  return LOCALE_MAP[key] || ''
}

export function normalizeFetchedUserInfo(data: Record<string, any>): PersonalCenterUserInfo {
  const basePhone = data.phone_num || data.phone_number || ''
  return {
    ...data,
    name: data.name || '',
    email: data.email || '',
    phone_number: basePhone,
    ...parsePhoneNumber(basePhone),
    authority: data.authority || '',
    additional_info: data.additional_info || data.additionalInfo || '{}',
    organization: data.organization || '',
    timezone: data.timezone || '',
    default_language: normalizeLocale(data.default_language),
    avatar_url: resolveUserAvatarPath(data),
    address: {
      province: data.address?.province || '',
      city: data.address?.city || '',
      district: data.address?.district || '',
      detailed_address: data.address?.detailed_address || ''
    }
  }
}

/** 提交载荷：剔除仅前端使用的拆分字段，手机号以拼接后的完整号码提交。 */
export function buildSubmitUserInfo(info: PersonalCenterUserInfo) {
  const { country_code, phone_only, ...rest } = info
  return { ...rest, phone_number: `${country_code}${phone_only}` }
}

export function formatDisplayPhone(info: Pick<PersonalCenterUserInfo, 'country_code' | 'phone_only' | 'phone_number'>) {
  const code = info.country_code?.trim()
  const phone = info.phone_only?.trim()
  return code && phone ? `${code} ${phone}` : info.phone_number || ''
}

const AUTHORITY_LOCALE_KEYS = {
  SYS_ADMIN: 'generate.SYS_ADMIN',
  TENANT_ADMIN: 'generate.TENANT_ADMIN',
  TENANT_USER: 'generate.TENANT_USER'
} as const

export function authorityLocaleKey(authority: unknown) {
  const value = String(authority || '').trim()
  if (!value) return null
  return AUTHORITY_LOCALE_KEYS[value as keyof typeof AUTHORITY_LOCALE_KEYS] || 'generate.user'
}
