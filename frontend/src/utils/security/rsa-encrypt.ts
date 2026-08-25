import { rsaPublicKey } from '@/config/security/rsa'

/**
 * 与后端 initialize.DecryptPassword 的填充参数保持一致：
 * RSA-OAEP + SHA-256。任何一侧变更算法都必须同步另一侧。
 *
 * node-forge 通过动态 import 按需加载：仅在首次调用时拉取，
 * 避免整个 forge 库进入登录首屏关键路径（所有调用点均已 await）。
 */
export async function encryptDataByRsa(data: string): Promise<string> {
  try {
    const forge = await import('node-forge')
    const publicKey = forge.pki.publicKeyFromPem(rsaPublicKey)
    const encrypted = publicKey.encrypt(data, 'RSA-OAEP', {
      md: forge.md.sha256.create()
    })
    return forge.util.encode64(encrypted)
  } catch (e) {
    return ''
  }
}
