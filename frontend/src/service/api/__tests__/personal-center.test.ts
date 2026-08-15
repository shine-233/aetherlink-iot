/**
 * 文件用途: 个人中心 wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证资料、邮箱、告警邮箱、语言、密码和头像接口。
 * 关键注意事项: 账号安全行为需要后端验证，前端测试只锁住请求路径和参数。
 * 重构建议: 按资料设置、账号安全、通知偏好和上传拆分，并补验证码失败边界。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet, mockPost, mockPut } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn()
}))

vi.mock('@/service/request', () => ({
  request: {
    get: mockGet,
    post: mockPost,
    put: mockPut
  }
}))

import {
  changeAccountEmail,
  changeInformation,
  fetchUserInfo,
  fetchWarningEmails,
  passwordModification,
  savePreferredLanguage,
  updateWarningEmails,
  uploadFile
} from '../personal-center'

describe('personal-center API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches board user profile from the personal center endpoint', async () => {
    mockGet.mockResolvedValue({ data: { name: 'operator' }, error: null })

    await fetchUserInfo()

    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('/board/user/info', {})
  })

  it('updates personal profile and account password through board user endpoints', async () => {
    mockPost.mockResolvedValue({ data: null, error: null })
    const profilePayload = { name: 'operator', phone: '13800000000' }
    const passwordPayload = { old_password: 'old', new_password: 'new' }

    await changeInformation(profilePayload)
    await passwordModification(passwordPayload)

    expect(mockPost).toHaveBeenNthCalledWith(1, '/board/user/update', profilePayload)
    expect(mockPost).toHaveBeenNthCalledWith(2, '/board/user/update/password', passwordPayload)
  })

  it('changes login email while preserving the verification-code payload', async () => {
    mockPost.mockResolvedValue({ data: null, error: null })
    const payload = {
      new_email: 'new@example.com',
      verify_code: '123456'
    }

    await changeAccountEmail(payload)

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/user/change-email', payload)
  })

  it('fetches and updates global warning email recipients', async () => {
    mockGet.mockResolvedValue({ data: ['ops@example.com'], error: null })
    mockPut.mockResolvedValue({ data: ['ops@example.com', 'audit@example.com'], error: null })

    await fetchWarningEmails()
    await updateWarningEmails({ emails: ['ops@example.com', 'audit@example.com'] })

    expect(mockGet).toHaveBeenCalledWith('/user/warning-email')
    expect(mockPut).toHaveBeenCalledWith('/user/warning-email', {
      emails: ['ops@example.com', 'audit@example.com']
    })
  })

  it('saves preferred language with both supported language payload fields', async () => {
    mockPost.mockResolvedValue({ data: null, error: null })
    const payload = {
      prefer_lang: 'zh-CN',
      default_language: 'zh-CN'
    }

    await savePreferredLanguage(payload)

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/user/prefer-lang', payload)
  })

  it('uploads personal-center files through /file/up without rewriting FormData payloads', async () => {
    mockPost.mockResolvedValue({ data: { url: '/uploads/avatar.png' }, error: null })
    const formData = new FormData()
    formData.append('file', new Blob(['avatar']), 'avatar.png')

    await uploadFile(formData)

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/file/up', formData)
  })
})
