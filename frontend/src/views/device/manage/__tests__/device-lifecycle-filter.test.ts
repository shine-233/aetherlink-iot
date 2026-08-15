import { describe, expect, it } from 'vitest'
import {
  LIFECYCLE_STATUS_VALUES,
  buildLifecycleStatusOptions,
  isValidLifecycleStatus
} from '../device-lifecycle-filter'

// 这些值必须与后端 GetDeviceListByPageReq.LifecycleStatus 的
// `validate:"omitempty,oneof=activated inactive transmitted all"` 逐字对齐。
// 若后端白名单变化而此处未同步，前端会发出被后端 400 的值。
const BACKEND_ONEOF = ['activated', 'inactive', 'transmitted', 'all']

describe('device-lifecycle-filter contract', () => {
  it('whitelist matches backend oneof exactly', () => {
    expect([...LIFECYCLE_STATUS_VALUES].sort()).toEqual([...BACKEND_ONEOF].sort())
  })

  it('every option value is in the backend whitelist', () => {
    const t = (key: string) => key
    const options = buildLifecycleStatusOptions(t)
    for (const opt of options) {
      expect(isValidLifecycleStatus(opt.value)).toBe(true)
    }
  })

  it('no option uses empty string (empty string is rejected by backend oneof, would 400)', () => {
    const t = (key: string) => key
    const options = buildLifecycleStatusOptions(t)
    for (const opt of options) {
      expect(opt.value).not.toBe('')
    }
  })

  it('default/first option is activated (matches its own label semantics, not a 400-prone empty)', () => {
    const t = (key: string) => key
    const options = buildLifecycleStatusOptions(t)
    expect(options[0].value).toBe('activated')
  })

  it('rejects values outside the whitelist', () => {
    expect(isValidLifecycleStatus('installed')).toBe(false)
    expect(isValidLifecycleStatus('transmitted')).toBe(true)
    expect(isValidLifecycleStatus('transfer_complete')).toBe(false)
    expect(isValidLifecycleStatus('active')).toBe(false)
    expect(isValidLifecycleStatus('')).toBe(false)
    expect(isValidLifecycleStatus('ACTIVATED')).toBe(false)
  })
})
