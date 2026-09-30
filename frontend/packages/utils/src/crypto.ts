/**
 * 文件用途：提供对象数据的 AES 加解密工具类。
 * 核心逻辑：使用 crypto-js 将对象序列化后加密，并在解密时解析回泛型对象。
 * 关键注意事项：密钥由调用方传入，解密失败或密文格式错误时需要调用方处理异常。
 * 重构建议：可增加安全的 tryDecrypt 方法，避免调用方重复编写异常保护。
 */
import CryptoJS from 'crypto-js'

export class Crypto<T extends object> {
  /** Secret */
  secret: string

  constructor(secret: string) {
    this.secret = secret
  }

  encrypt(data: T): string {
    const dataString = JSON.stringify(data)
    const encrypted = CryptoJS.AES.encrypt(dataString, this.secret)
    return encrypted.toString()
  }

  decrypt(encrypted: string) {
    const decrypted = CryptoJS.AES.decrypt(encrypted, this.secret)
    const dataString = decrypted.toString(CryptoJS.enc.Utf8)
    try {
      return JSON.parse(dataString) as T
    } catch {
      // avoid parse error
      return null
    }
  }
}
