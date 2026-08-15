/**
 * 文件用途：用于支撑Playwright 页面选择器集中管理模块。
 * 核心逻辑：集中导出登录、导航和业务页面选择器，降低各 spec 对 DOM 结构的直接依赖。
 * 关键注意事项：helper 不应隐藏关键业务断言；缺少数据或账号时要显式暴露前置条件。
 * 重构建议：新增复用能力时优先保持小接口，并把业务断言留在调用方或专门的断言 helper 中。
 */

module.exports = {
  // ===== 登录页 =====
  login: {
    email:
      '[data-testid="login-email"], input[type="email"], input[type="text"], input[placeholder*="邮箱" i], input[placeholder*="email" i], input[placeholder*="phone" i]',
    password:
      '[data-testid="login-password"], input[type="password"], input[placeholder*="密码" i], input[placeholder*="password" i]',
    submit: '[data-testid="login-submit"], button[type="submit"], button:has-text("登录"), button:has-text("Login")',
    register: '[data-testid="login-register"], a:has-text("注册"), a:has-text("Register")',
    forgotPassword:
      '[data-testid="login-forgot-password"], button:has-text("忘记密码"), a:has-text("忘记密码"), button:has-text("Forgot Password"), a:has-text("Forgot Password"), button:has-text("Mot de passe oublie"), a:has-text("Mot de passe oublie"), button:has-text("Olvido la contrasena"), a:has-text("Olvido la contrasena")',
    errorMessage: '[data-testid="login-error"], .n-message--error-type, .error-message',
    // 登录后主页标志
    homeIndicator: '[data-testid="home-indicator"], [data-testid="dashboard"], nav, .n-layout-sider'
  },

  // ===== 设备管理页 =====
  device: {
    list: '[data-testid="device-list"], .device-list, [data-testid="device-table"]',
    addDevice: '[data-testid="add-device"], button:has-text("添加设备"), button:has-text("新增设备")',
    pidInput: '[data-testid="pid-input"], input[placeholder*="PID" i]',
    activate: '[data-testid="activate-device"], button:has-text("激活"), button:has-text("绑定")',
    deviceDetail: '[data-testid="device-detail"], .device-detail',
    // 设备详情页 Tab
    tabBase: '[data-testid="tab-base"], [role="tab"]:has-text("基本信息")',
    tabTelemetry: '[data-testid="tab-telemetry"], [role="tab"]:has-text("遥测数据"), [role="tab"]:has-text("实时数据")',
    tabConfig: '[data-testid="tab-config"], [role="tab"]:has-text("配置")',
    tabAlarm: '[data-testid="tab-alarm"], [role="tab"]:has-text("告警")',
    tabShare: '[data-testid="tab-share"], [role="tab"]:has-text("分享")',
    // 搜索
    searchInput: '[data-testid="device-search"], input[placeholder*="搜索" i], input[placeholder*="设备" i]',
    searchSubmit: '[data-testid="device-search-submit"], button:has-text("搜索")',
    // 分享
    shareButton: '[data-testid="device-share"], button:has-text("分享")',
    shareLink: '[data-testid="share-link"], .share-link, input[readonly]',
    shareConfirm: '[data-testid="share-confirm"], button:has-text("确定"), button:has-text("复制")',
    // 分享链接生成（owner 在设备详情生成分享 Token）
    shareGenerate: '[data-testid="share-generate"], button:has-text("生成"), button:has-text("创建分享"), button:has-text("获取链接")',
    shareCopy: '[data-testid="share-copy"], button:has-text("复制")',
    shareToken: '[data-testid="share-token"], .share-token, [data-testid="share-link"], input[readonly]'
  },

  // ===== RDI 面板 =====
  rdi: {
    view: '[data-testid="rdi-view"], .rdi-view',
    temperature: '[data-testid="rdi-temperature"], .temperature-value, [data-testid="current-temperature"]',
    alarmConfig: '[data-testid="rdi-alarm-config"], .alarm-config',
    dryContact: '[data-testid="rdi-dry-contact"], .dry-contact',
    fieldSetting: '[data-testid="rdi-field-setting"], .field-setting',
    // 历史数据
    historyChart: '[data-testid="history-chart"], canvas, .chart-container',
    timeRange: '[data-testid="time-range"], .time-range-selector',
    timeRange1h: '[data-testid="time-range-1h"], button:has-text("1小时"), button:has-text("1H")',
    timeRange24h: '[data-testid="time-range-24h"], button:has-text("24小时"), button:has-text("24H")',
    timeRange7d: '[data-testid="time-range-7d"], button:has-text("7天"), button:has-text("7D")',
    // 单位切换
    unitC: '[data-testid="unit-c"], button:has-text("°C"), button:has-text("摄氏")',
    unitF: '[data-testid="unit-f"], button:has-text("°F"), button:has-text("华氏")',
    // 导出
    exportButton: '[data-testid="export-data"], button:has-text("导出"), button:has-text("下载")'
  },

  // ===== 告警页 =====
  alarm: {
    list: '[data-testid="alarm-list"], .alarm-list, [data-testid="alarm-table"]',
    configForm: '[data-testid="alarm-config-form"], .alarm-config-form',
    acknowledge: '[data-testid="alarm-acknowledge"], button:has-text("确认"), button:has-text("ACK")',
    reset: '[data-testid="alarm-reset"], button:has-text("重置"), button:has-text("Reset")',
    filter: '[data-testid="alarm-filter"], .alarm-filter',
    levelFilter: '[data-testid="alarm-level-filter"], select:has-text("级别"), [data-testid="level-select"]',
    // 全局告警总览 6 指标卡
    overviewCards: '[data-testid="alarm-overview"], .alarm-overview',
    cardTotal: '[data-testid="alarm-card-total"], .card:has-text("总告警")',
    cardActive: '[data-testid="alarm-card-active"], .card:has-text("活跃")',
    cardAcknowledged: '[data-testid="alarm-card-acknowledged"], .card:has-text("已确认")',
    cardCritical: '[data-testid="alarm-card-critical"], .card:has-text("严重")',
    cardWarning: '[data-testid="alarm-card-warning"], .card:has-text("警告")',
    cardInfo: '[data-testid="alarm-card-info"], .card:has-text("信息")',
    // 告警配置创建（新增弹窗 / 表单字段 / 提交 / 取消）
    add: '[data-testid="alarm-add"], button:has-text("新增"), button:has-text("添加告警"), button:has-text("新建")',
    configName: '[data-testid="alarm-config-name"], input[placeholder*="名称" i], input[placeholder*="告警" i]',
    configLevel: '[data-testid="alarm-config-level"], .n-select, select',
    configCondition: '[data-testid="alarm-config-condition"], input[placeholder*="条件" i], input[placeholder*="阈值" i]',
    configSubmit: '[data-testid="alarm-config-submit"], button:has-text("确定"):not(:has-text("取消")), button:has-text("提交")',
    configCancel: '[data-testid="alarm-config-cancel"], button:has-text("取消")'
  },

  // ===== 系统设置页 =====
  system: {
    languageSwitch: '[data-testid="language-switch"], .language-switcher, [data-testid="lang-select"]',
    languageZh: '[data-testid="lang-zh"], option:has-text("中文"), [data-lang="zh"]',
    languageEn: '[data-testid="lang-en"], option:has-text("English"), [data-lang="en"]',
    languageFr: '[data-testid="lang-fr"], option:has-text("Français"), option:has-text("法语"), [data-lang="fr"]',
    languageEs: '[data-testid="lang-es"], option:has-text("Español"), option:has-text("西班牙语"), [data-lang="es"]',
    logoUpload: '[data-testid="logo-upload"], input[type="file"][accept*="image"], .logo-upload',
    titleInput: '[data-testid="system-title"], input[placeholder*="标题" i], input[placeholder*="系统名称" i]',
    titleSave: '[data-testid="title-save"], button:has-text("保存")',
    // 主仪表盘 6 指标卡
    dashboardCards: '[data-testid="dashboard-cards"], .dashboard-cards',
    dashboardCard: '[data-testid="dashboard-card"], .stat-card, .metric-card'
  },

  // ===== 可视化 ThingsVis =====
  visualization: {
    projectList: '[data-testid="thingsvis-project-list"], [data-testid="thingsvis-list"], .thingsvis-project-list, .n-data-table',
    create: '[data-testid="thingsvis-create"], button:has-text("新建"), button:has-text("创建"), button:has-text("Create")',
    dashboardList: '[data-testid="thingsvis-dashboard-list"], .dashboard-list, .n-data-table',
    editorCanvas: '[data-testid="thingsvis-editor"], [data-testid="thingsvis-canvas"], .thingsvis-editor, canvas',
    previewFrame: '[data-testid="thingsvis-preview"], iframe, .thingsvis-preview'
  },

  // ===== 应用/服务市场 =====
  apply: {
    pluginList: '[data-testid="plugin-list"], [data-testid="apply-plugin-list"], .n-grid, .n-data-table',
    serviceList: '[data-testid="service-list"], [data-testid="apply-service-list"], .n-grid, .n-data-table',
    search: '[data-testid="apply-search"], input[placeholder*="搜索" i], input[placeholder*="Search" i]'
  },

  // ===== Management route alias selectors =====
  manage: {
    menuTree: '[data-testid="menu-tree"], .n-tree, .menu-tree',
    userTable: '[data-testid="manage-user-table"], .n-data-table',
    roleTable: '[data-testid="manage-role-table"], .n-data-table',
    detailForm: '[data-testid="user-detail-form"], form, .n-form'
  },

  // ===== 导航 catalog/异常页 =====
  routeCatalog: {
    tab: '[data-testid="tab-demo"], [role="tab"], .n-tabs',
    menu: '[data-testid="multi-menu"], .n-menu, nav',
    exceptionAction: 'a:has-text("Back to Home"), button:has-text("Back to Home"), a:has-text("首页"), button:has-text("首页")'
  },

  // ===== 设备配置（写流程） =====
  config: {
    save: '[data-testid="config-save"], button:has-text("保存")',
    cancel: '[data-testid="config-cancel"], button:has-text("取消")',
    // 干接点配置区域内的可填写输入框（电平 / 延时等）
    dryContactInput: '[data-testid="dry-contact-input"], .dry-contact input[type="number"], .dry-contact input[type="text"], [data-testid="rdi-dry-contact"] input'
  },

  // ===== 通用 =====
  common: {
    // 确认对话框
    confirmDialog: '[data-testid="confirm-dialog"], .n-modal, .confirm-dialog',
    confirmOk: '[data-testid="confirm-ok"], button:has-text("确定"), button:has-text("确认")',
    confirmCancel: '[data-testid="confirm-cancel"], button:has-text("取消")',
    // 消息提示
    successMessage: '[data-testid="success-message"], .n-message--success-type, .success-message',
    errorMessage: '[data-testid="error-message"], .n-message--error-type, .error-message',
    // 加载遮罩
    loading: '.n-spin-container, .loading, [data-testid="loading"]',
    // 退出登录
    logout: '[data-testid="logout"], button:has-text("退出"), button:has-text("登出"), a:has-text("退出登录")',
    userMenu: '[data-testid="user-menu"], .user-avatar, .user-dropdown'
  }
};
