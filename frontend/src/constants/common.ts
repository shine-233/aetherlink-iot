/**
 * 文件用途：定义跨模块通用常量。
 * 核心逻辑：提供分页、状态、默认值和 UI 共享配置。
 * 关键注意事项：通用常量影响面广，变更前需扫描视图、服务和测试引用。
 * 重构建议：可把高频业务常量迁回领域模块，保留真正通用的最小集合。
 */
/**
 * 文件：公共常量。
 * 作用：维护是/否选项、开发环境判断、静态资源基地址和看板尺寸默认值。
 * 依赖：依赖环境配置生成服务地址，并复用通用 option 转换工具。
 * 维护：环境变量或看板布局默认值变化时同步验证相关页面表现。
 */

import { transformRecordToOption } from '@/utils/common/options'

export const yesOrNoRecord: Record<CommonType.YesOrNo, App.I18n.I18nKey> = {
  Y: 'common.yesOrNo.yes',
  N: 'common.yesOrNo.no'
}

export const yesOrNoOptions = transformRecordToOption(yesOrNoRecord)

export const IS_DEV = import.meta.env.MODE === 'development'

export const KANBANCOLNUM = 24

export const KANBANROWHEIGHT = 30
