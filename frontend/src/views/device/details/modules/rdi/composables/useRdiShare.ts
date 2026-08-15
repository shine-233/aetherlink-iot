/**
 * 文件用途: RDI 分享链接 composable。
 * 核心逻辑: 管理分享过期时间、调用分享 token API，并生成可复制的分享链接。
 * 关键注意事项: deviceId 为空、token 过期时间和 window origin 会影响外部分享入口。
 * 重构建议: 抽出分享链接构造 helper，并补 token 缺失、接口失败和不同过期时间测试。
 */
import { computed, ref } from 'vue'
import { createRdiShareToken } from '@/service/api'
import { message } from '@/utils/common/discrete'
import { writeClipboardText } from '@/utils/clipboard'
import type { LabelKey } from '../constants/rdi-labels'

export function useRdiShare(
  deviceId: () => string,
  t: (key: LabelKey) => string
) {
  const shareLoading = ref(false)
  const shareExpiresIn = ref(7 * 24 * 60 * 60)
  const shareLink = ref('')

  const shareExpiryOptions = computed(() => [
    { label: t('oneDay'), value: 24 * 60 * 60 },
    { label: t('sevenDays'), value: 7 * 24 * 60 * 60 },
    { label: t('thirtyDays'), value: 30 * 24 * 60 * 60 }
  ])

  const shareExpiresAt = computed(() => {
    if (!shareLink.value) return ''
    return new Date((Math.floor(Date.now() / 1000) + shareExpiresIn.value) * 1000).toLocaleString()
  })

  function resetShareState() {
    shareLink.value = ''
    shareExpiresIn.value = shareExpiryOptions.value[1]?.value ?? 7 * 24 * 60 * 60
    return shareExpiresAt.value
  }

  async function createShareLink() {
    shareLoading.value = true
    try {
      const { error, data } = await createRdiShareToken(deviceId(), { expires_in: shareExpiresIn.value })
      if (!error && data) {
        const path = data.share_path || (data.token ? `/device/share?share_token=${encodeURIComponent(data.token)}` : '')
        if (!path) return
        shareLink.value = `${window.location.origin}${path.startsWith('/') ? path : `/${path}`}`
        await copyShareLink()
      }
    } finally {
      shareLoading.value = false
    }
  }

  async function copyShareLink() {
    if (!shareLink.value) return false
    const copied = await writeClipboardText(shareLink.value)
    if (copied) {
      message.success(t('sent'))
    }
    return copied
  }

  const shareActions = {
    create: createShareLink,
    copy: copyShareLink
  }

  return {
    shareLoading,
    shareExpiresIn,
    shareLink,
    shareExpiryOptions,
    shareExpiresAt,
    shareActions,
    resetShareState,
    createShareLink,
    copyShareLink
  }
}
