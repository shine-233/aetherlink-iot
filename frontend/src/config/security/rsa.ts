/**
 * 文件用途：集中管理前端 RSA 默认公钥与安全配置。
 * 核心逻辑：提供环境变量覆盖、公钥读取和 PEM 格式校验能力。
 * 关键注意事项：公钥、算法参数和覆盖值变更时，需同步后端解密配置与相关文档。
 * 重构建议：后续可将 PEM 边界与算法常量继续收敛到统一安全适配层。
 */

const RSA_PUBLIC_KEY_BEGIN = '-----BEGIN PUBLIC KEY-----'
const RSA_PUBLIC_KEY_END = '-----END PUBLIC KEY-----'

/** 默认 RSA 公钥。 */
export const rsaPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBCgKCAQEA35ATUHHwTvEHxaOKG/8xTETHq7+syHqEkDXSuKf2irYZefaKe4n2
GiM6uWFBgaXX/LxvxkbIQ0WK0R+mIziaF3mTa3gs7n5OiJgJDsqzZHzS9to6j9Mc
NG3v2R0wgjCs9FCR51ZEZIxxC5YYlHd2ZQoVZ8oLMdg9bhop5CsG9J1spkhx8cmY
r50hSenA7rxTQ7fSc8TmMgR6Env84rjUMgxBO7RgnbaURzde0UPOrEmc7FGCZJix
fSkMao0ZoWz5PNE7tNU9LYQJThy+T46HAu5V5zWOuo9AdBdJvQH43yhIptLB/Z1p
UsdVUZ0ESaoP326ag8R5EqBSa2+4gce14QIDAQAB
-----END PUBLIC KEY-----`

/** RSA 默认安全配置。 */
export const rsaConfig = {
  /** 密钥大小 */
  keySize: 2048,
  /** 加密算法 */
  algorithm: 'RSA-OAEP',
  /** 哈希算法 */
  hashAlgorithm: 'SHA-256',
  /** 是否启用环境变量覆盖 */
  enableEnvOverride: true
} as const

/**
 * 获取当前生效的 RSA 公钥。
 * 启用环境变量覆盖时，优先返回 `VITE_RSA_PUBLIC_KEY`。
 */
export function getRSAPublicKey(): string {
  const envPublicKey = rsaConfig.enableEnvOverride ? import.meta.env.VITE_RSA_PUBLIC_KEY : ''
  return envPublicKey || rsaPublicKey
}

/**
 * 校验 RSA 公钥是否包含标准 PEM 边界。
 * @param key 公钥字符串
 * @returns 是否满足基础 PEM 公钥格式要求
 */
export function validateRSAPublicKey(key: string): boolean {
  return (
    key.includes(RSA_PUBLIC_KEY_BEGIN) &&
    key.includes(RSA_PUBLIC_KEY_END) &&
    key.length > 100
  )
}
