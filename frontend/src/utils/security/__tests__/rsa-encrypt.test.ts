/*
 * 文件用途：覆盖 WebCrypto 版 RSA 加密 helper 的算法契约。
 * 核心逻辑：使用运行时 webcrypto（Node ≥20 全局内置）生成 RSA-OAEP/SHA-256 密钥对，
 * 断言密文为标准 RSA 长度、每次随机化（OAEP 填充引入随机性），
 * 并可用同构 OAEP/SHA-256 参数解开（等价于 Go 侧 rsa.DecryptOAEP）。
 * 关键注意事项：PEM 拼装需带标准头尾边界；依赖环境提供 crypto.subtle。
 */
import { describe, expect, it } from 'vitest'
import { encryptDataByRsa } from '@/utils/security/rsa-encrypt'

const RSA_MODULUS_LENGTH = 2048
const RSA_CIPHERTEXT_BYTES = RSA_MODULUS_LENGTH / 8

async function generateTestKeyPair(): Promise<{ publicKeyPem: string; privateKey: CryptoKey }> {
  const pair = await crypto.subtle.generateKey(
    { name: 'RSA-OAEP', modulusLength: RSA_MODULUS_LENGTH, publicExponent: new Uint8Array([1, 0, 1]), hash: 'SHA-256' },
    true,
    ['encrypt', 'decrypt']
  )
  const spki = await crypto.subtle.exportKey('spki', pair.publicKey)
  let base64 = ''
  const bytes = new Uint8Array(spki)
  bytes.forEach(byte => {
    base64 += String.fromCharCode(byte)
  })
  const publicKeyPem = `-----BEGIN PUBLIC KEY-----\n${btoa(base64)}\n-----END PUBLIC KEY-----`
  return { publicKeyPem, privateKey: pair.privateKey }
}

function base64ToBytes(base64: string): Uint8Array {
  const binary = atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

describe('encryptDataByRsa (WebCrypto RSA-OAEP/SHA-256)', () => {
  it('produces standard-length ciphertext decryptable with matching OAEP/SHA-256 params', async () => {
    const { publicKeyPem, privateKey } = await generateTestKeyPair()
    const plaintext = '123456salt'

    const ciphertext = await encryptDataByRsa(plaintext, publicKeyPem)

    expect(ciphertext).not.toBe('')
    const cipherBytes = base64ToBytes(ciphertext)
    // 标准RSA 密文长度等于模长字节数（2048 位 → 256 字节）
    expect(cipherBytes).toHaveLength(RSA_CIPHERTEXT_BYTES)

    // 用同构 OAEP/SHA-256 解密参数还原明文，验证与 Go 后端解密逻辑一致
    const decrypted = await crypto.subtle.decrypt(
      { name: 'RSA-OAEP' },
      privateKey,
      cipherBytes.buffer as ArrayBuffer
    )
    expect(new TextDecoder().decode(decrypted)).toBe(plaintext)
  })

  it('randomizes ciphertext across calls (OAEP padding)', async () => {
    const { publicKeyPem } = await generateTestKeyPair()

    const first = await encryptDataByRsa('same-input', publicKeyPem)
    const second = await encryptDataByRsa('same-input', publicKeyPem)

    expect(first).not.toBe('')
    expect(second).not.toBe('')
    expect(first).not.toBe(second)
  })

  it('returns empty string for invalid PEM input', async () => {
    const result = await encryptDataByRsa('payload', 'not-a-valid-pem')
    expect(result).toBe('')
  })
})
