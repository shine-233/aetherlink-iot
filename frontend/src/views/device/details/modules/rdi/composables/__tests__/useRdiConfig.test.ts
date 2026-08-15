/**
 * 文件用途: RDI composable useRdiConfig 的单元测试。
 * 核心逻辑: 通过 mock API、store 或时间行为验证 composable 的状态输出、动作和异常分支。
 * 关键注意事项: 测试应聚焦 composable 契约，避免依赖 RDI 操作视图 DOM 细节。
 * 重构建议: 继续补成功、失败、空数据和清理生命周期用例，提升组合函数边界可信度。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';

/**
 * 说明: useRdiConfig.ts 中的 defaultConfig 与 defaultSystemInfo 已导出,可直接测试。
 * setFieldValue / getFieldValue 通过 composable 返回,本测试通过创建 composable 实例进行验证。
 */

// Mock 外部依赖
vi.mock('@/service/api', () => ({
  rdiDeviceConfig: vi.fn(),
  updateRdiDeviceConfig: vi.fn()
}));

vi.mock('@/utils/common/discrete', () => ({
  message: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn()
  }
}));

// Mock useAppStore,提供 locale 属性
vi.mock('@/store/modules/app', () => ({
  useAppStore: () => ({ locale: 'zh-CN' })
}));

import { defaultConfig, defaultSystemInfo, useRdiConfig } from '../useRdiConfig';
import { rdiDeviceConfig, updateRdiDeviceConfig } from '@/service/api';
import { message } from '@/utils/common/discrete';

describe('useRdiConfig - 默认值与字段读写', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setActivePinia(createPinia());
  });

  describe('defaultConfig - 默认配置', () => {
    it('返回包含所有必填字段的对象', () => {
      const config = defaultConfig();
      expect(config).toMatchObject({
        data_collection_interval: 60,
        alarm_sensor_1_enabled: true,
        alarm_sensor_2_enabled: true,
        sensor_1_upper: 80,
        sensor_1_lower: -10,
        sensor_2_upper: 80,
        sensor_2_lower: -10,
        field_setting: {}
      });
    });

    it('data_collection_interval 默认为 60', () => {
      expect(defaultConfig().data_collection_interval).toBe(60);
    });

    it('传感器告警默认启用', () => {
      const config = defaultConfig();
      expect(config.alarm_sensor_1_enabled).toBe(true);
      expect(config.alarm_sensor_2_enabled).toBe(true);
    });

    it('传感器上下限默认值正确', () => {
      const config = defaultConfig();
      expect(config.sensor_1_upper).toBe(80);
      expect(config.sensor_1_lower).toBe(-10);
      expect(config.sensor_2_upper).toBe(80);
      expect(config.sensor_2_lower).toBe(-10);
    });

    it('传感器持续时间默认为 30 秒', () => {
      const config = defaultConfig();
      expect(config.sensor_1_duration).toBe(30);
      expect(config.sensor_2_duration).toBe(30);
    });

    it('开关告警模式默认为 disabled', () => {
      const config = defaultConfig();
      expect(config.switch_1_alarm_mode).toBe('disabled');
      expect(config.switch_2_alarm_mode).toBe('disabled');
    });

    it('开关告警持续时间默认为 30 秒', () => {
      const config = defaultConfig();
      expect(config.switch_1_alarm_duration).toBe(30);
      expect(config.switch_2_alarm_duration).toBe(30);
    });

    it('干触点告警/正常电平默认值正确', () => {
      const config = defaultConfig();
      expect(config.dry_contact_alarm_level).toBe('high');
      expect(config.dry_contact_normal_level).toBe('low');
    });

    it('干触点告警/正常延迟默认为 0', () => {
      const config = defaultConfig();
      expect(config.dry_contact_alarm_delay).toBe(0);
      expect(config.dry_contact_normal_delay).toBe(0);
    });

    it('通知默认关闭,但各告警类型默认开启', () => {
      const config = defaultConfig();
      expect(config.notification_enabled).toBe(false);
      expect(config.notification_temperature_alarm).toBe(true);
      expect(config.notification_switch_alarm).toBe(true);
      expect(config.notification_warranty_alarm).toBe(true);
    });

    it('所有邮件字段默认为空字符串', () => {
      const config = defaultConfig();
      expect(config.sensor_alarm_emails).toBe('');
      expect(config.switch_alarm_emails).toBe('');
      expect(config.warranty_alarm_emails).toBe('');
      expect(config.sensor_1_alarm_emails).toBe('');
      expect(config.sensor_2_alarm_emails).toBe('');
      expect(config.switch_1_alarm_emails).toBe('');
      expect(config.switch_2_alarm_emails).toBe('');
    });

    it('field_setting 默认为空对象', () => {
      expect(defaultConfig().field_setting).toEqual({});
    });

    it('每次调用返回新对象(无引用共享)', () => {
      const a = defaultConfig();
      const b = defaultConfig();
      expect(a).not.toBe(b);
      expect(a.field_setting).not.toBe(b.field_setting);
    });
  });

  describe('defaultSystemInfo - 默认系统信息', () => {
    it('返回包含所有字段的对象', () => {
      const info = defaultSystemInfo();
      expect(info).toMatchObject({
        installation_location: '',
        address: '',
        installation_date: '',
        installer_company: '',
        installer_contact: '',
        installer_name: '',
        installer_phone: '',
        installer_email: '',
        controller_serial_number: '',
        maintenance_technician: '',
        customer_name: '',
        contact_email: '',
        contact_phone: '',
        warranty_status: '',
        extra_fields: {}
      });
    });

    it('所有字符串字段默认为空字符串', () => {
      const info = defaultSystemInfo();
      expect(info.installation_location).toBe('');
      expect(info.address).toBe('');
      expect(info.installation_date).toBe('');
      expect(info.installer_company).toBe('');
      expect(info.installer_contact).toBe('');
      expect(info.installer_name).toBe('');
      expect(info.installer_phone).toBe('');
      expect(info.installer_email).toBe('');
      expect(info.controller_serial_number).toBe('');
      expect(info.maintenance_technician).toBe('');
      expect(info.customer_name).toBe('');
      expect(info.contact_email).toBe('');
      expect(info.contact_phone).toBe('');
      expect(info.warranty_status).toBe('');
    });

    it('extra_fields 默认为空对象', () => {
      expect(defaultSystemInfo().extra_fields).toEqual({});
    });

    it('每次调用返回新对象(无引用共享)', () => {
      const a = defaultSystemInfo();
      const b = defaultSystemInfo();
      expect(a).not.toBe(b);
      expect(a.extra_fields).not.toBe(b.extra_fields);
    });
  });

  describe('setFieldValue - 设置字段值', () => {
    function createComposable() {
      return useRdiConfig(() => 'dev-1', () => {});
    }

    it('n* 前缀字段:逗号分隔字符串被拆分为数组', () => {
      const { config, setFieldValue, getFieldValue } = createComposable();
      setFieldValue('n00', '1, 2, 3');
      expect(config.field_setting?.n00).toEqual(['1', '2', '3']);
      expect(getFieldValue('n00')).toBe('1,2,3');
    });

    it('n* 前缀字段:去除空白与空项', () => {
      const { config, setFieldValue } = createComposable();
      setFieldValue('n01', ' a , , b ,  ');
      expect(config.field_setting?.n01).toEqual(['a', 'b']);
    });

    it('sw* 前缀字段:JSON 字符串被解析为对象', () => {
      const { config, setFieldValue, getFieldValue } = createComposable();
      setFieldValue('sw1', '{"label":"开启","value":1}');
      expect(config.field_setting?.sw1).toEqual({ label: '开启', value: 1 });
      expect(getFieldValue('sw1')).toBe('开启');
    });

    it('sw* 前缀字段:非 JSON 字符串被包装为 { label } 对象', () => {
      const { config, setFieldValue, getFieldValue } = createComposable();
      setFieldValue('sw2', '关闭');
      expect(config.field_setting?.sw2).toEqual({ label: '关闭' });
      expect(getFieldValue('sw2')).toBe('关闭');
    });

    it('sw* 前缀字段:JSON 数组字符串被包装为 { label } 对象', () => {
      const { config, setFieldValue } = createComposable();
      setFieldValue('sw1', '[1,2,3]');
      // 数组不是普通对象,应被包装为 { label: '[1,2,3]' }
      expect(config.field_setting?.sw1).toEqual({ label: '[1,2,3]' });
    });

    it('普通字段:字符串原样保存', () => {
      const { config, setFieldValue, getFieldValue } = createComposable();
      setFieldValue('custom_field', 'hello');
      expect(config.field_setting?.custom_field).toBe('hello');
      expect(getFieldValue('custom_field')).toBe('hello');
    });

    it('空字符串删除字段', () => {
      const { config, setFieldValue, getFieldValue } = createComposable();
      setFieldValue('n00', '1,2');
      expect(config.field_setting?.n00).toEqual(['1', '2']);
      setFieldValue('n00', '   ');
      expect(config.field_setting?.n00).toBeUndefined();
      expect(getFieldValue('n00')).toBe('');
    });

    it('多次设置同一字段覆盖旧值', () => {
      const { config, setFieldValue } = createComposable();
      setFieldValue('n00', '1,2');
      setFieldValue('n00', '3,4,5');
      expect(config.field_setting?.n00).toEqual(['3', '4', '5']);
    });
  });

  describe('getFieldValue - 读取字段值', () => {
    function createComposable() {
      return useRdiConfig(() => 'dev-1', () => {});
    }

    it('数组字段被 join 为逗号分隔字符串', () => {
      const { setFieldValue, getFieldValue } = createComposable();
      setFieldValue('n00', 'a,b,c');
      expect(getFieldValue('n00')).toBe('a,b,c');
    });

    it('对象字段有 label 时返回 label', () => {
      const { setFieldValue, getFieldValue } = createComposable();
      setFieldValue('sw1', '{"label":"开启"}');
      expect(getFieldValue('sw1')).toBe('开启');
    });

    it('对象字段无 label 时返回 JSON 字符串', () => {
      const { config, getFieldValue } = createComposable();
      config.field_setting = { SW1: { value: 1 } };
      expect(getFieldValue('SW1')).toBe(JSON.stringify({ value: 1 }));
    });

    it('undefined 字段返回空字符串', () => {
      const { getFieldValue } = createComposable();
      expect(getFieldValue('not_exist')).toBe('');
    });

    it('null 字段返回空字符串', () => {
      const { config, getFieldValue } = createComposable();
      config.field_setting = { field1: null };
      expect(getFieldValue('field1')).toBe('');
    });

    it('原始值字段返回字符串形式', () => {
      const { config, getFieldValue } = createComposable();
      config.field_setting = { num: 42, bool: true, str: 'text' };
      expect(getFieldValue('num')).toBe('42');
      expect(getFieldValue('bool')).toBe('true');
      expect(getFieldValue('str')).toBe('text');
    });
  });

  describe('sensor1Range / sensor2Range - 传感器范围计算属性', () => {
    it('sensor1Range 返回 [lower, upper]', () => {
      const { sensor1Range, config } = useRdiConfig(() => 'dev-1', () => {});
      config.sensor_1_lower = -20;
      config.sensor_1_upper = 100;
      expect(sensor1Range.value).toEqual([-20, 100]);
    });

    it('sensor2Range 返回 [lower, upper]', () => {
      const { sensor2Range, config } = useRdiConfig(() => 'dev-1', () => {});
      config.sensor_2_lower = 0;
      config.sensor_2_upper = 50;
      expect(sensor2Range.value).toEqual([0, 50]);
    });

    it('sensor1Range 可写入并更新 config', () => {
      const { sensor1Range, config } = useRdiConfig(() => 'dev-1', () => {});
      sensor1Range.value = [-30, 90];
      expect(config.sensor_1_lower).toBe(-30);
      expect(config.sensor_1_upper).toBe(90);
    });
  });

  describe('fieldEntries - 字段条目计算属性', () => {
    it('返回 field_setting 的 [key, value] 数组', () => {
      const { config, fieldEntries, setFieldValue } = useRdiConfig(() => 'dev-1', () => {});
      setFieldValue('n00', '1,2');
      setFieldValue('sw1', '开启');
      const entries = fieldEntries.value;
      const keys = entries.map((e: [string, unknown]) => e[0]);
      expect(keys).toContain('n00');
      expect(keys).toContain('sw1');
    });

    it('field_setting 为空时返回空数组', () => {
      const { fieldEntries } = useRdiConfig(() => 'dev-1', () => {});
      expect(fieldEntries.value).toEqual([]);
    });
  });

  describe('setSystemExtraField / getSystemExtraField - 系统额外字段', () => {
    it('设置并读取额外字段', () => {
      const { setSystemExtraField, getSystemExtraField } = useRdiConfig(() => 'dev-1', () => {});
      setSystemExtraField('remark', '重要客户');
      expect(getSystemExtraField('remark')).toBe('重要客户');
    });

    it('空字符串删除字段', () => {
      const { setSystemExtraField, getSystemExtraField } = useRdiConfig(() => 'dev-1', () => {});
      setSystemExtraField('remark', 'test');
      setSystemExtraField('remark', '   ');
      expect(getSystemExtraField('remark')).toBe('');
    });

    it('未设置的字段返回空字符串', () => {
      const { getSystemExtraField } = useRdiConfig(() => 'dev-1', () => {});
      expect(getSystemExtraField('not_exist')).toBe('');
    });
  });

  describe('loadConfig - 加载设备配置', () => {
    it('成功加载后合并 config 与 system_info', async () => {
      const mockRdiDeviceConfig = rdiDeviceConfig as any;
      mockRdiDeviceConfig.mockResolvedValue({
        error: null,
        data: {
          config: { data_collection_interval: 45, sensor_1_upper: 100 },
          system_info: {
            installation_location: '上海',
            customer_name: '客户A',
            extra_fields: {
              address: 'Pudong 1',
              installation_date: '2026-07-09',
              installer_company: 'Installer Co',
              installer_contact: 'Alex',
              installer_name: 'Alex Name',
              installer_phone: '+1 555 0000',
              installer_email: 'alex@example.com',
              controller_serial_number: 'RDI-SN-001',
              room: 'A101'
            }
          }
        }
      });
      const { config, systemInfo, loadConfig } = useRdiConfig(() => 'dev-1', () => {});
      await loadConfig();
      expect(config.data_collection_interval).toBe(45);
      expect(config.sensor_1_upper).toBe(100);
      expect(systemInfo.installation_location).toBe('上海');
      expect(systemInfo.customer_name).toBe('客户A');
      expect(systemInfo.address).toBe('Pudong 1');
      expect(systemInfo.installation_date).toBe('2026-07-09');
      expect(systemInfo.installer_company).toBe('Installer Co');
      expect(systemInfo.installer_contact).toBe('Alex');
      expect(systemInfo.installer_name).toBe('Alex Name');
      expect(systemInfo.installer_phone).toBe('+1 555 0000');
      expect(systemInfo.installer_email).toBe('alex@example.com');
      expect(systemInfo.controller_serial_number).toBe('RDI-SN-001');
      expect(systemInfo.extra_fields?.room).toBe('A101');
    });

    it('加载后确保 field_setting 与 extra_fields 存在', async () => {
      const mockRdiDeviceConfig = rdiDeviceConfig as any;
      mockRdiDeviceConfig.mockResolvedValue({
        error: null,
        data: { config: {}, system_info: {} }
      });
      const { config, systemInfo, loadConfig } = useRdiConfig(() => 'dev-1', () => {});
      await loadConfig();
      expect(config.field_setting).toEqual({});
      expect(systemInfo.extra_fields).toEqual({});
    });

    it('deviceId 为空时不发起请求', async () => {
      const mockRdiDeviceConfig = rdiDeviceConfig as any;
      const { loadConfig } = useRdiConfig(() => '', () => {});
      await loadConfig();
      expect(mockRdiDeviceConfig).toHaveBeenCalledTimes(0);
    });
  });

  describe('saveConfig - 保存设备配置', () => {
    it('成功保存后更新 config 并触发 onChange 回调', async () => {
      const mockUpdateRdiDeviceConfig = updateRdiDeviceConfig as any;
      const onChange = vi.fn();
      mockUpdateRdiDeviceConfig.mockResolvedValue({
        error: null,
        data: {
          config: { data_collection_interval: 60 },
          system_info: { customer_name: '客户B' }
        }
      });
      const { config, saveConfig } = useRdiConfig(() => 'dev-1', onChange);
      await saveConfig();
      expect(config.data_collection_interval).toBe(60);
      expect(onChange).toHaveBeenCalledTimes(1);
    });

    it('加载旧数据时把非法采集间隔归一为默认 60', async () => {
      const mockRdiDeviceConfig = rdiDeviceConfig as any;
      mockRdiDeviceConfig.mockResolvedValue({
        error: null,
        data: { config: { data_collection_interval: 120 }, system_info: {} }
      });
      const { config, loadConfig } = useRdiConfig(() => 'dev-1', () => {});
      await loadConfig();
      expect(config.data_collection_interval).toBe(60);
    });

    it('保存时携带 apply_to_device 标志', async () => {
      const mockUpdateRdiDeviceConfig = updateRdiDeviceConfig as any;
      mockUpdateRdiDeviceConfig.mockResolvedValue({ error: null, data: { config: {}, system_info: {} } });
      const { saveConfig, applyToDevice } = useRdiConfig(() => 'dev-1', () => {});
      applyToDevice.value = false;
      await saveConfig();
      const callArgs = mockUpdateRdiDeviceConfig.mock.calls[0];
      expect(callArgs[0]).toBe('dev-1');
      expect(callArgs[1].apply_to_device).toBe(false);
    });

    it('saves promoted system fields as first-class system_info and strips duplicate extra_fields', async () => {
      const mockUpdateRdiDeviceConfig = updateRdiDeviceConfig as any;
      mockUpdateRdiDeviceConfig.mockResolvedValue({ error: null, data: { config: {}, system_info: {} } });
      const { saveConfig, systemInfo } = useRdiConfig(() => 'dev-1', () => {});
      systemInfo.address = 'Pudong 1';
      systemInfo.installation_date = '2026-07-09';
      systemInfo.installer_company = 'Installer Co';
      systemInfo.installer_contact = 'Alex';
      systemInfo.installer_name = 'Alex Name';
      systemInfo.installer_phone = '+1 555 0000';
      systemInfo.installer_email = 'alex@example.com';
      systemInfo.controller_serial_number = 'RDI-SN-001';
      systemInfo.extra_fields = {
        address: 'old address',
        installation_date: 'old date',
        installer_name: 'old name',
        installer_phone: 'old phone',
        installer_email: 'old email',
        room: 'A101'
      };

      await saveConfig();

      const payload = mockUpdateRdiDeviceConfig.mock.calls[0][1];
      expect(payload.system_info).toMatchObject({
        address: 'Pudong 1',
        installation_date: '2026-07-09',
        installer_company: 'Installer Co',
        installer_contact: 'Alex',
        installer_name: 'Alex Name',
        installer_phone: '+1 555 0000',
        installer_email: 'alex@example.com',
        controller_serial_number: 'RDI-SN-001'
      });
      expect(payload.system_info.extra_fields).toEqual({ room: 'A101' });
    });

    it('stores command tracking summary returned by alarm config save', async () => {
      const mockUpdateRdiDeviceConfig = updateRdiDeviceConfig as any;
      mockUpdateRdiDeviceConfig.mockResolvedValue({
        error: null,
        data: {
          config: {},
          system_info: {},
          command_tracking: {
            message_id: 'msg-ack-001',
            status: 'pending',
            device_id: 'dev-1',
            identifier: 'set_alarm_config',
            operation_type: 'command',
            log_recorded: true
          }
        }
      });

      const { saveConfig, lastConfigCommandTracking, configCommandTrackingSummary } = useRdiConfig(
        () => 'dev-1',
        () => {}
      );

      await saveConfig();

      expect(lastConfigCommandTracking.value).toMatchObject({
        message_id: 'msg-ack-001',
        status: 'pending',
        identifier: 'set_alarm_config',
        log_recorded: true
      });
      expect(configCommandTrackingSummary.value).toContain('message_id=msg-ack-001');
      expect(message.success).toHaveBeenCalledWith(expect.stringContaining('msg-ack-001'));
    });
  });
});
