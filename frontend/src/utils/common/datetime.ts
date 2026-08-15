/*
 * 文件用途：提供通用日期时间格式化工具。
 * 核心逻辑：将可空时间字符串转换为前端展示格式，并对空输入保持空返回。
 * 关键注意事项：调用方需要明确空值展示策略，避免把 null 当作有效时间。
 * 重构建议：后续可补充时区和非法日期输入测试。
 */
import dayjs from 'dayjs'

/**
 * 将时间戳格式化为 YYYY-MM-DD HH:mm:ss 格式的字符串（24小时制）
 *
 * @param {string | null | undefined} ts - 时间戳
 * @returns {string | null} - 格式化后的时间字符串
 */
export function formatDateTime(ts: string | null | undefined): string | null {
  return ts ? dayjs(ts).format('YYYY-MM-DD HH:mm:ss') : null
}
