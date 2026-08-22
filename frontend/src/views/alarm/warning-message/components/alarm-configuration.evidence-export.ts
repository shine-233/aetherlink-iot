/**
 * 文件用途：告警配置页闭环证据包的导出动作（下载 JSON 证据包、复制证据文本）。
 * 核心逻辑：由页面注入证据包构建器与各类文案来源，本模块只负责浏览器下载流程与剪贴板反馈。
 * 关键注意事项：下载文件名与成功/失败提示文案必须与拆分前保持一致。
 */
import dayjs from 'dayjs'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'
import { buildAlarmClosureEvidenceFileName } from './alarm-configuration.helpers'

export type UseAlarmClosureEvidenceExportOptions = {
  /** 聚合当前页面上下文构建完整证据包 */
  buildBundle: () => Record<string, unknown>
  /** 证据包文件名的主键来源（详情行 id 或最近单条操作的告警 id） */
  resolvePrimaryAlarmId: () => unknown
  /** 详情弹窗内的闭环证据纯文本 */
  closurePacketText: () => string
  /** 最近一次批量操作的证据复制文本（无证据时返回 undefined） */
  batchCopyText: () => string | undefined
}

export function useAlarmClosureEvidenceExport(options: UseAlarmClosureEvidenceExportOptions) {
  const downloadAlarmClosureEvidenceBundle = () => {
    try {
      const bundle = options.buildBundle() as Record<string, unknown> & { generatedAt: string }
      const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: 'application/json;charset=utf-8' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = buildAlarmClosureEvidenceFileName({
        id: options.resolvePrimaryAlarmId() as string,
        generatedAt: bundle.generatedAt,
        formatTimestamp: value => dayjs(value).format('YYYYMMDD-HHmmss')
      })
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      window.$message?.success($t('custom.alarmPage.evidenceBundleDownloaded'))
    } catch {
      window.$message?.warning($t('custom.alarmPage.evidenceBundleDownloadFailed'))
    }
  }

  const copyAlarmClosureEvidence = async () => {
    const copied = await writeClipboardText(options.closurePacketText())
    if (copied) {
      window.$message?.success($t('custom.alarmPage.closureEvidenceCopied'))
    } else {
      window.$message?.warning($t('custom.alarmPage.closureEvidenceCopyFailed'))
    }
  }

  const copyLastBatchActionEvidence = async () => {
    const copyText = options.batchCopyText()
    if (copyText === undefined) return
    const copied = await writeClipboardText(copyText)
    if (copied) {
      window.$message?.success($t('custom.alarmPage.batchActionEvidenceCopied'))
    } else {
      window.$message?.warning($t('custom.alarmPage.batchActionEvidenceCopyFailed'))
    }
  }

  return {
    downloadAlarmClosureEvidenceBundle,
    copyAlarmClosureEvidence,
    copyLastBatchActionEvidence
  }
}
