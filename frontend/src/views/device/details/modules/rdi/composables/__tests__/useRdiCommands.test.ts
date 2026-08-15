/**
 * 文件用途: RDI composable useRdiCommands 的单元测试。
 * 核心逻辑: 通过 mock API、store 或时间行为验证 composable 的状态输出、动作和异常分支。
 * 关键注意事项: 测试应聚焦 composable 契约，避免依赖 RDI 操作视图 DOM 细节。
 * 重构建议: 继续补成功、失败、空数据和清理生命周期用例，提升组合函数边界可信度。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick } from 'vue';

/**
 * 说明: useRdiCommands.ts 中以下纯函数为模块私有(未导出),无法直接进行单元测试:
 * - normalizeOtaPackages: OTA 包列表归一化(过滤无效项)
 * - parsePackageAdditionalInfo: 解析 additional_info JSON 字符串
 * - parsePackageSize: 包大小解析(数字/字符串)
 * - resolvePackageUrl: 包 URL 拼接(相对路径/绝对路径)
 * - normalizedFieldSetting: Field Setting 归一化(n* 前缀/SW 前缀)
 *
 * 如需对这些私有函数进行直接测试,需在源码中将它们导出(本次任务不修改源码)。
 * 本测试文件通过 composable 返回的方法间接验证这些逻辑:
 * - loadOtaPackages -> normalizeOtaPackages
 * - applyOtaPackage(通过 otaPackageId watch 触发) -> parsePackageSize / resolvePackageUrl / parsePackageAdditionalInfo
 * - sendFieldSetting -> normalizedFieldSetting
 */

// Mock 外部依赖
// 使用 vi.hoisted 声明 mock,确保 vi.mock 工厂函数能访问到(hoisting 安全)
const {
  mockSendRdiCommand,
  mockRdiLatestFirmware,
  mockGetOtaPackageList,
  mockMessage
} = vi.hoisted(() => ({
  mockSendRdiCommand: vi.fn(),
  mockRdiLatestFirmware: vi.fn(),
  mockGetOtaPackageList: vi.fn(),
  mockMessage: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn()
  }
}));

vi.mock('@/service/api', () => ({
  sendRdiCommand: (...args: any[]) => mockSendRdiCommand(...args),
  rdiLatestFirmware: (...args: any[]) => mockRdiLatestFirmware(...args)
}));

vi.mock('@/service/product/update-package', () => ({
  getOtaPackageList: (...args: any[]) => mockGetOtaPackageList(...args)
}));

vi.mock('@/utils/common/discrete', () => ({
  message: mockMessage
}));

vi.mock('@/utils/common/tool', () => ({
  getBaseServerUrl: () => 'http://localhost:8080/api/v1'
}));

import { useRdiCommands } from '../useRdiCommands';
import type { RDIConfig } from '@/service/api/rdi';

// 创建最小可用 config
function createConfig(overrides: Partial<RDIConfig> = {}): RDIConfig {
  return {
    data_collection_interval: 60,
    alarm_sensor_1_enabled: true,
    alarm_sensor_2_enabled: true,
    sensor_1_upper: 80,
    sensor_1_lower: -10,
    sensor_2_upper: 80,
    sensor_2_lower: -10,
    sensor_1_duration: 30,
    sensor_2_duration: 30,
    switch_1_alarm_mode: 'disabled',
    switch_2_alarm_mode: 'disabled',
    switch_1_alarm_duration: 30,
    switch_2_alarm_duration: 30,
    dry_contact_alarm_level: 'high',
    dry_contact_normal_level: 'low',
    dry_contact_alarm_delay: 0,
    dry_contact_normal_delay: 0,
    notification_enabled: false,
    notification_temperature_alarm: true,
    notification_switch_alarm: true,
    notification_warranty_alarm: true,
    sensor_alarm_emails: '',
    switch_alarm_emails: '',
    warranty_alarm_emails: '',
    sensor_1_alarm_emails: '',
    sensor_2_alarm_emails: '',
    switch_1_alarm_emails: '',
    switch_2_alarm_emails: '',
    field_setting: {},
    ...overrides
  } as RDIConfig;
}

// 创建 composable 实例
function createComposable(config: RDIConfig = createConfig()) {
  return useRdiCommands(() => 'dev-1', config, (key: any) => String(key));
}

describe('useRdiCommands - 命令与 OTA 归一化', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('setDryContact - 设置干触点', () => {
    it('发送 set_dry_contact 命令并携带 level 与 delay_seconds', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const composable = createComposable();
      composable.dryCommandDelay.value = 10;
      await composable.setDryContact('high');
      expect(mockSendRdiCommand).toHaveBeenCalledTimes(1);
      expect(mockSendRdiCommand).toHaveBeenCalledWith('dev-1', {
        identifier: 'set_dry_contact',
        params: { level: 'high', delay_seconds: 10 }
      });
    });

    it('支持 low 电平', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const composable = createComposable();
      await composable.setDryContact('low');
      expect(mockSendRdiCommand).toHaveBeenCalledWith('dev-1', {
        identifier: 'set_dry_contact',
        params: { level: 'low', delay_seconds: 0 }
      });
    });
  });

  describe('testDryContact - 测试干触点', () => {
    it('发送 test_dry_contact 命令并携带 config 中的告警电平与测试时长', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const config = createConfig({ dry_contact_alarm_level: 'low' });
      const composable = createComposable(config);
      composable.dryTestDuration.value = 60;
      await composable.testDryContact();
      expect(mockSendRdiCommand).toHaveBeenCalledWith('dev-1', {
        identifier: 'test_dry_contact',
        params: { level: 'low', duration_seconds: 60 }
      });
    });
  });

  describe('sendUnbindDevice / sendFactoryReset', () => {
    it('发送 unbind_device 命令', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const composable = createComposable();
      await composable.sendUnbindDevice();
      expect(mockSendRdiCommand).toHaveBeenCalledWith('dev-1', { identifier: 'unbind_device', params: {} });
    });

    it('发送 factory_reset 命令', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const composable = createComposable();
      await composable.sendFactoryReset();
      expect(mockSendRdiCommand).toHaveBeenCalledWith('dev-1', { identifier: 'factory_reset', params: {} });
    });
  });

  describe('sendFieldSetting - Field Setting 归一化(N00-N07/SW1-SW4)', () => {
    it('n* 前缀字段的逗号分隔字符串被拆分为数组', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const config = createConfig({
        field_setting: {
          n00: '1, 2, 3',
          n01: 'a,b,c'
        }
      });
      const composable = createComposable(config);
      await composable.sendFieldSetting();
      expect(mockSendRdiCommand).toHaveBeenCalledTimes(1);
      const callArgs = mockSendRdiCommand.mock.calls[0][1];
      expect(callArgs.identifier).toBe('set_field_setting');
      expect(callArgs.params.n00).toEqual(['1', '2', '3']);
      expect(callArgs.params.n01).toEqual(['a', 'b', 'c']);
    });

    it('n* 前缀字段已是数组时保持原样', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const config = createConfig({
        field_setting: { n02: [1, 2, 3] }
      });
      const composable = createComposable(config);
      await composable.sendFieldSetting();
      const callArgs = mockSendRdiCommand.mock.calls[0][1];
      expect(callArgs.params.n02).toEqual([1, 2, 3]);
    });

    it('sw* 前缀字段字符串被包装为 { label } 对象', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const config = createConfig({
        field_setting: { sw1: '开启' }
      });
      const composable = createComposable(config);
      await composable.sendFieldSetting();
      const callArgs = mockSendRdiCommand.mock.calls[0][1];
      expect(callArgs.params.sw1).toEqual({ label: '开启' });
    });

    it('sw* 前缀字段已是对象时保持原样', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const config = createConfig({
        field_setting: { sw2: { label: '关闭', extra: 1 } }
      });
      const composable = createComposable(config);
      await composable.sendFieldSetting();
      const callArgs = mockSendRdiCommand.mock.calls[0][1];
      expect(callArgs.params.sw2).toEqual({ label: '关闭', extra: 1 });
    });

    it('空 field_setting 时显示错误且不发送命令', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const composable = createComposable();
      await composable.sendFieldSetting();
      expect(mockSendRdiCommand).toHaveBeenCalledTimes(0);
      expect(mockMessage.error).toHaveBeenCalledTimes(1);
      expect(mockMessage.error).toHaveBeenCalledWith('empty');
    });
  });

  describe('sendOtaUpgrade - OTA 升级命令', () => {
    it('所有字段齐全时发送 ota_upgrade 命令', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const composable = createComposable();
      composable.otaCommand.firmware_url = 'http://example.com/fw.bin';
      composable.otaCommand.version = '1.2.3';
      composable.otaCommand.size = 1024;
      composable.otaCommand.md5 = 'abc123';
      await composable.sendOtaUpgrade();
      expect(mockSendRdiCommand).toHaveBeenCalledWith('dev-1', {
        identifier: 'ota_upgrade',
        params: {
          firmware_url: 'http://example.com/fw.bin',
          version: '1.2.3',
          size: 1024,
          md5: 'abc123'
        }
      });
    });

    it('缺少必填字段时显示错误且不发送命令', async () => {
      mockSendRdiCommand.mockResolvedValue({ error: null, data: {} });
      const composable = createComposable();
      // 仅设置部分字段,缺少 md5 与 size
      composable.otaCommand.firmware_url = 'http://example.com/fw.bin';
      composable.otaCommand.version = '1.0.0';
      await composable.sendOtaUpgrade();
      expect(mockSendRdiCommand).toHaveBeenCalledTimes(0);
      expect(mockMessage.error).toHaveBeenCalledTimes(1);
      expect(mockMessage.error).toHaveBeenCalledWith('otaMissingFields: size, md5');
    });
  });

  describe('loadOtaPackages - OTA 包归一化', () => {
    it('从 list 字段归一化并过滤无效项', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: {
          list: [
            { id: 'pkg-1', name: '固件A', version: '1.0.0' },
            { id: 'pkg-2', name: '固件B', version: '2.0.0' },
            { id: '', name: '无效' }, // 无 id,应被过滤
            null // null,应被过滤
          ]
        }
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      expect(composable.otaPackages.value.length).toBe(2);
      expect(composable.otaPackages.value[0].id).toBe('pkg-1');
      expect(composable.otaPackages.value[1].id).toBe('pkg-2');
    });

    it('直接数组 payload 也能被归一化', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: [{ id: 'pkg-x', version: '3.0.0' }]
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      expect(composable.otaPackages.value.length).toBe(1);
      expect(composable.otaPackages.value[0].id).toBe('pkg-x');
    });

    it('otaPackageOptions 生成正确的 label 与 value', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: {
          list: [{ id: 'pkg-1', name: '固件A', version: '1.0.0' }]
        }
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      const options = composable.otaPackageOptions.value;
      expect(options.length).toBe(1);
      expect(options[0].value).toBe('pkg-1');
      expect(options[0].label).toContain('固件A');
      expect(options[0].label).toContain('1.0.0');
    });
  });

  describe('applyOtaPackage - 包大小解析与 URL 拼接(通过 otaPackageId watch 触发)', () => {
    it('数字类型 size 被正确解析', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: {
          list: [
            {
              id: 'pkg-1',
              version: '1.0.0',
              package_url: 'http://example.com/fw.bin',
              size: 2048,
              signature: 'md5hash'
            }
          ]
        }
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      composable.otaPackageId.value = 'pkg-1';
      await nextTick();
      expect(composable.otaCommand.size).toBe(2048);
      expect(composable.otaCommand.version).toBe('1.0.0');
      expect(composable.otaCommand.md5).toBe('md5hash');
    });

    it('字符串类型 size 被解析为数字', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: {
          list: [
            {
              id: 'pkg-1',
              version: '1.0.0',
              package_url: 'http://example.com/fw.bin',
              package_size: '4096',
              signature: 'md5hash'
            }
          ]
        }
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      composable.otaPackageId.value = 'pkg-1';
      await nextTick();
      expect(composable.otaCommand.size).toBe(4096);
    });

    it('依次回退解析 size -> package_size -> file_size -> additional_info.size', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: {
          list: [
            {
              id: 'pkg-1',
              version: '1.0.0',
              package_url: 'http://example.com/fw.bin',
              file_size: 8192,
              signature: 'md5hash'
            }
          ]
        }
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      composable.otaPackageId.value = 'pkg-1';
      await nextTick();
      expect(composable.otaCommand.size).toBe(8192);
    });

    it('从 additional_info JSON 中解析 size 与 version', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: {
          list: [
            {
              id: 'pkg-1',
              package_url: 'http://example.com/fw.bin',
              additional_info: JSON.stringify({ size: 16384, version: '2.1.0', md5: 'abc' })
            }
          ]
        }
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      composable.otaPackageId.value = 'pkg-1';
      await nextTick();
      expect(composable.otaCommand.size).toBe(16384);
      expect(composable.otaCommand.version).toBe('2.1.0');
      expect(composable.otaCommand.md5).toBe('abc');
    });

    it('https 绝对路径 URL 保持不变', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: {
          list: [
            {
              id: 'pkg-1',
              version: '1.0.0',
              package_url: 'https://cdn.example.com/firmware/fw.bin',
              size: 1024,
              signature: 'md5'
            }
          ]
        }
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      composable.otaPackageId.value = 'pkg-1';
      await nextTick();
      expect(composable.otaCommand.firmware_url).toBe('https://cdn.example.com/firmware/fw.bin');
    });

    it('http 绝对路径 URL 保持不变', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: {
          list: [
            {
              id: 'pkg-1',
              version: '1.0.0',
              package_url: 'http://cdn.example.com/firmware/fw.bin',
              size: 1024,
              signature: 'md5'
            }
          ]
        }
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      composable.otaPackageId.value = 'pkg-1';
      await nextTick();
      expect(composable.otaCommand.firmware_url).toBe('http://cdn.example.com/firmware/fw.bin');
    });

    it('相对路径 URL 拼接 baseUrl(以 / 开头)', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: {
          list: [
            {
              id: 'pkg-1',
              version: '1.0.0',
              package_url: '/uploads/firmware/fw.bin',
              size: 1024,
              signature: 'md5'
            }
          ]
        }
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      composable.otaPackageId.value = 'pkg-1';
      await nextTick();
      // getBaseServerUrl() = 'http://localhost:8080/api/v1' -> replace('/api/v1','/') = 'http://localhost:8080/'
      // + 'uploads/firmware/fw.bin' (去掉前导 /)
      expect(composable.otaCommand.firmware_url).toBe('http://localhost:8080/uploads/firmware/fw.bin');
    });

    it('相对路径 URL 拼接 baseUrl(不以 / 开头)', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: {
          list: [
            {
              id: 'pkg-1',
              version: '1.0.0',
              package_url: 'uploads/firmware/fw.bin',
              size: 1024,
              signature: 'md5'
            }
          ]
        }
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      composable.otaPackageId.value = 'pkg-1';
      await nextTick();
      expect(composable.otaCommand.firmware_url).toBe('http://localhost:8080/uploads/firmware/fw.bin');
    });

    it('空 URL 返回空字符串', async () => {
      mockGetOtaPackageList.mockResolvedValue({
        error: null,
        data: {
          list: [{ id: 'pkg-1', version: '1.0.0', size: 1024, signature: 'md5' }]
        }
      });
      const composable = createComposable();
      await composable.loadOtaPackages();
      composable.otaPackageId.value = 'pkg-1';
      await nextTick();
      expect(composable.otaCommand.firmware_url).toBe('');
    });
  });

  describe('checkLatestFirmware - 检查最新固件', () => {
    it('有更新时应用最新固件包', async () => {
      mockRdiLatestFirmware.mockResolvedValue({
        error: null,
        data: {
          update_available: true,
          package: {
            id: 'fw-latest',
            version: '9.9.9',
            package_url: 'http://example.com/fw.bin',
            size: 512,
            signature: 'md5latest'
          }
        }
      });
      const composable = createComposable();
      await composable.checkLatestFirmware();
      expect(composable.latestFirmwarePackage.value).not.toBeNull();
      expect(composable.latestFirmwarePackage.value?.id).toBe('fw-latest');
      // 应自动应用:otaCommand 被填充
      expect(composable.otaCommand.version).toBe('9.9.9');
      expect(composable.otaCommand.size).toBe(512);
      expect(mockMessage.success).toHaveBeenCalledTimes(1);
      expect(mockMessage.success).toHaveBeenCalledWith('updateAvailable: 9.9.9');
    });

    it('无更新时清空最新固件包并提示', async () => {
      mockRdiLatestFirmware.mockResolvedValue({
        error: null,
        data: { update_available: false }
      });
      const composable = createComposable();
      await composable.checkLatestFirmware();
      expect(composable.latestFirmwarePackage.value).toBeNull();
      expect(mockMessage.success).toHaveBeenCalledTimes(1);
      expect(mockMessage.success).toHaveBeenCalledWith('alreadyLatest');
    });

    it('接口报错时不应用固件包', async () => {
      mockRdiLatestFirmware.mockResolvedValue({ error: 'network', data: null });
      const composable = createComposable();
      await composable.checkLatestFirmware();
      expect(composable.latestFirmwarePackage.value).toBeNull();
    });
  });
});
