import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  localStgGet: vi.fn(),
  t: vi.fn((key: string) => (key === 'title' ? '默认平台标题' : key))
}))

vi.mock('@aetherlink/utils', () => ({
  getRgbOfColor: vi.fn(() => ({
    r: 100,
    g: 108,
    b: 255
  }))
}))

vi.mock('@/locales', () => ({
  $t: hoisted.t
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: hoisted.localStgGet
  }
}))

import { setupLoading } from '../loading'

describe('setupLoading', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = '<div id="app"></div>'
    hoisted.localStgGet.mockImplementation((key: string) => {
      if (key === 'themeColor') return '#646cff'
      if (key === 'logoLoading') return ''
      if (key === 'systemName') return ''
      return ''
    })
  })

  it('uses the trimmed cached branding title when available', () => {
    hoisted.localStgGet.mockImplementation((key: string) => {
      if (key === 'themeColor') return '#646cff'
      if (key === 'logoLoading') return ''
      if (key === 'systemName') return '  AetherLink IoT  '
      return ''
    })

    setupLoading()

    expect(document.querySelector('#app h2')?.textContent).toBe('AetherLink IoT')
  })

  it('falls back to the default localized title when cached branding title is blank', () => {
    hoisted.localStgGet.mockImplementation((key: string) => {
      if (key === 'themeColor') return '#646cff'
      if (key === 'logoLoading') return ''
      if (key === 'systemName') return '   '
      return ''
    })

    setupLoading()

    expect(document.querySelector('#app h2')?.textContent).toBe('默认平台标题')
  })
})
