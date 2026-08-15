/**
 * 文件用途：配置 Day.js 日期时间插件和本地化能力。
 * 核心逻辑：注册项目需要的 dayjs 插件、locale 或默认格式扩展，供全局日期展示复用。
 * 关键注意事项：日期解析和时区行为会影响报表、遥测历史和导出结果，修改时需验证边界时间。
 * 重构建议：可将业务格式化函数迁移到独立时间工具，插件入口只保留初始化。
 */
import { extend } from 'dayjs'
import localeData from 'dayjs/plugin/localeData'
import { setDayjsLocale } from '../locales/dayjs'

export function setupDayjs() {
  extend(localeData)

  setDayjsLocale()
}
