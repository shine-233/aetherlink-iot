/**
 * 文件用途: 配置生成模块导出入口。
 * 核心逻辑: 统一导出简化配置生成器实例和类型，供编辑流程复用。
 * 关键注意事项: 导出路径变化会影响配置表单、向导和测试中的稳定导入。
 * 重构建议: 保持入口精简，新增生成器前先明确 public/internal 边界。
 */

// 配置生成器
export {
  SimpleConfigGenerator,
  simpleConfigGenerator
} from '@/core/data-architecture/config-generation/SimpleConfigGenerator'
