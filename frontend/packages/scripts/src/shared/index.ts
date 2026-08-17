/**
 * 文件用途：提供 scripts CLI 的共享命令执行工具。
 * 核心逻辑：动态导入 execa 执行子进程，并返回裁剪后的 stdout。
 * 关键注意事项：错误会沿 execa 抛出，调用方需要按命令风险决定是否捕获和提示。
 * 重构建议：可扩展 dry-run、统一日志和错误包装，提升 CLI 可诊断性。
 */
import type { Options } from 'execa'

function normalizeStdout(stdout: unknown): string {
  if (typeof stdout === 'string') {
    return stdout.trim()
  }

  if (stdout instanceof Uint8Array) {
    return Buffer.from(stdout).toString('utf8').trim()
  }

  if (Array.isArray(stdout)) {
    return stdout.filter((part): part is string => typeof part === 'string').join('\n').trim()
  }

  return ''
}

export async function execCommand(cmd: string, args: string[], options?: Options) {
  const { execa } = await import('execa')
  const res = await execa(cmd, args, options)
  return normalizeStdout(res?.stdout)
}
