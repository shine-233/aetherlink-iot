/**
 * 文件用途：封装项目清理命令。
 * 核心逻辑：调用 rimraf 按 glob 删除传入路径。
 * 关键注意事项：这是有删除副作用的命令，调用方必须确认 paths 已限制在预期范围内。
 * 重构建议：后续可增加 dry-run、路径白名单或交互确认，降低误删风险。
 */
import { rimraf } from 'rimraf'

export async function cleanup(paths: string[]) {
  await rimraf(paths, { glob: true })
}
