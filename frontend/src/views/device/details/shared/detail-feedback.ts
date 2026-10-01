/**
 * 文件用途: 设备详情各模块共享的“复制 / 下载 JSON / 值压缩显示”工具。
 * 核心逻辑: 原先 onboarding-ready-check、DeviceAccessGuide、TwinLiteCard 各自内联了
 * “writeClipboardText + 成功/失败 toast”与“Blob -> a[download] -> revokeObjectURL”的相同实现，这里收敛为一份。
 * 关键注意事项: toast 通过 window.$message 发出（全局 discrete message 注入），SSR/测试环境缺失时静默跳过。
 */
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'

type MessageLevel = 'success' | 'error' | 'warning'

function notify(level: MessageLevel, text: string) {
  if (typeof window === 'undefined') return
  ;(window as any).$message?.[level]?.(text)
}

/** 复制文本并给出统一的成功/失败 toast；返回是否复制成功。 */
export async function copyTextWithFeedback(text: string): Promise<boolean> {
  const copied = await writeClipboardText(text)
  notify(copied ? 'success' : 'error', copied ? $t('theme.configOperation.copySuccess') : $t('common.copyFailed'))
  return copied
}

export type DownloadJsonOptions = {
  fileName: string
  successKey: string
  failureKey: string
  /** 失败提示级别：证据包类下载历史上使用 warning，支持包使用 error。 */
  failureLevel?: 'error' | 'warning'
  mimeType?: string
}

/**
 * 以 JSON 文件形式下载 payload。payload 支持惰性构造：构造期抛错同样走失败提示。
 * 返回是否触发了下载。
 */
export function downloadJsonWithFeedback(payload: unknown | (() => unknown), options: DownloadJsonOptions): boolean {
  if (typeof window === 'undefined' || typeof document === 'undefined') return false
  try {
    const value = typeof payload === 'function' ? (payload as () => unknown)() : payload
    const blob = new Blob([JSON.stringify(value, null, 2)], { type: options.mimeType || 'application/json' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = options.fileName
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
    notify('success', $t(options.successKey))
    return true
  } catch {
    notify(options.failureLevel || 'error', $t(options.failureKey))
    return false
  }
}

/** 把任意值压缩成单行展示文本：空值为 '--'，对象 JSON 化，超过 maxLength 截断加省略号。 */
export function compactValueText(value: unknown, maxLength = 120): string {
  if (value === undefined || value === null || value === '') return '--'
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  if (!text) return '--'
  return text.length > maxLength ? `${text.slice(0, maxLength)}...` : text
}
