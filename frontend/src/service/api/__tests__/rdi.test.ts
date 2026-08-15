/**
 * 文件用途: RDI API wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证 RDI 激活、配置、历史、命令、分享、固件和共享设备请求。
 * 关键注意事项: RDI 设备真实状态、命令执行和分享权限仍需后端/API 自动化验证。
 * 重构建议: 按配置、遥测历史、命令、分享和固件拆分用例，并补权限失败分支。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';

// 使用 vi.hoisted 声明 mock,确保 vi.mock 工厂函数能访问到(hoisting 安全)
const { mockGet, mockPost, mockPut, mockDelete } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn()
}));

// Mock request 模块,验证各 API 函数调用的 URL、方法与参数
vi.mock('@/service/request', () => ({
  request: {
    get: mockGet,
    post: mockPost,
    put: mockPut,
    delete: mockDelete
  }
}));

// 导入被测模块(在 mock 之后导入,确保 mock 生效)
import {
  rdiThingModel,
  activateRdiDevice,
  rdiDeviceConfig,
  updateRdiDeviceConfig,
  rdiDeviceHistory,
  sendRdiCommand,
  createRdiShareToken,
  rdiLatestFirmware,
  acceptRdiSharedDevice,
  rdiSharedWithMeDevices,
  revokeRdiShareToken,
  revokeRdiShareRecipient
} from '../rdi';

describe('RDI API 层 - rdi.ts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('rdiThingModel', () => {
    it('调用 GET /rdi/thing-model 获取物模型', async () => {
      mockGet.mockResolvedValue({ error: null, data: { telemetry: [] } });
      await rdiThingModel();
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/rdi/thing-model');
    });
  });

  describe('activateRdiDevice', () => {
    it('调用 POST /rdi/devices/activate 并发送正确的 payload', async () => {
      mockPost.mockResolvedValue({ error: null, data: { device_id: 'dev-1' } });
      const params = { pid_number: 'PID-001', name: '我的设备' };
      await activateRdiDevice(params);
      expect(mockPost).toHaveBeenCalledTimes(1);
      expect(mockPost).toHaveBeenCalledWith('/rdi/devices/activate', params);
    });

    it('当 name 未提供时仅发送 pid_number', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      await activateRdiDevice({ pid_number: 'PID-002' });
      expect(mockPost).toHaveBeenCalledWith('/rdi/devices/activate', { pid_number: 'PID-002' });
    });
  });

  describe('rdiDeviceConfig', () => {
    it('调用 GET /rdi/devices/{deviceId}/config 获取设备配置', async () => {
      mockGet.mockResolvedValue({ error: null, data: { config: {} } });
      await rdiDeviceConfig('dev-123');
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/rdi/devices/dev-123/config', {});
    });

    it('对 deviceId 中的特殊字符进行 URL 编码', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await rdiDeviceConfig('dev/with space');
      expect(mockGet).toHaveBeenCalledWith('/rdi/devices/dev%2Fwith%20space/config', {});
    });

    it('支持传入额外的 requestConfig', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const requestConfig = { headers: { 'X-Custom': '1' } };
      await rdiDeviceConfig('dev-1', requestConfig);
      expect(mockGet).toHaveBeenCalledWith('/rdi/devices/dev-1/config', requestConfig);
    });
  });

  describe('updateRdiDeviceConfig', () => {
    it('调用 PUT /rdi/devices/{deviceId}/config 并发送正确的 payload', async () => {
      mockPut.mockResolvedValue({ error: null, data: { config: {} } });
      const params = {
        config: { data_collection_interval: 60 } as any,
        system_info: {
          installation_location: '北京',
          address: 'Pudong 1',
          installation_date: '2026-07-09',
          installer_company: 'Installer Co',
          installer_contact: 'Alex',
          installer_name: 'Alex Name',
          installer_phone: '+1 555 0000',
          installer_email: 'alex@example.com',
          controller_serial_number: 'RDI-SN-001'
        },
        apply_to_device: true
      };
      await updateRdiDeviceConfig('dev-456', params);
      expect(mockPut).toHaveBeenCalledTimes(1);
      expect(mockPut).toHaveBeenCalledWith('/rdi/devices/dev-456/config', params);
    });

    it('对 deviceId 进行 URL 编码', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      await updateRdiDeviceConfig('dev#1', { config: {} as any });
      expect(mockPut).toHaveBeenCalledWith('/rdi/devices/dev%231/config', { config: {} });
    });
  });

  describe('rdiDeviceHistory', () => {
    it('调用 GET /rdi/devices/{deviceId}/history 并携带正确的查询参数', async () => {
      mockGet.mockResolvedValue({ error: null, data: { list: [] } });
      const params = {
        key: 'temperature_1',
        start_time: 1700000000000,
        end_time: 1700003600000,
        page: 1,
        page_size: 100
      };
      await rdiDeviceHistory('dev-hist', params);
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/rdi/devices/dev-hist/history', { params });
    });

    it('支持合并额外的 requestConfig 与 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { key: 'switch_1', start_time: 1, end_time: 2 };
      const requestConfig = { timeout: 5000 };
      await rdiDeviceHistory('dev-1', params, requestConfig);
      expect(mockGet).toHaveBeenCalledWith('/rdi/devices/dev-1/history', { timeout: 5000, params });
    });

    it('对 deviceId 进行 URL 编码', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await rdiDeviceHistory('dev 1', { key: 'k', start_time: 1, end_time: 2 });
      expect(mockGet).toHaveBeenCalledWith('/rdi/devices/dev%201/history', {
        params: { key: 'k', start_time: 1, end_time: 2 }
      });
    });
  });

  describe('sendRdiCommand', () => {
    it('调用 POST /rdi/devices/{deviceId}/commands 并发送正确的命令结构', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { identifier: 'set_dry_contact', params: { level: 'high', delay_seconds: 5 } };
      await sendRdiCommand('dev-cmd', params);
      expect(mockPost).toHaveBeenCalledTimes(1);
      expect(mockPost).toHaveBeenCalledWith('/rdi/devices/dev-cmd/commands', params);
    });

    it('支持无 params 的命令(如 factory_reset)', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      await sendRdiCommand('dev-1', { identifier: 'factory_reset' });
      expect(mockPost).toHaveBeenCalledWith('/rdi/devices/dev-1/commands', { identifier: 'factory_reset' });
    });
  });

  describe('createRdiShareToken', () => {
    it('调用 POST /rdi/devices/{deviceId}/share-token 生成分享令牌', async () => {
      mockPost.mockResolvedValue({ error: null, data: { token: 'tok-1', share_path: '/s/abc' } });
      await createRdiShareToken('dev-share', { expires_in: 3600 });
      expect(mockPost).toHaveBeenCalledTimes(1);
      expect(mockPost).toHaveBeenCalledWith('/rdi/devices/dev-share/share-token', { expires_in: 3600 });
    });

    it('当未提供 expires_in 时发送空对象', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      await createRdiShareToken('dev-1', {});
      expect(mockPost).toHaveBeenCalledWith('/rdi/devices/dev-1/share-token', {});
    });
  });

  describe('rdiLatestFirmware', () => {
    it('调用 GET /rdi/devices/{deviceId}/latest-firmware 查询最新固件', async () => {
      mockGet.mockResolvedValue({ error: null, data: { update_available: false } });
      await rdiLatestFirmware('dev-fw');
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/rdi/devices/dev-fw/latest-firmware');
    });
  });

  describe('acceptRdiSharedDevice', () => {
    it('调用 POST /rdi/share-tokens/{token}/accept 接受共享设备', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      await acceptRdiSharedDevice('tok-accept');
      expect(mockPost).toHaveBeenCalledTimes(1);
      expect(mockPost).toHaveBeenCalledWith('/rdi/share-tokens/tok-accept/accept');
    });
  });

  // REQ-47: owner 主动撤销分享。token 撤销会连带清除接收人,recipient 撤销保留 token。
  describe('revokeRdiShareToken', () => {
    it('调用 DELETE /rdi/devices/{deviceId}/share-tokens/{token} 撤销整条分享链接', async () => {
      mockDelete.mockResolvedValue({ error: null, data: { revoked_tokens: 1, revoked_recipients: 2 } });
      await revokeRdiShareToken('dev-1', 'tok-1');
      expect(mockDelete).toHaveBeenCalledTimes(1);
      expect(mockDelete).toHaveBeenCalledWith('/rdi/devices/dev-1/share-tokens/tok-1');
    });

    it('对 deviceId 与 token 做 URL 编码,避免特殊字符越界成额外路径段', async () => {
      mockDelete.mockResolvedValue({ error: null, data: {} });
      await revokeRdiShareToken('dev/1', 'tok 1');
      expect(mockDelete).toHaveBeenCalledWith('/rdi/devices/dev%2F1/share-tokens/tok%201');
    });
  });

  describe('revokeRdiShareRecipient', () => {
    it('调用 DELETE /rdi/devices/{deviceId}/share-recipients/{userId} 只撤销单个接收人', async () => {
      mockDelete.mockResolvedValue({ error: null, data: { revoked_tokens: 0, revoked_recipients: 1 } });
      await revokeRdiShareRecipient('dev-1', 'user-9');
      expect(mockDelete).toHaveBeenCalledTimes(1);
      expect(mockDelete).toHaveBeenCalledWith('/rdi/devices/dev-1/share-recipients/user-9');
    });
  });

  describe('rdiSharedWithMeDevices', () => {
    it('调用 GET /rdi/shared-with-me/devices 并携带分页参数', async () => {
      mockGet.mockResolvedValue({ error: null, data: { total: 0, list: [] } });
      await rdiSharedWithMeDevices({ page: 2, page_size: 20 });
      expect(mockGet).toHaveBeenCalledTimes(1);
      expect(mockGet).toHaveBeenCalledWith('/rdi/shared-with-me/devices', { params: { page: 2, page_size: 20 } });
    });

    it('无参数时发送空 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: { total: 0, list: [] } });
      await rdiSharedWithMeDevices();
      expect(mockGet).toHaveBeenCalledWith('/rdi/shared-with-me/devices', { params: {} });
    });
  });
});
