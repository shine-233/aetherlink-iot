/*
 * 文件用途：提供数字相关通用工具。
 * 核心逻辑：当前封装指定范围内随机整数生成逻辑。
 * 关键注意事项：调用方需要确认边界是否包含结束值，避免随机范围误解。
 * 重构建议：后续可补充单元测试并明确命名中的闭区间/开区间语义。
 */
/** Return an integer in the half-open range [start, end). */
export function getRandomInteger(end: number, start = 0) {
  const range = end - start
  const random = Math.floor(Math.random() * range + start)
  return random
}
