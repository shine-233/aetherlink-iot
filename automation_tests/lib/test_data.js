/**
 * 文件用途：用于支撑 automation_tests 的API 自动化测试数据工厂模块。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：共享库变更会影响多类自动化套件，必须保持错误信息和前置条件可诊断。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const fs = require('fs');
const path = require('path');

const config = require('./runtime_config');

const testData = {
  /**
   * 获取配置中的测试设备 PID
   * @param {string} key - config.testDevice 中的 key
   * @returns {string} PID 字符串
   */
  getDevicePID(key = 'activated_pid') {
    if (process.env.AETHERLINK_RDI_FIXTURE_MODE === 'synthetic-rdi' && key === 'activated_pid') {
      const pid = String(
        process.env.AETHERLINK_RDI_FIXTURE_PID ||
        process.env.SYNTHETIC_RDI_PID ||
        ''
      ).trim().toUpperCase();
      if (!pid) {
        throw new Error('AETHERLINK_RDI_FIXTURE_MODE=synthetic-rdi requires AETHERLINK_RDI_FIXTURE_PID or SYNTHETIC_RDI_PID');
      }
      if (!/^[A-Z0-9]{12}$/.test(pid)) {
        throw new Error(`Synthetic RDI fixture PID must be exactly 12 alphanumeric characters: ${pid}`);
      }
      return pid;
    }
    return config.testDevice[key];
  },

  /**
   * 生成唯一的测试设备名称（基于时间戳，保证同一批运行内不重复）
   * @param {string} prefix - 名称前缀
   * @returns {string} 形如 prefix-<timestamp> 的设备名
   */
  generateDeviceName(prefix = 'AutoTest') {
    const ts = Date.now();
    return `${prefix}-${ts}`;
  },

  /**
   * 生成测试邮箱（时间戳 + 随机数，避免并发或重复运行冲突）
   * @param {string} prefix - 邮箱前缀
   * @returns {string} 形如 prefix_<ts>_<rand>@test.com 的邮箱
   */
  generateTestEmail(prefix = 'autotest') {
    const ts = Date.now();
    const rand = Math.floor(Math.random() * 10000);
    return `${prefix}_${ts}_${rand}@test.com`;
  },

  /**
   * 标准传感器告警配置（每次返回全新对象，调用方可安全修改）
   * @returns {object} RDI 设备告警配置对象
   */
  getSensorAlarmConfig() {
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
      switch_1_alarm_mode: 'powered_on',
      switch_2_alarm_mode: 'powered_off',
      switch_1_alarm_duration: 30,
      switch_2_alarm_duration: 30,
      dry_contact_alarm_level: 'high',
      dry_contact_normal_level: 'low',
      dry_contact_alarm_delay: 10,
      dry_contact_normal_delay: 5,
      notification_enabled: true,
      notification_temperature_alarm: true,
      notification_switch_alarm: true,
      notification_warranty_alarm: false,
      sensor_alarm_emails: 'sensor@test.com',
      switch_alarm_emails: 'switch@test.com',
      warranty_alarm_emails: '',
      sensor_1_alarm_emails: 'sensor1@test.com',
      sensor_2_alarm_emails: 'sensor2@test.com',
      switch_1_alarm_emails: 'switch1@test.com',
      switch_2_alarm_emails: 'switch2@test.com'
    };
  },

  /**
   * 边界值测试：温度超上限（125°C 为合法上限，126°C 越界）
   * @returns {object} sensor_1_upper 被置为 126 的配置对象
   */
  getInvalidTempUpperConfig() {
    const cfg = this.getSensorAlarmConfig();
    cfg.sensor_1_upper = 126;
    return cfg;
  },

  /**
   * 边界值测试：温度超下限（-40°C 为合法下限，-41°C 越界）
   * @returns {object} sensor_1_lower 被置为 -41 的配置对象
   */
  getInvalidTempLowerConfig() {
    const cfg = this.getSensorAlarmConfig();
    cfg.sensor_1_lower = -41;
    return cfg;
  },

  /**
   * 边界值测试：采集频率过低（10 秒为合法下限，9 秒越界）
   * @returns {object} data_collection_interval 被置为 9 的配置对象
   */
  getInvalidIntervalLowConfig() {
    const cfg = this.getSensorAlarmConfig();
    cfg.data_collection_interval = 9;
    return cfg;
  },

  /**
   * 边界值测试：采集频率过高（3600 秒为合法上限，3601 秒越界）
   * @returns {object} data_collection_interval 被置为 3601 的配置对象
   */
  getInvalidIntervalHighConfig() {
    const cfg = this.getSensorAlarmConfig();
    cfg.data_collection_interval = 3601;
    return cfg;
  },

  /**
   * 标准系统信息（设备安装/维护/客户信息）
   * @returns {object} 系统信息对象
   */
  getSystemInfo() {
    return {
      installation_location: '测试机房A',
      address: '测试园区1号楼',
      installation_date: '2026-07-09',
      installer_company: '测试安装公司',
      installer_contact: '测试安装联系人',
      installer_name: '测试安装人员',
      installer_phone: '+86 13900000000',
      installer_email: 'installer@test.com',
      controller_serial_number: 'RDI-SN-AUTO-001',
      maintenance_technician: '测试技师',
      customer_name: '测试客户',
      contact_email: 'contact@test.com',
      contact_phone: '+86 13800000000',
      warranty_status: 'active',
      extra_fields: {
        site_name: '测试站点',
        building: 'A栋',
        floor: '3F',
        notes: '自动化测试数据'
      }
    };
  },

  /**
   * 干接点测试命令参数（test_dry_contact 服务调用）
   * @returns {object} 含 level 与 duration_seconds 的参数对象
   */
  getTestDryContactParams() {
    return {
      level: 'high',
      duration_seconds: 5
    };
  },

  /**
   * Field Setting 命令参数（set_field_setting 服务调用）
   * @returns {object} 含 n00~n07 数值与 sw1~sw4 开关位的参数对象
   */
  getFieldSettingParams() {
    return {
      n00: ['temperature_1'],
      n01: ['temperature_2'],
      n02: ['switch_1'],
      n03: ['switch_2'],
      n04: ['dry_contact_output'],
      n05: ['electricity_consumption'],
      n06: ['wifi_rssi'],
      n07: ['connection_type'],
      sw1: { label: 'Switch 1', enabled: true },
      sw2: { label: 'Switch 2', enabled: true },
      sw3: { label: 'SW3', enabled: true },
      sw4: { label: 'Dry Contact', enabled: true }
    };
  },

  /**
   * OTA 升级命令参数
   * 注意：firmware_url 中的端口与 config.baseURL 一致，若后端端口变更需同步修改
   * @returns {object} 含 firmware_url/version/size/md5 的参数对象
   */
  getOtaUpgradeParams() {
    return {
      firmware_url: 'http://localhost:9999/files/upgradePackage/fw1.0.3.bin',
      version: '1.0.3',
      size: 102400,
      md5: 'test1234567890'
    };
  },

  /**
   * 告警配置创建请求（名称含时间戳保证唯一，避免重复创建冲突）
   * @returns {object} 含 name/description/alarm_level/enabled/remark 的请求体
   */
  getCreateAlarmConfigReq() {
    return {
      name: `自动化测试告警_${Date.now()}`,
      description: '由自动化测试脚本创建',
      alarm_level: 'H',
      enabled: 'Y',
      remark: 'auto-test'
    };
  },

  /**
   * 获取历史时间范围（过去24小时，秒级时间戳）
   * @returns {{startTime:number, endTime:number}} Unix 秒级时间戳范围
   */
  getHistoryTimeRange() {
    const endTime = Math.floor(Date.now() / 1000);
    const startTime = endTime - 24 * 60 * 60;
    return { startTime, endTime };
  },

  /**
   * 获取历史时间范围（过去1小时，秒级时间戳，用于导出测试减小数据量）
   * @returns {{startTime:number, endTime:number}} Unix 秒级时间戳范围
   */
  getRecentTimeRange() {
    const endTime = Math.floor(Date.now() / 1000);
    const startTime = endTime - 60 * 60;
    return { startTime, endTime };
  },

  /**
   * 分享 Token 请求
   * @param {number} expiresIn - 过期秒数，默认 7 天
   * @returns {object} 含 expires_in 的请求体
   */
  getShareTokenReq(expiresIn = 7 * 24 * 60 * 60) {
    return { expires_in: expiresIn };
  },

  /**
   * 获取配置对象
   * @returns {object} config.json 解析后的配置
   */
  getConfig() {
    return config;
  }
};

module.exports = testData;
