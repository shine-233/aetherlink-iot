/*
 * 文件用途：验证地图 SDK 地址只由部署环境密钥构造。
 * 核心逻辑：覆盖空密钥、空白密钥和特殊字符编码。
 * 关键注意事项：仓库不得提供可直接使用的供应商密钥。
 */
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  buildAmapSdkUrl,
  buildBaiduMapSdkUrl,
  buildTencentMapSdkUrl,
  ensureAmapSecurityConfig
} from './map-sdk'

describe('ensureAmapSecurityConfig', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
    delete window._AMapSecurityConfig
  })

  it('mounts the deployment security code before SDK load', () => {
    vi.stubEnv('VITE_AMAP_SECURITY_CODE', ' deployed-code ')
    ensureAmapSecurityConfig()
    expect(window._AMapSecurityConfig).toEqual({ securityJsCode: 'deployed-code' })
  })

  it('stays silent when the security code is absent or blank', () => {
    vi.stubEnv('VITE_AMAP_SECURITY_CODE', '')
    ensureAmapSecurityConfig()
    expect(window._AMapSecurityConfig).toBeUndefined()

    vi.stubEnv('VITE_AMAP_SECURITY_CODE', '   ')
    ensureAmapSecurityConfig()
    expect(window._AMapSecurityConfig).toBeUndefined()
  })
})

describe('buildBaiduMapSdkUrl', () => {
  it('disables the provider when no key is configured', () => {
    expect(buildBaiduMapSdkUrl()).toBe('')
    expect(buildBaiduMapSdkUrl('   ')).toBe('')
  })

  it('trims and encodes the deployment key', () => {
    expect(buildBaiduMapSdkUrl(' key/value ')).toBe(
      'https://api.map.baidu.com/getscript?v=3.0&ak=key%2Fvalue&services=&t=20210201100830&s=1'
    )
  })
})

describe('buildAmapSdkUrl', () => {
  it('disables the provider when no key is configured', () => {
    expect(buildAmapSdkUrl()).toBe('')
    expect(buildAmapSdkUrl('   ')).toBe('')
  })

  it('trims and encodes the deployment key', () => {
    expect(buildAmapSdkUrl(' key/value ')).toBe(
      'https://webapi.amap.com/maps?v=2.0&key=key%2Fvalue'
    )
  })
})

describe('buildTencentMapSdkUrl', () => {
  it('disables the provider when no key is configured', () => {
    expect(buildTencentMapSdkUrl()).toBe('')
    expect(buildTencentMapSdkUrl('   ')).toBe('')
  })

  it('trims and encodes the deployment key', () => {
    expect(buildTencentMapSdkUrl(' key/value ')).toBe(
      'https://map.qq.com/api/gljs?v=1.exp&key=key%2Fvalue'
    )
  })
})
