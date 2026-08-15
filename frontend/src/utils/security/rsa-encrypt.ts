import JSEncrypt from 'jsencrypt'
import { rsaPublicKey } from '@/config/security/rsa'

export function encryptDataByRsa(data: string): string {
  const encrypt = new JSEncrypt()
  encrypt.setPublicKey(rsaPublicKey)
  try {
    return encrypt.encrypt(data) || ''
  } catch (e) {
    return ''
  }
}
