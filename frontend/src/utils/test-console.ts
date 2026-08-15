/*
 * 文件用途：集中处理测试环境下未 mock console 的噪声抑制判断。
 * 核心逻辑：Vitest 环境里只有被 mock 的 console 方法才允许继续输出。
 * 关键注意事项：这是测试噪声边界工具，业务代码不要在这里加入运行时日志策略。
 */

export function shouldSuppressUnmockedTestConsole(method: (...args: any[]) => void): boolean {
  return (globalThis as any).process?.env?.VITEST === 'true' && !(method as any).mock
}
