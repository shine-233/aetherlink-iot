import forge from 'node-forge'
import { rsaPublicKey } from '@/config/security/rsa'

/**
 * 与后端 initialize.DecryptPassword 的填充参数保持一致：
 * RSA-OAEP + SHA-256。任何一侧变更算法都必须同步另一侧。
 */
export function encryptDataByRsa(data: string): string {
  try {
    const publicKey = forge.pki.publicKeyFromPem(rsaPublicKey)
    const encrypted = publicKey.encrypt(data, 'RSA-OAEP', {
      md: forge.md.sha256.create()
    })
    return forge.util.encode64(encrypted)
  } catch (e) {
    return ''
  }
}
