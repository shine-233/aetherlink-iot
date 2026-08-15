/**
 * 文件用途: 配置适配器目录的导出入口。
 * 核心逻辑: 集中导出适配器、工厂函数和转换结果类型，稳定外部导入路径。
 * 关键注意事项: 索引导出属于公共契约，删除或改名会放大到所有配置迁移调用方。
 * 重构建议: 区分 public 导出和测试/内部导出，降低适配器实验能力外泄风险。
 */

// ==================== 主要适配器类导出 ====================
export {
  ConfigurationAdapter,
  createConfigurationAdapter,
  type ConversionResult
} from '@/core/data-architecture/adapters/ConfigurationAdapter'

// ==================== 便捷函数导出 ====================
export { detectConfigVersion, upgradeToV2, downgradeToV1 } from '@/core/data-architecture/adapters/ConfigurationAdapter'

// ==================== 适配器版本信息 ====================
export const ADAPTER_VERSION = '1.0.0'

export const ADAPTER_FEATURES = {
  VERSION_DETECTION: true,
  // 已支持字段可无损升级；无法表达的 HTTP 参数配置会明确阻断。
  LOSSLESS_UPGRADE: false,
  COMPATIBLE_DOWNGRADE: true,
  BATCH_CONVERSION: true,
  VALIDATION: true
} as const
