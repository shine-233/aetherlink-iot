/**
 * 文件用途：汇总安全配置模块的公开入口。
 * 核心逻辑：导出 RSA、加密类型和安全策略相关能力。
 * 关键注意事项：这是登录和敏感请求的安全边界，导出项变更需同步调用方。
 * 重构建议：可用更明确的安全适配器接口隔离具体加密实现。
 */
/**
 * 文件：安全配置统一入口。
 * 作用：集中导出 RSA 安全配置、类型和运行时读取函数。
 * 依赖：依赖 rsa.ts 的默认配置与 types.ts 的类型定义。
 * 维护：新增安全能力时保持默认配置、导出项和类型定义同步。
 */

import { getRSAPublicKey, rsaConfig, rsaPublicKey, validateRSAPublicKey } from './rsa'

export { getRSAPublicKey, rsaConfig, rsaPublicKey, validateRSAPublicKey }

export type {
  RSASecurityConfig,
  SecurityConfig,
  RSAEncryptionOptions,
  RSADecryptionOptions,
  RSAKeyPair,
  SecurityConfigConstants
} from './types'

export const securityConfig = {
  rsa: {
    publicKey: '',
    keySize: 2048,
    algorithm: 'RSA-OAEP',
    hashAlgorithm: 'SHA-256',
    enableEnvOverride: true
  }
} as const

export function getSecurityConfig() {
  return {
    rsa: {
      publicKey: getRSAPublicKey(),
      keySize: rsaConfig.keySize,
      algorithm: rsaConfig.algorithm,
      hashAlgorithm: rsaConfig.hashAlgorithm,
      enableEnvOverride: rsaConfig.enableEnvOverride
    }
  }
}
