/**
 * 文件用途: 通知服务配置 wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证邮件、短信、推送配置读取保存和测试发送请求。
 * 关键注意事项: 通知可达性和凭证有效性不在 mock 层证明，仍需真实服务或后端测试。
 * 重构建议: 拆分邮件、短信、推送用例，并补充敏感字段不泄露的回归检查。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet, mockPost, mockPut, mockDelete } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn()
}))

vi.mock('@/service/request', () => ({
  request: {
    get: mockGet,
    post: mockPost,
    put: mockPut,
    delete: mockDelete
  }
}))

import {
  createAlarmEmailTemplate,
  deleteAlarmEmailTemplate,
  editNotificationServices,
  editPushNotificationServices,
  fetchAlarmEmailTemplates,
  fetchNotificationServicesEmail,
  fetchNotificationServicesSms,
  fetchPushNotificationServices,
  previewAlarmEmailTemplate,
  setDefaultAlarmEmailTemplate,
  sendTestEmail,
  updateAlarmEmailTemplate
} from '../notification-services'

describe('notification-services API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches EMAIL notification service configuration', async () => {
    mockGet.mockResolvedValue({ data: { host: 'smtp.example.com' }, error: null })

    const result = await fetchNotificationServicesEmail()

    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('/notification/services/config/EMAIL')
    expect(result).toEqual({ data: { host: 'smtp.example.com' }, error: null })
  })

  it('fetches SME_CODE notification service configuration for the SMS tab', async () => {
    mockGet.mockResolvedValue({ data: { provider: 'ALIYUN' }, error: null })

    const result = await fetchNotificationServicesSms()

    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('/notification/services/config/SME_CODE')
    expect(result).toEqual({ data: { provider: 'ALIYUN' }, error: null })
  })

  it('saves notification service configuration without dropping provider fields', async () => {
    mockPost.mockResolvedValue({ data: null, error: null })
    const payload = {
      notify_type: 'EMAIL',
      host: 'smtp.example.com',
      port: 465,
      username: 'ops@example.com',
      password: 'secret',
      from: 'ops@example.com',
      enable_tls: true
    }

    await editNotificationServices(payload)

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/notification/services/config', payload)
  })

  it('sends a test email with recipient and subject payload', async () => {
    mockPost.mockResolvedValue({ data: null, error: null })
    const payload = {
      to: 'maintainer@example.com',
      subject: 'alarm test',
      content: 'temperature alarm test'
    }

    await sendTestEmail(payload)

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/notification/services/config/e-mail/test', payload)
  })

  it('keeps the current push-message config endpoint', async () => {
    mockGet.mockResolvedValue({ data: { enable: true }, error: null })

    await fetchPushNotificationServices()

    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('/message_push/config')
  })

  it('updates push-message configuration through the same backend endpoint', async () => {
    mockPost.mockResolvedValue({ data: null, error: null })
    const payload = {
      enable: true,
      server_key: 'push-key',
      app_id: 'aetherlink-iot'
    }

    await editPushNotificationServices(payload)

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/message_push/config', payload)
  })

  it('keeps alarm email template CRUD, preview, and default routes stable', async () => {
    const payload = {
      name: 'Alarm default',
      subject_template: '[AetherLink] {{.Subject}}',
      body_template: '{{.Message}}',
      enabled: true,
      is_default: true
    }
    mockPost.mockResolvedValue({ data: null, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })
    mockDelete.mockResolvedValue({ data: null, error: null })

    await fetchAlarmEmailTemplates({ page: 1, page_size: 10 })
    await createAlarmEmailTemplate(payload)
    await updateAlarmEmailTemplate('template/1', payload)
    await previewAlarmEmailTemplate({ subject_template: payload.subject_template, body_template: payload.body_template })
    await setDefaultAlarmEmailTemplate('template/1')
    await deleteAlarmEmailTemplate('template/1')

    expect(mockGet).toHaveBeenCalledWith('/notification/e-mail/templates', { params: { page: 1, page_size: 10 } })
    expect(mockPost).toHaveBeenNthCalledWith(1, '/notification/e-mail/templates', payload)
    expect(mockPut).toHaveBeenCalledWith('/notification/e-mail/templates/template%2F1', payload)
    expect(mockPost).toHaveBeenNthCalledWith(2, '/notification/e-mail/templates/preview', {
      subject_template: payload.subject_template,
      body_template: payload.body_template
    })
    expect(mockPost).toHaveBeenNthCalledWith(3, '/notification/e-mail/templates/template%2F1/default')
    expect(mockDelete).toHaveBeenCalledWith('/notification/e-mail/templates/template%2F1')
  })
})
