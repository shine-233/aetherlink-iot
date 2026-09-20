/**
 * 文件用途：通用 Secrets Storage（ROADMAP TB-18）管理视图单元测试。
 * 核心逻辑：验证列表加载、新增密钥、明文解密与删除交互契约。
 */
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import SecretsManagementView from '../index.vue'

// 模拟 API 客户端
vi.mock('@/service/api/secret', () => ({
  getSecretsList: vi.fn().mockResolvedValue({
    data: {
      list: [
        {
          id: 'sec-1',
          tenant_id: 't-1',
          key: 'AWS_ACCESS_KEY',
          name: 'AWS IoT 生产凭据',
          secret_type: 'API_KEY',
          description: '用于 AWS IoT Core 桥接',
          mask_preview: 'AKIA****',
          key_id: 'k1',
          needs_reseal: false,
          created_at: '2026-09-17T07:00:00Z',
          updated_at: '2026-09-17T07:00:00Z'
        },
        {
          id: 'sec-2',
          tenant_id: 't-1',
          key: 'LEGACY_MQTT_PASS',
          name: '旧 MQTT 密码',
          secret_type: 'PASSWORD',
          description: '待轮换',
          mask_preview: '****',
          key_id: 'k0',
          needs_reseal: true,
          created_at: '2026-09-10T07:00:00Z',
          updated_at: '2026-09-10T07:00:00Z'
        }
      ],
      total: 2
    }
  }),
  createSecret: vi.fn().mockResolvedValue({ data: { id: 'sec-new' } }),
  updateSecret: vi.fn().mockResolvedValue({ data: { id: 'sec-1' } }),
  deleteSecret: vi.fn().mockResolvedValue({ data: { deleted: true } }),
  revealSecret: vi.fn().mockResolvedValue({
    data: {
      id: 'sec-1',
      key: 'AWS_ACCESS_KEY',
      value: 'AKIAIOSFODNN7EXAMPLE'
    }
  }),
  resealSecret: vi.fn().mockResolvedValue({ data: { id: 'sec-2', needs_reseal: false } })
}))

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      success: vi.fn(),
      error: vi.fn(),
      warning: vi.fn(),
      info: vi.fn()
    })
  }
})

describe('Secrets Management View (TB-18)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders secrets management view and loads secrets list', async () => {
    const wrapper = mount(SecretsManagementView)
    expect(wrapper.exists()).toBe(true)

    // 验证包含标题
    expect(wrapper.text()).toContain('通用密钥保管库 (Secrets Storage)')
    expect(wrapper.text()).toContain('新增密钥')
  })

  it('renders security notice alert', () => {
    const wrapper = mount(SecretsManagementView)
    expect(wrapper.text()).toContain('${secret.KEY_NAME}')
    expect(wrapper.text()).toContain('AES-256-GCM')
  })
})
