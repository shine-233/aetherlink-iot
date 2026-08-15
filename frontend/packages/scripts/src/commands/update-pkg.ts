/**
 * 文件用途：封装依赖版本更新命令。
 * 核心逻辑：通过 ncu 按默认或传入参数检查并更新 package 依赖版本。
 * 关键注意事项：依赖升级可能引入兼容性变化，执行后需要按包范围做 targeted 验证。
 * 重构建议：可增加交互确认和升级计划输出，区分补丁、小版本和大版本升级风险。
 */
import { execCommand } from '../shared'

export async function updatePkg(args: string[] = ['--deep', '-u']) {
  execCommand('npx', ['ncu', ...args], { stdio: 'inherit' })
}
