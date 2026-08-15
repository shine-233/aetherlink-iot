/*
 * 文件用途：提供静态资源、服务地址、深拷贝、JSON、密码校验和随机字符串等通用工具。
 * 核心逻辑：读取运行时环境和浏览器能力，组合 URL、校验字符串并生成随机十六进制内容。
 * 关键注意事项：该文件职责较宽，既有纯函数也有环境依赖函数，改动需谨慎评估调用方。
 * 重构建议：建议按 URL、校验、随机数和对象处理拆分模块。
 */
import { REG_PWD } from '@/constants/reg'
import { createServiceConfig } from '~/env.config'
import { smartDeepClone } from '@/utils/deep-clone'

/** Resolve the primary API base URL from Vite env, falling back to the same-origin preview proxy. */
export const getBaseServerUrl = (): string => {
  const { baseURL } = createServiceConfig(import.meta.env)
  return baseURL || `${window.location.origin}/api/v1`
}

/** Resolve the default platform API base URL used by assets and websocket helpers. */
export const getPlatformApiBaseUrl = (): string => {
  const { otherBaseURL } = createServiceConfig(import.meta.env)
  return otherBaseURL.platform ? otherBaseURL.platform : `${window.location.origin}/api/v1`
}

/**
 * get web socket server url
 *
 * @returns web socket server url
 */
export const getWebsocketServerUrl = (): string => {
  const platformApiBaseUrl = getPlatformApiBaseUrl()
  if (window.location.protocol === 'https:') {
    return platformApiBaseUrl.replace(window.location.protocol, 'wss:')
  }
  return platformApiBaseUrl.replace(window.location.protocol, 'ws:')
}

/** Compatibility wrapper around the shared clone helper used by existing imports. */
export function deepClone(data: any): any {
  return smartDeepClone(data)
}

/** Return true only for JSON strings whose parsed value is an object or array. */
export function isJSON(str: string): boolean {
  if (typeof str === 'string') {
    try {
      const obj = JSON.parse(str)
      if (typeof obj === 'object' && obj) {
        return true
      }
      return false
    } catch (error) {
      return false
    }
  }
  return false
}

/** 登录后弱密码提示用：仅要求非空（与登录表单一致，不做复杂度校验） */
export function validPassword(str: string): boolean {
  return typeof str === 'string' && str.length > 0
}

function getRandomBytes(length: number) {
  return window.crypto.getRandomValues(new Uint8Array(length))
}

function randomBytesToHex(bytes: Uint8Array) {
  return Array.from(bytes)
    .map((b: number) => b.toString(16).padStart(2, '0'))
    .join('')
}

/** Generate cryptographic random bytes and encode them as hex; length is a byte count, so output is length * 2 chars. */
export function generateRandomHexString(length: number) {
  const bytes = getRandomBytes(length)
  const hexString = randomBytesToHex(bytes)
  return hexString
}

export function validName(str: string) {
  if (!str || str?.length > 50) {
    return false
  }
  return true
}

/** 注册/改密等表单：8-20 位，必须包含大小写字母、数字和特殊字符。 */
export function validPasswordByExp(str: string) {
  return typeof str === 'string' && REG_PWD.test(str)
}
